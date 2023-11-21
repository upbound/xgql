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
	"runtime"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/clock"
	ktesting "k8s.io/utils/clock/testing"
)

func TestCleaner_Schedule(t *testing.T) {
	t.Parallel()
	type cleanup struct {
		At    time.Duration
		Items []int
	}
	type want struct {
		cleanups []cleanup
	}
	type step func(clock *ktesting.FakeClock, cleaner *cleaner[int, int])

	schedule := func(item int, exp time.Duration) step {
		return func(_ *ktesting.FakeClock, cleaner *cleaner[int, int]) {
			cleaner.Schedule(item, exp)
		}
	}
	sleep := func(duration time.Duration) step {
		return func(clock *ktesting.FakeClock, _ *cleaner[int, int]) {
			clock.Sleep(duration)
			runtime.Gosched()
		}
	}

	tests := map[string]struct {
		reason string
		steps  []step
		want   want
	}{
		"Success": {
			steps: []step{
				schedule(1, 1*time.Millisecond),
				schedule(1, 1*time.Millisecond),
				schedule(1, 1*time.Millisecond),
				schedule(1, 1*time.Millisecond),
				sleep(1 * time.Millisecond),
			},
			want: want{
				cleanups: []cleanup{
					{At: 1 * time.Millisecond, Items: []int{1}},
				},
			},
		},
		"Reschedules": {
			steps: []step{
				schedule(1, 2*time.Millisecond),
				sleep(1 * time.Millisecond),
				schedule(1, 2*time.Millisecond),
				sleep(1 * time.Millisecond),
				schedule(1, 2*time.Millisecond),
				sleep(1 * time.Millisecond),
				schedule(1, 2*time.Millisecond),
				sleep(1 * time.Millisecond),
				sleep(1 * time.Millisecond),
			},
			want: want{
				cleanups: []cleanup{
					{At: 5 * time.Millisecond, Items: []int{1}},
				},
			},
		},
		"Batches": {
			steps: []step{
				schedule(1, 1*time.Millisecond),
				schedule(2, 1*time.Millisecond),
				schedule(3, 1*time.Millisecond),
				schedule(4, 1*time.Millisecond),
				sleep(1 * time.Millisecond),
			},
			want: want{
				cleanups: []cleanup{
					{At: 1 * time.Millisecond, Items: []int{1, 2, 3, 4}},
				},
			},
		},
		"Reorders": {
			steps: []step{
				schedule(1, 5*time.Millisecond),
				schedule(2, 5*time.Millisecond),
				schedule(3, 5*time.Millisecond),
				schedule(4, 5*time.Millisecond),
				sleep(1 * time.Millisecond),
				schedule(4, 1*time.Millisecond),
				sleep(1 * time.Millisecond),
				schedule(3, 1*time.Millisecond),
				sleep(1 * time.Millisecond),
				schedule(2, 1*time.Millisecond),
				sleep(1 * time.Millisecond),
				sleep(1 * time.Millisecond),
			},
			want: want{
				cleanups: []cleanup{
					{At: 2 * time.Millisecond, Items: []int{4}},
					{At: 3 * time.Millisecond, Items: []int{3}},
					{At: 4 * time.Millisecond, Items: []int{2}},
					{At: 5 * time.Millisecond, Items: []int{1}},
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			now := time.Now()
			clock := ktesting.NewFakeClock(now)
			cleanupsCh := make(chan cleanup)
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			cleaner := NewCleaner(
				func(i int) int { return i },
				func(i []int) error {
					cleanupsCh <- cleanup{Items: i, At: clock.Now().Sub(now)}
					return nil
				},
				WithClock(clock),
				WithTick[int, int](time.Duration(0)),
			)
			startedCh := make(chan struct{})
			errCh := make(chan error, 1)
			go func() {
				close(startedCh)
				defer close(cleanupsCh)
				errCh <- cleaner.Start(ctx)
			}()
			go func() {
				<-startedCh
				for _, step := range tc.steps {
					step(clock, cleaner)
				}
			}()
			var cleanups []cleanup
			for cleanup := range cleanupsCh {
				cleanups = append(cleanups, cleanup)
				if !clock.HasWaiters() {
					break
				}
			}
			cancel()
			if diff := cmp.Diff(nil, <-errCh); diff != "" {
				t.Errorf("Start(...): -want err, +got err:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cleanups, cleanups); diff != "" {
				t.Errorf("Schedule(...): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff([]expKey[int]{}, cleaner.exps); diff != "" {
				t.Errorf("Expiration Queue: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(map[int]expRef[int]{}, cleaner.refs); diff != "" {
				t.Errorf("Expiration References: -want, +got:\n%s", diff)
			}
		})
	}
}

func WithClock(clock clock.Clock) CleanerOpt[int, int] {
	return func(c *cleaner[int, int]) {
		c.clock = clock
	}
}
