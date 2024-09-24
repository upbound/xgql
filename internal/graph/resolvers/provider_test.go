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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/clients"
	"github.com/upbound/xgql/internal/graph/generated"
	"github.com/upbound/xgql/internal/graph/model"
	xunstructured "github.com/upbound/xgql/internal/unstructured"
)

var (
	_ generated.ProviderResolver               = &provider{}
	_ generated.ProviderRevisionResolver       = &providerRevision{}
	_ generated.ProviderRevisionStatusResolver = &providerRevisionStatus{}
)

func TestProviderRevisions(t *testing.T) {
	errBoom := errors.New("boom")

	uid := "no-you-id"

	// The active ProviderRevision that we control.
	active := pkgv1.ProviderRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "coolconfig",
			OwnerReferences: []metav1.OwnerReference{meta.AsController(&xpv1.TypedReference{UID: types.UID(uid)})},
		},
		Spec: pkgv1.ProviderRevisionSpec{PackageRevisionSpec: pkgv1.PackageRevisionSpec{DesiredState: pkgv1.PackageRevisionActive}},
	}
	gactive := model.GetProviderRevision(&active)

	// A ProviderRevision we control, but that is inactive.
	inactive := pkgv1.ProviderRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "coolconfig",
			OwnerReferences: []metav1.OwnerReference{meta.AsController(&xpv1.TypedReference{UID: types.UID(uid)})},
		},
		Spec: pkgv1.ProviderRevisionSpec{PackageRevisionSpec: pkgv1.PackageRevisionSpec{DesiredState: pkgv1.PackageRevisionInactive}},
	}
	ginactive := model.GetProviderRevision(&inactive)

	// A ProviderRevision which we do not control.
	other := pkgv1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{Name: "not-ours"}}

	type args struct {
		ctx context.Context
		obj *model.Provider
	}
	type want struct {
		pc   model.ProviderRevisionConnection
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
		"ListRevisionsError": {
			reason: "If we can't list revisions we should add the error to the GraphQL context and return early.",
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
					gqlerror.Wrap(errors.Wrap(errBoom, errListProviderRevs)),
				},
			},
		},
		"AllRevisions": {
			reason: "We should successfully return any revisions we own that we can list and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*pkgv1.ProviderRevisionList) = pkgv1.ProviderRevisionList{
							Items: []pkgv1.ProviderRevision{other, active, inactive},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.Provider{
					Metadata: model.ObjectMeta{UID: uid},
				},
			},
			want: want{
				pc: model.ProviderRevisionConnection{
					Nodes:      []model.ProviderRevision{gactive, ginactive},
					TotalCount: 2,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &provider{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := c.Revisions(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Revisions(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Revisions(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.pc, got, cmpopts.IgnoreUnexported(model.ObjectMeta{}, fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\nq.Revisions(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestProviderActiveRevision(t *testing.T) {
	errBoom := errors.New("boom")

	uid := "no-you-id"

	// The active ProviderRevision that we control.
	active := pkgv1.ProviderRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "coolconfig",
			OwnerReferences: []metav1.OwnerReference{meta.AsController(&xpv1.TypedReference{UID: types.UID(uid)})},
		},
		Spec: pkgv1.ProviderRevisionSpec{PackageRevisionSpec: pkgv1.PackageRevisionSpec{DesiredState: pkgv1.PackageRevisionActive}},
	}
	gactive := model.GetProviderRevision(&active)

	// A ProviderRevision we control, but that is inactive.
	inactive := pkgv1.ProviderRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "coolconfig",
			OwnerReferences: []metav1.OwnerReference{meta.AsController(&xpv1.TypedReference{UID: types.UID(uid)})},
		},
		Spec: pkgv1.ProviderRevisionSpec{PackageRevisionSpec: pkgv1.PackageRevisionSpec{DesiredState: pkgv1.PackageRevisionInactive}},
	}

	// An active ProviderRevision which we do not control.
	otherActive := pkgv1.ProviderRevision{
		ObjectMeta: metav1.ObjectMeta{Name: "not-ours"},
		Spec:       pkgv1.ProviderRevisionSpec{PackageRevisionSpec: pkgv1.PackageRevisionSpec{DesiredState: pkgv1.PackageRevisionActive}},
	}

	type args struct {
		ctx context.Context
		obj *model.Provider
	}
	type want struct {
		pr   *model.ProviderRevision
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
		"ListRevisionsError": {
			reason: "If we can't list revisions we should add the error to the GraphQL context and return early.",
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
					gqlerror.Wrap(errors.Wrap(errBoom, errListProviderRevs)),
				},
			},
		},
		"FoundActiveRevision": {
			reason: "We should successfully return the active revision.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*pkgv1.ProviderRevisionList) = pkgv1.ProviderRevisionList{
							Items: []pkgv1.ProviderRevision{otherActive, inactive, active},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.Provider{
					Metadata: model.ObjectMeta{UID: uid},
				},
			},
			want: want{
				pr: &gactive,
			},
		},
		"NoActiveRevision": {
			reason: "If there is no active revision we should return nil.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*pkgv1.ProviderRevisionList) = pkgv1.ProviderRevisionList{
							Items: []pkgv1.ProviderRevision{otherActive, inactive},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.Provider{
					Metadata: model.ObjectMeta{UID: uid},
				},
			},
			want: want{
				pr: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &provider{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := c.ActiveRevision(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.ActiveRevision(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.ActiveRevision(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.pr, got, cmpopts.IgnoreUnexported(model.ObjectMeta{}, fieldpath.Paved{})); diff != "" {
				t.Errorf("\n%s\nq.ActiveRevision(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestProviderRevisionStatusObjects(t *testing.T) {
	errBoom := errors.New("boom")

	gcrd := model.GetCustomResourceDefinition(&xunstructured.CustomResourceDefinition{})

	type args struct {
		ctx context.Context
		obj *model.ProviderRevisionStatus
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
		"UnknownObject": {
			reason: "We should not attempt to get an object that doesn't seem to be a CRD.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ProviderRevisionStatus{
					ObjectRefs: []xpv1.TypedReference{
						{
							Kind: "wat",
						},
						{
							APIVersion: "wat",
							Kind:       "CustomResourceDefinition",
						},
					},
				},
			},
			want: want{
				krc: model.KubernetesResourceConnection{
					Nodes:      []model.KubernetesResource{},
					TotalCount: 0,
				},
			},
		},
		"GetCRDError": {
			reason: "If we can't get an CRD we should add the error to the GraphQL context and continue.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ProviderRevisionStatus{
					ObjectRefs: []xpv1.TypedReference{
						{
							APIVersion: schema.GroupVersion{Group: kextv1.GroupName, Version: "v1"}.String(),
							Kind:       "CustomResourceDefinition",
						},
					},
				},
			},
			want: want{
				krc: model.KubernetesResourceConnection{
					Nodes:      []model.KubernetesResource{},
					TotalCount: 0,
				},
				errs: gqlerror.List{
					gqlerror.Wrap(errors.Wrap(errBoom, errGetCRD)),
				},
			},
		},
		"Success": {
			reason: "We should return all the CRDs that we can get and model.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ProviderRevisionStatus{
					ObjectRefs: []xpv1.TypedReference{
						{
							APIVersion: schema.GroupVersion{Group: kextv1.GroupName, Version: "v1"}.String(),
							Kind:       "CustomResourceDefinition",
						},
					},
				},
			},
			want: want{
				krc: model.KubernetesResourceConnection{
					Nodes:      []model.KubernetesResource{gcrd},
					TotalCount: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &providerRevisionStatus{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := s.Objects(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Objects(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Objects(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.krc, got,
				cmpopts.IgnoreUnexported(model.ObjectMeta{}),
				cmpopts.IgnoreFields(model.CustomResourceDefinition{}, "PavedAccess"),
			); diff != "" {
				t.Errorf("\n%s\ns.Objects(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
