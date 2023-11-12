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
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/upbound/xgql/internal/clients"
	"github.com/upbound/xgql/internal/live_query"
)

// WithLiveQueries wraps NewCacheFn with a cache.Cache that tracks objects
// and object lists and notifies the live query in request context of changes.
func WithLiveQueries(fn clients.NewCacheFn) clients.NewCacheFn {
	return func(cfg *rest.Config, o cache.Options) (cache.Cache, error) {
		c, err := fn(cfg, o)
		if err != nil {
			return nil, err
		}
		return &liveQueryCache{
			Cache:   c,
			scheme:  o.Scheme,
			queries: make(map[uint64]*liveQueryTracker),
			handles: make(set[schema.GroupVersionKind]),
		}, nil
	}
}

var _ toolscache.ResourceEventHandler = (*liveQueryCache)(nil)

// liveQueryCache is a cache.Cache that registers cache.Informer listeners for any
// retrieved object if executed in the context of a live query. When liveQueryCache
// is notified of events, it will trigger any active live queries.
type liveQueryCache struct {
	cache.Cache
	scheme *runtime.Scheme

	lock    sync.Mutex
	queries map[uint64]*liveQueryTracker
	handles set[schema.GroupVersionKind]
}

// Get implements cache.Cache. It wraps an underlying cache.Cache and sets up an Informer
// event handler that marks current live query as dirty if the current context has a live query.
func (c *liveQueryCache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if err := c.Cache.Get(ctx, key, obj, opts...); err != nil {
		return err
	}
	return c.trackObject(ctx, obj)
}

// List implements cache.Cache. It wraps an underlying cache.Cache and sets up an Informer
// event handler that marks current live query as dirty if the current context has a live query.
func (c *liveQueryCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if err := c.Cache.List(ctx, list, opts...); err != nil {
		return err
	}
	return c.trackObject(ctx, list)
}

// trackObject registers object or object list with a tracker for the live query.
// any updated from cache.Informer is broadcast to all live query trackers, if the
// changed object is tracked by a given liveQueryTracker, the live query associated
// with the tracker is Trigger()'d.
func (c *liveQueryCache) trackObject(ctx context.Context, object runtime.Object) error {
	qid, ok := live_query.IsLive(ctx)
	// if this isn't a live query context, skip.
	if !ok {
		return nil
	}
	gvk, err := apiutil.GVKForObject(object, c.scheme)
	if err != nil {
		return err
	}
	if _, ok := object.(client.ObjectList); ok {
		// We need the non-list GVK, so chop off the "List" from the end of the kind.
		gvk.Kind = strings.TrimSuffix(gvk.Kind, "List")
	}
	i, err := c.getInformer(ctx, object, gvk)
	if err != nil {
		return err
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	// register event handler for the GVK that if we aren't watching it already.
	if c.handles.Add(gvk) {
		if _, err := i.AddEventHandler(c); err != nil {
			c.handles.Remove(gvk)
			return err
		}
	}
	// register live query tracker if we're not tracking it already.
	q, ok := c.queries[qid]
	if !ok {
		q = newLiveQueryTracker(ctx)
		c.queries[qid] = q
	}
	// register object or object list with the live query tracker.
	switch o := object.(type) {
	case client.Object:
		q.Track(o.GetUID(), gvk)
	case client.ObjectList:
		q.TrackList(gvk)
	}
	return nil
}

// getInformer gets cache.Informer for object and gvk.
func (c *liveQueryCache) getInformer(ctx context.Context, object runtime.Object, gvk schema.GroupVersionKind) (cache.Informer, error) {
	// Handle unstructured.UnstructuredList.
	if _, isUnstructured := object.(runtime.Unstructured); isUnstructured {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		return c.Cache.GetInformer(ctx, u)
	}
	// Handle metav1.PartialObjectMetadataList.
	if _, isPartialObjectMetadata := object.(*metav1.PartialObjectMetadataList); isPartialObjectMetadata {
		pom := &metav1.PartialObjectMetadata{}
		pom.SetGroupVersionKind(gvk)
		return c.Cache.GetInformer(ctx, pom)
	}
	return c.Cache.GetInformerForKind(ctx, gvk)
}

// OnAdd implements cache.ResourceEventHandler.
// Broadcasts the object change to all live query trackers after the initial sync.
func (c *liveQueryCache) OnAdd(obj interface{}, isInInitialList bool) {
	// we don't care about initial sync
	if isInInitialList {
		return
	}
	object, ok := obj.(client.Object)
	if !ok {
		return
	}
	gvk, err := apiutil.GVKForObject(object, c.scheme)
	if err != nil {
		return
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	for i := range c.queries {
		if !c.queries[i].IsLive() {
			delete(c.queries, i)
			continue
		}
		c.queries[i].OnCreate(object, gvk)
	}
}

// OnDelete implements cache.ResourceEventHandler.
// Broadcasts the object change to all live query trackers after the initial sync.
func (c *liveQueryCache) OnDelete(obj interface{}) {
	object, ok := obj.(client.Object)
	if !ok {
		return
	}
	gvk, err := apiutil.GVKForObject(object, c.scheme)
	if err != nil {
		return
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	for i := range c.queries {
		if !c.queries[i].IsLive() {
			delete(c.queries, i)
			continue
		}
		c.queries[i].OnDelete(object, gvk)
	}
}

// OnUpdate implements cache.ResourceEventHandler.
// Broadcasts the object change to all live query trackers after the initial sync.
func (c *liveQueryCache) OnUpdate(oldObj interface{}, newObj interface{}) {
	oldObject, ok := oldObj.(client.Object)
	if !ok {
		return
	}
	newObject, ok := newObj.(client.Object)
	if !ok {
		return
	}
	gvk, err := apiutil.GVKForObject(oldObject, c.scheme)
	if err != nil {
		return
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	for i := range c.queries {
		// cleanup any stale queries.
		if !c.queries[i].IsLive() {
			delete(c.queries, i)
			continue
		}
		c.queries[i].OnUpdate(oldObject, newObject, gvk)
	}
}

func newLiveQueryTracker(ctx context.Context) *liveQueryTracker {
	return &liveQueryTracker{ctx: ctx, oids: make(map[schema.GroupVersionKind]set[types.UID])}
}

// liveQueryTracker tracks objects of the same GVK for one live query.
// it can track individual objects as in when cache.Cache.Get() is
// called or the entire list when cache.Cache.List() is used.
type liveQueryTracker struct {
	ctx context.Context

	lock sync.Mutex
	oids map[schema.GroupVersionKind]set[types.UID]
}

// IsLive returns true if live query is still active.
func (q *liveQueryTracker) IsLive() bool {
	if _, ok := live_query.IsLive(q.ctx); ok {
		return true
	}
	return false
}

// OnCreate will notify the live query if tracking the entire GVK list.
func (q *liveQueryTracker) OnCreate(object client.Object, gvk schema.GroupVersionKind) {
	var notify bool
	// notify without holding the lock
	defer func() {
		if notify {
			live_query.Trigger(q.ctx)
		}
	}()
	q.lock.Lock()
	defer q.lock.Unlock()
	oids, ok := q.oids[gvk]
	notify = ok && oids == nil
}

// OnUpdate will notify the live query if tracking either object or the entire GVK list.
func (q *liveQueryTracker) OnUpdate(oldObject, newObject client.Object, gvk schema.GroupVersionKind) {
	var notify bool
	// notify without holding the lock
	defer func() {
		if notify {
			live_query.Trigger(q.ctx)
		}
	}()
	q.lock.Lock()
	defer q.lock.Unlock()
	oids, ok := q.oids[gvk]
	// notify if tracking gvk list or either of the objects.
	notify = ok && (oids == nil || oids.Contains(oldObject.GetUID()) || oids.Contains(newObject.GetUID()))
}

// OnDelete will notify the live query if tracking the object or the entire GVK list.
func (q *liveQueryTracker) OnDelete(object client.Object, gvk schema.GroupVersionKind) {
	var notify bool
	// notify without holding the lock
	defer func() {
		if notify {
			live_query.Trigger(q.ctx)
		}
	}()
	q.lock.Lock()
	defer q.lock.Unlock()
	oids, ok := q.oids[gvk]
	// notify if tracking gkv list or object.
	notify = ok && (oids == nil || oids.Remove(object.GetUID()))
}

// Track registers object for tracking.
func (q *liveQueryTracker) Track(oid types.UID, gvk schema.GroupVersionKind) {
	q.lock.Lock()
	defer q.lock.Unlock()
	if uids, ok := q.oids[gvk]; ok {
		// already tracking the entire list, skip.
		if uids == nil {
			return
		}
		// add object to track.
		uids.Add(oid)
		return
	}
	// register event handler for the new GVK.
	// track object.
	q.oids[gvk] = set[types.UID]{oid: struct{}{}}
}

// TrackList begins tacking all objects of a given GVK.
func (q *liveQueryTracker) TrackList(gvk schema.GroupVersionKind) {
	q.lock.Lock()
	defer q.lock.Unlock()
	// track list.
	q.oids[gvk] = nil
}
