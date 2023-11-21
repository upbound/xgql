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
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"net/http"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/version"
)

const (
	errNewClient        = "cannot create new write client"
	errNewCache         = "cannot create new read cache"
	errNewHTTPClient    = "cannot create new HTTP client"
	errDelegClient      = "cannot create cache-backed client"
	errWaitForCacheSync = "cannot sync client cache"
)

// A NewCacheFn creates a new controller-runtime cache.
type NewCacheFn func(cfg *rest.Config, o cache.Options) (cache.Cache, error)

// A NewClientFn creates a new controller-runtime client.
type NewClientFn func(cfg *rest.Config, o client.Options) (client.Client, error)

// A NewCacheMiddlewareFn can be used to wrap a new cache function with
// middleware.
type NewCacheMiddlewareFn func(NewCacheFn) NewCacheFn

// The default new cache and new controller functions.
var (
	DefaultNewCacheFn  NewCacheFn  = cache.New
	DefaultNewClientFn NewClientFn = client.New
)

// Config returns a REST config.
func Config() (*rest.Config, error) {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, errors.Wrap(err, "cannot create in-cluster configuration")
	}

	cfg.QPS = 50
	cfg.Burst = 300

	cfg.UserAgent = "xgql/" + version.Version

	return cfg, nil
}

// RESTMapper returns a 'REST mapper' that discovers an API server's available
// REST API endpoints. The returned REST mapper is intended to be shared by many
// clients. It is 'dynamic' in that it will attempt to rediscover API endpoints
// any time a client asks for a kind of resource that is unknown to it. Each
// discovery process may burst up to 100 API server requests per second, and
// average 20 requests per second. Rediscovery may not happen more frequently
// than once every 20 seconds.
func RESTMapper(cfg *rest.Config, httpClient *http.Client) (meta.RESTMapper, error) {
	dcfg := rest.CopyConfig(cfg)
	dcfg.QPS = 50
	dcfg.Burst = 300

	return apiutil.NewDynamicRESTMapper(dcfg, httpClient)
}

// Anonymize the supplied config by returning a copy with all authentication
// details and credentials removed.
func Anonymize(cfg *rest.Config) *rest.Config {
	return rest.AnonymousClientConfig(cfg)
}

// TODO(negz): There are a few gotchas with watch based caches. The chief issue
// is that 'read' errors surface at the watch level, not when the client reads
// from the cache. For example if the user doesn't have RBAC access to list and
// watch a particular type of resource these errors will be logged by the cache
// layer, but not surfaced to the caller when they interact with the cache. To
// the caller it will appear as if the resource simply does not exist. This is
// exacerbated by the fact that watches never stop; for example if a client gets
// a resource type that is defined by a custom resource definition that is later
// deleted the cache will indefinitely try and fail to watch that type. Ideally
// we'd be able to detect unhealthy caches and either reset them or surface the
// error to the caller somehow.

// A Cache of Kubernetes clients. Each client is associated with a particular
// bearer token, which is used to authenticate to an API server. Each client is
// backed by its own cache, which is populated by automatically watching any
// type the client is asked to get or list. Clients (and their caches) expire
// and are garbage collected if they are unused for five minutes.
type Cache struct {
	// a context that will be valid for the lifetime of Cache.
	ctx    context.Context
	active map[string]*session
	mx     sync.RWMutex

	cfg     *rest.Config
	scheme  *runtime.Scheme
	mapper  meta.RESTMapper
	nocache []client.Object
	expiry  time.Duration

	newCache  NewCacheFn
	newClient NewClientFn

	salt []byte
	log  logging.Logger
}

// A CacheOption configures the client cache.
type CacheOption func(c *Cache)

// WithLogger configures the logger used by the client cache. A no-op logger is
// used by default.
func WithLogger(l logging.Logger) CacheOption {
	return func(c *Cache) {
		c.log = l
	}
}

// WithRESTMapper configures the REST mapper used by cached clients. A mapper
// is created for each new client by default, which can take ~10 seconds.
func WithRESTMapper(m meta.RESTMapper) CacheOption {
	return func(c *Cache) {
		c.mapper = m
	}
}

// WithExpiry configures the duration until each client expires. Each time any
// of a client's methods are called the expiry time is reset to this value. When
// a client expires its cache will be garbage collected.
func WithExpiry(d time.Duration) CacheOption {
	return func(c *Cache) {
		c.expiry = d
	}
}

// DoNotCache configures clients not to cache objects of the supplied types.
// Note that the cache machinery extracts a GVK from these objects, so they can
// either be types known to the scheme or *unstructured.Unstructured with their
// APIVersion and Kind set.
func DoNotCache(o []client.Object) CacheOption {
	return func(c *Cache) {
		c.nocache = o
	}
}

// UseNewCacheMiddleware configures the cache to use the supplied middleware
// functions when creating new caches. This can be used to wrap the cache's
// default new cache function with additional functionality.
func UseNewCacheMiddleware(fns ...NewCacheMiddlewareFn) CacheOption {
	return func(c *Cache) {
		for _, fn := range fns {
			c.newCache = fn(c.newCache)
		}
	}
}

// NewCache creates a cache of Kubernetes clients. Clients use the supplied
// scheme, and connect to the API server using a copy of the supplied REST
// config with a specific bearer token injected.
func NewCache(s *runtime.Scheme, c *rest.Config, o ...CacheOption) *Cache {
	salt := make([]byte, 32)
	_, _ = io.ReadFull(rand.Reader, salt)

	ch := &Cache{
		ctx:    context.Background(),
		active: make(map[string]*session),

		cfg:    c,
		scheme: s,
		expiry: 5 * time.Minute,

		newCache:  DefaultNewCacheFn,
		newClient: DefaultNewClientFn,

		salt: salt,
		log:  logging.NewNopLogger(),
	}

	for _, fn := range o {
		fn(ch)
	}

	return ch
}

type getOptions struct {
}

// A GetOption modifies the kind of client returned.
type GetOption func(o *getOptions)

// Get a client that uses the specified bearer token.
func (c *Cache) Get(cr auth.Credentials, o ...GetOption) (client.Client, error) { //nolint:gocyclo
	extra := bytes.Buffer{}
	extra.Write(c.salt)
	id := cr.Hash(extra.Bytes())

	log := c.log.WithValues("client-id", id)

	c.mx.RLock()
	sn, ok := c.active[id]
	c.mx.RUnlock()

	if ok {
		log.Debug("Used existing cached client",
			"new-expiry", time.Now().Add(c.expiry),
		)
		sn.expiration.Reset(c.expiry)
		return sn.client, nil
	}

	started := time.Now()
	cfg := cr.Inject(c.cfg)
	hc, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return nil, errors.Wrap(err, errNewHTTPClient)
	}
	ca, err := c.newCache(cfg, cache.Options{
		HTTPClient: hc,
		Scheme:     c.scheme,
		Mapper:     c.mapper,
	})
	if err != nil {
		return nil, errors.Wrap(err, errNewCache)
	}

	wc, err := c.newClient(cfg, client.Options{
		HTTPClient: hc,
		Scheme:     c.scheme,
		Mapper:     c.mapper,
		Cache: &client.CacheOptions{
			Reader:     ca,
			DisableFor: c.nocache,
			// TODO(negz): Don't cache unstructured objects? Doing so allows us to
			// cache object types that aren't known at build time, like managed
			// resources and composite resources. On the other hand it could lead to
			// the cache starting a watch on any kind of resource it encounters,
			// e.g. arbitrary owner references.
			Unstructured: true,
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	// We use a distinct s.expiry ticker rather than a context deadline or timeout
	// because it's not possible to extend a context's deadline or timeout, but it
	// is possible to 'reset' (i.e. extend) a ticker.
	expiration := &tickerExpiration{t: time.NewTicker(c.expiry)}
	newExpiry := time.Now().Add(c.expiry)
	ctx, cancel := context.WithCancel(c.ctx)
	sn = &session{client: wc, cancel: cancel, expiration: expiration}

	c.mx.Lock()
	// another gorouting might have set the session.
	if sn, ok := c.active[id]; ok {
		c.mx.Unlock()
		log.Debug("Used existing cached client",
			"duration", time.Since(started),
			"new-expiry", newExpiry,
		)
		return sn.client, nil
	}
	c.active[id] = sn
	c.mx.Unlock()

	go func() {
		err := ca.Start(ctx)
		log.Debug("Cache stopped", "error", err)

		// Start blocks until ctx is closed, or it encounters an error. If we make
		// it here either the cache crashed, or the context was cancelled (e.g.
		// because our session expired).
		c.remove(id)
	}()

	// Stop our cache when we expire.
	go func() {
		select {
		case <-expiration.C():
			// We expired, and should remove ourself from the session cache.
			log.Debug("Client expired")
		case <-ctx.Done():
			log.Debug("Client stopped")
			// We're done for some other reason (e.g. the cache crashed).
		}
		c.remove(id)
	}()

	if !ca.WaitForCacheSync(ctx) {
		c.remove(id)
		return nil, errors.New(errWaitForCacheSync)
	}

	log.Debug("Created cached client",
		"duration", time.Since(started),
		"new-expiry", newExpiry,
	)

	return sn.client, nil
}

func (c *Cache) remove(id string) {
	c.mx.Lock()
	defer c.mx.Unlock()

	if sn, ok := c.active[id]; ok {
		sn.cancel()
		sn.expiration.Stop()
		delete(c.active, id)
		c.log.Debug("Removed client cache", "client-id", id)
	}
}

type expiration interface {
	Reset(d time.Duration)
	Stop()
	C() <-chan time.Time
}

type tickerExpiration struct{ t *time.Ticker }

func (e *tickerExpiration) Reset(d time.Duration) { e.t.Reset(d) }
func (e *tickerExpiration) Stop()                 { e.t.Stop() }
func (e *tickerExpiration) C() <-chan time.Time   { return e.t.C }

type session struct {
	client     client.Client
	cancel     context.CancelFunc
	expiration expiration
}
