// Copyright 2021 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resolvers

import (
	"context"
	"sort"
	"sync"

	"github.com/99designs/gqlgen/graphql"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/graph/model"
	xunstructured "github.com/upbound/xgql/internal/unstructured"
)

const (
	errGetResource   = "cannot get Kubernetes resource"
	errModelResource = "cannot model Kubernetes resource"
	errGetClient     = "cannot get client"
	errGetSecret     = "cannot get secret"
	errGetConfigMap  = "cannot get config map"
	errListProviders = "cannot list providers"
	errListConfigs   = "cannot list configurations"
)

type query struct {
	clients ClientCache
}

// Recursively collect `CrossplaneResourceTreeNode`s from the given KubernetesResource
func (r *query) getAllDescendant(ctx context.Context, res model.KubernetesResource, parentID *model.ReferenceID) []model.CrossplaneResourceTreeNode { //nolint:gocyclo
	// This isn't _really_ that complex; it's a long but simple switch.

	switch typedRes := res.(type) {
	case model.CompositeResource:
		list := []model.CrossplaneResourceTreeNode{{ParentID: parentID, Resource: typedRes}}

		compositeResolver := compositeResourceSpec{clients: r.clients}
		resources, err := compositeResolver.Resources(ctx, &typedRes.Spec)
		if err != nil || len(graphql.GetErrors(ctx)) > 0 {
			return nil
		}

		// Collect all concurrently.
		var (
			wg sync.WaitGroup
		)
		childLists := make([][]model.CrossplaneResourceTreeNode, len(resources.Nodes))
		for i, childRes := range resources.Nodes {
			i, childRes := i, childRes // So we don't capture the loop variable.
			wg.Add(1)
			go func() {
				defer wg.Done()
				childLists[i] = r.getAllDescendant(ctx, childRes, &typedRes.ID)
			}()
		}
		wg.Wait()

		if len(graphql.GetErrors(ctx)) > 0 {
			return nil
		}

		for _, childList := range childLists {
			list = append(list, childList...)
		}
		return list
	case model.CompositeResourceClaim:
		list := []model.CrossplaneResourceTreeNode{{ParentID: parentID, Resource: typedRes}}

		claimResolver := compositeResourceClaimSpec{clients: r.clients}
		composite, err := claimResolver.Resource(ctx, &typedRes.Spec)
		if err != nil || len(graphql.GetErrors(ctx)) > 0 {
			return nil
		}

		if composite == nil {
			return list
		}

		childList := r.getAllDescendant(ctx, *composite, &typedRes.ID)
		if err != nil || len(graphql.GetErrors(ctx)) > 0 {
			return nil
		}

		return append(list, childList...)
	default:
		return []model.CrossplaneResourceTreeNode{{ParentID: parentID, Resource: typedRes}}
	}
}

func (r *query) CrossplaneResourceTree(ctx context.Context, id model.ReferenceID) (model.CrossplaneResourceTreeConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rootRes, err := r.KubernetesResource(ctx, id)
	if err != nil || len(graphql.GetErrors(ctx)) > 0 {
		return model.CrossplaneResourceTreeConnection{}, err
	}

	list := r.getAllDescendant(ctx, rootRes, nil)
	if len(graphql.GetErrors(ctx)) > 0 {
		return model.CrossplaneResourceTreeConnection{}, nil
	}

	return model.CrossplaneResourceTreeConnection{Nodes: list, TotalCount: len(list)}, nil
}

func (r *query) KubernetesResource(ctx context.Context, id model.ReferenceID) (model.KubernetesResource, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	u := &kunstructured.Unstructured{}
	u.SetAPIVersion(id.APIVersion)
	u.SetKind(id.Kind)
	nn := types.NamespacedName{Namespace: id.Namespace, Name: id.Name}
	if err := c.Get(ctx, nn, u); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetResource))
		return nil, nil
	}

	out, err := model.GetKubernetesResource(u)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errModelResource))
		return nil, nil
	}
	return out, nil
}

func (r *query) KubernetesResources(ctx context.Context, apiVersion, kind string, listKind, namespace *string) (model.KubernetesResourceConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	lopts := []client.ListOption{}
	if namespace != nil {
		lopts = []client.ListOption{client.InNamespace(*namespace)}
	}

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.KubernetesResourceConnection{}, nil
	}

	in := &kunstructured.UnstructuredList{}
	in.SetAPIVersion(apiVersion)
	in.SetKind(kind + "List")
	if listKind != nil && *listKind != "" {
		in.SetKind(*listKind)
	}

	if err := c.List(ctx, in, lopts...); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListResources))
		return model.KubernetesResourceConnection{}, nil
	}

	out := &model.KubernetesResourceConnection{
		Nodes: make([]model.KubernetesResource, 0, len(in.Items)),
	}

	for i := range in.Items {
		kr, err := model.GetKubernetesResource(&in.Items[i])
		if err != nil {
			graphql.AddError(ctx, errors.Wrap(err, errModelResource))
			continue
		}
		out.Nodes = append(out.Nodes, kr)
		out.TotalCount++
	}

	sort.Stable(out)
	return *out, nil
}

func (r *query) Events(ctx context.Context, involved *model.ReferenceID) (model.EventConnection, error) {
	e := events{clients: r.clients}
	if involved == nil {
		// Resolve all events.
		return e.Resolve(ctx, nil)
	}

	// Resolve events pertaining to the supplied ID.
	return e.Resolve(ctx, &corev1.ObjectReference{
		APIVersion: involved.APIVersion,
		Kind:       involved.Kind,
		Namespace:  involved.Namespace,
		Name:       involved.Name,
	})
}

func (r *query) Secret(ctx context.Context, namespace, name string) (*model.Secret, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	s := &corev1.Secret{}
	nn := types.NamespacedName{Namespace: namespace, Name: name}
	if err := c.Get(ctx, nn, s); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetSecret))
		return nil, nil
	}

	out := model.GetSecret(s)
	return &out, nil
}

func (r *query) ConfigMap(ctx context.Context, namespace, name string) (*model.ConfigMap, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	cm := &corev1.ConfigMap{}
	nn := types.NamespacedName{Namespace: namespace, Name: name}
	if err := c.Get(ctx, nn, cm); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetConfigMap))
		return nil, nil
	}

	out := model.GetConfigMap(cm)
	return &out, nil
}

func (r *query) Providers(ctx context.Context) (model.ProviderConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.ProviderConnection{}, nil
	}

	in := &pkgv1.ProviderList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListProviders))
		return model.ProviderConnection{}, nil
	}

	out := &model.ProviderConnection{
		Nodes:      make([]model.Provider, 0, len(in.Items)),
		TotalCount: len(in.Items),
	}

	for i := range in.Items {
		out.Nodes = append(out.Nodes, model.GetProvider(&in.Items[i]))
	}

	sort.Stable(out)
	return *out, nil
}

func (r *query) ProviderRevisions(ctx context.Context, provider *model.ReferenceID, active *bool) (model.ProviderRevisionConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.ProviderRevisionConnection{}, nil
	}

	in := &pkgv1.ProviderRevisionList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListProviderRevs))
		return model.ProviderRevisionConnection{}, nil
	}

	out := &model.ProviderRevisionConnection{
		Nodes: make([]model.ProviderRevision, 0),
	}

	for i := range in.Items {
		pr := in.Items[i] // So we don't take the address of a range variable.

		// The supplied provider is not an owner of this PackageRevision.
		if provider != nil && !containsID(pr.OwnerReferences, *provider) {
			continue
		}

		// We only want the active PackageRevision, and this isn't it.
		if ptr.Deref(active, false) && pr.Spec.DesiredState != pkgv1.PackageRevisionActive {
			continue
		}

		out.Nodes = append(out.Nodes, model.GetProviderRevision(&pr))
		out.TotalCount++
	}

	sort.Stable(out)
	return *out, nil
}

func (r *query) CustomResourceDefinitions(ctx context.Context, revision *model.ReferenceID) (model.CustomResourceDefinitionConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.CustomResourceDefinitionConnection{}, nil
	}

	in := xunstructured.NewCRDList()
	if err := c.List(ctx, in.GetUnstructuredList()); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListConfigs))
		return model.CustomResourceDefinitionConnection{}, nil
	}

	out := &model.CustomResourceDefinitionConnection{
		Nodes: make([]model.CustomResourceDefinition, 0),
	}

	for i := range in.Items {
		xrd := &xunstructured.CustomResourceDefinition{Unstructured: in.Items[i]}

		// We only want CRDs owned by this config revision, but this one isn't.
		if revision != nil && !containsID(xrd.GetOwnerReferences(), *revision) {
			continue
		}

		out.Nodes = append(out.Nodes, model.GetCustomResourceDefinition(xrd))
		out.TotalCount++
	}

	sort.Stable(out)
	return *out, nil
}

func (r *query) Configurations(ctx context.Context) (model.ConfigurationConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.ConfigurationConnection{}, nil
	}

	in := &pkgv1.ConfigurationList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListConfigs))
		return model.ConfigurationConnection{}, nil
	}

	out := &model.ConfigurationConnection{
		Nodes:      make([]model.Configuration, 0, len(in.Items)),
		TotalCount: len(in.Items),
	}

	for i := range in.Items {
		out.Nodes = append(out.Nodes, model.GetConfiguration(&in.Items[i]))
	}

	sort.Stable(out)
	return *out, nil
}

func (r *query) ConfigurationRevisions(ctx context.Context, configuration *model.ReferenceID, active *bool) (model.ConfigurationRevisionConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.ConfigurationRevisionConnection{}, nil
	}

	in := &pkgv1.ConfigurationRevisionList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListConfigRevs))
		return model.ConfigurationRevisionConnection{}, nil
	}

	out := &model.ConfigurationRevisionConnection{
		Nodes: make([]model.ConfigurationRevision, 0),
	}

	for i := range in.Items {
		pr := in.Items[i] // So we don't take the address of a range variable.

		// The supplied configuration is not an owner of this PackageRevision.
		if configuration != nil && !containsID(pr.OwnerReferences, *configuration) {
			continue
		}

		// We only want the active PackageRevision, and this isn't it.
		if ptr.Deref(active, false) && pr.Spec.DesiredState != pkgv1.PackageRevisionActive {
			continue
		}

		out.Nodes = append(out.Nodes, model.GetConfigurationRevision(&pr))
		out.TotalCount++
	}

	sort.Stable(out)
	return *out, nil
}

func (r *query) CompositeResourceDefinitions(ctx context.Context, revision *model.ReferenceID, dangling *bool) (model.CompositeResourceDefinitionConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.CompositeResourceDefinitionConnection{}, nil
	}

	in := &extv1.CompositeResourceDefinitionList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListConfigs))
		return model.CompositeResourceDefinitionConnection{}, nil
	}

	out := &model.CompositeResourceDefinitionConnection{
		Nodes: make([]model.CompositeResourceDefinition, 0),
	}

	for i := range in.Items {
		xrd := &in.Items[i]

		// We only want dangling XRDs but this one is owned by a config revision.
		if ptr.Deref(dangling, false) && containsCR(xrd.GetOwnerReferences()) {
			continue
		}

		// We only want XRDs owned by this config revision, but this one isn't.
		if revision != nil && !containsID(xrd.GetOwnerReferences(), *revision) {
			continue
		}

		out.Nodes = append(out.Nodes, model.GetCompositeResourceDefinition(xrd))
		out.TotalCount++
	}

	sort.Stable(out)
	return *out, nil
}

func (r *query) Compositions(ctx context.Context, revision *model.ReferenceID, dangling *bool) (model.CompositionConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.CompositionConnection{}, nil
	}

	in := &extv1.CompositionList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListConfigs))
		return model.CompositionConnection{}, nil
	}

	out := &model.CompositionConnection{
		Nodes: make([]model.Composition, 0),
	}

	for i := range in.Items {
		cmp := &in.Items[i]

		// We only want dangling XRs but this one is owned by a config revision.
		if ptr.Deref(dangling, false) && containsCR(cmp.GetOwnerReferences()) {
			continue
		}

		// We only want XRs owned by this config revision, but this one isn't.
		if revision != nil && !containsID(cmp.GetOwnerReferences(), *revision) {
			continue
		}

		out.Nodes = append(out.Nodes, model.GetComposition(cmp))
		out.TotalCount++
	}

	sort.Stable(out)
	return *out, nil
}

func containsCR(in []metav1.OwnerReference) bool {
	for _, ref := range in {
		switch {
		case ref.APIVersion != pkgv1.ConfigurationRevisionGroupVersionKind.GroupVersion().String():
			continue
		case ref.Kind != pkgv1.ConfigurationRevisionKind:
			continue
		}
		return true
	}
	return false
}

func containsID(in []metav1.OwnerReference, id model.ReferenceID) bool {
	for _, ref := range in {
		switch {
		case ref.APIVersion != id.APIVersion:
			continue
		case ref.Kind != id.Kind:
			continue
		case ref.Name != id.Name:
			continue
		}
		return true
	}
	return false
}
