package resolvers

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"github.com/upbound/xgql/internal/graph/model"
	"github.com/upbound/xgql/internal/token"
)

type objectMetaResolver struct {
	clients ClientCache
}

func (r *objectMetaResolver) Owners(ctx context.Context, obj *model.ObjectMeta, controller *bool) (*model.OwnerConnection, error) {
	if len(obj.OwnerReferences) == 0 {
		return &model.OwnerConnection{}, nil
	}

	t, _ := token.FromContext(ctx)

	c, err := r.clients.Get(t)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, "cannot get client"))
		return nil, nil
	}

	out := &model.OwnerConnection{
		Items: make([]model.Owner, 0, len(obj.OwnerReferences)),
		Count: len(obj.OwnerReferences),
	}

	for _, ref := range obj.OwnerReferences {
		u := &kunstructured.Unstructured{}
		u.SetAPIVersion(ref.APIVersion)
		u.SetKind(ref.Kind)

		nn := types.NamespacedName{Namespace: pointer.StringPtrDerefOr(obj.Namespace, ""), Name: ref.Name}
		if err := c.Get(ctx, nn, u); err != nil {
			graphql.AddError(ctx, errors.Wrap(err, "cannot get owner"))
			continue
		}

		kr, err := model.GetKubernetesResource(u)
		if err != nil {
			graphql.AddError(ctx, errors.Wrap(err, "cannot model Kubernetes resource"))
			continue
		}

		owner := model.Owner{Controller: ref.Controller, Resource: kr}

		// There can be only one controller reference.
		// https://github.com/kubernetes/community/blob/0331e/contributors/design-proposals/api-machinery/controller-ref.md
		if pointer.BoolPtrDerefOr(controller, false) && pointer.BoolPtrDerefOr(ref.Controller, false) {
			return &model.OwnerConnection{Items: []model.Owner{owner}, Count: 1}, nil
		}

		out.Items = append(out.Items, owner)
	}

	return out, nil
}
