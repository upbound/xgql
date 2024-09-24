// Copyright 2021 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resolvers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/vektah/gqlparser/v2/gqlerror"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/clients"
	"github.com/upbound/xgql/internal/graph/generated"
	"github.com/upbound/xgql/internal/graph/model"
)

var _ generated.MutationResolver = &mutation{}

func TestIsRetriable(t *testing.T) {
	cases := map[string]struct {
		err  error
		want bool
	}{
		"Timeout": {
			err:  kerrors.NewTimeoutError("", 0),
			want: true,
		},
		"ServerTimeout": {
			err:  kerrors.NewServerTimeout(schema.GroupResource{}, "", 0),
			want: true,
		},
		"InternalError": {
			err:  kerrors.NewInternalError(errors.New("boom")),
			want: true,
		},
		"TooManyRequests": {
			err:  kerrors.NewTooManyRequests("", 0),
			want: true,
		},
		"UnexpectedServerError": {
			err:  kerrors.NewGenericServerResponse(http.StatusTeapot, "GET", schema.GroupResource{}, "", "", 0, true),
			want: true,
		},
		"UnknownReason": {
			err:  errors.New("boom"),
			want: true,
		},
		"NotFound": {
			err:  kerrors.NewNotFound(schema.GroupResource{}, ""),
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsRetriable(tc.err)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("IsRetriable(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestCreateKubernetesResource(t *testing.T) {
	errBoom := errors.New("boom")
	errFieldPath := fieldpath.Pave(map[string]interface{}{}).SetValue("..", nil)
	errUnmarshal := json.Unmarshal([]byte("\""), nil) //nolint:govet

	// Unmarshalling to an *interface{} results in a slightly different error.
	var v interface{}
	errUnmarshalPatch := json.Unmarshal([]byte("\""), &v)

	u := &unstructured.Unstructured{}
	u.SetAPIVersion("example.org/v1")
	u.SetKind("Example")
	u.SetName("example")
	uj, _ := json.Marshal(u)

	kr, _ := model.GetKubernetesResource(u)

	type args struct {
		ctx   context.Context
		input model.CreateKubernetesResourceInput
	}
	type want struct {
		payload model.CreateKubernetesResourcePayload
		err     error
		errs    gqlerror.List
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
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
				},
			},
		},
		"UnmarshalUnstructuredError": {
			reason: "If we can't get unmarshal the unstructured input we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				input: model.CreateKubernetesResourceInput{
					Unstructured: []byte("\""),
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errUnmarshal, errUnmarshalUnstructured)),
				},
			},
		},
		"UnmarshalPatchError": {
			reason: "If we can't get unmarshal an unstructured patch we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				input: model.CreateKubernetesResourceInput{
					Unstructured: uj,
					Patches: []model.Patch{{
						Unstructured: []byte("\""),
					}},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrapf(errUnmarshalPatch, errFmtUnmarshalPatch, 0)),
				},
			},
		},
		"SetPatchValueError": {
			reason: "If we can't apply a patch we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				input: model.CreateKubernetesResourceInput{
					Unstructured: uj,
					Patches: []model.Patch{{
						FieldPath:    "..", // An invalid field path.
						Unstructured: []byte("{}"),
					}},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrapf(errFieldPath, errFmtPatch, 0)),
				},
			},
		},
		"CreateError": {
			reason: "If we can't create a Kubernetes resource we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockCreate: test.NewMockCreateFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				input: model.CreateKubernetesResourceInput{
					Unstructured: uj,
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errCreateResource)),
				},
			},
		},
		"Success": {
			reason: "If we successfully create a Kubernetes resource we should model and return it.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockCreate: test.NewMockCreateFn(nil),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				input: model.CreateKubernetesResourceInput{
					Unstructured: uj,
				},
			},
			want: want{
				payload: model.CreateKubernetesResourcePayload{
					Resource: kr,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := &mutation{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := m.CreateKubernetesResource(tc.args.ctx, tc.args.input)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.CreateKubernetesResource(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.CreateKubernetesResource(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.payload, got, cmpopts.IgnoreFields(model.GenericResource{}, "PavedAccess"), cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\ns.CreateKubernetesResource(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestUpdateKubernetesResource(t *testing.T) {
	errBoom := errors.New("boom")
	errFieldPath := fieldpath.Pave(map[string]interface{}{}).SetValue("..", nil)
	errUnmarshal := json.Unmarshal([]byte("\""), nil) //nolint:govet

	// Unmarshalling to an *interface{} results in a slightly different error.
	var v interface{}
	errUnmarshalPatch := json.Unmarshal([]byte("\""), &v)

	u := &unstructured.Unstructured{}
	u.SetAPIVersion("example.org/v1")
	u.SetKind("Example")
	u.SetName("example")
	uj, _ := json.Marshal(u)

	kr, _ := model.GetKubernetesResource(u)

	type args struct {
		ctx   context.Context
		id    model.ReferenceID
		input model.UpdateKubernetesResourceInput
	}
	type want struct {
		payload model.UpdateKubernetesResourcePayload
		err     error
		errs    gqlerror.List
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
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
				},
			},
		},
		"UnmarshalUnstructuredError": {
			reason: "If we can't get unmarshal the unstructured input we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				input: model.UpdateKubernetesResourceInput{
					Unstructured: []byte("\""),
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errUnmarshal, errUnmarshalUnstructured)),
				},
			},
		},
		"UnmarshalPatchError": {
			reason: "If we can't get unmarshal an unstructured patch we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				input: model.UpdateKubernetesResourceInput{
					Unstructured: uj,
					Patches: []model.Patch{{
						Unstructured: []byte("\""),
					}},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrapf(errUnmarshalPatch, errFmtUnmarshalPatch, 0)),
				},
			},
		},
		"SetPatchValueError": {
			reason: "If we can't apply a patch we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				input: model.UpdateKubernetesResourceInput{
					Unstructured: uj,
					Patches: []model.Patch{{
						FieldPath:    "..", // An invalid field path.
						Unstructured: []byte("{}"),
					}},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrapf(errFieldPath, errFmtPatch, 0)),
				},
			},
		},
		"UpdateError": {
			reason: "If we can't update a Kubernetes resource we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				input: model.UpdateKubernetesResourceInput{
					Unstructured: uj,
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errUpdateResource)),
				},
			},
		},
		"Success": {
			reason: "If we successfully update a Kubernetes resource we should model and return it.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				id: model.ReferenceID{
					APIVersion: u.GetAPIVersion(),
					Kind:       u.GetKind(),
					Namespace:  u.GetNamespace(),
					Name:       u.GetName(),
				},
				input: model.UpdateKubernetesResourceInput{
					Unstructured: uj,
				},
			},
			want: want{
				payload: model.UpdateKubernetesResourcePayload{
					Resource: kr,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := &mutation{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := m.UpdateKubernetesResource(tc.args.ctx, tc.args.id, tc.args.input)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.UpdateKubernetesResource(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.UpdateKubernetesResource(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.payload, got, cmpopts.IgnoreFields(model.GenericResource{}, "PavedAccess"), cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\ns.UpdateKubernetesResource(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestDeleteKubernetesResource(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		ctx context.Context
		id  model.ReferenceID
	}
	type want struct {
		payload model.DeleteKubernetesResourcePayload
		err     error
		errs    gqlerror.List
	}
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("example.org/v1")
	u.SetKind("Example")
	u.SetName("example")

	kr, _ := model.GetKubernetesResource(u)

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
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
				},
			},
		},

		"DeleteError": {
			reason: "If we can't update a Kubernetes resource we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockDelete: test.NewMockDeleteFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errDeleteResource)),
				},
			},
		},
		"Success": {
			reason: "If we successfully update a Kubernetes resource we should model and return it.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockDelete: test.NewMockDeleteFn(nil),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				id: model.ReferenceID{
					APIVersion: u.GetAPIVersion(),
					Kind:       u.GetKind(),
					Namespace:  u.GetNamespace(),
					Name:       u.GetName(),
				},
			},
			want: want{
				payload: model.DeleteKubernetesResourcePayload{
					Resource: kr,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := &mutation{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := m.DeleteKubernetesResource(tc.args.ctx, tc.args.id)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.DeleteKubernetesResource(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.DeleteKubernetesResource(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.payload, got, cmpopts.IgnoreFields(model.GenericResource{}, "PavedAccess"), cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\ns.DeleteKubernetesResource(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
