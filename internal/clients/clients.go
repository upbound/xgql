package clients

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const expiry = 5 * time.Minute

// AnonymousConfig returns a REST config with no bearer token (or bearer token
// file) set. The config follows the controller-runtime precedence.
func AnonymousConfig() (*rest.Config, error) {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, errors.Wrap(err, "cannot create in-cluster configuration")
	}

	cfg.BearerTokenFile = ""
	cfg.BearerToken = ""

	// ctrl.GetConfig tunes QPS and burst for Kubernetes controllers. We're not
	// a controller and we expect to be creating many clients, so we tune these
	// back down to the client-go defaults.
	cfg.QPS = 5
	cfg.Burst = 10

	return cfg, nil
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

	cfg    *rest.Config
	scheme *runtime.Scheme
}

// NewCache creates a cache of Kubernetes clients. Clients use the supplied
// scheme, and connect to the API server using a copy of the supplied REST
// config with a specific bearer token injected.
func NewCache(s *runtime.Scheme, c *rest.Config) *Cache {
	return &Cache{
		active: make(map[string]*session),
		cfg:    c,
		scheme: s,
	}
}

// Get a client that uses the specified bearer token.
func (c *Cache) Get(token string) (client.Client, error) {
	c.mx.RLock()
	sn, ok := c.active[token]
	c.mx.RUnlock()

	if ok {
		return sn, nil
	}

	cfg := rest.CopyConfig(c.cfg)
	cfg.BearerToken = token
	cfg.BearerTokenFile = ""

	wc, err := client.New(cfg, client.Options{Scheme: c.scheme})
	if err != nil {
		return nil, errors.Wrap(err, "cannot create write client")
	}

	ca, err := cache.New(cfg, cache.Options{Scheme: c.scheme})
	if err != nil {
		return nil, errors.Wrap(err, "cannot create cache")
	}

	// TODO(negz): Is there any issue caching unstructured objects? The docstring
	// for client.delegatingReader implies it could result in 'unexpectedly
	// caching the entire cluster' if arbitrary references are loaded.
	dc, err := client.NewDelegatingClient(client.NewDelegatingClientInput{CacheReader: ca, Client: wc, CacheUnstructured: true})
	if err != nil {
		return nil, errors.Wrap(err, "cannot create delegating client")
	}

	// We use a distinct expiry ticker rather than a context deadline or timeout
	// because it's not possible to extend a context's deadline or timeout, but it
	// is possible to 'reset' (i.e. extend) a ticker.
	expired := time.NewTicker(expiry)
	ctx, cancel := context.WithCancel(context.Background())
	sn = &session{client: dc, cancel: cancel, expired: expired}

	c.mx.Lock()
	c.active[token] = sn
	c.mx.Unlock()

	go func() {
		_ = ca.Start(ctx)

		// Start blocks until ctx is closed, or it encounters an error. If we make
		// it here either the cache crashed, or the context was cancelled (e.g.
		// because our session expired).
		c.remove(token)
	}()

	// Stop our cache when we expire.
	go func() {
		select {
		case <-expired.C:
			// We expired, and should remove ourself from the session cache.
			c.remove(token)
		case <-ctx.Done():
			// We're done for some other reason (e.g. the cache crashed). We assume
			// whatever cancelled our context did so by calling done() - we just need
			// to let this goroutine finish.
		}
	}()

	if !ca.WaitForCacheSync(ctx) {
		c.remove(token)
		return nil, errors.New("cannot sync cache")
	}

	return sn, nil
}

func (c *Cache) remove(token string) {
	c.mx.Lock()
	defer c.mx.Unlock()

	if sn, ok := c.active[token]; ok {
		sn.cancel()
		sn.expired.Stop()
		delete(c.active, token)
	}
}

type session struct {
	client  client.Client
	cancel  context.CancelFunc
	expired *time.Ticker
}

func (s *session) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	s.expired.Reset(expiry)
	return s.client.Get(ctx, key, obj)
}

func (s *session) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	s.expired.Reset(expiry)
	return s.client.List(ctx, list, opts...)
}

func (s *session) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	s.expired.Reset(expiry)
	return s.client.Create(ctx, obj, opts...)
}

func (s *session) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	s.expired.Reset(expiry)
	return s.client.Delete(ctx, obj, opts...)
}

func (s *session) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	s.expired.Reset(expiry)
	return s.client.Update(ctx, obj, opts...)
}

func (s *session) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	s.expired.Reset(expiry)
	return s.client.Patch(ctx, obj, patch, opts...)
}

func (s *session) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	s.expired.Reset(expiry)
	return s.client.DeleteAllOf(ctx, obj, opts...)
}

func (s *session) Status() client.StatusWriter {
	s.expired.Reset(expiry)
	return s.client.Status()
}

func (s *session) Scheme() *runtime.Scheme {
	s.expired.Reset(expiry)
	return s.client.Scheme()
}

func (s *session) RESTMapper() meta.RESTMapper {
	s.expired.Reset(expiry)
	return s.client.RESTMapper()
}
