//
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

package services

import (
	"context"
	"iter"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/types"
)

type fakeEvents struct {
	events chan types.Event
}

// NewWatcher implements [types.Events].
func (f *fakeEvents) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	return &fakeWatcher{
		events: f.events,
		doneCh: make(chan struct{}),
	}, nil
}

type fakeWatcher struct {
	events <-chan types.Event
	once   sync.Once
	doneCh chan struct{}
}

// Close implements [types.Watcher].
func (f *fakeWatcher) Close() error {
	f.once.Do(func() { close(f.doneCh) })
	return nil
}

// Done implements [types.Watcher].
func (f *fakeWatcher) Done() <-chan struct{} {
	return f.doneCh
}

// Error implements [types.Watcher].
func (f *fakeWatcher) Error() error {
	return nil
}

// Events implements [types.Watcher].
func (f *fakeWatcher) Events() <-chan types.Event {
	return f.events
}

func putEvent(r testResource) types.Event {
	return types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToLegacy(r),
	}
}

func deleteEvent(r testResource) types.Event {
	return types.Event{
		Type:     types.OpDelete,
		Resource: types.Resource153ToLegacy(r),
	}
}

// TestReconciler makes sure appropriate callbacks are called during reconciliation.
func TestResourceMonitor(t *testing.T) {
	type modifiedResource struct{ New, Old testResource }
	tests := []struct {
		description              string
		selectors                []ResourceMatcher
		existingResources        []testResource
		events                   []types.Event
		expectedCreatedResources []testResource
		expectedUpdatedResources []modifiedResource
		expectedDeletedResources []testResource
		configure                func(cfg *ResourceMonitorConfig[testResource])
		resourceCompare          func(testResource, testResource) int
	}{
		{
			description: "new matching resource should be registered",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			events:                   []types.Event{putEvent(makeDynamicResource("res1", nil))},
			expectedCreatedResources: []testResource{makeDynamicResource("res1", nil)},
		},
		{
			description: "new non-matching resource should not be registered",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"env": []string{"prod"}},
			}},
			events: []types.Event{putEvent(makeDynamicResource("res1", map[string]string{"env": "dev"}))},
		},
		{
			description: "resources that equal don't overwrite each other",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			existingResources: []testResource{makeDynamicResource("res1", nil)},
			events: []types.Event{
				putEvent(makeDynamicResource("res1", nil, func(r *testResource) {
					r.Metadata.Labels = map[string]string{"env": "dev"}
				})),
			},
		},
		{
			description: "resources with different origins don't overwrite each other by default",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			existingResources: []testResource{makeStaticResource("res1", nil)},
			events:            []types.Event{putEvent(makeDynamicResource("res1", nil))},
		},
		{
			description: "resources with different origins overwrite each other when allowed",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			configure: func(cfg *ResourceMonitorConfig[testResource]) {
				cfg.AllowOriginChanges = true
			},
			existingResources: []testResource{makeStaticResource("res1", nil)},
			events:            []types.Event{putEvent(makeDynamicResource("res1", nil))},
			expectedUpdatedResources: []modifiedResource{
				{
					Old: makeStaticResource("res1", nil),
					New: makeDynamicResource("res1", nil),
				},
			},
		},
		{
			description: "resource that's no longer present should be removed",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			existingResources:        []testResource{makeDynamicResource("res1", nil)},
			events:                   []types.Event{deleteEvent(makeDynamicResource("res1", nil))},
			expectedDeletedResources: []testResource{makeDynamicResource("res1", nil)},
		},
		{
			description: "removing a resource that doesn't exist is not an error",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"foo": []string{"bar"}},
			}},
			// Note the label change below. This means the resource no longer matches and should be removed.
			existingResources: []testResource{makeDynamicResource("res1", map[string]string{"foo": "bar"})},
			events:            []types.Event{putEvent(makeDynamicResource("res1", map[string]string{"baz": "quux"}))},

			// Simulate the resource having already expired from the backend.
			configure: func(cfg *ResourceMonitorConfig[testResource]) {
				originalDelete := cfg.DeleteResource
				cfg.DeleteResource = func(ctx context.Context, tr testResource) error {
					originalDelete(ctx, tr)
					return trace.NotFound("resource does not exist")
				}
			},

			expectedDeletedResources: []testResource{makeDynamicResource("res1", map[string]string{"foo": "bar"})},
		},
		{
			description: "resource with updated matching labels should be updated",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			existingResources: []testResource{makeDynamicResource("res1", nil)},
			events:            []types.Event{putEvent(makeDynamicResource("res1", map[string]string{"env": "dev"}))},
			expectedUpdatedResources: []modifiedResource{
				{
					Old: makeDynamicResource("res1", nil),
					New: makeDynamicResource("res1", map[string]string{"env": "dev"}),
				},
			},
		},
		{
			description: "non-matching updated resource should be removed",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"env": []string{"prod"}},
			}},
			existingResources:        []testResource{makeDynamicResource("res1", map[string]string{"env": "prod"})},
			events:                   []types.Event{putEvent(makeDynamicResource("res1", map[string]string{"env": "dev"}))},
			expectedDeletedResources: []testResource{makeDynamicResource("res1", map[string]string{"env": "prod"})},
		},
		{
			description: "complex scenario with multiple created/updated/deleted resources",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"env": []string{"prod"}},
			}},
			existingResources: []testResource{
				makeStaticResource("res0", nil),
				makeDynamicResource("res1", map[string]string{"env": "prod"}),
				makeDynamicResource("res2", map[string]string{"env": "prod"}),
				makeDynamicResource("res3", map[string]string{"env": "prod"}),
				makeDynamicResource("res4", map[string]string{"env": "prod"}),
			},
			events: []types.Event{
				deleteEvent(makeDynamicResource("res1", nil)),
				putEvent(makeDynamicResource("res2", map[string]string{"env": "prod", "a": "b"})),
				putEvent(makeDynamicResource("res5", map[string]string{"env": "prod"})),
				deleteEvent(makeDynamicResource("res4", nil)),
			},
			expectedCreatedResources: []testResource{
				makeDynamicResource("res5", map[string]string{"env": "prod"}),
			},
			expectedUpdatedResources: []modifiedResource{
				{
					New: makeDynamicResource("res2", map[string]string{"env": "prod", "a": "b"}),
					Old: makeDynamicResource("res2", map[string]string{"env": "prod"}),
				},
			},
			expectedDeletedResources: []testResource{
				makeDynamicResource("res1", map[string]string{"env": "prod"}),
				makeDynamicResource("res4", map[string]string{"env": "prod"}),
			},
		}, {
			description: "custom comparison function",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"env": []string{"prod"}},
			}},
			existingResources: []testResource{
				makeDynamicResource("res0", map[string]string{"env": "prod"}),
				makeDynamicResource("res1", map[string]string{"env": "prod"}),
				makeDynamicResource("res2", map[string]string{"env": "prod"}),
				makeDynamicResource("res3", map[string]string{"env": "prod"}),
				makeDynamicResource("res4", map[string]string{"env": "prod"}),
			},
			events: []types.Event{
				putEvent(makeDynamicResource("res0", map[string]string{"env": "prod", "updated": "yes"})),
				putEvent(makeDynamicResource("res1", map[string]string{"env": "prod", "updated": "no"})),
				putEvent(makeDynamicResource("res2", map[string]string{"env": "prod", "updated": "no"})),
				putEvent(makeDynamicResource("res3", map[string]string{"env": "prod", "updated": "yes"})),
				putEvent(makeDynamicResource("res4", map[string]string{"env": "prod", "updated": "yes"})),
			},
			resourceCompare: func(a, b testResource) int {
				updated, ok := a.Metadata.Labels["updated"]
				if !ok {
					updated, ok = b.Metadata.Labels["updated"]
					if !ok {
						panic(`neither resource has "updated" label`)
					}
				}

				if updated == "yes" {
					return Different
				}
				return Equal
			},
			expectedUpdatedResources: []modifiedResource{
				{
					New: makeDynamicResource("res0", map[string]string{"env": "prod", "updated": "yes"}),
					Old: makeDynamicResource("res0", map[string]string{"env": "prod"}),
				}, {
					New: makeDynamicResource("res3", map[string]string{"env": "prod", "updated": "yes"}),
					Old: makeDynamicResource("res3", map[string]string{"env": "prod"}),
				}, {
					New: makeDynamicResource("res4", map[string]string{"env": "prod", "updated": "yes"}),
					Old: makeDynamicResource("res4", map[string]string{"env": "prod"}),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			fakeEvents := &fakeEvents{
				events: make(chan types.Event, 1),
			}

			fakeEvents.events <- types.Event{Type: types.OpInit}

			// All events defined in the test case are sent first. Once they
			// have all been processed a final OpUnreliable is sent to validate
			// that all prior events have been consumed. This prevents any races
			// with canceling the context when this function returns and processing
			// the final test event.
			go func() {
				defer cancel()
				for _, e := range test.events {
					select {
					case fakeEvents.events <- e:
					case <-ctx.Done():
					}
				}

				select {
				case fakeEvents.events <- types.Event{Type: types.OpUnreliable}:
				case <-ctx.Done():
				}
			}()

			var created, deleted []testResource
			var updated []modifiedResource
			cfg := ResourceMonitorConfig[testResource]{
				Kind:   "testResource",
				Key:    testResource.GetName,
				Events: fakeEvents,
				ResourceHeaderKey: func(rh *types.ResourceHeader) string {
					return rh.GetName()
				},
				Matches: func(tr testResource) bool {
					return MatchResourceLabels(test.selectors, tr.GetMetadata().Labels)
				},
				CompareResources: func(tr1, tr2 testResource) int {
					if test.resourceCompare == nil {
						return CompareResources(tr1.Metadata, tr2.Metadata)
					}

					return test.resourceCompare(tr1, tr2)
				},
				DeleteResource: func(ctx context.Context, tr testResource) error {
					deleted = append(deleted, tr)
					return nil
				},
				CreateResource: func(ctx context.Context, tr testResource) error {
					created = append(created, tr)
					return nil
				},
				UpdateResource: func(ctx context.Context, new, old testResource) error {
					updated = append(updated, modifiedResource{New: new, Old: old})
					return nil
				},
				CurrentResources: func(ctx context.Context) iter.Seq2[testResource, error] {
					return func(yield func(testResource, error) bool) {
						for _, r := range test.existingResources {
							if !yield(r, nil) {
								return
							}
						}
					}
				},
			}

			if test.configure != nil {
				test.configure(&cfg)
			}

			monitor, err := NewResourceMonitor(cfg)
			require.NoError(t, err)

			// Reconcile and make sure we got all expected callback calls.
			monitor.Run(ctx)

			require.Empty(t, cmp.Diff(test.expectedCreatedResources, created, protocmp.Transform()))
			require.Empty(t, cmp.Diff(test.expectedUpdatedResources, updated, protocmp.Transform()))
			require.Empty(t, cmp.Diff(test.expectedDeletedResources, deleted, protocmp.Transform()))
		})
	}
}
