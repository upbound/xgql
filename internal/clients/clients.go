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
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/upbound/xgql/internal/auth"
	"github.com/upbound/xgql/internal/version"
)

const (
	errNewClient        = "cannot create new write client"
	errNewCache         = "cannot create new read cache"
	errDelegClient      = "cannot create cache-backed client"
	errWaitForCacheSync = "cannot sync client cache"
)

// A NewCacheFn creates a new controller-runtime cache.
type NewCacheFn func(cfg *rest.Config, o cache.Options) (cache.Cache, error)

// A NewClientFn creates a new controller-runtime client.
type NewClientFn func(cfg *rest.Config, o client.Options) (client.Client, error)

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

	// ctrl.GetConfig tunes QPS and burst for Kubernetes controllers. We're not
	// a controller and we expect to be creating many clients, so we tune these
	// back down to the client-go defaults.
	cfg.QPS = 5
	cfg.Burst = 20

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
func RESTMapper(cfg *rest.Config) (meta.RESTMapper, error) {
	dcfg := rest.CopyConfig(cfg)
	dcfg.QPS = 20
	dcfg.Burst = 100

	return apiutil.NewDynamicRESTMapper(dcfg, apiutil.WithLimiter(rate.NewLimiter(rate.Limit(0.05), 1)))
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

// NewCache creates a cache of Kubernetes clients. Clients use the supplied
// scheme, and connect to the API server using a copy of the supplied REST
// config with a specific bearer token injected.
func NewCache(s *runtime.Scheme, c *rest.Config, o ...CacheOption) *Cache {
	salt := make([]byte, 32)
	_, _ = io.ReadFull(rand.Reader, salt)

	ch := &Cache{
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
	Namespace string
}

// A GetOption modifies the kind of client returned.
type GetOption func(o *getOptions)

// ForNamespace returns a client backed by a cache scoped to the supplied
// namespace.
func ForNamespace(n string) GetOption {
	return func(o *getOptions) {
		o.Namespace = n
	}
}

// Get a client that uses the specified bearer token.
func (c *Cache) Get(cr auth.Credentials, o ...GetOption) (client.Client, error) {
	opts := &getOptions{}
	for _, fn := range o {
		fn(opts)
	}

	extra := bytes.Buffer{}
	extra.Write(c.salt)
	extra.WriteString(opts.Namespace)
	id := cr.Hash(extra.Bytes())

	log := c.log.WithValues("client-id", id)
	if opts.Namespace != "" {
		log = log.WithValues("namespace", opts.Namespace)
	}

	c.mx.RLock()
	sn, ok := c.active[id]
	c.mx.RUnlock()

	if ok {
		log.Debug("Used existing client")
		return sn, nil
	}

	started := time.Now()
	cfg := cr.Inject(c.cfg)

	wc, err := c.newClient(cfg, client.Options{Scheme: c.scheme, Mapper: c.mapper})
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	ca, err := c.newCache(cfg, cache.Options{Scheme: c.scheme, Mapper: c.mapper, Namespace: opts.Namespace})
	if err != nil {
		return nil, errors.Wrap(err, errNewCache)
	}

	dci := client.NewDelegatingClientInput{
		CacheReader:     ca,
		Client:          wc,
		UncachedObjects: c.nocache,

		// TODO(negz): Don't cache unstructured objects? Doing so allows us to
		// cache object types that aren't known at build time, like managed
		// resources and composite resources. On the other hand it could lead to
		// the cache starting a watch on any kind of resource it encounters,
		// e.g. arbitrary owner references.
		CacheUnstructured: true,
	}
	dc, err := client.NewDelegatingClient(dci)
	if err != nil {
		return nil, errors.Wrap(err, errDelegClient)
	}

	// We use a distinct s.expiry ticker rather than a context deadline or timeout
	// because it's not possible to extend a context's deadline or timeout, but it
	// is possible to 'reset' (i.e. extend) a ticker.
	expiration := &tickerExpiration{t: time.NewTicker(c.expiry)}
	newExpiry := time.Now().Add(c.expiry)
	ctx, cancel := context.WithCancel(context.Background())
	sn = &session{client: dc, cancel: cancel, expiry: c.expiry, expiration: expiration, log: log}

	c.mx.Lock()
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
			c.remove(id)
		case <-ctx.Done():
			log.Debug("Client stopped")
			// We're done for some other reason (e.g. the cache crashed). We assume
			// whatever cancelled our context did so by calling done() - we just need
			// to let this goroutine finish.
		}
	}()

	if !ca.WaitForCacheSync(ctx) {
		c.remove(id)
		return nil, errors.New(errWaitForCacheSync)
	}

	log.Debug("Created client",
		"duration", time.Since(started),
		"new-expiry", newExpiry,
	)

	return sn, nil
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
	expiry     time.Duration
	expiration expiration

	log logging.Logger
}

func (s *session) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	t := time.Now()
	s.expiration.Reset(s.expiry)
	err := s.client.Get(ctx, key, obj)
	s.log.Debug("Client called",
		"operation", "Get",
		"duration", time.Since(t),
		"new-expiry", t.Add(s.expiry),
	)
	return err
}

func (s *session) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	t := time.Now()
	s.expiration.Reset(s.expiry)
	err := s.client.List(ctx, list, opts...)
	s.log.Debug("Client called",
		"operation", "List",
		"duration", time.Since(t),
		"new-expiry", t.Add(s.expiry),
	)
	return err
}

func (s *session) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	t := time.Now()
	s.expiration.Reset(s.expiry)
	err := s.client.Create(ctx, obj, opts...)
	s.log.Debug("Client called",
		"operation", "Create",
		"duration", time.Since(t),
		"new-expiry", t.Add(s.expiry),
	)
	return err
}

func (s *session) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	t := time.Now()
	s.expiration.Reset(s.expiry)
	err := s.client.Delete(ctx, obj, opts...)
	s.log.Debug("Client called",
		"operation", "Delete",
		"duration", time.Since(t),
		"new-expiry", t.Add(s.expiry),
	)
	return err
}

func (s *session) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	t := time.Now()
	s.expiration.Reset(s.expiry)
	err := s.client.Update(ctx, obj, opts...)
	s.log.Debug("Client called",
		"operation", "Update",
		"duration", time.Since(t),
		"new-expiry", t.Add(s.expiry),
	)
	return err
}

func (s *session) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	t := time.Now()
	s.expiration.Reset(s.expiry)
	err := s.client.Patch(ctx, obj, patch, opts...)
	s.log.Debug("Client called",
		"operation", "Patch",
		"duration", time.Since(t),
		"new-expiry", t.Add(s.expiry),
	)
	return err
}

func (s *session) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	t := time.Now()
	s.expiration.Reset(s.expiry)
	err := s.client.DeleteAllOf(ctx, obj, opts...)
	s.log.Debug("Client called",
		"operation", "DeleteallOf",
		"duration", time.Since(t),
		"new-expiry", t.Add(s.expiry),
	)
	return err
}

func (s *session) Status() client.StatusWriter {
	t := time.Now()
	s.expiration.Reset(s.expiry)
	sw := s.client.Status()
	s.log.Debug("Client called",
		"operation", "Status",
		"duration", time.Since(t),
		"new-expiry", t.Add(s.expiry),
	)
	return sw
}

func (s *session) Scheme() *runtime.Scheme {
	t := time.Now()
	s.expiration.Reset(s.expiry)
	sc := s.client.Scheme()
	s.log.Debug("Client called",
		"operation", "Scheme",
		"duration", time.Since(t),
		"new-expiry", t.Add(s.expiry),
	)
	return sc
}

func (s *session) RESTMapper() meta.RESTMapper {
	t := time.Now()
	s.expiration.Reset(s.expiry)
	rm := s.client.RESTMapper()
	s.log.Debug("Client called",
		"operation", "Scheme",
		"duration", time.Since(t),
		"new-expiry", t.Add(s.expiry),
	)
	return rm
}

// SubResource returns the underlying client's SubResource client, unwrapped.
func (s *session) SubResource(subResource string) client.SubResourceClient {
	return s.client.SubResource(subResource)
}
