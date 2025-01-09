/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"context"
	"maps"
	"testing"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

type updateCall struct{ new, old testResource }

// TestReconciler makes sure appropriate callbacks are called during reconciliation.
func TestReconciler(t *testing.T) {
	type updateCall struct{ new, old testResource }
	tests := []struct {
		description         string
		selectors           []ResourceMatcher
		registeredResources []testResource
		newResources        []testResource
		onCreateCalls       []testResource
		onUpdateCalls       []updateCall
		onDeleteCalls       []testResource
		configure           func(cfg *ReconcilerConfig[testResource])
		comparator          func(testResource, testResource) int
	}{
		{
			description: "new matching resource should be registered",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			newResources:  []testResource{makeDynamicResource("res1", nil)},
			onCreateCalls: []testResource{makeDynamicResource("res1", nil)},
		},
		{
			description: "new non-matching resource should not be registered",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"env": []string{"prod"}},
			}},
			newResources: []testResource{makeDynamicResource("res1", map[string]string{"env": "dev"})},
		},
		{
			description: "resources that equal don't overwrite each other ",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			registeredResources: []testResource{makeDynamicResource("res1", nil)},
			newResources: []testResource{
				makeDynamicResource("res1", nil, func(r *testResource) {
					r.Metadata.Labels = map[string]string{"env": "dev"}
				}),
			},
		},
		{
			description: "resources with different origins don't overwrite each other by default",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			registeredResources: []testResource{makeStaticResource("res1", nil)},
			newResources:        []testResource{makeDynamicResource("res1", nil)},
		},
		{
			description: "resources with different origins overwrite each other when allowed",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			configure: func(cfg *ReconcilerConfig[testResource]) {
				cfg.AllowOriginChanges = true
			},
			registeredResources: []testResource{makeStaticResource("res1", nil)},
			newResources:        []testResource{makeDynamicResource("res1", nil)},
			onUpdateCalls: []updateCall{
				{
					old: makeStaticResource("res1", nil),
					new: makeDynamicResource("res1", nil),
				},
			},
		},
		{
			description: "resource that's no longer present should be removed",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			registeredResources: []testResource{makeDynamicResource("res1", nil)},
			onDeleteCalls:       []testResource{makeDynamicResource("res1", nil)},
		},
		{
			description: "resource with updated matching labels should be updated",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			registeredResources: []testResource{makeDynamicResource("res1", nil)},
			newResources:        []testResource{makeDynamicResource("res1", map[string]string{"env": "dev"})},
			onUpdateCalls: []updateCall{
				{
					old: makeDynamicResource("res1", nil),
					new: makeDynamicResource("res1", map[string]string{"env": "dev"}),
				},
			},
		},
		{
			description: "non-matching updated resource should be removed",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"env": []string{"prod"}},
			}},
			registeredResources: []testResource{makeDynamicResource("res1", map[string]string{"env": "prod"})},
			newResources:        []testResource{makeDynamicResource("res1", map[string]string{"env": "dev"})},
			onDeleteCalls:       []testResource{makeDynamicResource("res1", map[string]string{"env": "prod"})},
		},
		{
			description: "complex scenario with multiple created/updated/deleted resources",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"env": []string{"prod"}},
			}},
			registeredResources: []testResource{
				makeStaticResource("res0", nil),
				makeDynamicResource("res1", map[string]string{"env": "prod"}),
				makeDynamicResource("res2", map[string]string{"env": "prod"}),
				makeDynamicResource("res3", map[string]string{"env": "prod"}),
				makeDynamicResource("res4", map[string]string{"env": "prod"}),
			},
			newResources: []testResource{
				makeDynamicResource("res0", map[string]string{"env": "prod"}),
				makeDynamicResource("res2", map[string]string{"env": "prod", "a": "b"}),
				makeDynamicResource("res3", map[string]string{"env": "prod"}),
				makeDynamicResource("res4", map[string]string{"env": "dev"}),
				makeDynamicResource("res5", map[string]string{"env": "prod"}),
				makeDynamicResource("res6", map[string]string{"env": "dev"}),
			},
			onCreateCalls: []testResource{
				makeDynamicResource("res5", map[string]string{"env": "prod"}),
			},
			onUpdateCalls: []updateCall{
				{
					new: makeDynamicResource("res2", map[string]string{"env": "prod", "a": "b"}),
					old: makeDynamicResource("res2", map[string]string{"env": "prod"}),
				},
			},
			onDeleteCalls: []testResource{
				makeDynamicResource("res1", map[string]string{"env": "prod"}),
				makeDynamicResource("res4", map[string]string{"env": "prod"}),
			},
		}, {
			description: "custom comparison function",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"env": []string{"prod"}},
			}},
			registeredResources: []testResource{
				makeDynamicResource("res0", map[string]string{"env": "prod"}),
				makeDynamicResource("res1", map[string]string{"env": "prod"}),
				makeDynamicResource("res2", map[string]string{"env": "prod"}),
				makeDynamicResource("res3", map[string]string{"env": "prod"}),
				makeDynamicResource("res4", map[string]string{"env": "prod"}),
			},
			newResources: []testResource{
				makeDynamicResource("res0", map[string]string{"env": "prod", "updated": "yes"}),
				makeDynamicResource("res1", map[string]string{"env": "prod", "updated": "no"}),
				makeDynamicResource("res2", map[string]string{"env": "prod", "updated": "no"}),
				makeDynamicResource("res3", map[string]string{"env": "prod", "updated": "yes"}),
				makeDynamicResource("res4", map[string]string{"env": "prod", "updated": "yes"}),
			},
			comparator: func(a, b testResource) int {
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
			onUpdateCalls: []updateCall{
				{
					new: makeDynamicResource("res0", map[string]string{"env": "prod", "updated": "yes"}),
					old: makeDynamicResource("res0", map[string]string{"env": "prod"}),
				}, {
					new: makeDynamicResource("res3", map[string]string{"env": "prod", "updated": "yes"}),
					old: makeDynamicResource("res3", map[string]string{"env": "prod"}),
				}, {
					new: makeDynamicResource("res4", map[string]string{"env": "prod", "updated": "yes"}),
					old: makeDynamicResource("res4", map[string]string{"env": "prod"}),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			// Reconciler will record all callback calls in these lists.
			var onCreateCalls, onDeleteCalls []testResource
			var onUpdateCalls []updateCall

			cfg := ReconcilerConfig[testResource]{
				Matcher: func(tr testResource) bool {
					return MatchResourceLabels(test.selectors, tr.GetMetadata().Labels)
				},
				GetCurrentResources: func() map[string]testResource {
					return utils.FromSlice[testResource](test.registeredResources, func(t testResource) string {
						return t.Metadata.Name
					})
				},
				GetNewResources: func() map[string]testResource {
					return utils.FromSlice[testResource](test.newResources, func(t testResource) string {
						return t.Metadata.Name
					})
				},
				CompareResources: test.comparator,
				OnCreate: func(ctx context.Context, tr testResource) error {
					onCreateCalls = append(onCreateCalls, tr)
					return nil
				},
				OnUpdate: func(ctx context.Context, tr, old testResource) error {
					onUpdateCalls = append(onUpdateCalls, updateCall{new: tr, old: old})
					return nil
				},
				OnDelete: func(ctx context.Context, tr testResource) error {
					onDeleteCalls = append(onDeleteCalls, tr)
					return nil
				},
			}

			if test.configure != nil {
				test.configure(&cfg)
			}

			reconciler, err := NewReconciler[testResource](cfg)
			require.NoError(t, err)

			// Reconcile and make sure we got all expected callback calls.
			err = reconciler.Reconcile(context.Background())
			require.NoError(t, err)
			require.ElementsMatch(t, test.onCreateCalls, onCreateCalls)
			require.ElementsMatch(t, test.onUpdateCalls, onUpdateCalls)
			require.ElementsMatch(t, test.onDeleteCalls, onDeleteCalls)
		})
	}
}

// TestGenericReconciler asserts that a GenericReconciler with a non-string key
// can perform a reconciliation properly.
func TestGenericReconciler(t *testing.T) {
	type resourceID int

	// GIVEN some existing resources
	registeredResources := map[resourceID]testResource{
		0: makeStaticResource("res0", nil),
		1: makeDynamicResource("res1", map[string]string{"env": "prod"}),
		2: makeDynamicResource("res2", map[string]string{"env": "prod"}),
		3: makeDynamicResource("res3", map[string]string{"env": "prod"}),
		4: makeDynamicResource("res4", map[string]string{"env": "prod"}),
	}

	// GIVEN some new resources to be merged into the existing set
	newResources := map[resourceID]testResource{
		0: makeDynamicResource("res0", map[string]string{"env": "prod"}),
		2: makeDynamicResource("res2", map[string]string{"env": "prod", "a": "b"}),
		3: makeDynamicResource("res3", map[string]string{"env": "prod"}),
		4: makeDynamicResource("res4", map[string]string{"env": "dev"}),
		5: makeDynamicResource("res5", map[string]string{"env": "prod"}),
		6: makeDynamicResource("res6", map[string]string{"env": "dev"}),
	}

	var onCreateCalls []testResource
	var onUpdateCalls []updateCall
	var onDeleteCalls []testResource

	// GIVEN a reconciler bound to the above data sets, rigged to record each
	// callback invocation
	r, err := NewGenericReconciler(GenericReconcilerConfig[resourceID, testResource]{
		Matcher: func(tr testResource) bool {
			selectors := []ResourceMatcher{{
				Labels: types.Labels{"env": []string{"prod"}},
			}}
			return MatchResourceLabels(selectors, tr.GetMetadata().Labels)
		},
		GetCurrentResources: func() map[resourceID]testResource {
			return registeredResources
		},
		GetNewResources: func() map[resourceID]testResource {
			return newResources
		},
		OnCreate: func(ctx context.Context, tr testResource) error {
			onCreateCalls = append(onCreateCalls, tr)
			return nil
		},
		OnUpdate: func(ctx context.Context, tr, old testResource) error {
			onUpdateCalls = append(onUpdateCalls, updateCall{new: tr, old: old})
			return nil
		},
		OnDelete: func(ctx context.Context, tr testResource) error {
			onDeleteCalls = append(onDeleteCalls, tr)
			return nil
		},
	})
	require.NoError(t, err)

	// WHEN I run a reconciliation
	err = r.Reconcile(context.Background())

	// EXPECT the reconciliation to succeed
	require.NoError(t, err)

	// EXPECT that the matching resources in the "new" set that are missing from
	// the "old" have had the creation callback invoked on them
	expectedCreateCalls := []testResource{
		makeDynamicResource("res5", map[string]string{"env": "prod"}),
	}
	require.ElementsMatch(t, expectedCreateCalls, onCreateCalls)

	// EXPECT that the matching resources updated in the "new" set have been
	// had an update callback invoked on them
	expectedUpdateCalls := []updateCall{
		{
			new: makeDynamicResource("res2", map[string]string{"env": "prod", "a": "b"}),
			old: makeDynamicResource("res2", map[string]string{"env": "prod"}),
		},
	}
	require.ElementsMatch(t, expectedUpdateCalls, onUpdateCalls)

	// EXPECT that the elements in the "old" set missing from the "new" set have
	// had the delete callback invoked on them.
	expectedDeleteCalls := []testResource{
		makeDynamicResource("res1", map[string]string{"env": "prod"}),
		makeDynamicResource("res4", map[string]string{"env": "prod"}),
	}
	require.ElementsMatch(t, expectedDeleteCalls, onDeleteCalls)
}

func makeStaticResource(name string, labels map[string]string) testResource {
	return makeResource(name, labels, map[string]string{
		types.OriginLabel: types.OriginConfigFile,
	})
}

func makeDynamicResource(name string, labels map[string]string, opts ...func(*testResource)) testResource {
	return makeResource(name, labels, map[string]string{
		types.OriginLabel: types.OriginDynamic,
	}, opts...)
}

func makeResource(name string, labels map[string]string, additionalLabels map[string]string, opts ...func(*testResource)) testResource {
	if labels == nil {
		labels = make(map[string]string)
	}
	maps.Copy(labels, additionalLabels)
	r := testResource{
		Metadata: &headerv1.Metadata{
			Name:   name,
			Labels: labels,
		},
	}
	for _, opt := range opts {
		opt(&r)
	}
	return r
}

type testResource struct {
	Metadata *headerv1.Metadata
}

func (r testResource) GetMetadata() *headerv1.Metadata {
	return r.Metadata
}

func (r testResource) GetName() string {
	return r.Metadata.GetName()
}

func (r testResource) GetKind() string {
	return "testResource"
}
