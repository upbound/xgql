package resolvers

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/pointer"

	"github.com/negz/xgql/internal/graph/model"
	"github.com/negz/xgql/internal/token"
)

type objectMetaResolver struct {
	clients ClientCache
}

func (r *objectMetaResolver) Owners(ctx context.Context, obj *model.ObjectMeta, limit *int, controller *bool) (*model.OwnerConnection, error) {
	if len(obj.OwnerReferences) == 0 {
		return &model.OwnerConnection{}, nil
	}

	t, _ := token.FromContext(ctx)

	c, err := r.clients.Get(t)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get client")
	}

	owners := make([]model.Owner, len(obj.OwnerReferences))
	for i, ref := range obj.OwnerReferences {
		u := &unstructured.Unstructured{}
		u.SetAPIVersion(ref.APIVersion)
		u.SetKind(ref.Kind)

		nn := types.NamespacedName{Namespace: pointer.StringPtrDerefOr(obj.Namespace, ""), Name: ref.Name}
		if err := c.Get(ctx, nn, u); err != nil {
			return nil, errors.Wrap(err, "cannot get owner")
		}

		raw, err := json.Marshal(u.Object)
		if err != nil {
			return nil, errors.Wrap(err, "cannot marshal JSON")
		}

		owner := model.Owner{
			Controller: ref.Controller,
			Resource: model.GenericResource{
				APIVersion: u.GetAPIVersion(),
				Kind:       u.GetKind(),
				Metadata:   model.GetObjectMeta(u),
				Raw:        string(raw),
			},
		}

		// There can be only one controller reference.
		// https://github.com/kubernetes/community/blob/0331e/contributors/design-proposals/api-machinery/controller-ref.md
		if pointer.BoolPtrDerefOr(controller, false) && pointer.BoolPtrDerefOr(ref.Controller, false) {
			return &model.OwnerConnection{Items: []model.Owner{owner}, Count: 1}, nil
		}

		owners[i] = owner
	}

	if limit != nil && *limit < len(owners) {
		owners = owners[:*limit]
	}

	return &model.OwnerConnection{Items: owners, Count: len(obj.OwnerReferences)}, nil
}
