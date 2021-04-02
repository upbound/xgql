package resolvers

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/gqlerror"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	extv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/upbound/xgql/internal/clients"
	"github.com/upbound/xgql/internal/graph/generated"
	"github.com/upbound/xgql/internal/graph/model"
)

var (
	_ generated.ConfigurationResolver               = &configuration{}
	_ generated.ConfigurationRevisionResolver       = &configurationRevision{}
	_ generated.ConfigurationRevisionStatusResolver = &configurationRevisionStatus{}
)

func TestConfigurationRevisions(t *testing.T) {
	errBoom := errors.New("boom")

	uid := "no-you-id"

	// The active ConfigurationRevision that we control.
	active := pkgv1.ConfigurationRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "coolconfig",
			OwnerReferences: []metav1.OwnerReference{meta.AsController(&xpv1.TypedReference{UID: types.UID(uid)})},
		},
		Spec: pkgv1.PackageRevisionSpec{DesiredState: pkgv1.PackageRevisionActive},
	}
	gactive, _ := model.GetConfigurationRevision(&active)

	// A ConfigurationRevision we control, but that is inactive.
	inactive := pkgv1.ConfigurationRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "coolconfig",
			OwnerReferences: []metav1.OwnerReference{meta.AsController(&xpv1.TypedReference{UID: types.UID(uid)})},
		},
		Spec: pkgv1.PackageRevisionSpec{DesiredState: pkgv1.PackageRevisionInactive},
	}
	ginactive, _ := model.GetConfigurationRevision(&inactive)

	// A ConfigurationRevision which we do not control.
	other := pkgv1.ConfigurationRevision{ObjectMeta: metav1.ObjectMeta{Name: "not-ours"}}

	type args struct {
		ctx    context.Context
		obj    *model.Configuration
		active *bool
	}
	type want struct {
		crc  *model.ConfigurationRevisionConnection
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
			clients: ClientCacheFn(func(_ string, _ ...clients.GetOption) (client.Client, error) {
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
		"ListRevisionsError": {
			reason: "If we can't list revisions we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ string, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errListConfigRevs).Error()),
				},
			},
		},
		"AllRevisions": {
			reason: "We should successfully return any revisions we own that we can list and model.",
			clients: ClientCacheFn(func(_ string, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*pkgv1.ConfigurationRevisionList) = pkgv1.ConfigurationRevisionList{
							Items: []pkgv1.ConfigurationRevision{other, active, inactive},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.Configuration{
					Metadata: &model.ObjectMeta{UID: uid},
				},
			},
			want: want{
				crc: &model.ConfigurationRevisionConnection{
					Items: []model.ConfigurationRevision{gactive, ginactive},
					Count: 2,
				},
			},
		},
		"ActiveRevisions": {
			reason: "We should successfully return any active revisions we own that we can list and model.",
			clients: ClientCacheFn(func(_ string, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*pkgv1.ConfigurationRevisionList) = pkgv1.ConfigurationRevisionList{
							Items: []pkgv1.ConfigurationRevision{other, active, inactive},
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.Configuration{
					Metadata: &model.ObjectMeta{UID: uid},
				},
				active: pointer.BoolPtr(true),
			},
			want: want{
				crc: &model.ConfigurationRevisionConnection{
					Items: []model.ConfigurationRevision{gactive},
					Count: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &configuration{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := c.Revisions(tc.args.ctx, tc.args.obj, tc.args.active)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Revisions(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nq.Revisions(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.crc, got); diff != "" {
				t.Errorf("\n%s\nq.Revisions(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestConfigurationRevisionStatusObjects(t *testing.T) {
	errBoom := errors.New("boom")

	gxrd, _ := model.GetCompositeResourceDefinition(&extv1.CompositeResourceDefinition{})
	gcmp, _ := model.GetComposition(&extv1.Composition{})

	type args struct {
		ctx context.Context
		obj *model.ConfigurationRevisionStatus
	}
	type want struct {
		krc  *model.KubernetesResourceConnection
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
			clients: ClientCacheFn(func(_ string, _ ...clients.GetOption) (client.Client, error) {
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
		"UnknownObject": {
			reason: "We should not attempt to get an object that doesn't seem to be part of the API extensions group.",
			clients: ClientCacheFn(func(_ string, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ConfigurationRevisionStatus{
					ObjectRefs: []xpv1.TypedReference{
						{
							APIVersion: "wat",
						},
					},
				},
			},
			want: want{
				krc: &model.KubernetesResourceConnection{
					Items: []model.KubernetesResource{},
					Count: 0,
				},
			},
		},
		"GetXRDError": {
			reason: "If we can't get an XRD we should add the error to the GraphQL context and continue.",
			clients: ClientCacheFn(func(_ string, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if _, ok := obj.(*extv1.CompositeResourceDefinition); ok {
							return errBoom
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ConfigurationRevisionStatus{
					ObjectRefs: []xpv1.TypedReference{
						{
							APIVersion: schema.GroupVersion{Group: extv1.Group, Version: extv1.Version}.String(),
							Kind:       extv1.CompositeResourceDefinitionKind,
						},
						{
							APIVersion: schema.GroupVersion{Group: extv1.Group, Version: extv1.Version}.String(),
							Kind:       extv1.CompositionKind,
						},
					},
				},
			},
			want: want{
				krc: &model.KubernetesResourceConnection{
					Items: []model.KubernetesResource{
						gcmp,
					},
					Count: 1,
				},
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetXRD).Error()),
				},
			},
		},
		"GetCompositionError": {
			reason: "If we can't get a Composition we should add the error to the GraphQL context and continue.",
			clients: ClientCacheFn(func(_ string, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if _, ok := obj.(*extv1.Composition); ok {
							return errBoom
						}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.ConfigurationRevisionStatus{
					ObjectRefs: []xpv1.TypedReference{
						{
							APIVersion: schema.GroupVersion{Group: extv1.Group, Version: extv1.Version}.String(),
							Kind:       extv1.CompositionKind,
						},
						{
							APIVersion: schema.GroupVersion{Group: extv1.Group, Version: extv1.Version}.String(),
							Kind:       extv1.CompositeResourceDefinitionKind,
						},
					},
				},
			},
			want: want{
				krc: &model.KubernetesResourceConnection{
					Items: []model.KubernetesResource{
						gxrd,
					},
					Count: 1,
				},
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetComp).Error()),
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &configurationRevisionStatus{clients: tc.clients}

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
			if diff := cmp.Diff(tc.want.krc, got); diff != "" {
				t.Errorf("\n%s\ns.Objects(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
