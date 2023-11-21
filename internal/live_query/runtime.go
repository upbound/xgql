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
	"time"

	"k8s.io/utils/clock"
)

// liveQuery is set in context by LiveQuery extension.
// Resolvers that need to refresh the query can use IsLive() and NotifyChanged()
// to check if the live query is still running and trigger a live query refresh
// accordingly.
type liveQuery struct {
	// a unique id to make it easier to differentiate queries from resolvers.
	id        uint64
	throttle  time.Duration
	doneCh    <-chan struct{}
	actionsCh chan liveQueryAction
	changesCh chan struct{}
	clock     clock.Clock
}

// liveQueryAction is a signal
type liveQueryAction int

const (
	fire liveQueryAction = iota
	rearm
)

// debounce ensures that live query only triggers Ready() channel
// at most every throttle interval.
func (lq *liveQuery) debounce() { //nolint:gocyclo
	var (
		// channel that will trigger after at least throttle interval since the previous trigger.
		timer <-chan time.Time

		// the debounce loop can be in armed state, at which point it will debounce fired event
		// onto the changes channel after the throttle period.
		// if debounce loop is not armed, it means the live query is being resolved. in this case,
		// the fact that an event has occurred is marked in the fired bool. then, the next time
		// the live query is rearmed, the throttle timer will be set at the same time.
		// this way the query becomes ready after the throttle period and no changes are lost.
		armed, fired bool
	)
	defer close(lq.changesCh)
	// Start debouncing
	for {
		select {
		case a := <-lq.actionsCh:
			switch a {
			case fire:
				if armed {
					if timer == nil {
						timer = lq.clock.After(lq.throttle)
					}
					continue
				}
				fired = true
				continue
			case rearm:
				if fired {
					if timer == nil {
						timer = lq.clock.After(lq.throttle)
					}
					continue
				}
				armed = true
				continue
			}
		case <-timer:
		case <-lq.doneCh:
			return
		}
		fired = false
		armed = false
		timer = nil
		select {
		case lq.changesCh <- struct{}{}:
		case <-lq.doneCh:
			return
		}
	}
}

// Ready returns a channel that will be notified when a new change is ready.
func (lq *liveQuery) Ready() <-chan struct{} {
	select {
	// if query is done, return nil channel
	case <-lq.doneCh:
		return nil
	default:
		return lq.changesCh
	}
}

// Reset resets the live query throttling mechanism.
func (lq *liveQuery) Reset() {
	select {
	case lq.actionsCh <- rearm:
	case <-lq.doneCh:
	}
}

// Trigger triggers the live query's Fired channel after the throttle period.
func (lq *liveQuery) Trigger() {
	select {
	case lq.actionsCh <- fire:
	case <-lq.doneCh:
	}
}

type liveQueryKey struct{}

var (
	liveQueryCtxKey = liveQueryKey{}
	liveQueryIds    = atomic.Uint64{}
)

// withLiveQuery creates a new liveQuery and returns it with a modified context.
func withLiveQuery(ctx context.Context, throttle time.Duration) (*liveQuery, context.Context) {
	lq := &liveQuery{
		id:        liveQueryIds.Add(1),
		throttle:  throttle,
		clock:     clock.RealClock{},
		doneCh:    ctx.Done(),
		actionsCh: make(chan liveQueryAction),
		changesCh: make(chan struct{}),
	}
	go lq.debounce()
	return lq, context.WithValue(ctx, liveQueryCtxKey, lq)
}

// IsLive returns query id and true if this is a live query context and query is active.
// TODO(avalanche123): add tests.
func IsLive(ctx context.Context) (uint64, bool) {
	if lq, ok := ctx.Value(liveQueryCtxKey).(*liveQuery); ok {
		select {
		case <-lq.doneCh:
			return 0, false
		default:
			return lq.id, true
		}
	}
	return 0, false
}

// Trigger notifies live query of a change.
// TODO(avalanche123): add tests.
func Trigger(ctx context.Context) {
	if lq, ok := ctx.Value(liveQueryCtxKey).(*liveQuery); ok {
		lq.Trigger()
	}
}
