package resolvers

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/graph/model"
)

const (
	errGetClient     = "cannot get client"
	errListProviders = "cannot list providers"
	errListConfigs   = "cannot list configurations"
)

type query struct {
	clients ClientCache
}

func (r *query) KubernetesResource(ctx context.Context, id model.ReferenceID) (model.KubernetesResource, error) {
	return nil, nil
}

func (r *query) Events(ctx context.Context, involved *model.ReferenceID) (*model.EventConnection, error) {
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

func (r *query) Secret(ctx context.Context, namespace string, name string) (*model.Secret, error) {
	return nil, nil
}

func (r *query) ConfigMap(ctx context.Context, namespace string, name string) (*model.ConfigMap, error) {
	return nil, nil
}

func (r *query) Providers(ctx context.Context) (*model.ProviderConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	in := &pkgv1.ProviderList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListProviders))
		return nil, nil
	}

	out := &model.ProviderConnection{
		Nodes:      make([]model.Provider, 0, len(in.Items)),
		TotalCount: len(in.Items),
	}

	for i := range in.Items {
		out.Nodes = append(out.Nodes, model.GetProvider(&in.Items[i]))
	}

	return out, nil
}

func (r *query) ProviderRevisions(ctx context.Context, provider *model.ReferenceID) (*model.ProviderRevisionConnection, error) {
	return nil, nil
}

func (r *query) CustomResourceDefinitions(ctx context.Context, revision *model.ReferenceID) (*model.CustomResourceDefinitionConnection, error) {
	return nil, nil
}

func (r *query) ManagedResources(ctx context.Context, crd model.ReferenceID) (*model.ManagedResourceConnection, error) {
	return nil, nil
}

func (r *query) ProviderConfigs(ctx context.Context, crd model.ReferenceID) (*model.ProviderConfigConnection, error) {
	return nil, nil
}

func (r *query) Configurations(ctx context.Context) (*model.ConfigurationConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	in := &pkgv1.ConfigurationList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListConfigs))
		return nil, nil
	}

	out := &model.ConfigurationConnection{
		Nodes:      make([]model.Configuration, 0, len(in.Items)),
		TotalCount: len(in.Items),
	}

	for i := range in.Items {
		out.Nodes = append(out.Nodes, model.GetConfiguration(&in.Items[i]))
	}

	return out, nil
}

func (r *query) ConfigurationRevisions(ctx context.Context, configuration *model.ReferenceID) (*model.ConfigurationRevisionConnection, error) {
	return nil, nil
}

func (r *query) CompositeResourceDefinitions(ctx context.Context, revision *model.ReferenceID, dangling *bool) (*model.CompositeResourceDefinitionConnection, error) {
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
		graphql.AddError(ctx, errors.Wrap(err, errListConfigs))
		return nil, nil
	}

	out := &model.CompositeResourceDefinitionConnection{
		Nodes: make([]model.CompositeResourceDefinition, 0),
	}

	for i := range in.Items {
		xrd := &in.Items[i]

		// We only want dangling XRs but this one is owned by a config revision.
		if pointer.BoolPtrDerefOr(dangling, false) && containsCR(xrd.GetOwnerReferences()) {
			continue
		}

		// We only want XRs owned by this config revision, but this one isn't.
		if revision != nil && !containsID(xrd.GetOwnerReferences(), *revision) {
			continue
		}

		out.Nodes = append(out.Nodes, model.GetCompositeResourceDefinition(xrd))
		out.TotalCount++
	}

	return out, nil
}

func (r *query) Compositions(ctx context.Context, revision *model.ReferenceID, dangling *bool) (*model.CompositionConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	in := &extv1.CompositionList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListConfigs))
		return nil, nil
	}

	out := &model.CompositionConnection{
		Nodes: make([]model.Composition, 0),
	}

	for i := range in.Items {
		cmp := &in.Items[i]

		// We only want dangling XRs but this one is owned by a config revision.
		if pointer.BoolPtrDerefOr(dangling, false) && containsCR(cmp.GetOwnerReferences()) {
			continue
		}

		// We only want XRs owned by this config revision, but this one isn't.
		if revision != nil && !containsID(cmp.GetOwnerReferences(), *revision) {
			continue
		}

		out.Nodes = append(out.Nodes, model.GetComposition(cmp))
		out.TotalCount++
	}

	return out, nil
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

func (r *query) CompositeResources(ctx context.Context, xrd model.ReferenceID) (*model.CompositeResourceConnection, error) {
	return nil, nil
}

func (r *query) CompositeResource(ctx context.Context, xrc model.ReferenceID) (*model.CompositeResource, error) {
	return nil, nil
}

func (r *query) CompositeResourceClaims(ctx context.Context, xrd model.ReferenceID) (*model.CompositeResourceClaimConnection, error) {
	return nil, nil
}

func (r *query) CompositeResourceClaim(ctx context.Context, xr model.ReferenceID) (*model.CompositeResourceClaim, error) {
	return nil, nil
}
