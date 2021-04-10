package resolvers

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/gqlerror"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/clients"
	"github.com/upbound/xgql/internal/graph/generated"
	"github.com/upbound/xgql/internal/graph/model"
)

var _ generated.QueryResolver = &query{}

func TestQueryProviders(t *testing.T) {
	errBoom := errors.New("boom")

	p := pkgv1.Provider{ObjectMeta: metav1.ObjectMeta{Name: "coolprovider"}}
	gp := model.GetProvider(&p)

	type args struct {
		ctx context.Context
	}
	type want struct {
		pc   *model.ProviderConnection
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"GetClientError": {
			reason: "If we can't get a client we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, errBoom
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"ListProvidersError": {
			reason: "If we can't list providers we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errListProviders).Error()),
				},
			},
		},
		"Success": {
			reason: "We should successfully return any providers we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*pkgv1.ProviderList) = pkgv1.ProviderList{Items: []pkgv1.Provider{p}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				pc: &model.ProviderConnection{
					Nodes:      []model.Provider{gp},
					TotalCount: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			q := &query{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := q.Providers(tc.args.ctx)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Providers(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Providers(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.pc, got); diff != "" {
				t.Errorf("\n%s\nq.Providers(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestQueryConfigurations(t *testing.T) {
	errBoom := errors.New("boom")

	c := pkgv1.Configuration{ObjectMeta: metav1.ObjectMeta{Name: "coolconfig"}}
	gc := model.GetConfiguration(&c)

	type args struct {
		ctx context.Context
	}
	type want struct {
		cc   *model.ConfigurationConnection
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"GetClientError": {
			reason: "If we can't get a client we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, errBoom
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"ListConfigurationsError": {
			reason: "If we can't list configurations we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errListConfigs).Error()),
				},
			},
		},
		"Success": {
			reason: "We should successfully return any configurations we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*pkgv1.ConfigurationList) = pkgv1.ConfigurationList{Items: []pkgv1.Configuration{c}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				cc: &model.ConfigurationConnection{
					Nodes:      []model.Configuration{gc},
					TotalCount: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			q := &query{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := q.Configurations(tc.args.ctx)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cc, got); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestQueryCompositeResourceDefinitions(t *testing.T) {
	errBoom := errors.New("boom")

	id := model.ReferenceID{
		APIVersion: pkgv1.ConfigurationRevisionGroupVersionKind.GroupVersion().String(),
		Kind:       pkgv1.ConfigurationRevisionKind,
		Name:       "example",
	}

	owned := extv1.CompositeResourceDefinition{ObjectMeta: metav1.ObjectMeta{
		Name: "coolconfig",
		OwnerReferences: []metav1.OwnerReference{
			// Some spurious owner references that we should ignore.
			{
				APIVersion: "wat",
			},
			{
				APIVersion: id.APIVersion,
				Kind:       "wat",
			},
			{
				APIVersion: id.APIVersion,
				Kind:       id.Kind,
				Name:       "wat",
			},
			// The reference that indicates this XRD is owned by our desired
			// ConfigurationRevision (or a ConfigurationRevision generally).
			{
				APIVersion: id.APIVersion,
				Kind:       id.Kind,
				Name:       id.Name,
			},
		},
	}}
	gowned := model.GetCompositeResourceDefinition(&owned)

	dangler := extv1.CompositeResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "coolconfig"}}
	gdangler := model.GetCompositeResourceDefinition(&dangler)

	type args struct {
		ctx      context.Context
		revision *model.ReferenceID
		dangling *bool
	}
	type want struct {
		xrdc *model.CompositeResourceDefinitionConnection
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"GetClientError": {
			reason: "If we can't get a client we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, errBoom
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"ListXRDsError": {
			reason: "If we can't list XRDs we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errListConfigs).Error()),
				},
			},
		},
		"AllXRDs": {
			reason: "We should successfully return all XRDs we can list and model when no arguments are supplied.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*extv1.CompositeResourceDefinitionList) = extv1.CompositeResourceDefinitionList{
							Items: []extv1.CompositeResourceDefinition{
								dangler,
								owned,
							},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				xrdc: &model.CompositeResourceDefinitionConnection{
					Nodes: []model.CompositeResourceDefinition{
						gdangler,
						gowned,
					},
					TotalCount: 2,
				},
			},
		},
		"DanglingXRDs": {
			reason: "We should successfully return dangling XRDs we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*extv1.CompositeResourceDefinitionList) = extv1.CompositeResourceDefinitionList{
							Items: []extv1.CompositeResourceDefinition{
								dangler,
								owned,
							},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx:      graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				dangling: pointer.BoolPtr(true),
			},
			want: want{
				xrdc: &model.CompositeResourceDefinitionConnection{
					Nodes: []model.CompositeResourceDefinition{
						gdangler,
					},
					TotalCount: 1,
				},
			},
		},
		"OwnedXRDs": {
			reason: "We should successfully return the XRDs we can list and model that are owned by the supplied ID.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*extv1.CompositeResourceDefinitionList) = extv1.CompositeResourceDefinitionList{
							Items: []extv1.CompositeResourceDefinition{
								dangler,
								owned,
							},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx:      graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				revision: &id,
			},
			want: want{
				xrdc: &model.CompositeResourceDefinitionConnection{
					Nodes: []model.CompositeResourceDefinition{
						gowned,
					},
					TotalCount: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			q := &query{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := q.CompositeResourceDefinitions(tc.args.ctx, tc.args.revision, tc.args.dangling)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.xrdc, got); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestQueryCompositions(t *testing.T) {
	errBoom := errors.New("boom")

	id := model.ReferenceID{
		APIVersion: pkgv1.ConfigurationRevisionGroupVersionKind.GroupVersion().String(),
		Kind:       pkgv1.ConfigurationRevisionKind,
		Name:       "example",
	}

	owned := extv1.Composition{ObjectMeta: metav1.ObjectMeta{
		Name: "coolconfig",
		OwnerReferences: []metav1.OwnerReference{
			// Some spurious owner references that we should ignore.
			{
				APIVersion: "wat",
			},
			{
				APIVersion: id.APIVersion,
				Kind:       "wat",
			},
			{
				APIVersion: id.APIVersion,
				Kind:       id.Kind,
				Name:       "wat",
			},
			// The reference that indicates this Composition is owned by our
			// desired ConfigurationRevision (or a ConfigurationRevision
			// generally).
			{
				APIVersion: id.APIVersion,
				Kind:       id.Kind,
				Name:       id.Name,
			},
		},
	}}
	gowned := model.GetComposition(&owned)

	dangler := extv1.Composition{ObjectMeta: metav1.ObjectMeta{Name: "coolconfig"}}
	gdangler := model.GetComposition(&dangler)

	type args struct {
		ctx      context.Context
		revision *model.ReferenceID
		dangling *bool
	}
	type want struct {
		cc   *model.CompositionConnection
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"GetClientError": {
			reason: "If we can't get a client we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, errBoom
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"ListCompositionsError": {
			reason: "If we can't list compositions we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errListConfigs).Error()),
				},
			},
		},
		"AllCompositions": {
			reason: "We should successfully return all compositions we can list and model when no arguments are supplied.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*extv1.CompositionList) = extv1.CompositionList{
							Items: []extv1.Composition{
								dangler,
								owned,
							},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				cc: &model.CompositionConnection{
					Nodes: []model.Composition{
						gdangler,
						gowned,
					},
					TotalCount: 2,
				},
			},
		},
		"DanglingCompositions": {
			reason: "We should successfully return dangling compositions we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*extv1.CompositionList) = extv1.CompositionList{
							Items: []extv1.Composition{
								dangler,
								owned,
							},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx:      graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				dangling: pointer.BoolPtr(true),
			},
			want: want{
				cc: &model.CompositionConnection{
					Nodes: []model.Composition{
						gdangler,
					},
					TotalCount: 1,
				},
			},
		},
		"OwnedCompositions": {
			reason: "We should successfully return the compositions we can list and model that are owned by the supplied ID.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*extv1.CompositionList) = extv1.CompositionList{
							Items: []extv1.Composition{
								dangler,
								owned,
							},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx:      graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				revision: &id,
			},
			want: want{
				cc: &model.CompositionConnection{
					Nodes: []model.Composition{
						gowned,
					},
					TotalCount: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			q := &query{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := q.Compositions(tc.args.ctx, tc.args.revision, tc.args.dangling)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cc, got); diff != "" {
				t.Errorf("\n%s\nq.Configurations(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
