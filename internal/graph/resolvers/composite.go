package resolvers

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/graph/model"
)

const (
	errGetComposition = "cannot get composition"
	errGetXR          = "cannot get composite resource"
	errGetXRC         = "cannot get composite resource claim"
	errGetComposed    = "cannot get composed resource"
	errModelComposed  = "cannot model composed resource"
)

type compositeResource struct {
	clients ClientCache
}

func (r *compositeResource) Events(ctx context.Context, obj *model.CompositeResource) (*model.EventConnection, error) {
	e := &events{clients: r.clients}
	return e.Resolve(ctx, &corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.Metadata.Name,
		UID:        types.UID(obj.Metadata.UID),
	})
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
		graphql.AddError(ctx, errors.Wrap(err, errGetComposition))
		return nil, nil
	}

	out := model.GetComposition(cmp)
	return &out, nil
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
		graphql.AddError(ctx, errors.Wrap(err, errGetXRC))
		return nil, nil
	}

	out := model.GetCompositeResourceClaim(xrc)
	return &out, nil
}

func (r *compositeResourceSpec) Resources(ctx context.Context, obj *model.CompositeResourceSpec) (*model.KubernetesResourceConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	out := &model.KubernetesResourceConnection{
		Nodes: make([]model.KubernetesResource, 0, len(obj.ResourceReferences)),
	}

	for _, ref := range obj.ResourceReferences {
		xrc := &unstructured.Unstructured{}
		xrc.SetAPIVersion(ref.APIVersion)
		xrc.SetKind(ref.Kind)
		nn := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
		if err := c.Get(ctx, nn, xrc); err != nil {
			graphql.AddError(ctx, errors.Wrap(err, errGetComposed))
			continue
		}

		kr, err := model.GetKubernetesResource(xrc)
		if err != nil {
			graphql.AddError(ctx, errors.Wrap(err, errModelComposed))
			continue
		}

		out.Nodes = append(out.Nodes, kr)
		out.TotalCount++
	}

	return out, nil
}

func (r *compositeResourceSpec) ConnectionSecret(ctx context.Context, obj *model.CompositeResourceSpec) (*model.Secret, error) {
	if obj.WritesConnectionSecretToReference == nil {
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
		Namespace: obj.WritesConnectionSecretToReference.Namespace,
		Name:      obj.WritesConnectionSecretToReference.Name,
	}
	if err := c.Get(ctx, nn, s); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetSecret))
		return nil, nil
	}

	out := model.GetSecret(s)
	return &out, nil
}

type compositeResourceClaim struct {
	clients ClientCache
}

func (r *compositeResourceClaim) Events(ctx context.Context, obj *model.CompositeResourceClaim) (*model.EventConnection, error) {
	e := &events{clients: r.clients}
	return e.Resolve(ctx, &corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.Metadata.Name,
		UID:        types.UID(obj.Metadata.UID),
	})
}

type compositeResourceClaimSpec struct {
	clients ClientCache
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
		graphql.AddError(ctx, errors.Wrap(err, errGetComposition))
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
		graphql.AddError(ctx, errors.Wrap(err, errGetXR))
		return nil, nil
	}

	out := model.GetCompositeResource(xr)
	return &out, nil
}

func (r *compositeResourceClaimSpec) ConnectionSecret(ctx context.Context, obj *model.CompositeResourceClaimSpec) (*model.Secret, error) {
	if obj.WritesConnectionSecretToReference == nil {
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
		Namespace: obj.WritesConnectionSecretToReference.Namespace,
		Name:      obj.WritesConnectionSecretToReference.Name,
	}
	if err := c.Get(ctx, nn, s); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetSecret))
		return nil, nil
	}

	out := model.GetSecret(s)
	return &out, nil
}
