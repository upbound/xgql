package resolvers

import (
	"context"

	"github.com/pkg/errors"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/upbound/xgql/internal/graph/model"
	"github.com/upbound/xgql/internal/token"
	"github.com/upbound/xgql/internal/unstructured"
)

type crd struct {
	clients ClientCache
}

func (r *crd) DefinedResources(ctx context.Context, obj *model.CustomResourceDefinition, limit *int, version *string) (*model.KubernetesResourceConnection, error) { //nolint:gocyclo
	// TODO(negz): This method is over our complexity goal; it may be worth
	// breaking the switch out into its own function.

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
		gv.Version = pickCRDVersion(obj.Spec.Versions)
	}

	in := &kunstructured.UnstructuredList{}
	in.SetAPIVersion(gv.String())
	in.SetKind(obj.Spec.Names.Kind + "List")
	if lk := obj.Spec.Names.ListKind; lk != nil && *lk != "" {
		in.SetKind(*lk)
	}

	// TODO(negz): Support filtering this by a particular namespace? Currently
	// we assume the caller has access to list the defined resource in all
	// namespaces (or at cluster scope). In practice we expect this call to be
	// used by platform operators to list managed resources, which are cluster
	// scoped, but in theory a CRD could define any kind of custom resource. We
	// could accept an optional 'namespace' argument and pass client.InNamespace
	// to c.List, but I believe the underlying cache's ListWatch will still try
	// (and fail) to list namespaced resources across all namespaces, so we may
	// also need a way to fetch a client with a namespaced cache.
	if err := c.List(ctx, in); err != nil {
		return nil, errors.Wrap(err, "cannot list resources")
	}

	out := &model.KubernetesResourceConnection{
		Items: make([]model.KubernetesResource, getLimit(limit, len(in.Items))),
		Count: len(in.Items),
	}

	for i := range out.Items {
		u := in.Items[i]

		raw, err := json.Marshal(u)
		if err != nil {
			return nil, errors.Wrap(err, "could not marshal JSON")
		}

		switch {
		case hasCategory(obj.Spec.Names, "managed"), unstructured.ProbablyManaged(&u):
			m := &unstructured.Managed{Unstructured: u}
			out.Items[i] = model.ManagedResource{
				APIVersion: u.GetAPIVersion(),
				Kind:       u.GetKind(),
				Metadata:   model.GetObjectMeta(&u),
				Spec: &model.ManagedResourceSpec{
					WritesConnectionSecretToRef: m.GetWriteConnectionSecretToReference(),
					ProviderConfigRef:           m.GetProviderConfigReference(),
					DeletionPolicy:              model.GetDeletionPolicy(m.GetDeletionPolicy()),
				},
				Status: &model.ManagedResourceStatus{
					Conditions: model.GetConditions(m.GetConditions()),
				},
				Raw: string(raw),
			}

		case hasCategory(obj.Spec.Names, "provider") && obj.Spec.Names.Kind != "ProviderConfigUsage", unstructured.ProbablyProviderConfig(&u):
			pc := &unstructured.ProviderConfig{Unstructured: u}
			users := int(pc.GetUsers())
			out.Items[i] = model.ProviderConfig{
				APIVersion: u.GetAPIVersion(),
				Kind:       u.GetKind(),
				Metadata:   model.GetObjectMeta(&u),
				Status: &model.ProviderConfigStatus{
					Conditions: model.GetConditions(pc.GetConditions()),
					Users:      &users,
				},
				Raw: string(raw),
			}

		default:
			out.Items[i] = model.GenericResource{
				APIVersion: u.GetAPIVersion(),
				Kind:       u.GetKind(),
				Metadata:   model.GetObjectMeta(&u),
				Raw:        string(raw),
			}
		}

	}

	return out, nil
}

// TODO(negz): Try to pick the 'highest' version (e.g. v2 > v1 > v1beta1),
// rather than returning the first served one. There's no guarantee versions
// will actually follow this convention, but it's ubiquitous.
func pickCRDVersion(vs []model.CustomResourceDefinitionVersion) string {
	candidates := make([]string, 0, len(vs))

	for _, v := range vs {
		if v.Served {
			candidates = append(candidates, v.Name)
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	return candidates[0]
}

func hasCategory(n *model.CustomResourceDefinitionNames, cat string) bool {
	if n == nil {
		return false
	}

	for _, c := range n.Categories {
		if c == cat {
			return true
		}
	}
	return false
}
