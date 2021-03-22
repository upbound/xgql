package resolvers

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/upbound/xgql/internal/graph/model"
	"github.com/upbound/xgql/internal/token"
)

type managedResourceSpec struct {
	clients ClientCache
}

func (r *managedResourceSpec) ConnectionSecret(ctx context.Context, obj *model.ManagedResourceSpec) (*model.Secret, error) {
	if obj.WritesConnectionSecretToRef == nil {
		return nil, nil
	}

	t, _ := token.FromContext(ctx)

	c, err := r.clients.Get(t)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get client")
	}

	s := &corev1.Secret{}
	nn := types.NamespacedName{
		Namespace: obj.WritesConnectionSecretToRef.Namespace,
		Name:      obj.WritesConnectionSecretToRef.Name,
	}
	if err := c.Get(ctx, nn, s); err != nil {
		return nil, errors.Wrap(err, "cannot get secret")
	}

	raw, err := json.Marshal(s)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal JSON")
	}

	out := &model.Secret{
		// NOTE(negz): Apparently secrets don't have their type meta set?
		APIVersion: "v1",
		Kind:       "Secret",
		Metadata:   model.GetObjectMeta(s),
		Raw:        string(raw),
	}

	return out, nil

}
func (r *managedResourceSpec) ProviderConfig(ctx context.Context, obj *model.ManagedResourceSpec) (*model.ProviderConfig, error) {
	// TODO(negz): How can we resolve this? We only know the name of the config,
	// not its apiVersion and kind. I can't immediately think of an approach
	// that isn't hacky and/or expensive.
	return nil, nil
}
