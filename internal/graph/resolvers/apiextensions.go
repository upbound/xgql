package resolvers

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/upbound/xgql/internal/graph/model"
	"github.com/upbound/xgql/internal/token"
)

type xrd struct {
	clients ClientCache
}

func (r *xrd) Events(ctx context.Context, obj *model.CompositeResourceDefinition, limit *int) (*model.EventConnection, error) {
	return nil, nil
}

func (r *xrd) DefinedCompositeResources(ctx context.Context, obj *model.CompositeResourceDefinition, limit *int, version *string) (*model.CompositeResourceConnection, error) {
	t, _ := token.FromContext(ctx)

	c, err := r.clients.Get(t)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, "cannot get client"))
		return nil, nil
	}

	gv := schema.GroupVersion{Group: obj.Spec.Group}
	switch {
	case version != nil:
		gv.Version = *version
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
		graphql.AddError(ctx, errors.Wrap(err, "cannot list resources"))
		return nil, nil
	}

	lim := getLimit(limit, len(in.Items))
	out := &model.CompositeResourceConnection{
		Items: make([]model.CompositeResource, 0, lim),
		Count: len(in.Items),
	}

	for i := range in.Items[:lim] {
		cr, err := model.GetCompositeResource(&in.Items[i])
		if err != nil {
			graphql.AddError(ctx, errors.Wrap(err, "cannot model composite resource"))
			continue
		}
		out.Items = append(out.Items, cr)
	}

	return out, nil
}

func (r *xrd) DefinedCompositeResourceClaims(ctx context.Context, obj *model.CompositeResourceDefinition, limit *int, version, namespace *string) (*model.CompositeResourceClaimConnection, error) {
	return nil, nil
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