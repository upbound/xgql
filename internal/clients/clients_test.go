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

package clients

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/upbound/xgql/internal/auth"
)

func WithNewClientFn(fn NewClientFn) CacheOption {
	return func(c *Cache) {
		c.newClient = fn
	}
}

func WithNewCacheFn(fn NewCacheFn) CacheOption {
	return func(c *Cache) {
		c.newCache = fn
	}
}

type MockCache struct {
	cache.Cache

	MockStart            func(stop context.Context) error
	MockWaitForCacheSync func(ctx context.Context) bool
}

func (c *MockCache) Start(stop context.Context) error {
	return c.MockStart(stop)
}

func (c *MockCache) WaitForCacheSync(ctx context.Context) bool {
	return c.MockWaitForCacheSync(ctx)
}

func TestGet(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		creds auth.Credentials
		o     []GetOption
	}

	type want struct {
		err    error
		active int
	}

	cases := map[string]struct {
		reason string
		c      *Cache
		args   args
		want   want
	}{
		"NewClientError": {
			reason: "Errors creating a new controller-runtime client should be returned.",
			c: NewCache(runtime.NewScheme(), &rest.Config{},
				WithNewClientFn(NewClientFn(func(cfg *rest.Config, o client.Options) (client.Client, error) {
					return nil, errBoom
				})),
				WithNewCacheFn(NewCacheFn(func(cfg *rest.Config, o cache.Options) (cache.Cache, error) {
					return nil, nil
				})),
			),
			want: want{
				err: errors.Wrap(errBoom, errNewClient),
			},
		},
		"NewCacheError": {
			reason: "Errors creating a new controller-runtime cache should be returned.",
			c: NewCache(runtime.NewScheme(), &rest.Config{},
				WithNewCacheFn(NewCacheFn(func(cfg *rest.Config, o cache.Options) (cache.Cache, error) {
					return nil, errBoom
				})),
			),
			want: want{
				err: errors.Wrap(errBoom, errNewCache),
			},
		},
		"CacheCrash": {
			reason: "Caches should be removed from the active map when they crash.",
			c: NewCache(runtime.NewScheme(), &rest.Config{},
				WithNewClientFn(NewClientFn(func(cfg *rest.Config, o client.Options) (client.Client, error) {
					return test.NewMockClient(), nil
				})),
				WithNewCacheFn(NewCacheFn(func(cfg *rest.Config, o cache.Options) (cache.Cache, error) {
					ca := &MockCache{
						MockStart:            func(stop context.Context) error { return errBoom },
						MockWaitForCacheSync: func(ctx context.Context) bool { return true },
					}
					return ca, nil
				})),
			),
			want: want{
				err:    nil,
				active: 0,
			},
		},
		"CacheDidNotSync": {
			reason: "Caches should be removed from the active map if they don't sync.",
			c: NewCache(runtime.NewScheme(), &rest.Config{},
				WithNewClientFn(NewClientFn(func(cfg *rest.Config, o client.Options) (client.Client, error) {
					return test.NewMockClient(), nil
				})),
				WithNewCacheFn(NewCacheFn(func(cfg *rest.Config, o cache.Options) (cache.Cache, error) {
					ca := &MockCache{
						MockStart:            func(stop context.Context) error { return nil },
						MockWaitForCacheSync: func(ctx context.Context) bool { return false },
					}
					return ca, nil
				})),
			),
			want: want{
				err:    errors.New(errWaitForCacheSync),
				active: 0,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := tc.c.Get(tc.args.creds, tc.args.o...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nc.Get(...): -want error, +got:\n%s", tc.reason, diff)
			}

			// Give goroutines a second to crash, if they're going to.
			time.Sleep(1 * time.Second)

			active := len(tc.c.active)
			if diff := cmp.Diff(tc.want.active, active); diff != "" {
				t.Errorf("\n%s\nc.Get(...): -want active clients, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

type mockExpiration struct{ expiry time.Duration }

func (e *mockExpiration) Reset(d time.Duration) { e.expiry = d }
func (e *mockExpiration) Stop()                 {}
func (e *mockExpiration) C() <-chan time.Time   { return make(<-chan time.Time) }

func TestSessionGet(t *testing.T) {
	errBoom := errors.New("boom")
	expiry := 1 * time.Minute

	type fields struct {
		client     client.Client
		cancel     context.CancelFunc
		expiry     time.Duration
		expiration *mockExpiration
		log        logging.Logger
	}

	type args struct {
		ctx context.Context
		key client.ObjectKey
		obj client.Object
	}

	type want struct {
		err    error
		expiry time.Duration
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"GetError": {
			reason: "We should reset our expiration, and return errors from our underlying client.",
			fields: fields{
				client:     &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
				expiry:     expiry,
				expiration: &mockExpiration{},
				log:        logging.NewNopLogger(),
			},
			want: want{
				err:    errBoom,
				expiry: expiry,
			},
		},
		"Success": {
			reason: "We should reset our expiration when our underlying client is called.",
			fields: fields{
				client:     &test.MockClient{MockGet: test.NewMockGetFn(nil)},
				expiry:     expiry,
				expiration: &mockExpiration{},
				log:        logging.NewNopLogger(),
			},
			want: want{
				expiry: expiry,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := session{
				client:     tc.fields.client,
				cancel:     tc.fields.cancel,
				expiry:     tc.fields.expiry,
				expiration: tc.fields.expiration,
				log:        tc.fields.log,
			}

			err := s.Get(tc.args.ctx, tc.args.key, tc.args.obj)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Get(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.expiry, tc.fields.expiration.expiry); diff != "" {
				t.Errorf("\n%s\ns.Get(...): -want expiry, +got expiry:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSessionList(t *testing.T) {
	errBoom := errors.New("boom")
	expiry := 1 * time.Minute

	type fields struct {
		client     client.Client
		cancel     context.CancelFunc
		expiry     time.Duration
		expiration *mockExpiration
		log        logging.Logger
	}

	type args struct {
		ctx  context.Context
		obj  client.ObjectList
		opts []client.ListOption
	}

	type want struct {
		err    error
		expiry time.Duration
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"ListError": {
			reason: "We should reset our expiration, and return errors from our underlying client.",
			fields: fields{
				client:     &test.MockClient{MockList: test.NewMockListFn(errBoom)},
				expiry:     expiry,
				expiration: &mockExpiration{},
				log:        logging.NewNopLogger(),
			},
			want: want{
				err:    errBoom,
				expiry: expiry,
			},
		},
		"Success": {
			reason: "We should reset our expiration when our underlying client is called.",
			fields: fields{
				client:     &test.MockClient{MockList: test.NewMockListFn(nil)},
				expiry:     expiry,
				expiration: &mockExpiration{},
				log:        logging.NewNopLogger(),
			},
			want: want{
				expiry: expiry,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := session{
				client:     tc.fields.client,
				cancel:     tc.fields.cancel,
				expiry:     tc.fields.expiry,
				expiration: tc.fields.expiration,
				log:        tc.fields.log,
			}

			err := s.List(tc.args.ctx, tc.args.obj, tc.args.opts...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.List(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.expiry, tc.fields.expiration.expiry); diff != "" {
				t.Errorf("\n%s\ns.List(...): -want expiry, +got expiry:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSessionCreate(t *testing.T) {
	errBoom := errors.New("boom")
	expiry := 1 * time.Minute

	type fields struct {
		client     client.Client
		cancel     context.CancelFunc
		expiry     time.Duration
		expiration *mockExpiration
		log        logging.Logger
	}

	type args struct {
		ctx  context.Context
		obj  client.Object
		opts []client.CreateOption
	}

	type want struct {
		err    error
		expiry time.Duration
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"CreateError": {
			reason: "We should reset our expiration, and return errors from our underlying client.",
			fields: fields{
				client:     &test.MockClient{MockCreate: test.NewMockCreateFn(errBoom)},
				expiry:     expiry,
				expiration: &mockExpiration{},
				log:        logging.NewNopLogger(),
			},
			want: want{
				err:    errBoom,
				expiry: expiry,
			},
		},
		"Success": {
			reason: "We should reset our expiration when our underlying client is called.",
			fields: fields{
				client:     &test.MockClient{MockCreate: test.NewMockCreateFn(nil)},
				expiry:     expiry,
				expiration: &mockExpiration{},
				log:        logging.NewNopLogger(),
			},
			want: want{
				expiry: expiry,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := session{
				client:     tc.fields.client,
				cancel:     tc.fields.cancel,
				expiry:     tc.fields.expiry,
				expiration: tc.fields.expiration,
				log:        tc.fields.log,
			}

			err := s.Create(tc.args.ctx, tc.args.obj, tc.args.opts...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Create(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.expiry, tc.fields.expiration.expiry); diff != "" {
				t.Errorf("\n%s\ns.Create(...): -want expiry, +got expiry:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSessionDelete(t *testing.T) {
	errBoom := errors.New("boom")
	expiry := 1 * time.Minute

	type fields struct {
		client     client.Client
		cancel     context.CancelFunc
		expiry     time.Duration
		expiration *mockExpiration
		log        logging.Logger
	}

	type args struct {
		ctx  context.Context
		obj  client.Object
		opts []client.DeleteOption
	}

	type want struct {
		err    error
		expiry time.Duration
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"DeleteError": {
			reason: "We should reset our expiration, and return errors from our underlying client.",
			fields: fields{
				client:     &test.MockClient{MockDelete: test.NewMockDeleteFn(errBoom)},
				expiry:     expiry,
				expiration: &mockExpiration{},
				log:        logging.NewNopLogger(),
			},
			want: want{
				err:    errBoom,
				expiry: expiry,
			},
		},
		"Success": {
			reason: "We should reset our expiration when our underlying client is called.",
			fields: fields{
				client:     &test.MockClient{MockDelete: test.NewMockDeleteFn(nil)},
				expiry:     expiry,
				expiration: &mockExpiration{},
				log:        logging.NewNopLogger(),
			},
			want: want{
				expiry: expiry,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := session{
				client:     tc.fields.client,
				cancel:     tc.fields.cancel,
				expiry:     tc.fields.expiry,
				expiration: tc.fields.expiration,
				log:        tc.fields.log,
			}

			err := s.Delete(tc.args.ctx, tc.args.obj, tc.args.opts...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.Delete(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.expiry, tc.fields.expiration.expiry); diff != "" {
				t.Errorf("\n%s\ns.Delete(...): -want expiry, +got expiry:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSessionDeleteAllOf(t *testing.T) {
	errBoom := errors.New("boom")
	expiry := 1 * time.Minute

	type fields struct {
		client     client.Client
		cancel     context.CancelFunc
		expiry     time.Duration
		expiration *mockExpiration
		log        logging.Logger
	}

	type args struct {
		ctx  context.Context
		obj  client.Object
		opts []client.DeleteAllOfOption
	}

	type want struct {
		err    error
		expiry time.Duration
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"DeleteAllOfError": {
			reason: "We should reset our expiration, and return errors from our underlying client.",
			fields: fields{
				client:     &test.MockClient{MockDeleteAllOf: test.NewMockDeleteAllOfFn(errBoom)},
				expiry:     expiry,
				expiration: &mockExpiration{},
				log:        logging.NewNopLogger(),
			},
			want: want{
				err:    errBoom,
				expiry: expiry,
			},
		},
		"Success": {
			reason: "We should reset our expiration when our underlying client is called.",
			fields: fields{
				client:     &test.MockClient{MockDeleteAllOf: test.NewMockDeleteAllOfFn(nil)},
				expiry:     expiry,
				expiration: &mockExpiration{},
				log:        logging.NewNopLogger(),
			},
			want: want{
				expiry: expiry,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := session{
				client:     tc.fields.client,
				cancel:     tc.fields.cancel,
				expiry:     tc.fields.expiry,
				expiration: tc.fields.expiration,
				log:        tc.fields.log,
			}

			err := s.DeleteAllOf(tc.args.ctx, tc.args.obj, tc.args.opts...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ns.DeleteAllOf(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.expiry, tc.fields.expiration.expiry); diff != "" {
				t.Errorf("\n%s\ns.DeleteAllOf(...): -want expiry, +got expiry:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSessionStatus(t *testing.T) {
	expiry := 1 * time.Minute

	type fields struct {
		client     client.Client
		cancel     context.CancelFunc
		expiry     time.Duration
		expiration *mockExpiration
		log        logging.Logger
	}

	type want struct {
		status client.StatusWriter
		expiry time.Duration
	}

	cases := map[string]struct {
		reason string
		fields fields
		want   want
	}{
		"Success": {
			reason: "We should reset our expiration when our underlying client is called.",
			fields: fields{
				client:     &test.MockClient{},
				expiry:     expiry,
				expiration: &mockExpiration{},
				log:        logging.NewNopLogger(),
			},
			want: want{
				status: &test.MockSubResourceClient{},
				expiry: expiry,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := session{
				client:     tc.fields.client,
				cancel:     tc.fields.cancel,
				expiry:     tc.fields.expiry,
				expiration: tc.fields.expiration,
				log:        tc.fields.log,
			}

			status := s.Status()
			if diff := cmp.Diff(tc.want.status, status); diff != "" {
				t.Errorf("\n%s\ns.Status(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.expiry, tc.fields.expiration.expiry); diff != "" {
				t.Errorf("\n%s\ns.Status(...): -want expiry, +got expiry:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSessionScheme(t *testing.T) {
	expiry := 1 * time.Minute

	type fields struct {
		client     client.Client
		cancel     context.CancelFunc
		expiry     time.Duration
		expiration *mockExpiration
		log        logging.Logger
	}

	type want struct {
		scheme *runtime.Scheme
		expiry time.Duration
	}

	cases := map[string]struct {
		reason string
		fields fields
		want   want
	}{
		"Success": {
			reason: "We should reset our expiration when our underlying client is called.",
			fields: fields{
				client:     test.NewMockClient(),
				expiry:     expiry,
				expiration: &mockExpiration{},
				log:        logging.NewNopLogger(),
			},
			want: want{
				scheme: nil,
				expiry: expiry,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := session{
				client:     tc.fields.client,
				cancel:     tc.fields.cancel,
				expiry:     tc.fields.expiry,
				expiration: tc.fields.expiration,
				log:        tc.fields.log,
			}

			scheme := s.Scheme()
			if diff := cmp.Diff(tc.want.scheme, scheme); diff != "" {
				t.Errorf("\n%s\ns.Scheme(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.expiry, tc.fields.expiration.expiry); diff != "" {
				t.Errorf("\n%s\ns.Scheme(...): -want expiry, +got expiry:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSessionRESTMapper(t *testing.T) {
	expiry := 1 * time.Minute

	type fields struct {
		client     client.Client
		cancel     context.CancelFunc
		expiry     time.Duration
		expiration *mockExpiration
		log        logging.Logger
	}

	type want struct {
		rm     meta.RESTMapper
		expiry time.Duration
	}

	cases := map[string]struct {
		reason string
		fields fields
		want   want
	}{
		"Success": {
			reason: "We should reset our expiration when our underlying client is called.",
			fields: fields{
				client:     &test.MockClient{},
				expiry:     expiry,
				expiration: &mockExpiration{},
				log:        logging.NewNopLogger(),
			},
			want: want{
				rm:     nil,
				expiry: expiry,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := session{
				client:     tc.fields.client,
				cancel:     tc.fields.cancel,
				expiry:     tc.fields.expiry,
				expiration: tc.fields.expiration,
				log:        tc.fields.log,
			}

			rm := s.RESTMapper()
			if diff := cmp.Diff(tc.want.rm, rm); diff != "" {
				t.Errorf("\n%s\ns.RESTMapper(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.expiry, tc.fields.expiration.expiry); diff != "" {
				t.Errorf("\n%s\ns.RESTMapper(...): -want expiry, +got expiry:\n%s", tc.reason, diff)
			}
		})
	}
}
