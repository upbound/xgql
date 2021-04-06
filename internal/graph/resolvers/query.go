package resolvers

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/graph/model"
)

const (
	errGetClient     = "cannot get client"
	errListProviders = "cannot list providers"
	errListConfigs   = "cannot list configurations"
)

type query struct {
	clients ClientCache
}

func (r *query) Providers(ctx context.Context) (*model.ProviderConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	in := &pkgv1.ProviderList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListProviders))
		return nil, nil
	}

	out := &model.ProviderConnection{
		Nodes:      make([]model.Provider, 0, len(in.Items)),
		TotalCount: len(in.Items),
	}

	for i := range in.Items {
		out.Nodes = append(out.Nodes, model.GetProvider(&in.Items[i]))
	}

	return out, nil
}

func (r *query) Configurations(ctx context.Context) (*model.ConfigurationConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	creds, _ := auth.FromContext(ctx)
	c, err := r.clients.Get(creds)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errGetClient))
		return nil, nil
	}

	in := &pkgv1.ConfigurationList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, errListConfigs))
		return nil, nil
	}

	out := &model.ConfigurationConnection{
		Nodes:      make([]model.Configuration, 0, len(in.Items)),
		TotalCount: len(in.Items),
	}

	for i := range in.Items {
		out.Nodes = append(out.Nodes, model.GetConfiguration(&in.Items[i]))
	}

	return out, nil
}

func (r *query) CompositeResourceDefinitions(ctx context.Context, dangling *bool) (*model.CompositeResourceDefinitionConnection, error) {
	return nil, nil
}

func (r *query) Compositions(ctx context.Context, dangling *bool) (*model.CompositionConnection, error) {
	return nil, nil
}
