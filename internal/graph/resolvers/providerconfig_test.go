package resolvers

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/gqlerror"
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/clients"
	"github.com/upbound/xgql/internal/graph/generated"
	"github.com/upbound/xgql/internal/graph/model"
)

var _ generated.ProviderConfigResolver = &providerConfig{}

func TestProviderConfigDefinition(t *testing.T) {
	errBoom := errors.New("boom")

	crd := kextv1.CustomResourceDefinition{
		Spec: kextv1.CustomResourceDefinitionSpec{
			Group: "example.org",
			Names: kextv1.CustomResourceDefinitionNames{Kind: "Example"},
		},
	}
	gcrd := model.GetCustomResourceDefinition(&crd)

	otherGroup := kextv1.CustomResourceDefinition{
		Spec: kextv1.CustomResourceDefinitionSpec{
			Group: "example.net",
			Names: kextv1.CustomResourceDefinitionNames{Kind: "Example"},
		},
	}

	otherKind := kextv1.CustomResourceDefinition{
		Spec: kextv1.CustomResourceDefinitionSpec{
			Group: "example.org",
			Names: kextv1.CustomResourceDefinitionNames{Kind: "Illustration"},
		},
	}

	type args struct {
		ctx context.Context
		obj *model.ProviderConfig
	}
	type want struct {
		mrd  model.ProviderConfigDefinition
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
				obj: &model.ProviderConfig{},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"ListCRDsError": {
			reason: "If we can't list CRDs we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ProviderConfig{},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errListCRDs).Error()),
				},
			},
		},
		"FoundCRD": {
			reason: "If we can get and model the CRD that defines this provider config we should return it.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*kextv1.CustomResourceDefinitionList) = kextv1.CustomResourceDefinitionList{
							Items: []kextv1.CustomResourceDefinition{otherGroup, otherKind, crd},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ProviderConfig{
					APIVersion: crd.Spec.Group + "/v1",
					Kind:       crd.Spec.Names.Kind,
				},
			},
			want: want{
				mrd: &gcrd,
			},
		},
		"NoCRD": {
			reason: "If we can't get and model the CRD that defines this provider config we should return nil.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*kextv1.CustomResourceDefinitionList) = kextv1.CustomResourceDefinitionList{
							Items: []kextv1.CustomResourceDefinition{otherGroup, otherKind},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ProviderConfig{
					APIVersion: crd.Spec.Group + "/v1",
					Kind:       crd.Spec.Names.Kind,
				},
			},
			want: want{
				mrd: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pc := &providerConfig{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := pc.Definition(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Definition(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Definition(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.mrd, got, cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\ns.Definition(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
