package resolvers

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/gqlerror"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/clients"
	"github.com/upbound/xgql/internal/graph/generated"
	"github.com/upbound/xgql/internal/graph/model"
)

var _ generated.ObjectMetaResolver = &objectMeta{}

func TestObjectMetaOwners(t *testing.T) {
	errBoom := errors.New("boom")

	// The controller
	ctrl := unstructured.Unstructured{}
	ctrl.SetAPIVersion("example.org/v1")
	ctrl.SetKind("TheController")

	// An owner
	own := unstructured.Unstructured{}
	own.SetAPIVersion("example.org/v1")
	own.SetKind("AnOwner")
	gown, _ := model.GetKubernetesResource(&own)

	type args struct {
		ctx context.Context
		obj *model.ObjectMeta
	}
	type want struct {
		oc   *model.OwnerConnection
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
		"GetOwnerError": {
			reason: "If we can't get an owner we should add the error to the GraphQL context and continue.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if k, ok := obj.(interface {
							GetKind() string
						}); ok && k.GetKind() == ctrl.GetKind() {
							return errBoom
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: own.GetAPIVersion(),
							Kind:       own.GetKind(),
						},
						{
							APIVersion: ctrl.GetAPIVersion(),
							Kind:       ctrl.GetKind(),
							Controller: pointer.BoolPtr(true),
						},
					},
				},
			},
			want: want{
				oc: &model.OwnerConnection{
					Nodes:      []model.Owner{{Resource: gown}},
					TotalCount: 1,
				},
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetOwner).Error()),
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := &objectMeta{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := m.Owners(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Owners(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Owners(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.oc, got); diff != "" {
				t.Errorf("\n%s\nq.Owners(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
