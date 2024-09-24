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
	kextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/clients"
	"github.com/upbound/xgql/internal/graph/generated"
	"github.com/upbound/xgql/internal/graph/model"
)

var (
	_ generated.CompositeResourceResolver          = &compositeResource{}
	_ generated.CompositeResourceSpecResolver      = &compositeResourceSpec{}
	_ generated.CompositeResourceClaimResolver     = &compositeResourceClaim{}
	_ generated.CompositeResourceClaimSpecResolver = &compositeResourceClaimSpec{}
)

func TestCompositeResourceDefinition(t *testing.T) {
	errBoom := errors.New("boom")

	xrd := extv1.CompositeResourceDefinition{
		Spec: extv1.CompositeResourceDefinitionSpec{
			Group: "example.org",
			Names: kextv1.CustomResourceDefinitionNames{Kind: "Example"},
		},
	}
	gxrd := model.GetCompositeResourceDefinition(&xrd)

	otherGroup := extv1.CompositeResourceDefinition{
		Spec: extv1.CompositeResourceDefinitionSpec{
			Group: "example.net",
			Names: kextv1.CustomResourceDefinitionNames{Kind: "Example"},
		},
	}

	otherKind := extv1.CompositeResourceDefinition{
		Spec: extv1.CompositeResourceDefinitionSpec{
			Group: "example.org",
			Names: kextv1.CustomResourceDefinitionNames{Kind: "Illustration"},
		},
	}

	type args struct {
		ctx context.Context
		obj *model.CompositeResource
	}
	type want struct {
		xrd  *model.CompositeResourceDefinition
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
				obj: &model.CompositeResource{},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
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
				obj: &model.CompositeResource{},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errListXRDs)),
				},
			},
		},
		"FoundXRD": {
			reason: "If we can get and model the XRD that defines this XR we should return it.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*extv1.CompositeResourceDefinitionList) = extv1.CompositeResourceDefinitionList{
							Items: []extv1.CompositeResourceDefinition{otherGroup, otherKind, xrd},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResource{
					APIVersion: xrd.Spec.Group + "/v1",
					Kind:       xrd.Spec.Names.Kind,
				},
			},
			want: want{
				xrd: &gxrd,
			},
		},
		"NoXRD": {
			reason: "If we can't get and model the XRD that defines this XR we should return nil.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*extv1.CompositeResourceDefinitionList) = extv1.CompositeResourceDefinitionList{
							Items: []extv1.CompositeResourceDefinition{otherGroup, otherKind},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResource{
					APIVersion: xrd.Spec.Group + "/v1",
					Kind:       xrd.Spec.Names.Kind,
				},
			},
			want: want{
				xrd: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xr := &compositeResource{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := xr.Definition(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Definition(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Definition(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.xrd, got, cmpopts.IgnoreUnexported(model.ObjectMeta{}, fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\ns.Definition(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourceSpecComposition(t *testing.T) {
	errBoom := errors.New("boom")
	errNotFound := apierrors.NewNotFound(schema.GroupResource{}, "somename")

	gcmp := model.GetComposition(&extv1.Composition{})

	type args struct {
		ctx context.Context
		obj *model.CompositeResourceSpec
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
				obj: &model.CompositeResourceSpec{},
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
				obj: &model.CompositeResourceSpec{
					CompositionReference: &corev1.ObjectReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
				},
			},
		},
		"GetCompositionError": {
			reason: "If we can't get the composition we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceSpec{
					CompositionReference: &corev1.ObjectReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetComposition)),
				},
			},
		},
		"GetCompositionNotFound": {
			reason: "If the composition is not found we return nil",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errNotFound),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceSpec{
					CompositionReference: &corev1.ObjectReference{},
				},
			},
			want: want{
				cmp: nil,
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
				obj: &model.CompositeResourceSpec{
					CompositionReference: &corev1.ObjectReference{},
				},
			},
			want: want{
				cmp: &gcmp,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &compositeResourceSpec{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := s.Composition(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Composition(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Composition(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cmp, got, cmpopts.IgnoreUnexported(model.ObjectMeta{}, fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\ns.Composition(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourceSpecCompositionRef(t *testing.T) {
	type args struct {
		ctx context.Context
		obj *model.CompositeResourceSpec
	}
	type want struct {
		ref  *model.LocalObjectReference
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"NilReference": {
			reason:  "If the reference is nil then the response should be nil.",
			clients: nil,
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceSpec{CompositionReference: nil},
			},
			want: want{
				ref: nil,
			},
		},
		"NonNilReference": {
			reason:  "Should return a valid LocalObjectReference.",
			clients: nil,
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceSpec{CompositionReference: &corev1.ObjectReference{Name: "some-ref-name"}},
			},
			want: want{
				ref: &model.LocalObjectReference{Name: "some-ref-name"},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &compositeResourceSpec{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := s.CompositionRef(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.CompositionRef(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.CompositionRef(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.ref, got, cmpopts.IgnoreUnexported(model.ObjectMeta{}, fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\ns.CompositionRef(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourceSpecClaim(t *testing.T) {
	errBoom := errors.New("boom")
	errNotFound := apierrors.NewNotFound(schema.GroupResource{}, "somename")

	gxrc := model.GetCompositeResourceClaim(&unstructured.Unstructured{})

	type args struct {
		ctx context.Context
		obj *model.CompositeResourceSpec
	}
	type want struct {
		xrc  *model.CompositeResourceClaim
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
			reason: "If there is no claim we should return early.",
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceSpec{},
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
				obj: &model.CompositeResourceSpec{
					ClaimReference: &claim.Reference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
				},
			},
		},
		"GetClaimError": {
			reason: "If we can't get the claim we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceSpec{
					ClaimReference: &claim.Reference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetXRC)),
				},
			},
		},
		"GetClaimNotFound": {
			reason: "If the claim is not found we return nil",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errNotFound),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceSpec{
					ClaimReference: &claim.Reference{},
				},
			},
			want: want{
				xrc: nil,
			},
		},
		"Success": {
			reason: "If we can get and model the claim we should return it.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceSpec{
					ClaimReference: &claim.Reference{},
				},
			},
			want: want{
				xrc: &gxrc,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &compositeResourceSpec{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := s.Claim(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Claim(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Claim(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.xrc, got, cmpopts.IgnoreFields(model.CompositeResourceClaim{}, "PavedAccess"), cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\ns.Claim(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourceSpecClaimRef(t *testing.T) {
	type args struct {
		ctx context.Context
		obj *model.CompositeResourceSpec
	}
	type want struct {
		ref  *model.ObjectReference
		err  error
		errs gqlerror.List
	}

	name := "some-ref-name"

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"NilReference": {
			reason:  "If the reference is nil then the response should be nil.",
			clients: nil,
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceSpec{ClaimReference: nil},
			},
			want: want{
				ref: nil,
			},
		},
		"NonNilReference": {
			reason:  "Should return a valid LocalObjectReference.",
			clients: nil,
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceSpec{ClaimReference: &claim.Reference{Name: name}},
			},
			want: want{
				ref: &model.ObjectReference{Name: &name},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &compositeResourceSpec{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := s.ClaimRef(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.ClaimRef(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.ClaimRef(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.ref, got, cmpopts.IgnoreUnexported(model.ObjectMeta{}, fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\ns.ClaimRef(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourceSpecResources(t *testing.T) {
	errBoom := errors.New("boom")

	kra := &unstructured.Unstructured{}
	kra.SetKind("A")
	gkra, _ := model.GetKubernetesResource(kra)

	krb := &unstructured.Unstructured{}
	krb.SetKind("B")
	gkrb, _ := model.GetKubernetesResource(krb)

	krc := &unstructured.Unstructured{}
	krc.SetKind("C")

	type args struct {
		ctx context.Context
		obj *model.CompositeResourceSpec
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
		"GetComposedError": {
			reason: "If we can't get a composed resource we should add the error to the GraphQL context and continue.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						// Return an error only if this is KR 'A'.
						if obj.GetObjectKind().GroupVersionKind().Kind == kra.GetKind() {
							return errBoom
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceSpec{
					ResourceReferences: []corev1.ObjectReference{
						{Kind: kra.GetKind(), Name: "an-a"},
						{Kind: krb.GetKind(), Name: "a-b"},
					},
				},
			},
			want: want{
				// KR 'A' returned an error, but 'B' did not.
				krc: model.KubernetesResourceConnection{
					TotalCount: 1,
					Nodes:      []model.KubernetesResource{gkrb},
				},
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetComposed)),
				},
			},
		},
		"IgnoreEmptyName": {
			reason: "If we can get and model composed resources we should return them.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceSpec{
					ResourceReferences: []corev1.ObjectReference{
						{Kind: kra.GetKind(), Name: "an-a"},
						{Kind: krc.GetKind()}, // No name
						{Kind: krb.GetKind(), Name: "a-b"},
					},
				},
			},
			want: want{
				krc: model.KubernetesResourceConnection{
					TotalCount: 2,
					Nodes:      []model.KubernetesResource{gkra, gkrb},
				},
			},
		},
		"IgnoreMissingResources": {
			reason: "If the resource is not found, skip it.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == "not-existing" {
							return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
						}
						return nil
					},
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceSpec{
					ResourceReferences: []corev1.ObjectReference{
						{Kind: kra.GetKind(), Name: "an-a"},
						{Kind: krc.GetKind(), Name: "not-existing"},
						{Kind: krb.GetKind(), Name: "a-b"},
					},
				},
			},
			want: want{
				krc: model.KubernetesResourceConnection{
					TotalCount: 2,
					Nodes:      []model.KubernetesResource{gkra, gkrb},
				},
			},
		},
		"Success": {
			reason: "If we can get and model composed resources we should return them.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceSpec{
					ResourceReferences: []corev1.ObjectReference{
						{Kind: kra.GetKind(), Name: "an-a"},
						{Kind: krb.GetKind(), Name: "a-b"},
					},
				},
			},
			want: want{
				krc: model.KubernetesResourceConnection{
					TotalCount: 2,
					Nodes:      []model.KubernetesResource{gkra, gkrb},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &compositeResourceSpec{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := s.Resources(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Claim(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Claim(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.krc, got, cmpopts.IgnoreFields(model.GenericResource{}, "PavedAccess"), cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\ns.Claim(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourceSpecConnectionSecret(t *testing.T) {
	errBoom := errors.New("boom")
	errNotFound := apierrors.NewNotFound(schema.GroupResource{}, "somename")

	gsec := model.GetSecret(&corev1.Secret{})

	type args struct {
		ctx context.Context
		obj *model.CompositeResourceSpec
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
				obj: &model.CompositeResourceSpec{},
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
				obj: &model.CompositeResourceSpec{
					WriteConnectionSecretToReference: &xpv1.SecretReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
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
				obj: &model.CompositeResourceSpec{
					WriteConnectionSecretToReference: &xpv1.SecretReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetSecret)),
				},
			},
		},
		"GetSecretNotFound": {
			reason: "If the secret is not found we return nil",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errNotFound),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceSpec{
					WriteConnectionSecretToReference: &xpv1.SecretReference{},
				},
			},
			want: want{
				sec: nil,
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
				obj: &model.CompositeResourceSpec{
					WriteConnectionSecretToReference: &xpv1.SecretReference{},
				},
			},
			want: want{
				sec: &gsec,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &compositeResourceSpec{clients: tc.clients}

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
			if diff := cmp.Diff(tc.want.sec, got, cmp.AllowUnexported(model.Secret{}), cmpopts.IgnoreUnexported(model.ObjectMeta{}, fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\ns.ConnectionSecret(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourceSpecWriteConnectionSecretToReference(t *testing.T) {
	type args struct {
		ctx context.Context
		obj *model.CompositeResourceSpec
	}
	type want struct {
		ref  *model.SecretReference
		err  error
		errs gqlerror.List
	}

	name := "some-ref-name"
	namespace := "some-namespace"

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"NilReference": {
			reason:  "If the reference is nil then the response should be nil.",
			clients: nil,
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceSpec{WriteConnectionSecretToReference: nil},
			},
			want: want{
				ref: nil,
			},
		},
		"NonNilReference": {
			reason:  "Should return a valid LocalObjectReference.",
			clients: nil,
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceSpec{WriteConnectionSecretToReference: &xpv1.SecretReference{Name: name, Namespace: namespace}},
			},
			want: want{
				ref: &model.SecretReference{Name: name, Namespace: namespace},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &compositeResourceSpec{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := s.WriteConnectionSecretToReference(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.WriteConnectionSecretToReference(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.WriteConnectionSecretToReference(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.ref, got, cmpopts.IgnoreUnexported(model.ObjectMeta{}, fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\ns.WriteConnectionSecretToReference(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourceClaimDefinition(t *testing.T) {
	errBoom := errors.New("boom")

	xrd := extv1.CompositeResourceDefinition{
		Spec: extv1.CompositeResourceDefinitionSpec{
			Group:      "example.org",
			ClaimNames: &kextv1.CustomResourceDefinitionNames{Kind: "Example"},
		},
	}
	gxrd := model.GetCompositeResourceDefinition(&xrd)

	noClaim := extv1.CompositeResourceDefinition{
		Spec: extv1.CompositeResourceDefinitionSpec{
			Group: "example.net",
			Names: kextv1.CustomResourceDefinitionNames{Kind: "Example"},
		},
	}

	otherGroup := extv1.CompositeResourceDefinition{
		Spec: extv1.CompositeResourceDefinitionSpec{
			Group:      "example.net",
			ClaimNames: &kextv1.CustomResourceDefinitionNames{Kind: "Example"},
		},
	}

	otherKind := extv1.CompositeResourceDefinition{
		Spec: extv1.CompositeResourceDefinitionSpec{
			Group:      "example.org",
			ClaimNames: &kextv1.CustomResourceDefinitionNames{Kind: "Illustration"},
		},
	}

	type args struct {
		ctx context.Context
		obj *model.CompositeResourceClaim
	}
	type want struct {
		xrd  *model.CompositeResourceDefinition
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
				obj: &model.CompositeResourceClaim{},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
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
				obj: &model.CompositeResourceClaim{},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errListXRDs)),
				},
			},
		},
		"FoundXRD": {
			reason: "If we can get and model the XRD that defines this XR we should return it.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*extv1.CompositeResourceDefinitionList) = extv1.CompositeResourceDefinitionList{
							Items: []extv1.CompositeResourceDefinition{noClaim, otherGroup, otherKind, xrd},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceClaim{
					APIVersion: xrd.Spec.Group + "/v1",
					Kind:       xrd.Spec.ClaimNames.Kind,
				},
			},
			want: want{
				xrd: &gxrd,
			},
		},
		"NoXRD": {
			reason: "If we can't get and model the XRD that defines this XR we should return nil.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*extv1.CompositeResourceDefinitionList) = extv1.CompositeResourceDefinitionList{
							Items: []extv1.CompositeResourceDefinition{noClaim, otherGroup, otherKind},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceClaim{
					APIVersion: xrd.Spec.Group + "/v1",
					Kind:       xrd.Spec.ClaimNames.Kind,
				},
			},
			want: want{
				xrd: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xrc := &compositeResourceClaim{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := xrc.Definition(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Definition(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Definition(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.xrd, got, cmpopts.IgnoreUnexported(model.ObjectMeta{}, fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\ns.Definition(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourceClaimSpecComposition(t *testing.T) {
	errBoom := errors.New("boom")
	errNotFound := apierrors.NewNotFound(schema.GroupResource{}, "somename")

	gcmp := model.GetComposition(&extv1.Composition{})

	type args struct {
		ctx context.Context
		obj *model.CompositeResourceClaimSpec
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
				obj: &model.CompositeResourceClaimSpec{},
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
				obj: &model.CompositeResourceClaimSpec{
					CompositionReference: &corev1.ObjectReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
				},
			},
		},
		"GetCompositionError": {
			reason: "If we can't get the composition we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceClaimSpec{
					CompositionReference: &corev1.ObjectReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetComposition)),
				},
			},
		},
		"GetCompositionNotFound": {
			reason: "If the composition is not found we return nil",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errNotFound),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceClaimSpec{
					CompositionReference: &corev1.ObjectReference{},
				},
			},
			want: want{
				cmp: nil,
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
				obj: &model.CompositeResourceClaimSpec{
					CompositionReference: &corev1.ObjectReference{},
				},
			},
			want: want{
				cmp: &gcmp,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &compositeResourceClaimSpec{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := s.Composition(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Composition(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Composition(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.cmp, got, cmpopts.IgnoreUnexported(model.ObjectMeta{}, fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\ns.Composition(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourceClaimSpecCompositionRef(t *testing.T) {
	type args struct {
		ctx context.Context
		obj *model.CompositeResourceClaimSpec
	}
	type want struct {
		ref  *model.LocalObjectReference
		err  error
		errs gqlerror.List
	}

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"NilReference": {
			reason:  "If the reference is nil then the response should be nil.",
			clients: nil,
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceClaimSpec{CompositionReference: nil},
			},
			want: want{
				ref: nil,
			},
		},
		"NonNilReference": {
			reason:  "Should return a valid LocalObjectReference.",
			clients: nil,
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceClaimSpec{CompositionReference: &corev1.ObjectReference{Name: "some-ref-name"}},
			},
			want: want{
				ref: &model.LocalObjectReference{Name: "some-ref-name"},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &compositeResourceClaimSpec{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := s.CompositionRef(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.CompositionRef(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.CompositionRef(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.ref, got, cmpopts.IgnoreUnexported(model.ObjectMeta{}, fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\ns.CompositionRef(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourceClaimSpecResource(t *testing.T) {
	errBoom := errors.New("boom")
	errNotFound := apierrors.NewNotFound(schema.GroupResource{}, "somename")

	gxr := model.GetCompositeResource(&unstructured.Unstructured{})

	type args struct {
		ctx context.Context
		obj *model.CompositeResourceClaimSpec
	}
	type want struct {
		xr   *model.CompositeResource
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
			reason: "If there is no resource we should return early.",
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceClaimSpec{},
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
				obj: &model.CompositeResourceClaimSpec{
					ResourceReference: &corev1.ObjectReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
				},
			},
		},
		"GetResourceError": {
			reason: "If we can't get the resource we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceClaimSpec{
					ResourceReference: &corev1.ObjectReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetXR)),
				},
			},
		},
		"GetResourceNotFound": {
			reason: "If the resource is not found we return nil",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errNotFound),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceClaimSpec{
					ResourceReference: &corev1.ObjectReference{},
				},
			},
			want: want{
				xr: nil,
			},
		},
		"Success": {
			reason: "If we can get and model the resource we should return it.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceClaimSpec{
					ResourceReference: &corev1.ObjectReference{},
				},
			},
			want: want{
				xr: &gxr,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &compositeResourceClaimSpec{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := s.Resource(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Claim(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Claim(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.xr, got, cmpopts.IgnoreFields(model.CompositeResource{}, "PavedAccess"), cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\ns.Claim(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourceClaimSpecResourceReference(t *testing.T) {
	type args struct {
		ctx context.Context
		obj *model.CompositeResourceClaimSpec
	}
	type want struct {
		ref  *model.ObjectReference
		err  error
		errs gqlerror.List
	}

	name := "some-ref-name"

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"NilReference": {
			reason:  "If the reference is nil then the response should be nil.",
			clients: nil,
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceClaimSpec{ResourceReference: nil},
			},
			want: want{
				ref: nil,
			},
		},
		"NonNilReference": {
			reason:  "Should return a valid LocalObjectReference.",
			clients: nil,
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceClaimSpec{ResourceReference: &corev1.ObjectReference{Name: name}},
			},
			want: want{
				ref: &model.ObjectReference{Name: &name},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &compositeResourceClaimSpec{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := s.ResourceRef(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.ResourceRef(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.ResourceRef(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.ref, got, cmpopts.IgnoreUnexported(model.ObjectMeta{}, fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\ns.ResourceRef(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourceClaimSpecConnectionSecret(t *testing.T) {
	errBoom := errors.New("boom")
	errNotFound := apierrors.NewNotFound(schema.GroupResource{}, "somename")

	gsec := model.GetSecret(&corev1.Secret{})

	type args struct {
		ctx context.Context
		obj *model.CompositeResourceClaimSpec
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
				obj: &model.CompositeResourceClaimSpec{},
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
				obj: &model.CompositeResourceClaimSpec{
					WriteConnectionSecretToReference: &xpv1.SecretReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetClient)),
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
				obj: &model.CompositeResourceClaimSpec{
					WriteConnectionSecretToReference: &xpv1.SecretReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetSecret)),
				},
			},
		},
		"GetSecretNotFound": {
			reason: "If the secret is not found we return nil",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errNotFound),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceClaimSpec{
					WriteConnectionSecretToReference: &xpv1.SecretReference{},
				},
			},
			want: want{
				sec: nil,
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
				obj: &model.CompositeResourceClaimSpec{
					WriteConnectionSecretToReference: &xpv1.SecretReference{},
				},
			},
			want: want{
				sec: &gsec,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &compositeResourceClaimSpec{clients: tc.clients}

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
			if diff := cmp.Diff(tc.want.sec, got, cmp.AllowUnexported(model.Secret{}), cmpopts.IgnoreUnexported(model.ObjectMeta{}, fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\ns.ConnectionSecret(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestCompositeResourceClaimSpecWriteConnectionSecretToReference(t *testing.T) {
	type args struct {
		ctx context.Context
		obj *model.CompositeResourceClaimSpec
	}
	type want struct {
		ref  *model.SecretReference
		err  error
		errs gqlerror.List
	}

	name := "some-ref-name"
	namespace := "some-namespace"

	cases := map[string]struct {
		reason  string
		clients ClientCache
		args    args
		want    want
	}{
		"NilReference": {
			reason:  "If the reference is nil then the response should be nil.",
			clients: nil,
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceClaimSpec{WriteConnectionSecretToReference: nil},
			},
			want: want{
				ref: nil,
			},
		},
		"NonNilReference": {
			reason:  "Should return a valid LocalObjectReference.",
			clients: nil,
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.CompositeResourceClaimSpec{WriteConnectionSecretToReference: &xpv1.SecretReference{Name: name, Namespace: namespace}},
			},
			want: want{
				ref: &model.SecretReference{Name: name, Namespace: namespace},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &compositeResourceClaimSpec{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := s.WriteConnectionSecretToReference(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.WriteConnectionSecretToReference(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.WriteConnectionSecretToReference(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.ref, got, cmpopts.IgnoreUnexported(model.ObjectMeta{}, fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\ns.WriteConnectionSecretToReference(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
