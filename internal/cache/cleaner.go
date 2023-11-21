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
	"cmp"
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/utils/clock"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// Cleaner cleans up objects after a given duration.
type Cleaner[T any] interface {
	// Schedule adds obj to clean in exp interval.
	Schedule(obj T, exp time.Duration)
	// Start runs until context is done.
	Start(context.Context) error
}

var _ Cleaner[any] = (*cleaner[any, int])(nil)

// expKey is object expiry entry, it is going to be cleaned
// when its at timestamp older than current timestamp.
// before cleaning, its gen generation is compared to
// the generation of the ref with the matching key, if
// generations match, the ref is removed and object will
// be cleaned.
type expKey[K cmp.Ordered] struct {
	gen uint64
	key K
	at  time.Time
}

// expRef is an object reference entry. when an expiry key
// reaches its expiration timestamp, its generation is compared
// to the reference's generation, if they match, the reference
// gets cleaned.
type expRef[T any] struct {
	gen uint64
	obj T
}

// cleaner implements Cleaner.
// executes provided cleanFn for all objects that have
// expired at a given time.
// uses keyFn for deduplicating and keeping object references.
// all scheduled cleanups are kept in exps slice ordered by
// at time.Time.
// all objects are help in refs map indexed by key generated
// using provided keyFn.
// When an object is Schedule'd for cleanup, a new expKey
// entry is added to the exps slice at an index determined
// by the at time. The scheduled object is added to the refs
// map and its gen is incremented. The new value of gen is
// stored with the expKey entry.
// When clock reaches the expiration time of one of the
// scheduled objects, its expKey gen is compared with the
// expRef entry is the refs map, object is going to be cleaned
// up using the cleanFn if values of gen match. This allows
// scheduling object repeatedly without modifying existing
// exps entries and only the last schedule being respected.
// there is a potential for an unexpected cleanup if a previously
// scheduled object is rescheduled at a lower time and then again
// at a higher time, leaving a conflicting dangling expKey that
// should be ignored. To fix this, we can add a cleaner level
// gen counter, but since we're only scheduling cleanups at a
// constant interval, this is not needed.
type cleaner[T any, K cmp.Ordered] struct {
	keyFn   func(T) K
	cleanFn func([]T) error

	tick    time.Duration
	clock   clock.Clock
	signal  chan struct{}
	running atomic.Bool
	mu      sync.Mutex
	exps    []expKey[K]
	refs    map[K]expRef[T]

	log logging.Logger
}

type CleanerOpt[T any, K cmp.Ordered] func(*cleaner[T, K])

// WithLoggerCleanerOpt wires the logger into the cleaner.
func WithLoggerCleanerOpt[T any, K cmp.Ordered](log logging.Logger) CleanerOpt[T, K] {
	return func(c *cleaner[T, K]) {
		c.log = log
	}
}

// WithTick sets the tick interval for the cleaner. Set it to zero to clean up
// as soon as possible.
func WithTick[T any, K cmp.Ordered](tick time.Duration) CleanerOpt[T, K] {
	return func(c *cleaner[T, K]) {
		c.tick = tick
	}
}

// NewCleaner creates a cleaner for objects of type T, identified by comparable key K,
// using cleanFn for cleanup.
func NewCleaner[T any, K cmp.Ordered](keyFn func(T) K, cleanFn func([]T) error, opts ...CleanerOpt[T, K]) *cleaner[T, K] {
	c := &cleaner[T, K]{
		keyFn:   keyFn,
		cleanFn: cleanFn,
		tick:    time.Second,
		clock:   clock.RealClock{},
		signal:  make(chan struct{}),
		refs:    make(map[K]expRef[T]),
		log:     logging.NewNopLogger(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Schedule implements Cleaner.
func (c *cleaner[T, K]) Schedule(obj T, exp time.Duration) {
	if exp < 0 {
		panic("negative expiration")
	}
	if c.add(obj, c.clock.Now().Add(exp)) {
		select {
		case c.signal <- struct{}{}:
		default:
		}
	}
}

// Start implements Cleaner.
func (c *cleaner[T, K]) Start(ctx context.Context) error {
	if !c.running.CompareAndSwap(false, true) {
		return errors.New("already running")
	}
	defer c.running.Store(false)
	var wakeup <-chan time.Time
	for {
		select {
		case <-wakeup:
			wakeup = nil
		case <-c.signal:
		case <-ctx.Done():
			return nil
		}
		objs, exp := c.collect(c.clock.Now())
		if len(objs) > 0 {
			c.log.Debug("cleaning up objects", "count", len(objs), "next", exp)
			if err := c.cleanFn(objs); err != nil {
				// exit Start and stop the cache
				return err
			}
		}
		if !exp.IsZero() {
			after := exp.Sub(c.clock.Now())
			if after < c.tick {
				after = c.tick
			}
			wakeup = c.clock.After(after)
		}
	}
}

// add adds a expKey to the exps slices and a expRef to the refs map.
// if new expKey is added at the beginning of the slice because its
// expiration is sooner than any existing entries, add returns true.
// This is used to signal the select in the cleanup goroutine loop
// to re-set the wakeup timer.
func (c *cleaner[T, K]) add(obj T, at time.Time) bool {
	// generate object's key using the provided keyFn.
	k := c.keyFn(obj)
	c.mu.Lock()
	defer c.mu.Unlock()
	// determine if this is going to be the first key to expire.
	newest := len(c.exps) == 0 || c.exps[0].at.After(at)
	// find the index for insertion ordered by expiration time.
	// when a key at the same expiration time exists, order by key.
	i := sort.Search(len(c.exps), func(i int) bool {
		if c.exps[i].at.Equal(at) {
			return c.exps[i].key >= k
		}
		return c.exps[i].at.After(at)
	})
	// key already exists at expiration, skip it.
	if i < len(c.exps) && c.exps[i].key == k && c.exps[i].at.Equal(at) {
		return false
	}
	// store key and increment generation, this will void previously
	// scheduled expirations for this object.
	g := c.refs[k]
	g.obj = obj
	g.gen++
	c.refs[k] = g
	// insert expKey at index.
	if i < len(c.exps) {
		// glow exps slice.
		c.exps = append(c.exps[:i+1], c.exps[i:]...)
		// set expKey.
		c.exps[i].gen = g.gen
		c.exps[i].key = k
		c.exps[i].at = at
		// re-schedule cleaning if our shortest expiration changed
		return newest
	}
	// append expKey generation.
	c.exps = append(c.exps, expKey[K]{gen: g.gen, key: k, at: at})
	return newest
}

// collect drops all expired expKeys from exps slice and returns all
// valid objects for cleanup. object is valid for cleanup if its expKey
// gen matches is expRef gen.
func (c *cleaner[T, K]) collect(now time.Time) (objs []T, exp time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for {
		if len(c.exps) == 0 {
			return
		}
		if c.exps[0].at.After(now) {
			exp = c.exps[0].at
			return
		}
		k := c.exps[0].key
		g := c.exps[0].gen
		if gen, ok := c.refs[k]; ok && gen.gen == g {
			delete(c.refs, k)
			objs = append(objs, gen.obj)
		}
		c.exps = c.exps[1:]
	}
}
