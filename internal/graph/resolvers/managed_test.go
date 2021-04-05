package resolvers

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/gqlerror"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/clients"
	"github.com/upbound/xgql/internal/graph/generated"
	"github.com/upbound/xgql/internal/graph/model"
)

var _ generated.ManagedResourceSpecResolver = &managedResourceSpec{}

func TestManagedResourceSpecConnectionSecret(t *testing.T) {
	errBoom := errors.New("boom")

	gsec := model.GetSecret(&corev1.Secret{})

	type args struct {
		ctx context.Context
		obj *model.ManagedResourceSpec
	}
	type want struct {
		sec  *model.Secret
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"NoOp": {
			reason: "If there is no connection secret we should return early.",
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ManagedResourceSpec{},
			},
			want: want{},
		},
		"GetClientError": {
			reason: "If we can't get a client we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, errBoom
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ManagedResourceSpec{
					WritesConnectionSecretToRef: &xpv1.SecretReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"GetSecretError": {
			reason: "If we can't get the secret we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ManagedResourceSpec{
					WritesConnectionSecretToRef: &xpv1.SecretReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetSecret).Error()),
				},
			},
		},
		"Success": {
			reason: "If we can get and model the secret we should return it.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ManagedResourceSpec{
					WritesConnectionSecretToRef: &xpv1.SecretReference{},
				},
			},
			want: want{
				sec: &gsec,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &managedResourceSpec{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := s.ConnectionSecret(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.ConnectionSecret(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.ConnectionSecret(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.sec, got); diff != "" {
				t.Errorf("\n%s\ns.ConnectionSecret(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
