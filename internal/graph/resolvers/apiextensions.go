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
	"fmt"
	"sort"

	"github.com/99designs/gqlgen/graphql"
	corev1 "k8s.io/api/core/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/graph/model"
	"github.com/upbound/xgql/internal/unstructured"
)

const (
	errListResources = "cannot list defined resources"
)

type xrd struct {
	clients ClientCache
}

func (r *xrd) Events(ctx context.Context, obj *model.CompositeResourceDefinition) (model.EventConnection, error) {
	e := &events{clients: r.clients}
	return e.Resolve(ctx, &corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.Metadata.Name,
		UID:        types.UID(obj.Metadata.UID),
	})
}

func (r *xrd) getCrd(ctx context.Context, group string, names *model.CompositeResourceDefinitionNames) (*model.CustomResourceDefinition, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if names == nil {
		return nil, nil
	}

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	nn := types.NamespacedName{Name: fmt.Sprintf("%s.%s", names.Plural, group)}
	in := unstructured.NewCRD()
	if err := c.Get(ctx, nn, in.GetUnstructured()); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetCRD))
		return nil, nil
	}

	out := model.GetCustomResourceDefinition(in)
	return &out, nil
}

func (r *xrd) CompositeResourceCrd(ctx context.Context, obj *model.CompositeResourceDefinition) (*model.CustomResourceDefinition, error) {
	return r.getCrd(ctx, obj.Spec.Group, &obj.Spec.Names)
}

func (r *xrd) CompositeResourceClaimCrd(ctx context.Context, obj *model.CompositeResourceDefinition) (*model.CustomResourceDefinition, error) {
	return r.getCrd(ctx, obj.Spec.Group, obj.Spec.ClaimNames)
}

func (r *xrd) DefinedCompositeResources(ctx context.Context, obj *model.CompositeResourceDefinition, version *string, options *model.DefinedCompositeResourceOptionsInput) (model.CompositeResourceConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if options == nil {
		options = &model.DefinedCompositeResourceOptionsInput{}
	}

	options.DeprecationPatch(version)

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.CompositeResourceConnection{}, nil
	}

	gv := schema.GroupVersion{Group: obj.Spec.Group}
	switch {
	case options.Version != nil:
		gv.Version = *options.Version
	default:
		gv.Version = pickXRDVersion(obj.Spec.Versions)
	}

	in := &kunstructured.UnstructuredList{}
	in.SetAPIVersion(gv.String())
	in.SetKind(obj.Spec.Names.Kind + "List")
	if lk := obj.Spec.Names.ListKind; lk != nil && *lk != "" {
		in.SetKind(*lk)
	}

	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListResources))
		return model.CompositeResourceConnection{}, nil
	}

	return getCompositeResourceConnection(in, options), nil
}

/*
Produce a CompositeResourceClaimConnection from the raw k8s UnstructuredList
that is filtered and sorted
*/
func getCompositeResourceConnection(in *kunstructured.UnstructuredList, options *model.DefinedCompositeResourceOptionsInput) model.CompositeResourceConnection {
	xrs := []model.CompositeResource{}

	for i := range in.Items {
		xr := model.GetCompositeResource(&in.Items[i])
		if readyMatches(options.Ready, &xr) {
			xrs = append(xrs, xr)
		}
	}
	out := &model.CompositeResourceConnection{
		Nodes:      xrs,
		TotalCount: len(xrs),
	}

	sort.Stable(out)
	return *out
}

func (r *xrd) DefinedCompositeResourceClaims(ctx context.Context, obj *model.CompositeResourceDefinition, version *string, namespace *string, options *model.DefinedCompositeResourceClaimOptionsInput) (model.CompositeResourceClaimConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Return early if this XRD doesn't offer a claim.
	if obj.Spec.ClaimNames == nil {
		return model.CompositeResourceClaimConnection{}, nil
	}

	if options == nil {
		options = &model.DefinedCompositeResourceClaimOptionsInput{}
	}

	options.DeprecationPatch(version, namespace)

	lopts := []client.ListOption{}
	if options.Namespace != nil {
		lopts = []client.ListOption{client.InNamespace(*options.Namespace)}
	}

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.CompositeResourceClaimConnection{}, nil
	}

	gv := schema.GroupVersion{Group: obj.Spec.Group}
	switch {
	case options.Version != nil:
		gv.Version = *options.Version
	default:
		gv.Version = pickXRDVersion(obj.Spec.Versions)
	}

	in := &kunstructured.UnstructuredList{}
	in.SetAPIVersion(gv.String())
	in.SetKind(obj.Spec.ClaimNames.Kind + "List")
	if lk := obj.Spec.ClaimNames.ListKind; lk != nil && *lk != "" {
		in.SetKind(*lk)
	}

	if err := c.List(ctx, in, lopts...); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListResources))
		return model.CompositeResourceClaimConnection{}, nil
	}

	return getCompositeResourceClaimConnection(in, options), nil
}

/*
Produce a CompositeResourceClaimConnection from the raw k8s UnstructuredList
that is filtered and sorted
*/
func getCompositeResourceClaimConnection(in *kunstructured.UnstructuredList, options *model.DefinedCompositeResourceClaimOptionsInput) model.CompositeResourceClaimConnection {
	claims := []model.CompositeResourceClaim{}

	for i := range in.Items {
		claim := model.GetCompositeResourceClaim(&in.Items[i])
		if readyMatches(options.Ready, &claim) {
			claims = append(claims, claim)
		}
	}

	out := &model.CompositeResourceClaimConnection{
		Nodes:      claims,
		TotalCount: len(claims),
	}

	sort.Stable(out)
	return *out
}

/* Check that ready matches filter
 * If nil is passed then any ready state is allowed
 * If true is passed then only ready state `True` is required
 * If false is passed then ready state `True` is excluded
 */
func readyMatches(ready *bool, m model.ConditionedModel) bool {
	if ready == nil {
		return true
	}

	for _, c := range m.GetConditions() {
		if c.Type == "Ready" {
			return (c.Status == model.ConditionStatusTrue) == *ready
		}
	}
	return !*ready

}

// TODO(negz): Try to pick the 'highest' version (e.g. v2 > v1 > v1beta1),
// rather than returning the first served one. There's no guarantee versions
// will actually follow this convention, but it's ubiquitous.
func pickXRDVersion(vs []model.CompositeResourceDefinitionVersion) string {
	for _, v := range vs {
		if v.Referenceable {
			return v.Name
		}
	}

	// Technically one version of the XRD must be marked as referenceable, but
	// nothing enforces that. We fall back to picking a served version just in
	// case.
	for _, v := range vs {
		if v.Served {
			return v.Name
		}
	}

	// We shouldn't get here, unless the XRD is serving no versions?
	return ""
}

type xrdSpec struct {
	clients ClientCache
}

func (r *xrdSpec) DefaultComposition(ctx context.Context, obj *model.CompositeResourceDefinitionSpec) (*model.Composition, error) {
	if obj.DefaultCompositionReference == nil {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	cmp := &extv1.Composition{}
	nn := types.NamespacedName{Name: obj.DefaultCompositionReference.Name}
	if err := c.Get(ctx, nn, cmp); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetComposition))
		return nil, nil
	}

	out := model.GetComposition(cmp)
	return &out, nil
}

func (r *xrdSpec) EnforcedComposition(ctx context.Context, obj *model.CompositeResourceDefinitionSpec) (*model.Composition, error) {
	if obj.EnforcedCompositionReference == nil {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	cmp := &extv1.Composition{}
	nn := types.NamespacedName{Name: obj.EnforcedCompositionReference.Name}
	if err := c.Get(ctx, nn, cmp); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetComposition))
		return nil, nil
	}

	out := model.GetComposition(cmp)
	return &out, nil
}

type composition struct {
	clients ClientCache
}

func (r *composition) Events(ctx context.Context, obj *model.Composition) (model.EventConnection, error) {
	e := &events{clients: r.clients}
	return e.Resolve(ctx, &corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.Metadata.Name,
		UID:        types.UID(obj.Metadata.UID),
	})
}
