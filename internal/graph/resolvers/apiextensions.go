package resolvers

import (
	"context"

	"github.com/pkg/errors"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/negz/xgql/internal/graph/model"
	"github.com/negz/xgql/internal/token"
	"github.com/negz/xgql/internal/unstructured"
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
		return nil, errors.Wrap(err, "cannot get client")
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
		return nil, errors.Wrap(err, "cannot list resources")
	}

	out := &model.CompositeResourceConnection{
		Items: make([]model.CompositeResource, getLimit(limit, len(in.Items))),
		Count: len(in.Items),
	}

	for i := range out.Items {
		xr := &unstructured.Composite{Unstructured: in.Items[i]}

		raw, err := json.Marshal(xr)
		if err != nil {
			return nil, errors.Wrap(err, "could not marshal JSON")
		}

		out.Items[i] = model.CompositeResource{
			APIVersion: xr.GetAPIVersion(),
			Kind:       xr.GetKind(),
			Metadata:   model.GetObjectMeta(xr),
			Spec: &model.CompositeResourceSpec{
				CompositionSelector:               model.GetLabelSelector(xr.GetCompositionSelector()),
				CompositionReference:              xr.GetCompositionReference(),
				ClaimReference:                    xr.GetClaimReference(),
				ResourceReferences:                xr.GetResourceReferences(),
				WritesConnectionSecretToReference: xr.GetWriteConnectionSecretToReference(),
			},
			Status: &model.CompositeResourceStatus{
				Conditions: model.GetConditions(xr.GetConditions()),
				ConnectionDetails: &model.CompositeResourceConnectionDetails{
					LastPublishedTime: model.GetConnectionDetailsLastPublishedTime(xr.GetConnectionDetailsLastPublishedTime()),
				},
			},
			Raw: string(raw),
		}
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
