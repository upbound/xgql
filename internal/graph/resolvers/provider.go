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
	"strings"
	"sync"

	"github.com/99designs/gqlgen/graphql"
	corev1 "k8s.io/api/core/v1"
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/graph/model"
	xunstructured "github.com/upbound/xgql/internal/unstructured"
)

const (
	errListProviderRevs = "cannot list provider revisions"
	errGetCRD           = "cannot get custom resource definition"
)

type provider struct {
	clients ClientCache
}

func (r *provider) Events(ctx context.Context, obj *model.Provider) (model.EventConnection, error) {
	e := &events{clients: r.clients}
	return e.Resolve(ctx, &corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.Metadata.Name,
		UID:        types.UID(obj.Metadata.UID),
	})
}

func (r *provider) Revisions(ctx context.Context, obj *model.Provider) (model.ProviderRevisionConnection, error) {
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

		// We're not the controller reference of this ProviderRevision; it's not
		// one of ours.
		// https://github.com/kubernetes/community/blob/0331e/contributors/design-proposals/api-machinery/controller-ref.md
		if c := metav1.GetControllerOf(&pr); c == nil || c.UID != types.UID(obj.Metadata.UID) {
			continue
		}

		out.Nodes = append(out.Nodes, model.GetProviderRevision(&pr))
		out.TotalCount++
	}

	sort.Stable(out)
	return *out, nil
}

func (r *provider) ActiveRevision(ctx context.Context, obj *model.Provider) (*model.ProviderRevision, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	in := &pkgv1.ProviderRevisionList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListProviderRevs))
		return nil, nil
	}

	for i := range in.Items {
		pr := in.Items[i] // So we don't take the address of a range variable.

		// This revision is not active.
		if pr.Spec.DesiredState != pkgv1.PackageRevisionActive {
			continue
		}

		// We're not the controller reference of this ProviderRevision;
		// it's not one of ours.
		// https://github.com/kubernetes/community/blob/0331e/contributors/design-proposals/api-machinery/controller-ref.md
		if c := metav1.GetControllerOf(&pr); c == nil || c.UID != types.UID(obj.Metadata.UID) {
			continue
		}

		out := model.GetProviderRevision(&pr)
		return &out, nil
	}

	return nil, nil
}

type providerRevision struct {
	clients ClientCache
}

func (r *providerRevision) Events(ctx context.Context, obj *model.ProviderRevision) (model.EventConnection, error) {
	e := &events{clients: r.clients}
	return e.Resolve(ctx, &corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.Metadata.Name,
		UID:        types.UID(obj.Metadata.UID),
	})
}

type providerRevisionStatus struct {
	clients ClientCache
}

func (r *providerRevisionStatus) Objects(ctx context.Context, obj *model.ProviderRevisionStatus) (model.KubernetesResourceConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.KubernetesResourceConnection{}, nil
	}

	out := &model.KubernetesResourceConnection{
		Nodes: make([]model.KubernetesResource, 0, len(obj.ObjectRefs)),
	}

	// Collect all concurrently.
	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	for _, ref := range obj.ObjectRefs {
		// Crossplane lints provider packages to ensure they only contain CRDs,
		// but this isn't enforced at the API level. We filter out anything that
		// isn't a CRD, just in case.
		if ref.Kind != "CustomResourceDefinition" {
			continue
		}
		if strings.Split(ref.APIVersion, "/")[0] != kextv1.GroupName {
			continue
		}

		ref := ref // So we don't take the address of a range variable.
		wg.Add(1)
		go func() {
			defer wg.Done()
			crd := xunstructured.NewCRD()
			if err := c.Get(ctx, types.NamespacedName{Name: ref.Name}, crd.GetUnstructured()); err != nil {
				graphql.AddError(ctx, errors.Wrap(err, errGetCRD))
				return
			}

			mu.Lock()
			defer mu.Unlock()
			out.Nodes = append(out.Nodes, model.GetCustomResourceDefinition(crd))
			out.TotalCount++
		}()
	}
	wg.Wait()

	return *out, nil
}
