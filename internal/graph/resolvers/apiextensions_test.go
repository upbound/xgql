package resolvers

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
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

func TestXRDDefinedCompositeResources(t *testing.T) {
	errBoom := errors.New("boom")

	xr := unstructured.Unstructured{}
	gxr := model.GetCompositeResource(&xr)

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
	}
	type want struct {
		crc  *model.CompositeResourceConnection
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
					Spec: &model.CompositeResourceDefinitionSpec{
						Group: group,
						Names: &model.CompositeResourceDefinitionNames{Kind: kind},
					},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errListResources).Error()),
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
					Spec: &model.CompositeResourceDefinitionSpec{
						Group: group,
						Names: &model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: pointer.StringPtr(listKind),
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
				crc: &model.CompositeResourceConnection{
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
					Spec: &model.CompositeResourceDefinitionSpec{
						Group: group,
						Names: &model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: pointer.StringPtr(listKind),
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
				crc: &model.CompositeResourceConnection{
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
					Spec: &model.CompositeResourceDefinitionSpec{
						Group: group,
						Names: &model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: pointer.StringPtr(listKind),
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
				version: pointer.StringPtr(version),
			},
			want: want{
				crc: &model.CompositeResourceConnection{
					Nodes:      []model.CompositeResource{gxr},
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
			got, err := x.DefinedCompositeResources(tc.args.ctx, tc.args.obj, tc.args.version)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.DefinedCompositeResources(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.DefinedCompositeResources(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.crc, got); diff != "" {
				t.Errorf("\n%s\nq.DefinedCompositeResources(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestXRDDefinedCompositeResourceClaims(t *testing.T) {
	errBoom := errors.New("boom")

	xrc := unstructured.Unstructured{}
	gxrc := model.GetCompositeResourceClaim(&xrc)

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
	}
	type want struct {
		crcc *model.CompositeResourceClaimConnection
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
					Spec: &model.CompositeResourceDefinitionSpec{},
				},
			},
			want: want{
				crcc: &model.CompositeResourceClaimConnection{},
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
					Spec: &model.CompositeResourceDefinitionSpec{
						ClaimNames: &model.CompositeResourceDefinitionNames{},
					},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
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
					Spec: &model.CompositeResourceDefinitionSpec{
						Group:      group,
						ClaimNames: &model.CompositeResourceDefinitionNames{Kind: kind},
					},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errListResources).Error()),
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
					Spec: &model.CompositeResourceDefinitionSpec{
						Group: group,
						ClaimNames: &model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: pointer.StringPtr(listKind),
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
				crcc: &model.CompositeResourceClaimConnection{
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
					Spec: &model.CompositeResourceDefinitionSpec{
						Group: group,
						ClaimNames: &model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: pointer.StringPtr(listKind),
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
				crcc: &model.CompositeResourceClaimConnection{
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
					Spec: &model.CompositeResourceDefinitionSpec{
						Group: group,
						ClaimNames: &model.CompositeResourceDefinitionNames{
							Kind:     kind,
							ListKind: pointer.StringPtr(listKind),
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
				version: pointer.StringPtr(version),
			},
			want: want{
				crcc: &model.CompositeResourceClaimConnection{
					Nodes:      []model.CompositeResourceClaim{gxrc},
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
			got, err := x.DefinedCompositeResourceClaims(tc.args.ctx, tc.args.obj, tc.args.version, tc.args.namespace)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.DefinedCompositeResourceClaims(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.DefinedCompositeResourceClaims(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.crcc, got); diff != "" {
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
					DefaultCompositionReference: &xpv1.Reference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
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
					DefaultCompositionReference: &xpv1.Reference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetComposition).Error()),
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
					DefaultCompositionReference: &xpv1.Reference{},
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
			if diff := cmp.Diff(tc.want.cmp, got); diff != "" {
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
					EnforcedCompositionReference: &xpv1.Reference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
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
					EnforcedCompositionReference: &xpv1.Reference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetComposition).Error()),
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
					EnforcedCompositionReference: &xpv1.Reference{},
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
			if diff := cmp.Diff(tc.want.cmp, got); diff != "" {
				t.Errorf("\n%s\ns.EnforcedComposition(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
