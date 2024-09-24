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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/clients"
	"github.com/upbound/xgql/internal/graph/generated"
	"github.com/upbound/xgql/internal/graph/model"
)

var (
	_ generated.CompositeResourceDefinitionResolver     = &xrd{}
	_ generated.CompositeResourceDefinitionSpecResolver = &xrdSpec{}
	_ generated.CompositionResolver                     = &composition{}
)

func TestCompositeResourceCrd(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		ctx context.Context
		obj *model.CompositeResourceDefinition
	}
	type want struct {
		crd  *model.CustomResourceDefinition
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
				obj: &model.CompositeResourceDefinition{Spec: model.CompositeResourceDefinitionSpec{Names: model.CompositeResourceDefinitionNames{}}},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
				},
			},
		},
		"GetError": {
			reason: "If we can't get a CompositeResourceDefinition we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						return errBoom
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{Spec: model.CompositeResourceDefinitionSpec{Names: model.CompositeResourceDefinitionNames{}}},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetCRD)),
				},
			},
		},
		"Success": {
			reason: "Successfully return a CompositeResourceDefinition",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if diff := cmp.Diff(client.ObjectKey{Name: "things.some.group"}, key); diff != "" {
							t.Errorf("\n%s\nmock get key: -want error, +got error:\n%s\n", "Successfully return a CompositeResourceDefinition", diff)
						}
						obj.SetName("some.crd")
						return nil
					},
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{Spec: model.CompositeResourceDefinitionSpec{Group: "some.group", Names: model.CompositeResourceDefinitionNames{Plural: "things"}}},
			},
			want: want{
				crd: &model.CustomResourceDefinition{
					APIVersion: crdAPIVersion,
					Kind:       crdKind,
					ID:         model.ReferenceID{Name: "some.crd", APIVersion: crdAPIVersion, Kind: crdKind},
					Metadata:   model.ObjectMeta{Name: "some.crd"},
					Spec:       model.CustomResourceDefinitionSpec{Names: model.CustomResourceDefinitionNames{}, Versions: []model.CustomResourceDefinitionVersion{}},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			x := &xrd{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := x.CompositeResourceCrd(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.CompositeResourceCrd(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.CompositeResourceCrd(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.crd, got, cmpopts.IgnoreFields(model.CustomResourceDefinition{}, "PavedAccess"), cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nq.CompositeResourceCrd(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourceClaimCrd(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		ctx context.Context
		obj *model.CompositeResourceDefinition
	}
	type want struct {
		crd  *model.CustomResourceDefinition
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"NoClaimNames": {
			reason: "If the XRD has no Names we return nil.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{Spec: model.CompositeResourceDefinitionSpec{}},
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
				obj: &model.CompositeResourceDefinition{Spec: model.CompositeResourceDefinitionSpec{ClaimNames: &model.CompositeResourceDefinitionNames{}}},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
				},
			},
		},
		"GetError": {
			reason: "If we can't get a CompositeResourceDefinition we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						return errBoom
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{Spec: model.CompositeResourceDefinitionSpec{ClaimNames: &model.CompositeResourceDefinitionNames{}}},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetCRD)),
				},
			},
		},
		"Success": {
			reason: "Successfully return a CompositeResourceDefinition",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if diff := cmp.Diff(client.ObjectKey{Name: "things.some.group"}, key); diff != "" {
							t.Errorf("\n%s\nmock get key: -want error, +got error:\n%s\n", "Successfully return a CompositeResourceDefinition", diff)
						}
						obj.SetName("some.crd")
						return nil
					},
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{Spec: model.CompositeResourceDefinitionSpec{Group: "some.group", ClaimNames: &model.CompositeResourceDefinitionNames{Plural: "things"}}},
			},
			want: want{
				crd: &model.CustomResourceDefinition{
					APIVersion: crdAPIVersion,
					Kind:       crdKind,
					ID:         model.ReferenceID{Name: "some.crd", APIVersion: crdAPIVersion, Kind: crdKind},
					Metadata:   model.ObjectMeta{Name: "some.crd"},
					Spec:       model.CustomResourceDefinitionSpec{Names: model.CustomResourceDefinitionNames{}, Versions: []model.CustomResourceDefinitionVersion{}},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			x := &xrd{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := x.CompositeResourceClaimCrd(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.CompositeResourceClaimCrd(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.CompositeResourceClaimCrd(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.crd, got, cmpopts.IgnoreFields(model.CustomResourceDefinition{}, "PavedAccess"), cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\nq.CompositeResourceClaimCrd(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestXRDDefinedCompositeResources(t *testing.T) {
	errBoom := errors.New("boom")

	xr := unstructured.Unstructured{}
	gxr := model.GetCompositeResource(&xr)
	xrReady := unstructured.Unstructured{Object: map[string]interface{}{}}
	fieldpath.Pave(xrReady.Object).SetValue("status.conditions", []xpv1.Condition{{Type: xpv1.TypeReady, Status: corev1.ConditionTrue}})
	gxrReady := model.GetCompositeResource(&xrReady)
	xrNotReady := unstructured.Unstructured{Object: map[string]interface{}{}}
	fieldpath.Pave(xrNotReady.Object).SetValue("status.conditions", []xpv1.Condition{{Type: xpv1.TypeReady, Status: corev1.ConditionFalse}})
	gxrNotReady := model.GetCompositeResource(&xrNotReady)
	xrReadyUnknown := unstructured.Unstructured{Object: map[string]interface{}{}}
	fieldpath.Pave(xrReadyUnknown.Object).SetValue("status.conditions", []xpv1.Condition{{Type: xpv1.TypeReady, Status: corev1.ConditionUnknown}})
	gxrReadyUnknown := model.GetCompositeResource(&xrReadyUnknown)

	group := "example.org"
	version := "v1"
	kind := "Example"

	// In almost all real cases this would be 'ExampleList', but we infer that
	// when ListKind is not set, and want to test that this will override it.
	listKind := "Examples"

	type args struct {
		ctx     context.Context
		obj     *model.CompositeResourceDefinition
		version *string
		options *model.DefinedCompositeResourceOptionsInput
	}
	type want struct {
		crc  model.CompositeResourceConnection
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
		"ListDefinedCompositeResourcesError": {
			reason: "If we can't list defined resources we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						Names: model.CompositeResourceDefinitionNames{Kind: kind},
					},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errListResources)),
				},
			},
		},
		"InferReferencableVersion": {
			reason: "We should successfully infer the referencable version and return any defined resources we can list and model.",
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

						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xr}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						Names: model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
							{
								// This version appears first, but we should
								// ignore it in favor of the referencable one.
								Name:   "v2",
								Served: true,
							},
							{
								Name:          version,
								Referenceable: true,
							},
						},
					},
				},
			},
			want: want{
				crc: model.CompositeResourceConnection{
					Nodes:      []model.CompositeResource{gxr},
					TotalCount: 1,
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

						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xr}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						Names: model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
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
				crc: model.CompositeResourceConnection{
					Nodes:      []model.CompositeResource{gxr},
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

						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xr}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						Names: model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
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
				options: &model.DefinedCompositeResourceOptionsInput{Version: ptr.To(version)},
			},
			want: want{
				crc: model.CompositeResourceConnection{
					Nodes:      []model.CompositeResource{gxr},
					TotalCount: 1,
				},
			},
		},
		"SpecificVersionDeprecated": {
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

						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xr}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						Names: model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
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
				crc: model.CompositeResourceConnection{
					Nodes:      []model.CompositeResource{gxr},
					TotalCount: 1,
				},
			},
		},
		"SpecificVersionPerferNonDeprecated": {
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

						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xr}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						Names: model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
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
				options: &model.DefinedCompositeResourceOptionsInput{Version: ptr.To(version)},
				version: ptr.To("v2"),
			},
			want: want{
				crc: model.CompositeResourceConnection{
					Nodes:      []model.CompositeResource{gxr},
					TotalCount: 1,
				},
			},
		},
		"ReadyNull": {
			reason: "We should successfully return any defined claims of any ready status",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xr, xrNotReady, xrReady, xrReadyUnknown}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						Names: model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
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
				options: &model.DefinedCompositeResourceOptionsInput{Ready: nil},
			},
			want: want{
				crc: model.CompositeResourceConnection{
					Nodes:      []model.CompositeResource{gxr, gxrNotReady, gxrReady, gxrReadyUnknown},
					TotalCount: 4,
				},
			},
		},
		"ReadyFalse": {
			reason: "We should successfully return any defined claims that are not ready",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xr, xrNotReady, xrReady, xrReadyUnknown}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						Names: model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
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
				options: &model.DefinedCompositeResourceOptionsInput{Ready: ptr.To(false)},
			},
			want: want{
				crc: model.CompositeResourceConnection{
					Nodes:      []model.CompositeResource{gxr, gxrNotReady, gxrReadyUnknown},
					TotalCount: 3,
				},
			},
		},
		"ReadyTrue": {
			reason: "We should successfully return any defined claims that are ready",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xr, xrNotReady, xrReady, xrReadyUnknown}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						Names: model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
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
				options: &model.DefinedCompositeResourceOptionsInput{Ready: ptr.To(true)},
			},
			want: want{
				crc: model.CompositeResourceConnection{
					Nodes:      []model.CompositeResource{gxrReady},
					TotalCount: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			x := &xrd{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := x.DefinedCompositeResources(tc.args.ctx, tc.args.obj, tc.args.version, tc.args.options)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.DefinedCompositeResources(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.DefinedCompositeResources(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.crc, got,
				cmpopts.IgnoreUnexported(model.ObjectMeta{}),
				cmpopts.IgnoreFields(model.CompositeResource{}, "PavedAccess"),
			); diff != "" {
				t.Errorf("\n%s\nq.DefinedCompositeResources(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestXRDDefinedCompositeResourceClaims(t *testing.T) {
	errBoom := errors.New("boom")

	xrc := unstructured.Unstructured{}
	gxrc := model.GetCompositeResourceClaim(&xrc)
	xrcReady := unstructured.Unstructured{Object: map[string]interface{}{}}
	fieldpath.Pave(xrcReady.Object).SetValue("status.conditions", []xpv1.Condition{{Type: xpv1.TypeReady, Status: corev1.ConditionTrue}})
	gxrcReady := model.GetCompositeResourceClaim(&xrcReady)
	xrcNotReady := unstructured.Unstructured{Object: map[string]interface{}{}}
	fieldpath.Pave(xrcNotReady.Object).SetValue("status.conditions", []xpv1.Condition{{Type: xpv1.TypeReady, Status: corev1.ConditionFalse}})
	gxrcNotReady := model.GetCompositeResourceClaim(&xrcNotReady)
	xrcReadyUnknown := unstructured.Unstructured{Object: map[string]interface{}{}}
	fieldpath.Pave(xrcReadyUnknown.Object).SetValue("status.conditions", []xpv1.Condition{{Type: xpv1.TypeReady, Status: corev1.ConditionUnknown}})
	gxrcReadyUnknown := model.GetCompositeResourceClaim(&xrcReadyUnknown)

	group := "example.org"
	version := "v1"
	kind := "Example"

	// In almost all real cases this would be 'ExampleList', but we infer that
	// when ListKind is not set, and want to test that this will override it.
	listKind := "Examples"

	type args struct {
		ctx       context.Context
		obj       *model.CompositeResourceDefinition
		version   *string
		namespace *string
		options   *model.DefinedCompositeResourceClaimOptionsInput
	}
	type want struct {
		crcc model.CompositeResourceClaimConnection
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
			reason: "We should return early if this XRD doesn't offer a claim,",
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{},
				},
			},
			want: want{
				crcc: model.CompositeResourceClaimConnection{},
			},
		},
		"GetClientError": {
			reason: "If we can't get a client we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, errBoom
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						ClaimNames: &model.CompositeResourceDefinitionNames{},
					},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
				},
			},
		},
		"ListDefinedCompositeResourceClaimsError": {
			reason: "If we can't list defined claims we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group:      group,
						ClaimNames: &model.CompositeResourceDefinitionNames{Kind: kind},
					},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errListResources)),
				},
			},
		},
		"InferReferencableVersion": {
			reason: "We should successfully infer the referencable version and return any defined claims we can list and model.",
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

						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xrc}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						ClaimNames: &model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
							{
								// This version appears first, but we should
								// ignore it in favor of the referencable one.
								Name:   "v2",
								Served: true,
							},
							{
								Name:          version,
								Referenceable: true,
							},
						},
					},
				},
			},
			want: want{
				crcc: model.CompositeResourceClaimConnection{
					Nodes:      []model.CompositeResourceClaim{gxrc},
					TotalCount: 1,
				},
			},
		},
		"InferServedVersion": {
			reason: "We should successfully infer the served version (if none is referenceable) and return any defined claims we can list and model.",
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

						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xrc}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						ClaimNames: &model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
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
				crcc: model.CompositeResourceClaimConnection{
					Nodes:      []model.CompositeResourceClaim{gxrc},
					TotalCount: 1,
				},
			},
		},
		"SpecificVersion": {
			reason: "We should successfully return any defined claims of the requested version that we can list and model.",
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

						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xrc}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						ClaimNames: &model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
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
				options: &model.DefinedCompositeResourceClaimOptionsInput{Version: ptr.To(version)},
			},
			want: want{
				crcc: model.CompositeResourceClaimConnection{
					Nodes:      []model.CompositeResourceClaim{gxrc},
					TotalCount: 1,
				},
			},
		},
		"SpecificVersionDeprecated": {
			reason: "We should successfully return any defined claims of the requested deprecated version that we can list and model.",
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

						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xrc}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						ClaimNames: &model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
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
				crcc: model.CompositeResourceClaimConnection{
					Nodes:      []model.CompositeResourceClaim{gxrc},
					TotalCount: 1,
				},
			},
		},
		"SpecificVersionPreferNonDeprecated": {
			reason: "We should successfully return any defined claims of the requested version ignoring deprecated version that we can list and model.",
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

						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xrc}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						ClaimNames: &model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
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
				options: &model.DefinedCompositeResourceClaimOptionsInput{Version: ptr.To(version)},
				version: ptr.To("v2"),
			},
			want: want{
				crcc: model.CompositeResourceClaimConnection{
					Nodes:      []model.CompositeResourceClaim{gxrc},
					TotalCount: 1,
				},
			},
		},
		"Namespace": {
			reason: "We should successfully return any defined claims in namespace that we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
						u := *list.(*unstructured.UnstructuredList)

						if diff := cmp.Diff(client.InNamespace("some-namespace"), opts[0]); diff != "" {
							t.Errorf("-want, +got Namespace:\n%s", diff)
						}

						// Ensure we're being asked to list the expected GVK.
						got := u.GetObjectKind().GroupVersionKind()
						want := schema.GroupVersionKind{Group: group, Version: version, Kind: listKind}
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("-want GVK, +got GVK:\n%s", diff)
						}

						*list.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xrc}}
						return nil
					},
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						ClaimNames: &model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
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
				options: &model.DefinedCompositeResourceClaimOptionsInput{Version: ptr.To(version), Namespace: ptr.To("some-namespace")},
			},
			want: want{
				crcc: model.CompositeResourceClaimConnection{
					Nodes:      []model.CompositeResourceClaim{gxrc},
					TotalCount: 1,
				},
			},
		},
		"NamespaceDeprecated": {
			reason: "We should successfully return any defined claims in deprecated namespace that we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
						u := *list.(*unstructured.UnstructuredList)

						if diff := cmp.Diff(client.InNamespace("some-namespace"), opts[0]); diff != "" {
							t.Errorf("-want, +got Namespace:\n%s", diff)
						}

						// Ensure we're being asked to list the expected GVK.
						got := u.GetObjectKind().GroupVersionKind()
						want := schema.GroupVersionKind{Group: group, Version: version, Kind: listKind}
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("-want GVK, +got GVK:\n%s", diff)
						}

						*list.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xrc}}
						return nil
					},
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						ClaimNames: &model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
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
				options:   &model.DefinedCompositeResourceClaimOptionsInput{Version: ptr.To(version)},
				namespace: ptr.To("some-namespace"),
			},
			want: want{
				crcc: model.CompositeResourceClaimConnection{
					Nodes:      []model.CompositeResourceClaim{gxrc},
					TotalCount: 1,
				},
			},
		},
		"NamespacePreferNonDeprecated": {
			reason: "We should successfully return any defined claims in namespace ignoring deprecated namespace that we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
						u := *list.(*unstructured.UnstructuredList)

						if diff := cmp.Diff(client.InNamespace("some-namespace"), opts[0]); diff != "" {
							t.Errorf("-want, +got Namespace:\n%s", diff)
						}

						// Ensure we're being asked to list the expected GVK.
						got := u.GetObjectKind().GroupVersionKind()
						want := schema.GroupVersionKind{Group: group, Version: version, Kind: listKind}
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("-want GVK, +got GVK:\n%s", diff)
						}

						*list.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xrc}}
						return nil
					},
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						ClaimNames: &model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
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
				options:   &model.DefinedCompositeResourceClaimOptionsInput{Version: ptr.To(version), Namespace: ptr.To("some-namespace")},
				namespace: ptr.To("some-other-namespace"),
			},
			want: want{
				crcc: model.CompositeResourceClaimConnection{
					Nodes:      []model.CompositeResourceClaim{gxrc},
					TotalCount: 1,
				},
			},
		},
		"ReadyNull": {
			reason: "We should successfully return any defined claims of any ready status",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xrc, xrcNotReady, xrcReady, xrcReadyUnknown}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						ClaimNames: &model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
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
				options: &model.DefinedCompositeResourceClaimOptionsInput{Ready: nil},
			},
			want: want{
				crcc: model.CompositeResourceClaimConnection{
					Nodes:      []model.CompositeResourceClaim{gxrc, gxrcNotReady, gxrcReady, gxrcReadyUnknown},
					TotalCount: 4,
				},
			},
		},
		"ReadyFalse": {
			reason: "We should successfully return any defined claims that are not ready",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xrc, xrcNotReady, xrcReady, xrcReadyUnknown}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						ClaimNames: &model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
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
				options: &model.DefinedCompositeResourceClaimOptionsInput{Ready: ptr.To(false)},
			},
			want: want{
				crcc: model.CompositeResourceClaimConnection{
					Nodes:      []model.CompositeResourceClaim{gxrc, gxrcNotReady, gxrcReadyUnknown},
					TotalCount: 3,
				},
			},
		},
		"ReadyTrue": {
			reason: "We should successfully return any defined claims that are ready",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*unstructured.UnstructuredList) = unstructured.UnstructuredList{Items: []unstructured.Unstructured{xrc, xrcNotReady, xrcReady, xrcReadyUnknown}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinition{
					Spec: model.CompositeResourceDefinitionSpec{
						Group: group,
						ClaimNames: &model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: ptr.To(listKind),
						},
						Versions: []model.CompositeResourceDefinitionVersion{
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
				options: &model.DefinedCompositeResourceClaimOptionsInput{Ready: ptr.To(true)},
			},
			want: want{
				crcc: model.CompositeResourceClaimConnection{
					Nodes:      []model.CompositeResourceClaim{gxrcReady},
					TotalCount: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			x := &xrd{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := x.DefinedCompositeResourceClaims(tc.args.ctx, tc.args.obj, tc.args.version, tc.args.namespace, tc.args.options)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.DefinedCompositeResourceClaims(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.DefinedCompositeResourceClaims(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.crcc, got,
				cmpopts.IgnoreUnexported(model.ObjectMeta{}),
				cmpopts.IgnoreFields(model.CompositeResourceClaim{}, "PavedAccess"),
			); diff != "" {
				t.Errorf("\n%s\nq.DefinedCompositeResourceClaims(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourceDefinitionSpecDefaultComposition(t *testing.T) {
	errBoom := errors.New("boom")

	gcmp := model.GetComposition(&extv1.Composition{})

	type args struct {
		ctx context.Context
		obj *model.CompositeResourceDefinitionSpec
	}
	type want struct {
		cmp  *model.Composition
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
			reason: "If there is no composition we should return early.",
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinitionSpec{},
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
				obj: &model.CompositeResourceDefinitionSpec{
					DefaultCompositionReference: &extv1.CompositionReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
				},
			},
		},
		"GetDefaultCompositionError": {
			reason: "If we can't get the composition we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinitionSpec{
					DefaultCompositionReference: &extv1.CompositionReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetComposition)),
				},
			},
		},
		"Success": {
			reason: "If we can get and model the composition we should return it.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinitionSpec{
					DefaultCompositionReference: &extv1.CompositionReference{},
				},
			},
			want: want{
				cmp: &gcmp,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &xrdSpec{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := s.DefaultComposition(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.DefaultComposition(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.DefaultComposition(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cmp, got, cmpopts.IgnoreUnexported(model.ObjectMeta{}, fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\ns.DefaultComposition(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourceDefinitionSpecEnforcedComposition(t *testing.T) {
	errBoom := errors.New("boom")

	gcmp := model.GetComposition(&extv1.Composition{})

	type args struct {
		ctx context.Context
		obj *model.CompositeResourceDefinitionSpec
	}
	type want struct {
		cmp  *model.Composition
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
			reason: "If there is no composition we should return early.",
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinitionSpec{},
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
				obj: &model.CompositeResourceDefinitionSpec{
					EnforcedCompositionReference: &extv1.CompositionReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
				},
			},
		},
		"GetEnforcedCompositionError": {
			reason: "If we can't get the composition we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinitionSpec{
					EnforcedCompositionReference: &extv1.CompositionReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetComposition)),
				},
			},
		},
		"Success": {
			reason: "If we can get and model the composition we should return it.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceDefinitionSpec{
					EnforcedCompositionReference: &extv1.CompositionReference{},
				},
			},
			want: want{
				cmp: &gcmp,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &xrdSpec{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := s.EnforcedComposition(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.EnforcedComposition(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.EnforcedComposition(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cmp, got, cmpopts.IgnoreUnexported(model.ObjectMeta{}, fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\ns.EnforcedComposition(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
