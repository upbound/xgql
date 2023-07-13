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
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/gqlerror"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/clients"
	"github.com/upbound/xgql/internal/graph/generated"
	"github.com/upbound/xgql/internal/graph/model"
)

func TestEvent(t *testing.T) {
	errBoom := errors.New("boom")
	involved := &corev1.ObjectReference{UID: "deeply-involved"}

	related := corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "coolevent"},
		InvolvedObject: corev1.ObjectReference{UID: involved.UID},
	}

	unrelated := corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "coolevent"},
		InvolvedObject: corev1.ObjectReference{UID: "wat"},
	}

	type args struct {
		ctx      context.Context
		involved *corev1.ObjectReference
	}
	type want struct {
		ec   model.EventConnection
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
		"ListEventsError": {
			reason: "If we can't list events we should add the error to the GraphQL context and return early.",
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
					gqlerror.Errorf(errors.Wrap(errBoom, errListEvents).Error()),
				},
			},
		},
		"ListAllEvents": {
			reason: "We should successfully return events for all resources if the involved object is nil.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*corev1.EventList) = corev1.EventList{Items: []corev1.Event{related, unrelated}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
			},
			want: want{
				ec: model.EventConnection{
					Nodes:      []model.Event{model.GetEvent(&related), model.GetEvent(&unrelated)},
					TotalCount: 2,
				},
			},
		},
		"ListEventsInvolving": {
			reason: "We should successfully return events for all resources if the involved object is nil.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*corev1.EventList) = corev1.EventList{Items: []corev1.Event{related, unrelated}}
						return nil
					}),
				}, nil
			}),
			args: args{
				ctx:      graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				involved: involved,
			},
			want: want{
				ec: model.EventConnection{
					Nodes:      []model.Event{model.GetEvent(&related)},
					TotalCount: 1,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &events{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := e.Resolve(tc.args.ctx, tc.args.involved)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Resolve(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Resolve(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.ec, got, cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\ns.Resolve(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestInvolves(t *testing.T) {
	wuid := &corev1.ObjectReference{UID: "so-unique"}

	wid := &corev1.ObjectReference{
		APIVersion: "example.org/v1",
		Kind:       "Example",
		Namespace:  "default",
		Name:       "cool",
	}

	type args struct {
		e *corev1.Event
		o *corev1.ObjectReference
	}

	cases := map[string]struct {
		args args
		want bool
	}{
		"UIDMatch": {
			args: args{
				e: &corev1.Event{
					InvolvedObject: corev1.ObjectReference{UID: wuid.UID},
				},
				o: wuid,
			},
			want: true,
		},
		"GroupMismatch": {
			args: args{
				e: &corev1.Event{
					InvolvedObject: corev1.ObjectReference{
						APIVersion: "other.net/v1",
					},
				},
				o: wid,
			},
			want: false,
		},
		"KindMismatch": {
			args: args{
				e: &corev1.Event{
					InvolvedObject: corev1.ObjectReference{
						APIVersion: wid.APIVersion,
						Kind:       "Other",
					},
				},
				o: wid,
			},
			want: false,
		},
		"NamespaceMismatch": {
			args: args{
				e: &corev1.Event{
					InvolvedObject: corev1.ObjectReference{
						APIVersion: wid.APIVersion,
						Kind:       wid.Kind,
						Namespace:  "other",
					},
				},
				o: wid,
			},
			want: false,
		},
		"NameMismatch": {
			args: args{
				e: &corev1.Event{
					InvolvedObject: corev1.ObjectReference{
						APIVersion: wid.APIVersion,
						Kind:       wid.Kind,
						Namespace:  wid.Namespace,
						Name:       "other",
					},
				},
				o: wid,
			},
			want: false,
		},
		"Match": {
			args: args{
				e: &corev1.Event{
					InvolvedObject: corev1.ObjectReference{
						APIVersion: wid.APIVersion,
						Kind:       wid.Kind,
						Namespace:  wid.Namespace,
						Name:       wid.Name,
					},
				},
				o: wid,
			},
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := involves(tc.args.e, tc.args.o)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("involves(...): -want, +got:\n%s", diff)
			}
		})
	}
}

var _ generated.EventResolver = &event{}

func TestEventInvolvedObject(t *testing.T) {
	errBoom := errors.New("boom")

	gu, _ := model.GetKubernetesResource(&unstructured.Unstructured{}, model.SelectAll)

	type args struct {
		ctx context.Context
		obj *model.Event
	}
	type want struct {
		kr   model.KubernetesResource
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
				obj: &model.Event{
					InvolvedObjectRef: corev1.ObjectReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetClient).Error()),
				},
			},
		},
		"GetInvolvedError": {
			reason: "If we can't get the involved object we should add the error to the GraphQL context and return early.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.Event{
					InvolvedObjectRef: corev1.ObjectReference{},
				},
			},
			want: want{
				errs: gqlerror.List{
					gqlerror.Errorf(errors.Wrap(errBoom, errGetInvolved).Error()),
				},
			},
		},
		"Success": {
			reason: "If we can get and model the involved object we should return it.",
			clients: ClientCacheFn(func(_ auth.Credentials, _ ...clients.GetOption) (client.Client, error) {
				return &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				}, nil
			}),
			args: args{
				ctx: graphql.WithResponseContext(context.Background(), graphql.DefaultErrorPresenter, graphql.DefaultRecover),
				obj: &model.Event{
					InvolvedObjectRef: corev1.ObjectReference{},
				},
			},
			want: want{
				kr: gu,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := &event{clients: tc.clients}

			// Our GraphQL resolvers never return errors. We instead add an
			// error to the GraphQL context and return early.
			got, err := s.InvolvedObject(tc.args.ctx, tc.args.obj)
			errs := graphql.GetErrors(tc.args.ctx)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.InvolvedObject(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.errs, errs, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.InvolvedObject(...): -want GraphQL errors, +got GraphQL errors:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.kr, got, cmpopts.IgnoreFields(model.GenericResource{}, "Unstructured"), cmpopts.IgnoreUnexported(model.ObjectMeta{})); diff != "" {
				t.Errorf("\n%s\ns.InvolvedObject(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
