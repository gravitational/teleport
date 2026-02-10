// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package server

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/types"
)

type mockInstance struct {
	ID string
}

type mockFetcher struct {
	instances []mockInstance
	err       error
}

func (m *mockFetcher) GetInstances(ctx context.Context, rotation bool) ([]mockInstance, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.instances, nil
}

func (m *mockFetcher) GetMatchingInstances(ctx context.Context, nodes []types.Server, rotation bool) ([]mockInstance, error) {
	return nil, trace.NotImplemented("GetMatchingInstances not implemented")
}

func (m *mockFetcher) GetDiscoveryConfigName() string {
	panic("GetDiscoveryConfigName should not be called")
}

func (m *mockFetcher) IntegrationName() string {
	panic("IntegrationName should not be called")
}

func TestWatcherFetchAndSubmit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		fetchers    map[string][]Fetcher[mockInstance]
		expectedIDs []string
	}{
		{
			name:        "no fetchers",
			fetchers:    nil,
			expectedIDs: nil,
		},
		{
			name: "single fetcher single instance",
			fetchers: map[string][]Fetcher[mockInstance]{
				"dc1": {&mockFetcher{instances: []mockInstance{{ID: "i-1"}}}},
			},
			expectedIDs: []string{"i-1"},
		},
		{
			name: "single fetcher multiple instances",
			fetchers: map[string][]Fetcher[mockInstance]{
				"dc1": {&mockFetcher{instances: []mockInstance{{ID: "i-1"}, {ID: "i-2"}}}},
			},
			expectedIDs: []string{"i-1", "i-2"},
		},
		{
			name: "multiple fetchers multiple configs",
			fetchers: map[string][]Fetcher[mockInstance]{
				"dc1": {&mockFetcher{instances: []mockInstance{{ID: "i-1"}}}},
				"dc2": {&mockFetcher{instances: []mockInstance{{ID: "i-2"}, {ID: "i-3"}}}},
			},
			expectedIDs: []string{"i-1", "i-2", "i-3"},
		},
		{
			name: "fetcher with error continues to next",
			fetchers: map[string][]Fetcher[mockInstance]{
				"dc1": {
					&mockFetcher{err: trace.BadParameter("fail")},
					&mockFetcher{instances: []mockInstance{{ID: "good"}}},
				},
			},
			expectedIDs: []string{"good"},
		},
		{
			name: "fetcher with NotFound error silent",
			fetchers: map[string][]Fetcher[mockInstance]{
				"dc1": {&mockFetcher{err: trace.NotFound("gone")}},
			},
			expectedIDs: nil,
		},
		{
			name: "fetcher returns nil slice",
			fetchers: map[string][]Fetcher[mockInstance]{
				"dc1": {&mockFetcher{instances: nil}},
			},
			expectedIDs: nil,
		},
		{
			name: "fetcher returns empty slice",
			fetchers: map[string][]Fetcher[mockInstance]{
				"dc1": {&mockFetcher{instances: []mockInstance{}}},
			},
			expectedIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var collected []mockInstance
			var mu sync.Mutex

			w := NewWatcher[mockInstance](t.Context(),
				WithPerInstanceHookFn[mockInstance](func(inst []mockInstance) {
					mu.Lock()
					collected = append(collected, inst...)
					mu.Unlock()
				}),
			)

			for dc, f := range tt.fetchers {
				w.SetFetchers(dc, f)
			}

			w.fetchAndSubmit()

			var ids []string
			for _, inst := range collected {
				ids = append(ids, inst.ID)
			}
			assert.ElementsMatch(t, tt.expectedIDs, ids)
		})
	}
}

func TestWatcherRunLoop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		pollInterval time.Duration
		run          func(t *testing.T, w *Watcher[mockInstance], triggerC chan struct{}, fetchCount *atomic.Int32)
		wantFetches  int32
	}{
		{
			name:         "poll timer triggers fetch",
			pollInterval: 10 * time.Second,
			run: func(t *testing.T, w *Watcher[mockInstance], triggerC chan struct{}, fetchCount *atomic.Int32) {
				go w.Run()
				synctest.Wait()
				assert.Equal(t, int32(1), fetchCount.Load())

				time.Sleep(10 * time.Second)
				synctest.Wait()
			},
			wantFetches: 2,
		},
		{
			name:         "manual trigger fetches",
			pollInterval: time.Hour,
			run: func(t *testing.T, w *Watcher[mockInstance], triggerC chan struct{}, fetchCount *atomic.Int32) {
				go w.Run()
				synctest.Wait()
				assert.Equal(t, int32(1), fetchCount.Load())

				triggerC <- struct{}{}
				synctest.Wait()
			},
			wantFetches: 2,
		},
		{
			name:         "multiple poll cycles",
			pollInterval: 10 * time.Second,
			run: func(t *testing.T, w *Watcher[mockInstance], triggerC chan struct{}, fetchCount *atomic.Int32) {
				go w.Run()
				synctest.Wait()
				assert.Equal(t, int32(1), fetchCount.Load())

				time.Sleep(10 * time.Second)
				synctest.Wait()
				assert.Equal(t, int32(2), fetchCount.Load())

				time.Sleep(10 * time.Second)
				synctest.Wait()
			},
			wantFetches: 3,
		},
		{
			name:         "manual trigger resets poll timer",
			pollInterval: 10 * time.Second,
			run: func(t *testing.T, w *Watcher[mockInstance], triggerC chan struct{}, fetchCount *atomic.Int32) {
				go w.Run()
				synctest.Wait()
				assert.Equal(t, int32(1), fetchCount.Load())

				time.Sleep(5 * time.Second)
				triggerC <- struct{}{}
				synctest.Wait()
				assert.Equal(t, int32(2), fetchCount.Load())

				time.Sleep(5 * time.Second)
				synctest.Wait()
				assert.Equal(t, int32(2), fetchCount.Load())

				time.Sleep(5 * time.Second)
				synctest.Wait()
			},
			wantFetches: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				triggerC := make(chan struct{}, 10)
				var fetchCount atomic.Int32

				w := NewWatcher[mockInstance](t.Context(),
					WithPollInterval[mockInstance](tt.pollInterval),
					WithTriggerFetchC[mockInstance](triggerC),
					WithPerInstanceHookFn[mockInstance](func([]mockInstance) {
						fetchCount.Add(1)
					}),
				)
				w.SetFetchers("dc1", []Fetcher[mockInstance]{&mockFetcher{instances: []mockInstance{{ID: "x"}}}})

				tt.run(t, w, triggerC, &fetchCount)
				w.Stop()

				assert.Equal(t, tt.wantFetches, fetchCount.Load())
			})
		})
	}
}

func TestWatcherHooks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		trigger   func(triggerC chan struct{})
		wantOrder []string
	}{
		{
			name:      "poll triggers pre/per/post hooks in order",
			trigger:   func(triggerC chan struct{}) {},
			wantOrder: []string{"pre", "per", "post"},
		},
		{
			name: "manual trigger calls trigger hook first",
			trigger: func(triggerC chan struct{}) {
				triggerC <- struct{}{}
				synctest.Wait()
			},
			wantOrder: []string{"pre", "per", "post", "trigger", "pre", "per", "post"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				triggerC := make(chan struct{}, 1)
				var order []string

				w := NewWatcher[mockInstance](t.Context(),
					WithPollInterval[mockInstance](time.Hour),
					WithTriggerFetchC[mockInstance](triggerC),
					WithTriggerFetchHookFn[mockInstance](func() { order = append(order, "trigger") }),
					WithPreFetchHookFn[mockInstance](func([]Fetcher[mockInstance]) { order = append(order, "pre") }),
					WithPerInstanceHookFn[mockInstance](func([]mockInstance) { order = append(order, "per") }),
					WithPostFetchHookFn[mockInstance](func() { order = append(order, "post") }),
				)
				w.SetFetchers("dc1", []Fetcher[mockInstance]{&mockFetcher{instances: []mockInstance{{ID: "x"}}}})

				done := make(chan struct{})
				go func() {
					w.Run()
					close(done)
				}()

				synctest.Wait()

				tt.trigger(triggerC)
				w.Stop()

				select {
				case <-done:
					// ok, Run exited
				case <-time.After(time.Second):
					t.Fatal("watcher didn't exit after shutdown")
				}

				assert.Equal(t, tt.wantOrder, order)
			})
		})
	}
}

func TestWatcherShutdown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		shutdown func(w *Watcher[mockInstance], cancel context.CancelFunc)
	}{
		{
			name: "Stop method",
			shutdown: func(w *Watcher[mockInstance], cancel context.CancelFunc) {
				w.Stop()
			},
		},
		{
			name: "context cancellation",
			shutdown: func(w *Watcher[mockInstance], cancel context.CancelFunc) {
				cancel()
			},
		},
		{
			name: "multiple Stop calls are safe",
			shutdown: func(w *Watcher[mockInstance], cancel context.CancelFunc) {
				w.Stop()
				w.Stop()
				w.Stop()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				w := NewWatcher[mockInstance](ctx,
					WithPollInterval[mockInstance](time.Hour),
				)

				done := make(chan struct{})
				go func() {
					w.Run()
					close(done)
				}()

				synctest.Wait()
				tt.shutdown(w, cancel)
				synctest.Wait()

				select {
				case <-done:
					// ok, Run exited
				case <-time.After(time.Second):
					t.Fatal("watcher didn't exit after shutdown")
				}
			})
		})
	}
}
