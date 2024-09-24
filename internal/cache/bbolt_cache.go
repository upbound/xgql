// Copyright 2023 Upbound Inc
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

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/upbound/xgql/internal/clients"
)

const (
	errObjectPut       = "failed to write object"
	errObjectGet       = "failed to get object"
	errObjectList      = "failed to list object"
	errObjectMarshal   = "failed to marshal object"
	errObjectUnmarshal = "failed to unmarshal object"
	errObjectZero      = "failed to zero out object"
	errBucketCreate    = "failed to create bucket"
	errTxBegin         = "failed to begin transaction"
	errTxRollback      = "failed to rollback transaction"
	errFileCreate      = "failed to create db file"
	errFileClose       = "failed to close db file"
	errFileRemove      = "failed to remove db file"
	errDBOpen          = "failed to open db"
	errCacheCreate     = "failed to create cache"
	errBucketGet       = "failed to get bucket for object"

	errFmtNoBucket   = "bucket %q does not exist"
	errFmtObjectGVK  = "failed to get GVK for %+#v"
	errFmtNoKey      = "key %q does not exist"
	errFmtObjectType = "unable to convert %+#v to client.Object"
)

// NewBoltDBFn is a function that creates a new bolt db.
type NewBoltDBFn func(string) (boltDB, error)

// MarshalFn is a function that marshals an object.
type MarshalFn func(interface{}) ([]byte, error)

// UnmarshalFn is a function that unmarshals an object.
type UnmarshalFn func([]byte, interface{}) error

var (
	DefaultMarshalFn   MarshalFn   = json.Marshal
	DefaultUnmarshalFn UnmarshalFn = json.Unmarshal
)

type boltTxCtxKeyType int

var boltTxCtxKey boltTxCtxKeyType

// boltTxCtx is a context value that holds functions to get objects bucket and commit transaction.
// if boltTxCtxKey is set in context, it will be of this type and will get populated with functions
// when a transaction is needed for reading.
type boltTxCtx struct {
	mu        sync.Mutex
	used      int
	getBucket func() (boltBucket, error)
	done      func() error
}

// BoltTxMiddleware adds a boltTxCtx to request context.
func BoltTxMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), boltTxCtxKey, &boltTxCtx{})))
	})
}

// WithBBoltCache wraps NewCacheFn with a BBoltCache.
func WithBBoltCache(file string, opts ...Option) clients.NewCacheMiddlewareFn {
	return func(fn clients.NewCacheFn) clients.NewCacheFn {
		return func(cfg *rest.Config, o cache.Options) (cache.Cache, error) {
			h := fnv.New32()
			if _, err := h.Write([]byte(cfg.String())); err != nil {
				return nil, err
			}
			// since we can have many clients for different credentials existing concurrently,
			// and because boltdb cannot be opened for modification concurrently,
			// add rest.Config hash suffix the the cache file.
			file = fmt.Sprintf("%s.%d", file, h.Sum32())
			bc, err := NewBBoltCache(nil, o.Scheme, file, opts...)
			if err != nil {
				return nil, err
			}
			// set or wrap DefaultTransform.
			o.DefaultTransform = wrapCacheTranform(o.DefaultTransform, bc.putObject)
			// get undelying object from cache without copying.
			o.DefaultUnsafeDisableDeepCopy = ptr.To(true)
			// deep copy objects before returning by default.
			bc.deepCopy = o.DefaultUnsafeDisableDeepCopy == nil || !*o.DefaultUnsafeDisableDeepCopy
			ca, err := fn(cfg, o)
			if err != nil {
				return nil, err
			}
			bc.Cache = ca
			return bc, nil
		}
	}
}

// wrapCacheTranform returns next if prev is null or creates a new toolscache.TransformFunc
// that will call both prev and next sequentially.
func wrapCacheTranform(prev, next toolscache.TransformFunc) toolscache.TransformFunc {
	if prev == nil {
		return next
	}
	return func(i interface{}) (interface{}, error) {
		i, err := next(i)
		if err != nil {
			return nil, err
		}
		return prev(i)
	}
}

// Option is an option for a cache.
type Option func(*BBoltCache)

// WithLogger wires a logger into the bbolt cache.
func WithLogger(o logging.Logger) Option {
	return func(c *BBoltCache) {
		c.log = o
	}
}

// Cache implements cache.Cache.
var _ cache.Cache = &BBoltCache{}

// BBoltCache is a wrapper around a cache.Cache
// that periodically persists objects to disk and zeroes them out in memory.
type BBoltCache struct {
	cache.Cache
	scheme *runtime.Scheme
	db     boltDB
	file   string
	bucket []byte

	// defaults to true unless cache was created with UnsafeDisableDeepCopy.
	deepCopy bool

	cleaner Cleaner[client.Object]

	marshalFn   MarshalFn
	unmarshalFn UnmarshalFn

	running atomic.Bool

	log logging.Logger
}

// NewBBoltCache creates a new cache.
func NewBBoltCache(cache cache.Cache, scheme *runtime.Scheme, file string, opts ...Option) (*BBoltCache, error) {
	ca := &BBoltCache{
		Cache:       cache,
		file:        file,
		bucket:      []byte("objects"),
		scheme:      scheme,
		marshalFn:   DefaultMarshalFn,
		unmarshalFn: DefaultUnmarshalFn,
	}
	for _, opt := range opts {
		opt(ca)
	}
	if ca.db == nil {
		db, err := newBoltDB(ca.file)
		if err != nil {
			return nil, errors.Wrap(err, errDBOpen)
		}
		ca.db = db
	}
	if ca.cleaner == nil {
		ca.cleaner = NewCleaner[client.Object, string](getKey, ca.cleanup, WithLoggerCleanerOpt[client.Object, string](ca.log))
	}
	return ca, nil
}

// cleanup is called by cleaner when objects expire. they will be then
// stored in the "objects" bucket in boltdb and zeroed out in memory.
func (c *BBoltCache) cleanup(objects []client.Object) error {
	// sort by key for better transaction performance
	// https://github.com/etcd-io/bbolt#caveats--limitations.
	sort.Slice(objects, func(i, j int) bool {
		return getKey(objects[i]) < getKey(objects[j])
	})
	// write objects to the bucket and zero them out in memory.
	return c.db.Update(func(tx boltTx) error {
		b, err := tx.CreateBucketIfNotExists(c.bucket)
		if err != nil {
			return errors.Wrap(err, errBucketCreate)
		}
		for i := range objects {
			k := []byte(getKey(objects[i]))
			v, err := c.marshalFn(objects[i])
			if err != nil {
				return errors.Wrap(err, errObjectMarshal)
			}
			if err := b.Put(k, v); err != nil {
				return errors.Wrap(err, errObjectPut)
			}
			if err := setZero(objects[i]); err != nil {
				return errors.Wrap(err, errObjectZero)
			}
		}
		return nil
	})
}

// getGVK returns schema.GroupVersionKind for a client.Object or client.ObjectList.
func (c *BBoltCache) getGVK(obj runtime.Object) (schema.GroupVersionKind, error) {
	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	if _, ok := obj.(client.ObjectList); ok {
		gvk.Kind = strings.TrimSuffix(gvk.Kind, "List")
	}
	return gvk, nil
}

// Get implements cache.Cache.
// We delegate to the underlying cache. After retrieving the object, we reschedule its cleanup,
// re-hydrate its data from the database if empty, and deep copy it before returning.
func (c *BBoltCache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	// delegate to the underlying cache, which will start the informers
	// and begin calling Cache.putObject for each object.
	if err := c.Cache.Get(ctx, key, obj, opts...); err != nil {
		return err
	}
	getBucket, done := c.txOnce(ctx)
	return errors.Wrap(errors.Join(c.rehydrateObject(obj, c.deepCopy, getBucket), done()), errObjectGet)
}

// List implements cache.Cache.
// We find the bucket for storage based on the object's GVK.
// If we haven't seend this GVK before, we create a new errgroup.Group
// that will be used to concurrently persist all discovered objects
// to disk.
func (c *BBoltCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	// collect list options as we'll use them below to respect deep copy preference.
	listOpts := client.ListOptions{}
	listOpts.ApplyOptions(opts)
	// delegate to the underlying cache, which will start the informers
	// and begin calling Cache.putObject for each object.
	if err := c.Cache.List(ctx, list, &listOpts); err != nil {
		return err
	}
	n := apimeta.LenList(list)
	// if the list is empty, we're done.
	if n == 0 {
		return nil
	}
	getBucket, done := c.txOnce(ctx)
	// deepCopy output object before returning
	deepCopy := c.deepCopy && (listOpts.UnsafeDisableDeepCopy == nil || !*listOpts.UnsafeDisableDeepCopy)
	return errors.Wrap(errors.Join(c.rehydrateObjectList(list, deepCopy, getBucket), done()), errObjectList)
}

// txOnce creates functions to get boltBucket and rollback read-only transaction.
// if provided context contains a boltTxCtxKey of type *boltTxCtx, then it will be used to
// get a shared read transaction for the entire request that will be rolled back when the
// last reader calls done(). otherwise will return functions that will create a unique read
// transaction.
func (c *BBoltCache) txOnce(ctx context.Context) (func() (boltBucket, error), func() error) {
	if bx, ok := ctx.Value(boltTxCtxKey).(*boltTxCtx); ok {
		bx.mu.Lock()
		defer bx.mu.Unlock()
		// each call to txOnce increments the used counter.
		bx.used++
		// if we have already set these functions, return them
		if bx.getBucket != nil {
			return bx.getBucket, bx.done
		}
		var (
			tx boltTx
			b  boltBucket
		)
		bx.getBucket = func() (boltBucket, error) {
			bx.mu.Lock()
			defer bx.mu.Unlock()
			// we bucket already created, return it.
			if b != nil {
				return b, nil
			}
			var err error
			// start a read-only transaction and memoize it in tx.
			tx, err = c.db.Begin(false)
			if err != nil {
				return nil, errors.Wrap(err, errTxBegin)
			}
			// retrieve the "objects" bucket and memoize it in b.
			b = tx.Bucket(c.bucket)
			return b, nil
		}
		bx.done = func() error {
			bx.mu.Lock()
			defer bx.mu.Unlock()
			bx.used--
			if bx.used > 0 {
				return nil
			}
			if tx == nil {
				return nil
			}
			// we have an active tx and b, and no more users, roll it back and clear.
			defer func() { tx, b = nil, nil }()
			return errors.Wrap(tx.Rollback(), errTxRollback)
		}
		return bx.getBucket, bx.done
	}
	var tx boltTx
	return sync.OnceValues(func() (boltBucket, error) {
			var err error
			tx, err = c.db.Begin(false)
			if err != nil {
				return nil, errors.Wrap(err, errTxBegin)
			}
			return tx.Bucket(c.bucket), err
		}),
		sync.OnceValue(func() error {
			if tx == nil {
				return nil
			}
			return errors.Wrap(tx.Rollback(), errTxRollback)
		})
}

// rehydrateObject reschedules the object for cleanup,
// and rehydrates it from bolt db bucket if it is currently zeroed out.
func (c *BBoltCache) rehydrateObject(object client.Object, deepCopy bool, getBucket func() (boltBucket, error)) (rErr error) {
	// zero out object after duration.
	c.cleaner.Schedule(object, 1*time.Minute)
	// deepcopy object before returning.
	if deepCopy {
		gvk, err := c.getGVK(object)
		if err != nil {
			return errors.Wrapf(err, errFmtObjectGVK, object)
		}
		defer func() {
			if rErr != nil {
				return
			}
			reflect.Indirect(reflect.ValueOf(object)).Set(reflect.Indirect(reflect.ValueOf(object.DeepCopyObject())))
			object.GetObjectKind().SetGroupVersionKind(gvk)
		}()
	}
	if !isZero(object) {
		return nil
	}
	b, err := getBucket()
	if err != nil {
		return err
	}
	if b == nil {
		return errors.Errorf(errFmtNoBucket, c.bucket)
	}
	k := getKey(object)
	v := b.Get([]byte(k))
	if v == nil {
		return errors.Errorf(errFmtNoKey, k)
	}
	return errors.Wrap(c.unmarshalFn(v, object), errObjectUnmarshal)
}

// rehydrateObjectList calls rehydrateObject for each object in the list.
func (c *BBoltCache) rehydrateObjectList(list client.ObjectList, deepCopy bool, getBucket func() (boltBucket, error)) (rErr error) {
	// here we set up a func to iterate over all objects in the list,
	// rescheduling clean up and re-hydrating any empty objects from bolt db.
	rehydrateObject := c.rehydrateObject
	if deepCopy {
		// get GVK for deepcopy
		gvk, err := c.getGVK(list)
		if err != nil {
			return errors.Wrapf(err, errFmtObjectGVK, list)
		}
		objsCopy := make([]runtime.Object, 0, apimeta.LenList(list))
		// wrap original rescheduleCleanupAndRehydrateObjectIfZero to copy objects.
		rehydrateObject = func(object client.Object, _ bool, getBucket func() (boltBucket, error)) error {
			if err := c.rehydrateObject(object, false, getBucket); err != nil {
				return err
			}
			// create a deep copy.
			obj := object.DeepCopyObject()
			obj.GetObjectKind().SetGroupVersionKind(gvk)
			objsCopy = append(objsCopy, obj)
			return nil
		}
		// replace list with copy.
		defer func() {
			if rErr != nil {
				return
			}
			rErr = errors.Wrap(apimeta.SetList(list, objsCopy), errObjectList)
		}()
	}
	return apimeta.EachListItem(list, func(obj runtime.Object) error {
		object, ok := obj.(client.Object)
		if !ok {
			return errors.Errorf(errFmtObjectType, obj)
		}
		return rehydrateObject(object, false, getBucket)
	})
}

// Start implements cache.Cache.
// it will close the bolt db when the cache is stopped.
func (c *BBoltCache) Start(ctx context.Context) (rErr error) {
	if !c.running.CompareAndSwap(false, true) {
		return errors.New("already running")
	}
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		<-ctx.Done()
		return errors.Join(c.db.Close(), os.Remove(c.file))
	})
	g.Go(func() error {
		return c.cleaner.Start(ctx)
	})
	g.Go(func() error {
		return c.Cache.Start(ctx)
	})
	return g.Wait()
}

// putObject schedules a cleanup for the cached object to reduce memory pressure.
func (c *BBoltCache) putObject(obj interface{}) (interface{}, error) {
	if !c.running.Load() {
		return nil, errors.New("not running")
	}
	if object, ok := obj.(client.Object); ok {
		c.cleaner.Schedule(object, 1*time.Minute)
	}
	return obj, nil
}

// setZero zeros out the supplied object, preserving its namespace and name.
func setZero(obj client.Object) error {
	// restore object uid
	defer obj.SetUID(obj.GetUID())
	return runtime.SetZeroValue(obj)
}

// isZero checks if the object has been zeroed out.
func isZero(obj client.Object) bool {
	return obj.GetName() == ""
}

// index objects by their UID.
func getKey(object client.Object) string {
	return string(object.GetUID())
}
