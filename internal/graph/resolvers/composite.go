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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/graph/model"
)

const (
	errListXRDs            = "cannot list composite resource definitions"
	errMalformedAPIVersion = "cannot parse malformed API version"
	errGetComposition      = "cannot get composition"
	errGetXR               = "cannot get composite resource"
	errGetXRC              = "cannot get composite resource claim"
	errGetComposed         = "cannot get composed resource"
	errModelComposed       = "cannot model composed resource"
)

type compositeResource struct {
	clients ClientCache
}

func (r *compositeResource) Events(ctx context.Context, obj *model.CompositeResource) (model.EventConnection, error) {
	e := &events{clients: r.clients}
	return e.Resolve(ctx, &corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.Metadata.Name,
		UID:        types.UID(obj.Metadata.UID),
	})
}

func (r *compositeResource) Definition(ctx context.Context, obj *model.CompositeResource) (*model.CompositeResourceDefinition, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	in := &extv1.CompositeResourceDefinitionList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListXRDs))
		return nil, nil
	}

	gv, err := schema.ParseGroupVersion(obj.APIVersion)
	if err != nil {
		// This should be pretty much impossible - the API server should not
		// return resources with malformed API versions.
		graphql.AddError(ctx, errors.Wrap(err, errMalformedAPIVersion))
		return nil, nil
	}

	for i := range in.Items {
		xrd := in.Items[i] // So we don't take the address of a range variable.

		if xrd.Spec.Group != gv.Group {
			continue
		}

		if xrd.Spec.Names.Kind != obj.Kind {
			continue
		}

		out := model.GetCompositeResourceDefinition(&xrd)
		return &out, nil
	}

	// This should also be impossible - all XRs should be defined by an XRD. If
	// we get here we've hit an edge case like finding  a resource that quacks
	// like an XR but is not one.
	return nil, nil
}

type compositeResourceSpec struct {
	clients ClientCache
}

func (r *compositeResourceSpec) Composition(ctx context.Context, obj *model.CompositeResourceSpec) (*model.Composition, error) {
	if obj.CompositionReference == nil {
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
	nn := types.NamespacedName{Name: obj.CompositionReference.Name}
	if err := c.Get(ctx, nn, cmp); err != nil {
		if !apierrors.IsNotFound(err) {
			graphql.AddError(ctx, errors.Wrap(err, errGetComposition))
		}

		return nil, nil
	}

	out := model.GetComposition(cmp)
	return &out, nil
}

func (r *compositeResourceSpec) CompositionRef(ctx context.Context, obj *model.CompositeResourceSpec) (*model.LocalObjectReference, error) {
	if obj.CompositionReference == nil {
		return nil, nil
	}

	return &model.LocalObjectReference{Name: obj.CompositionReference.Name}, nil
}

func (r *compositeResourceSpec) Claim(ctx context.Context, obj *model.CompositeResourceSpec) (*model.CompositeResourceClaim, error) {
	if obj.ClaimReference == nil {
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

	xrc := &unstructured.Unstructured{}
	xrc.SetAPIVersion(obj.ClaimReference.APIVersion)
	xrc.SetKind(obj.ClaimReference.Kind)
	nn := types.NamespacedName{
		Namespace: obj.ClaimReference.Namespace,
		Name:      obj.ClaimReference.Name,
	}
	if err := c.Get(ctx, nn, xrc); err != nil {
		if !apierrors.IsNotFound(err) {
			graphql.AddError(ctx, errors.Wrap(err, errGetXRC))
		}

		return nil, nil
	}

	out := model.GetCompositeResourceClaim(xrc)
	return &out, nil
}

func (r *compositeResourceSpec) ClaimRef(ctx context.Context, obj *model.CompositeResourceSpec) (*model.ObjectReference, error) {
	if obj == nil || obj.ClaimReference == nil {
		return nil, nil
	}

	return model.GetObjectReference(&corev1.ObjectReference{
		Kind:       obj.ClaimReference.Kind,
		Namespace:  obj.ClaimReference.Namespace,
		Name:       obj.ClaimReference.Name,
		APIVersion: obj.ClaimReference.APIVersion,
	}), nil
}

func (r *compositeResourceSpec) Resources(ctx context.Context, obj *model.CompositeResourceSpec) (model.KubernetesResourceConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return model.KubernetesResourceConnection{}, nil
	}

	out := &model.KubernetesResourceConnection{
		Nodes: make([]model.KubernetesResource, 0, len(obj.ResourceReferences)),
	}

	// Collect all concurrently.
	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	for _, ref := range obj.ResourceReferences {
		// Ignore nameless resource references
		if ref.Name == "" {
			continue
		}

		ref := ref // So we don't take the address of a range variable.
		wg.Add(1)
		go func() {
			defer wg.Done()
			xrc := &unstructured.Unstructured{}
			xrc.SetAPIVersion(ref.APIVersion)
			xrc.SetKind(ref.Kind)
			nn := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
			if err := c.Get(ctx, nn, xrc); err != nil {
				if !apierrors.IsNotFound(err) {
					graphql.AddError(ctx, errors.Wrap(err, errGetComposed))
				}
				return
			}

			kr, err := model.GetKubernetesResource(xrc)
			if err != nil {
				graphql.AddError(ctx, errors.Wrap(err, errModelComposed))
				return
			}

			mu.Lock()
			defer mu.Unlock()
			out.Nodes = append(out.Nodes, kr)
			out.TotalCount++
		}()
	}
	wg.Wait()

	sort.Stable(out)
	return *out, nil
}

func (r *compositeResourceSpec) ResourceRefs(ctx context.Context, obj *model.CompositeResourceSpec) ([]model.ObjectReference, error) {
	resourceRefs := make([]model.ObjectReference, 0, len(obj.ResourceReferences))
	for i := range obj.ResourceReferences {
		resourceRefs = append(resourceRefs, *model.GetObjectReference(&obj.ResourceReferences[i]))
	}
	return resourceRefs, nil
}

func (r *compositeResourceSpec) ConnectionSecret(ctx context.Context, obj *model.CompositeResourceSpec) (*model.Secret, error) {
	if obj.WriteConnectionSecretToReference == nil {
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

	s := &corev1.Secret{}
	nn := types.NamespacedName{
		Namespace: obj.WriteConnectionSecretToReference.Namespace,
		Name:      obj.WriteConnectionSecretToReference.Name,
	}
	if err := c.Get(ctx, nn, s); err != nil {
		if !apierrors.IsNotFound(err) {
			graphql.AddError(ctx, errors.Wrap(err, errGetSecret))
		}

		return nil, nil
	}

	out := model.GetSecret(s)
	return &out, nil
}

func (r *compositeResourceSpec) WriteConnectionSecretToReference(ctx context.Context, obj *model.CompositeResourceSpec) (*model.SecretReference, error) {
	return model.GetSecretReference(obj.WriteConnectionSecretToReference), nil
}

type compositeResourceClaim struct {
	clients ClientCache
}

func (r *compositeResourceClaim) Events(ctx context.Context, obj *model.CompositeResourceClaim) (model.EventConnection, error) {
	e := &events{clients: r.clients}
	return e.Resolve(ctx, &corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.Metadata.Name,
		UID:        types.UID(obj.Metadata.UID),
	})
}

func (r *compositeResourceClaim) Definition(ctx context.Context, obj *model.CompositeResourceClaim) (*model.CompositeResourceDefinition, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	in := &extv1.CompositeResourceDefinitionList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListXRDs))
		return nil, nil
	}

	gv, err := schema.ParseGroupVersion(obj.APIVersion)
	if err != nil {
		// This should be pretty much impossible - the API server should not
		// return resources with malformed API versions.
		graphql.AddError(ctx, errors.Wrap(err, errMalformedAPIVersion))
		return nil, nil
	}

	for i := range in.Items {
		xrd := in.Items[i] // So we don't take the address of a range variable.

		if xrd.Spec.ClaimNames == nil {
			continue
		}

		if xrd.Spec.Group != gv.Group {
			continue
		}

		if xrd.Spec.ClaimNames.Kind != obj.Kind {
			continue
		}

		out := model.GetCompositeResourceDefinition(&xrd)
		return &out, nil
	}

	// This should also be impossible - all XRCs should be defined by an XRD. If
	// we get here we've hit an edge case like finding  a resource that quacks
	// like an XRC but is not one.
	return nil, nil
}

type compositeResourceClaimSpec struct {
	clients ClientCache
}

func (r *compositeResourceClaimSpec) CompositionRef(ctx context.Context, obj *model.CompositeResourceClaimSpec) (*model.LocalObjectReference, error) {
	if obj.CompositionReference == nil {
		return nil, nil
	}

	return &model.LocalObjectReference{Name: obj.CompositionReference.Name}, nil
}

func (r *compositeResourceClaimSpec) Composition(ctx context.Context, obj *model.CompositeResourceClaimSpec) (*model.Composition, error) {
	if obj.CompositionReference == nil {
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
	nn := types.NamespacedName{Name: obj.CompositionReference.Name}
	if err := c.Get(ctx, nn, cmp); err != nil {
		if !apierrors.IsNotFound(err) {
			graphql.AddError(ctx, errors.Wrap(err, errGetComposition))
		}

		return nil, nil
	}

	out := model.GetComposition(cmp)
	return &out, nil
}

func (r *compositeResourceClaimSpec) Resource(ctx context.Context, obj *model.CompositeResourceClaimSpec) (*model.CompositeResource, error) {
	if obj.ResourceReference == nil {
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

	xr := &unstructured.Unstructured{}
	xr.SetAPIVersion(obj.ResourceReference.APIVersion)
	xr.SetKind(obj.ResourceReference.Kind)
	nn := types.NamespacedName{Name: obj.ResourceReference.Name}
	if err := c.Get(ctx, nn, xr); err != nil {
		if !apierrors.IsNotFound(err) {
			graphql.AddError(ctx, errors.Wrap(err, errGetXR))
		}

		return nil, nil
	}

	out := model.GetCompositeResource(xr)
	return &out, nil
}

func (r *compositeResourceClaimSpec) ResourceRef(ctx context.Context, obj *model.CompositeResourceClaimSpec) (*model.ObjectReference, error) {
	return model.GetObjectReference(obj.ResourceReference), nil
}

func (r *compositeResourceClaimSpec) ConnectionSecret(ctx context.Context, obj *model.CompositeResourceClaimSpec) (*model.Secret, error) {
	if obj.WriteConnectionSecretToReference == nil {
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

	s := &corev1.Secret{}
	nn := types.NamespacedName{
		Namespace: obj.WriteConnectionSecretToReference.Namespace,
		Name:      obj.WriteConnectionSecretToReference.Name,
	}
	if err := c.Get(ctx, nn, s); err != nil {
		if !apierrors.IsNotFound(err) {
			graphql.AddError(ctx, errors.Wrap(err, errGetSecret))
		}

		return nil, nil
	}

	out := model.GetSecret(s)
	return &out, nil
}

func (r *compositeResourceClaimSpec) WriteConnectionSecretToReference(ctx context.Context, obj *model.CompositeResourceClaimSpec) (*model.SecretReference, error) {
	return model.GetSecretReference(obj.WriteConnectionSecretToReference), nil
}
