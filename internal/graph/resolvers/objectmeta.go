package resolvers

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/graph/model"
)

const (
	errGetOwner   = "cannot get owner"
	errModelOwner = "cannot model owner"
)

type objectMeta struct {
	clients ClientCache
}

func (r *objectMeta) Owners(ctx context.Context, obj *model.ObjectMeta) (*model.OwnerConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	owners := make([]model.Owner, 0, len(obj.OwnerReferences))
	for _, ref := range obj.OwnerReferences {
		u := &kunstructured.Unstructured{}
		u.SetAPIVersion(ref.APIVersion)
		u.SetKind(ref.Kind)

		nn := types.NamespacedName{Namespace: pointer.StringPtrDerefOr(obj.Namespace, ""), Name: ref.Name}
		if err := c.Get(ctx, nn, u); err != nil {
			graphql.AddError(ctx, errors.Wrap(err, errGetOwner))
			continue
		}

		kr, err := model.GetKubernetesResource(u)
		if err != nil {
			graphql.AddError(ctx, errors.Wrap(err, errModelOwner))
			continue
		}

		owners = append(owners, model.Owner{Controller: ref.Controller, Resource: kr})
	}

	return &model.OwnerConnection{Nodes: owners, TotalCount: len(owners)}, nil
}
