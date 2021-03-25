package resolvers

import (
	"context"

	"github.com/pkg/errors"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/graph/model"
	"github.com/upbound/xgql/internal/token"
)

type query struct {
	clients ClientCache
}

func (r *query) Providers(ctx context.Context, limit *int) (*model.ProviderList, error) {
	t, _ := token.FromContext(ctx)

	c, err := r.clients.Get(t)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get client")
	}

	in := &pkgv1.ProviderList{}
	if err := c.List(ctx, in); err != nil {
		return nil, errors.Wrap(err, "cannot list providers")
	}

	out := &model.ProviderList{
		Items: make([]model.Provider, getLimit(limit, len(in.Items))),
		Count: len(in.Items),
	}

	for i := range out.Items {
		if out.Items[i], err = model.GetProvider(&in.Items[i]); err != nil {
			return nil, errors.Wrap(err, "cannot model provider")
		}
	}

	return out, nil
}

func (r *query) Configurations(ctx context.Context, limit *int) (*model.ConfigurationList, error) {
	t, _ := token.FromContext(ctx)

	c, err := r.clients.Get(t)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get client")
	}

	in := &pkgv1.ConfigurationList{}
	if err := c.List(ctx, in); err != nil {
		return nil, errors.Wrap(err, "cannot list configurations")
	}

	out := &model.ConfigurationList{
		Items: make([]model.Configuration, getLimit(limit, len(in.Items))),
		Count: len(in.Items),
	}

	for i := range out.Items {
		if out.Items[i], err = model.GetConfiguration(&in.Items[i]); err != nil {
			return nil, errors.Wrap(err, "cannot model configuration")
		}
	}

	return out, nil
}

func (r *query) CompositeResourceDefinitions(ctx context.Context, limit *int, dangling *bool) (*model.CompositeResourceDefinitionList, error) {
	return nil, nil
}

func (r *query) Compositions(ctx context.Context, limit *int, dangling *bool) (*model.CompositionList, error) {
	return nil, nil
}
