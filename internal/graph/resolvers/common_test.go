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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/clients"
	"github.com/upbound/xgql/internal/graph/generated"
	"github.com/upbound/xgql/internal/graph/model"
)

var (
	_ generated.GenericResourceResolver          = &genericResource{}
	_ generated.SecretResolver                   = &secret{}
	_ generated.ConfigMapResolver                = &configMap{}
	_ generated.CustomResourceDefinitionResolver = &crd{}

	crdAPIVersion = "apiextensions.k8s.io/v1"
	crdKind       = "CustomResourceDefinition"
)

func TestCRDDefinedResources(t *testing.T) {
	errBoom := errors.New("boom")

	gr := unstructured.Unstructured{}
	ggr := model.GetGenericResource(&gr)

	group := "example.org"
	version := "v1"
	kind := "Example"

	// In almost all real cases this would be 'ExampleList', but we infer that
	// when ListKind is not set, and want to test that this will override it.
	listKind := "Examples"

	type args struct {
		ctx     context.Context
		obj     *model.CustomResourceDefinition
		version *string
	}
	type want struct {
		krc  model.KubernetesResourceConnection
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
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
				},
			},
		},
		"ListDefinedResourcesError": {
			reason: "If we can't list defined resources we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CustomResourceDefinition{
					Spec: model.CustomResourceDefinitionSpec{
						Group: group,
						Names: model.CustomResourceDefinitionNames{Kind: kind},
					},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errListResources)),
				},
			},
		},
		"InferServedVersion": {
			reason: "We should successfully infer the served version (if none is referenceable) and return any defined resources we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						u := *obj.(*unstructured.UnstructuredList)

						// Ensure we're being asked to list the expected GVK.
						got := u.GetObjectKind().GroupVersionKind()
						want := schema.GroupVersionKind{Group: group, Version: version, Kind: listKind}
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("-want GVK, +got GVK:\n%s", diff)
						}

						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{gr}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CustomResourceDefinition{
					Spec: model.CustomResourceDefinitionSpec{
						Group: group,
						Names: model.CustomResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CustomResourceDefinitionVersion{
							// This version should be ignored because it is
							// neither referenceable nor served.
							{
								Name: "v3",
							},
							{
								Name:   version,
								Served: true,
							},
						},
					},
				},
			},
			want: want{
				krc: model.KubernetesResourceConnection{
					Nodes:      []model.KubernetesResource{ggr},
					TotalCount: 1,
				},
			},
		},
		"SpecificVersion": {
			reason: "We should successfully return any defined resources of the requested version that we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						u := *obj.(*unstructured.UnstructuredList)

						// Ensure we're being asked to list the expected GVK.
						got := u.GetObjectKind().GroupVersionKind()
						want := schema.GroupVersionKind{Group: group, Version: version, Kind: listKind}
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("-want GVK, +got GVK:\n%s", diff)
						}

						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{gr}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CustomResourceDefinition{
					Spec: model.CustomResourceDefinitionSpec{
						Group: group,
						Names: model.CustomResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CustomResourceDefinitionVersion{
							// Normally we'd pick this version first, but in
							// this case the caller asked us to list a specific
							// version.
							{
								Name:   "v2",
								Served: true,
							},
							{
								Name:   version,
								Served: true,
							},
						},
					},
				},
				version: ptr.To(version),
			},
			want: want{
				krc: model.KubernetesResourceConnection{
					Nodes:      []model.KubernetesResource{ggr},
					TotalCount: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			x := &crd{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := x.DefinedResources(tc.args.ctx, tc.args.obj, tc.args.version)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.DefinedResources(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.DefinedResources(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.krc, got, cmpopts.IgnoreUnexported(model.ObjectMeta{}, fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\nq.DefinedResources(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
