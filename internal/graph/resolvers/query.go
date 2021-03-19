package resolvers

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/pointer"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/negz/xgql/internal/graph/model"
	"github.com/negz/xgql/internal/token"
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

	// Trim our input list early to avoid doing more conversions than we must.
	items := in.Items[:getLimit(limit, len(in.Items))]

	out := &model.ProviderList{
		Count: len(in.Items),
		Items: make([]model.Provider, len(items)),
	}

	for i := range items {
		p := items[i] // So we don't take the address of a range variable.

		raw, err := json.Marshal(p)
		if err != nil {
			return nil, errors.Wrap(err, "could not marshal JSON")
		}

		out.Items[i] = model.Provider{
			APIVersion: p.APIVersion,
			Kind:       p.Kind,
			Metadata:   model.GetObjectMeta(&p),
			Spec: &model.ProviderSpec{
				Package:                     p.Spec.Package,
				RevisionActivationPolicy:    model.GetRevisionActivationPolicy(p.Spec.RevisionActivationPolicy),
				RevisionHistoryLimit:        getIntPtr(p.Spec.RevisionHistoryLimit),
				PackagePullPolicy:           model.GetPackagePullPolicy(p.Spec.PackagePullPolicy),
				IgnoreCrossplaneConstraints: p.Spec.IgnoreCrossplaneConstraints,
				SkipDependencyResolution:    p.Spec.SkipDependencyResolution,
				PackagePullSecrets:          p.Spec.PackagePullSecrets,
			},
			Status: &model.ProviderStatus{
				Conditions:        model.GetConditions(p.Status.Conditions),
				CurrentRevision:   pointer.StringPtr(p.Status.CurrentRevision),
				CurrentIdentifier: &p.Status.CurrentIdentifier,
			},
			Raw: string(raw),
		}
	}

	return out, nil
}

func (r *query) Configurations(ctx context.Context, limit *int) (*model.ConfigurationList, error) {
	return nil, nil
}

func (r *query) CompositeResourceDefinitions(ctx context.Context, limit *int, dangling *bool) (*model.CompositeResourceDefinitionList, error) {
	return nil, nil
}

func (r *query) Compositions(ctx context.Context, limit *int, dangling *bool) (*model.CompositionList, error) {
	return nil, nil
}
