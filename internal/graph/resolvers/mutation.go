package resolvers

import (
	"context"
	"encoding/json"

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/graph/model"
)

const (
	errCreateResource = "cannot create Kubernetes resource"
	errUnmarshalRaw   = "cannot unmarshal input raw JSON"
)

type mutation struct {
	clients ClientCache
}

func (r *mutation) CreateKubernetesResource(ctx context.Context, input model.CreateKubernetesResourceInput) (*model.CreateKubernetesResourcePayload, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	u := &unstructured.Unstructured{}
	if err := json.Unmarshal(input.Unstructured, u); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errUnmarshalRaw))
		return nil, nil
	}

	if err := c.Create(ctx, u); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errCreateResource))
		return nil, nil
	}

	kr, err := model.GetKubernetesResource(u)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errModelResource))
		return nil, nil
	}
	return &model.CreateKubernetesResourcePayload{Resource: kr}, nil
}

func (r *mutation) UpdateKubernetesResource(ctx context.Context, _ model.ReferenceID, input model.UpdateKubernetesResourceInput) (*model.UpdateKubernetesResourcePayload, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	u := &unstructured.Unstructured{}
	if err := json.Unmarshal(input.Unstructured, u); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errUnmarshalRaw))
		return nil, nil
	}

	// TODO(negz): Validate that the unmarshalled resource matches the supplied
	// ReferenceID? Or maybe just don't take ReferenceID? We don't actually need
	// it if the caller is supplying us with the whole object as JSON.

	if err := c.Update(ctx, u); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errCreateResource))
		return nil, nil
	}

	kr, err := model.GetKubernetesResource(u)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errModelResource))
		return nil, nil
	}
	return &model.UpdateKubernetesResourcePayload{Resource: kr}, nil
}

func (r *mutation) DeleteKubernetesResource(ctx context.Context, id model.ReferenceID) (*model.DeleteKubernetesResourcePayload, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	u := &unstructured.Unstructured{}
	u.SetAPIVersion(id.APIVersion)
	u.SetKind(id.Kind)
	u.SetNamespace(id.Namespace)
	u.SetName(id.Name)
	if err := c.Delete(ctx, u); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errCreateResource))
		return nil, nil
	}

	kr, err := model.GetKubernetesResource(u)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errModelResource))
		return nil, nil
	}
	return &model.DeleteKubernetesResourcePayload{Resource: kr}, nil
}
