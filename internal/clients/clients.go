package clients

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

const expiry = 5 * time.Minute

// Config returns a REST config.
func Config() (*rest.Config, error) {
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

// WithoutBearerToken returns a copy of the supplied REST config wihout its own
// bearer token. This allows a bearer token to be injected into the config at
// client creation time.
func WithoutBearerToken(cfg *rest.Config) *rest.Config {
	out := rest.CopyConfig(cfg)
	out.BearerToken = ""
	out.BearerTokenFile = ""
	return out
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
		salt:   salt,
		log:    logging.NewNopLogger(),
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
func (c *Cache) Get(token string, o ...GetOption) (client.Client, error) {
	opts := &getOptions{}
	for _, fn := range o {
		fn(opts)
	}

	// TODO(negz): Is this sufficient to anonymize our bearer token?
	h := sha256.New()
	_, _ = h.Write(c.salt)
	_, _ = h.Write([]byte(token))
	_, _ = h.Write([]byte(opts.Namespace))
	id := fmt.Sprintf("%x", h.Sum(nil))

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

	cfg := rest.CopyConfig(c.cfg)
	cfg.BearerToken = token
	cfg.BearerTokenFile = ""

	wc, err := client.New(cfg, client.Options{Scheme: c.scheme, Mapper: c.mapper})
	if err != nil {
		return nil, errors.Wrap(err, "cannot create write client")
	}

	ca, err := cache.New(cfg, cache.Options{Scheme: c.scheme, Mapper: c.mapper})
	if err != nil {
		return nil, errors.Wrap(err, "cannot create cache")
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
		return nil, errors.Wrap(err, "cannot create delegating client")
	}

	// We use a distinct expiry ticker rather than a context deadline or timeout
	// because it's not possible to extend a context's deadline or timeout, but it
	// is possible to 'reset' (i.e. extend) a ticker.
	expired := time.NewTicker(expiry)
	newExpiry := time.Now().Add(expiry)
	ctx, cancel := context.WithCancel(context.Background())
	sn = &session{client: dc, cancel: cancel, expired: expired, log: log}

	c.mx.Lock()
	c.active[id] = sn
	c.mx.Unlock()

	go func() {
		_ = ca.Start(ctx)

		// Start blocks until ctx is closed, or it encounters an error. If we make
		// it here either the cache crashed, or the context was cancelled (e.g.
		// because our session expired).
		c.remove(id)
	}()

	// Stop our cache when we expire.
	go func() {
		select {
		case <-expired.C:
			// We expired, and should remove ourself from the session cache.
			c.remove(id)
		case <-ctx.Done():
			// We're done for some other reason (e.g. the cache crashed). We assume
			// whatever cancelled our context did so by calling done() - we just need
			// to let this goroutine finish.
		}
	}()

	if !ca.WaitForCacheSync(ctx) {
		c.remove(id)
		return nil, errors.New("cannot sync cache")
	}

	log.Debug("Created new client",
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
		sn.expired.Stop()
		delete(c.active, id)
		c.log.Debug("Removed client from cache", "client-id", id)
	}
}

type session struct {
	client  client.Client
	cancel  context.CancelFunc
	expired *time.Ticker

	log logging.Logger
}

func (s *session) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	t := time.Now()
	s.expired.Reset(expiry)
	err := s.client.Get(ctx, key, obj)
	s.log.Debug("Client called",
		"operation", "Get",
		"duration", time.Since(t),
		"new-expiry", t.Add(expiry),
	)
	return err
}

func (s *session) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	t := time.Now()
	s.expired.Reset(expiry)
	err := s.client.List(ctx, list, opts...)
	s.log.Debug("Client called",
		"operation", "List",
		"duration", time.Since(t),
		"new-expiry", t.Add(expiry),
	)
	return err
}

func (s *session) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	t := time.Now()
	s.expired.Reset(expiry)
	err := s.client.Create(ctx, obj, opts...)
	s.log.Debug("Client called",
		"operation", "Create",
		"duration", time.Since(t),
		"new-expiry", t.Add(expiry),
	)
	return err
}

func (s *session) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	t := time.Now()
	s.expired.Reset(expiry)
	err := s.client.Delete(ctx, obj, opts...)
	s.log.Debug("Client called",
		"operation", "Delete",
		"duration", time.Since(t),
		"new-expiry", t.Add(expiry),
	)
	return err
}

func (s *session) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	t := time.Now()
	s.expired.Reset(expiry)
	err := s.client.Update(ctx, obj, opts...)
	s.log.Debug("Client called",
		"operation", "Update",
		"duration", time.Since(t),
		"new-expiry", t.Add(expiry),
	)
	return err
}

func (s *session) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	t := time.Now()
	s.expired.Reset(expiry)
	err := s.client.Patch(ctx, obj, patch, opts...)
	s.log.Debug("Client called",
		"operation", "Patch",
		"duration", time.Since(t),
		"new-expiry", t.Add(expiry),
	)
	return err
}

func (s *session) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	t := time.Now()
	s.expired.Reset(expiry)
	err := s.client.DeleteAllOf(ctx, obj, opts...)
	s.log.Debug("Client called",
		"operation", "DeleteallOf",
		"duration", time.Since(t),
		"new-expiry", t.Add(expiry),
	)
	return err
}

func (s *session) Status() client.StatusWriter {
	t := time.Now()
	s.expired.Reset(expiry)
	err := s.client.Status()
	s.log.Debug("Client called",
		"operation", "Status",
		"duration", time.Since(t),
		"new-expiry", t.Add(expiry),
	)
	return err
}

func (s *session) Scheme() *runtime.Scheme {
	t := time.Now()
	s.expired.Reset(expiry)
	err := s.client.Scheme()
	s.log.Debug("Client called",
		"operation", "Scheme",
		"duration", time.Since(t),
		"new-expiry", t.Add(expiry),
	)
	return err
}

func (s *session) RESTMapper() meta.RESTMapper {
	t := time.Now()
	s.expired.Reset(expiry)
	err := s.client.RESTMapper()
	s.log.Debug("Client called",
		"operation", "Scheme",
		"duration", time.Since(t),
		"new-expiry", t.Add(expiry),
	)
	return err
}
