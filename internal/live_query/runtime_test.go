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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	ktesting "k8s.io/utils/clock/testing"
)

func Test_liveQuery_debounce(t *testing.T) {
	t.Parallel()

	type step func(clock *ktesting.FakeClock, lq *liveQuery)
	type steps []step
	trigger := func() step {
		return func(_ *ktesting.FakeClock, lq *liveQuery) {
			lq.Trigger()
		}
	}
	reset := func() step {
		return func(_ *ktesting.FakeClock, lq *liveQuery) {
			lq.Reset()
		}
	}
	wait := func() step {
		return func(clock *ktesting.FakeClock, lq *liveQuery) {
			clock.Sleep(lq.throttle)
			// allow the clock timer to fire before applying the next step
			time.Sleep(1 * time.Millisecond)
		}
	}

	tests := map[string]struct {
		reason   string
		throttle time.Duration
		steps    steps
		changes  []time.Duration
	}{
		"MultipleFirings": {
			reason:   "coalesces all fired events into one",
			throttle: 1 * time.Second,
			steps: steps{
				reset(),
				trigger(),
				trigger(),
				trigger(),
				wait(),
			},
			changes: []time.Duration{
				1 * time.Second,
			},
		},
		"NotArmed": {
			reason:   "doesn't fire until armed",
			throttle: 1 * time.Second,
			steps: steps{
				trigger(),
				trigger(),
				trigger(),
				wait(),
			},
		},
		"Rearmed": {
			reason:   "fires again when rearmed",
			throttle: 1 * time.Second,
			steps: steps{
				reset(),
				trigger(),
				wait(),
				reset(),
				trigger(),
				wait(),
				reset(),
				trigger(),
				wait(),
			},
			changes: []time.Duration{
				1 * time.Second,
				2 * time.Second,
				3 * time.Second,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			now := time.Now()
			clock := ktesting.NewFakeClock(now)
			doneCh := make(chan struct{})
			changesCh := make(chan struct{})
			actionsCh := make(chan liveQueryAction)
			lq := &liveQuery{
				throttle:  tt.throttle,
				doneCh:    doneCh,
				actionsCh: actionsCh,
				changesCh: changesCh,
				clock:     clock,
			}
			var changes []time.Duration
			startCh := make(chan struct{})
			go func() {
				defer close(doneCh)
				close(startCh)
				for _, step := range tt.steps {
					step(clock, lq)
				}
			}()
			go lq.debounce()
			<-startCh
			for range lq.Ready() {
				changes = append(changes, clock.Now().Sub(now))
			}
			if diff := cmp.Diff(tt.changes, changes); diff != "" {
				t.Errorf("debounce(...): -want, +got:\n%s", diff)
			}
		})
	}
}
