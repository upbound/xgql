package resolvers

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
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
		graphql.AddError(ctx, errors.Wrap(err, "cannot get client"))
		return nil, nil
	}

	in := &pkgv1.ProviderList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, "cannot list providers"))
		return nil, nil
	}

	lim := getLimit(limit, len(in.Items))
	out := &model.ProviderList{
		Items: make([]model.Provider, 0, lim),
		Count: len(in.Items),
	}

	for i := range in.Items[:lim] {
		p, err := model.GetProvider(&in.Items[i])
		if err != nil {
			graphql.AddError(ctx, errors.Wrap(err, "cannot model provider"))
			continue
		}
		out.Items = append(out.Items, p)
	}

	return out, nil
}

func (r *query) Configurations(ctx context.Context, limit *int) (*model.ConfigurationList, error) {
	t, _ := token.FromContext(ctx)

	c, err := r.clients.Get(t)
	if err != nil {
		graphql.AddError(ctx, errors.Wrap(err, "cannot get client"))
		return nil, nil
	}

	in := &pkgv1.ConfigurationList{}
	if err := c.List(ctx, in); err != nil {
		graphql.AddError(ctx, errors.Wrap(err, "cannot list configurations"))
		return nil, nil
	}

	lim := getLimit(limit, len(in.Items))
	out := &model.ConfigurationList{
		Items: make([]model.Configuration, 0, lim),
		Count: len(in.Items),
	}

	for i := range in.Items[:lim] {
		c, err := model.GetConfiguration(&in.Items[i])
		if err != nil {
			graphql.AddError(ctx, errors.Wrap(err, "cannot model configuration"))
			continue
		}
		out.Items = append(out.Items, c)
	}

	return out, nil
}

func (r *query) CompositeResourceDefinitions(ctx context.Context, limit *int, dangling *bool) (*model.CompositeResourceDefinitionList, error) {
	return nil, nil
}

func (r *query) Compositions(ctx context.Context, limit *int, dangling *bool) (*model.CompositionList, error) {
	return nil, nil
}
