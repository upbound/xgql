package clients

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"
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
		token string
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
			),
			want: want{
				err: errors.Wrap(errBoom, errNewClient),
			},
		},
		"NewCacheError": {
			reason: "Errors creating a new controller-runtime cache should be returned.",
			c: NewCache(runtime.NewScheme(), &rest.Config{},
				WithNewClientFn(NewClientFn(func(cfg *rest.Config, o client.Options) (client.Client, error) {
					return nil, nil
				})),
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
			_, err := tc.c.Get(tc.args.token, tc.args.o...)
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
