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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
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
		copts  []CacheOption
		args   args
		want   want
	}{
		"NewClientError": {
			reason: "Errors creating a new controller-runtime client should be returned.",
			copts: []CacheOption{
				WithNewClientFn(NewClientFn(func(cfg *rest.Config, o client.Options) (client.Client, error) {
					return nil, errBoom
				})),
				WithNewCacheFn(NewCacheFn(func(cfg *rest.Config, o cache.Options) (cache.Cache, error) {
					return nil, nil
				})),
			},
			want: want{
				err: errors.Wrap(errBoom, errNewClient),
			},
		},
		"NewCacheError": {
			reason: "Errors creating a new controller-runtime cache should be returned.",
			copts: []CacheOption{
				WithNewCacheFn(NewCacheFn(func(cfg *rest.Config, o cache.Options) (cache.Cache, error) {
					return nil, errBoom
				})),
			},
			want: want{
				err: errors.Wrap(errBoom, errNewCache),
			},
		},
		"CacheCrash": {
			reason: "Caches should be removed from the active map when they crash.",
			copts: []CacheOption{
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
			},
			want: want{
				err:    nil,
				active: 0,
			},
		},
		"CacheDidNotSync": {
			reason: "Caches should be removed from the active map if they don't sync.",
			copts: []CacheOption{
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
			},
			want: want{
				err:    errors.New(errWaitForCacheSync),
				active: 0,
			},
		},
		"Success": {
			reason: "Caches should be removed from the active map if they don't sync.",
			copts: []CacheOption{
				WithNewClientFn(NewClientFn(func(cfg *rest.Config, o client.Options) (client.Client, error) {
					return test.NewMockClient(), nil
				})),
				WithNewCacheFn(NewCacheFn(func(cfg *rest.Config, o cache.Options) (cache.Cache, error) {
					ca := &MockCache{
						MockStart: func(stop context.Context) error {
							<-stop.Done()
							return nil
						},
						MockWaitForCacheSync: func(ctx context.Context) bool { return true },
					}
					return ca, nil
				})),
			},
			want: want{
				active: 1,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			copts := append([]CacheOption{WithContext(ctx)}, tc.copts...)
			c := NewCache(runtime.NewScheme(), &rest.Config{}, copts...)
			_, err := c.Get(tc.args.creds, tc.args.o...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nc.Get(...): -want error, +got:\n%s", tc.reason, diff)
			}

			// Give goroutines a second to crash, if they're going to.
			time.Sleep(1 * time.Second)

			active := len(c.active)
			if diff := cmp.Diff(tc.want.active, active); diff != "" {
				t.Errorf("\n%s\nc.Get(...): -want active clients, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func WithContext(ctx context.Context) CacheOption {
	return func(c *Cache) {
		c.ctx = ctx
	}
}
