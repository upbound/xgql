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

package live_query

import (
	"context"
	"sync/atomic"
)

// liveQuery is set in context by the generated liveQuery resolver.
// Resolvers that need to refresh the query can use IsLive() and NotifyChanged()
// to check if the live query is still running and trigger a live query refresh
// accordingly.
type liveQuery struct {
	doneCh     <-chan struct{}
	hasChanges uint32
}

// HasChangesFn is a func that can be used to check if live query needs to be
// refreshed. It is used in generated live query resolver.
type HasChangesFn func() bool

type liveQueryKey struct{}

var liveQueryCtxKey = liveQueryKey{}

// WithLiveQuery sets LiveQuery on derived context and returns a callable for
// checking if live query needs to be refreshed. This is used in generated
// live query resolver to set up periodic live query refresh if changes occurred.
func WithLiveQuery(ctx context.Context) (context.Context, HasChangesFn) {
	lq := &liveQuery{doneCh: ctx.Done()}
	return context.WithValue(ctx, liveQueryCtxKey, lq), func() bool {
		return atomic.CompareAndSwapUint32(&lq.hasChanges, 1, 0)
	}
}

// IsLive returns true if this is a live query context and query is active.
func IsLive(ctx context.Context) bool {
	if lq, ok := ctx.Value(liveQueryCtxKey).(*liveQuery); ok {
		select {
		case <-lq.doneCh:
			return false
		default:
			return true
		}
	}
	return false
}

// NotifyChanged notifies live query of a change.
func NotifyChanged(ctx context.Context) {
	if lq, ok := ctx.Value(liveQueryCtxKey).(*liveQuery); ok {
		atomic.StoreUint32(&lq.hasChanges, 1)
	}
}
