package resolvers

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/graph/model"
)

const (
	errGetSecret = "cannot get secret"
)

type managedResourceSpec struct {
	clients ClientCache
}

func (r *managedResourceSpec) ConnectionSecret(ctx context.Context, obj *model.ManagedResourceSpec) (*model.Secret, error) {
	if obj.WritesConnectionSecretToRef == nil {
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
		Namespace: obj.WritesConnectionSecretToRef.Namespace,
		Name:      obj.WritesConnectionSecretToRef.Name,
	}
	if err := c.Get(ctx, nn, s); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetSecret))
		return nil, nil
	}

	out := model.GetSecret(s)
	return &out, nil
}

func (r *managedResourceSpec) ProviderConfig(ctx context.Context, obj *model.ManagedResourceSpec) (*model.ProviderConfig, error) {
	// TODO(negz): How can we resolve this? We only know the name of the config,
	// not its apiVersion and kind. I can't immediately think of an approach
	// that isn't hacky and/or expensive.
	return nil, nil
}
