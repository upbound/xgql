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
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/vektah/gqlparser/v2/gqlerror"
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/clients"
	"github.com/upbound/xgql/internal/graph/generated"
	"github.com/upbound/xgql/internal/graph/model"
	"github.com/upbound/xgql/internal/unstructured"
)

var _ generated.ProviderConfigResolver = &providerConfig{}

func TestProviderConfigDefinition(t *testing.T) {
	errBoom := errors.New("boom")
	errNotFound := &kerrors.StatusError{
		ErrStatus: metav1.Status{
			Reason: metav1.StatusReasonNotFound,
		},
	}

	crd := unstructured.NewCRD()
	crd.SetSpecGroup("example.org")
	crd.SetSpecNames(kextv1.CustomResourceDefinitionNames{Kind: "Example"})

	crdDifferingPlural := unstructured.NewCRD()
	crdDifferingPlural.SetSpecGroup("example.org")
	crdDifferingPlural.SetSpecNames(kextv1.CustomResourceDefinitionNames{Kind: "Example", Plural: "Examplii"})

	gcrd := model.GetCustomResourceDefinition(crd)
	dcrd := model.GetCustomResourceDefinition(crdDifferingPlural)

	otherGroup := unstructured.NewCRD()
	otherGroup.SetSpecGroup("example.net")
	otherGroup.SetSpecNames(kextv1.CustomResourceDefinitionNames{Kind: "Example"})

	otherKind := unstructured.NewCRD()
	otherKind.SetSpecGroup("example.org")
	otherKind.SetSpecNames(kextv1.CustomResourceDefinitionNames{Kind: "Illustration"})

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
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
				},
			},
		},
		"GetCRDError": {
			reason: "If we can't get the CRD we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ProviderConfig{},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetCRD)),
				},
			},
		},
		"ListCRDsError": {
			reason: "If we can't list CRDs we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						return errNotFound
					}),
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ProviderConfig{},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errListCRDs)),
				},
			},
		},
		"FoundCRD": {
			reason: "If we can get and model the CRD that defines this provider config we should return it.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						*obj.(*kunstructured.Unstructured) = *crd.GetUnstructured()
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ProviderConfig{
					APIVersion: crd.GetSpecGroup() + "/v1",
					Kind:       crd.GetSpecNames().Kind,
				},
			},
			want: want{
				mrd: &gcrd,
			},
		},
		"DifferentPlural": {
			reason: `In the event we get a request for an object whose CRD has 
			a non-predictable plural form, ensure the CRD list contains the 
			expected resource.`,
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						return errNotFound
					}),
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*kunstructured.UnstructuredList) = kunstructured.UnstructuredList{
							Items: []kunstructured.Unstructured{otherGroup.Unstructured, otherKind.Unstructured, crdDifferingPlural.Unstructured},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ProviderConfig{
					APIVersion: crd.GetSpecGroup() + "/v1",
					Kind:       crd.GetSpecNames().Kind,
				},
			},
			want: want{
				mrd: &dcrd,
			},
		},
		"NoCRD": {
			reason: "If we can't get and model the CRD that defines this provider config we should return nil.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						return errNotFound
					}),
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*kunstructured.UnstructuredList) = kunstructured.UnstructuredList{
							Items: []kunstructured.Unstructured{otherGroup.Unstructured, otherKind.Unstructured},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ProviderConfig{
					APIVersion: crd.GetSpecGroup() + "/v1",
					Kind:       crd.GetSpecNames().Kind,
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
			if diff := cmp.Diff(tc.want.mrd, got,
				cmpopts.IgnoreUnexported(model.ObjectMeta{}),
				cmpopts.IgnoreFields(model.CustomResourceDefinition{}, "PavedAccess"),
			); diff != "" {
				t.Errorf("\n%s\ns.Definition(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
