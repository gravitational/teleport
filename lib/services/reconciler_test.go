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

	"github.com/gravitational/teleport/api/types"
)

// TestReconciler makes sure appropriate callbacks are called during reconciliation.
func TestReconciler(t *testing.T) {
	tests := []struct {
		description         string
		selectors           []ResourceMatcher
		registeredResources types.ResourcesWithLabels
		newResources        types.ResourcesWithLabels
		onCreateCalls       types.ResourcesWithLabels
		onUpdateCalls       types.ResourcesWithLabels
		onDeleteCalls       types.ResourcesWithLabels
	}{
		{
			description: "new matching resource should be registered",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			registeredResources: types.ResourcesWithLabels{},
			newResources: types.ResourcesWithLabels{
				makeDynamicResource("res1", nil),
			},
			onCreateCalls: types.ResourcesWithLabels{
				makeDynamicResource("res1", nil),
			},
		},
		{
			description: "new non-matching resource should not be registered",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"env": []string{"prod"}},
			}},
			registeredResources: types.ResourcesWithLabels{},
			newResources: types.ResourcesWithLabels{
				makeDynamicResource("res1", map[string]string{"env": "dev"}),
			},
		},
		{
			description: "resources that equal don't overwrite each other ",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			registeredResources: types.ResourcesWithLabels{
				makeDynamicResource("res1", nil),
			},
			newResources: types.ResourcesWithLabels{
				makeDynamicResource("res1", nil, func(r *testResource) {
					// XXX_unrecognized should be ignored by CompareResources.
					r.Metadata.XXX_unrecognized = []byte{11, 0}
				}),
			},
		},
		{
			description: "resources with different origins don't overwrite each other",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			registeredResources: types.ResourcesWithLabels{
				makeStaticResource("res1", nil),
			},
			newResources: types.ResourcesWithLabels{
				makeDynamicResource("res1", nil),
			},
		},
		{
			description: "resource that's no longer present should be removed",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			registeredResources: types.ResourcesWithLabels{
				makeDynamicResource("res1", nil),
			},
			newResources: types.ResourcesWithLabels{},
			onDeleteCalls: types.ResourcesWithLabels{
				makeDynamicResource("res1", nil),
			},
		},
		{
			description: "resource with updated matching labels should be updated",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			registeredResources: types.ResourcesWithLabels{
				makeDynamicResource("res1", nil),
			},
			newResources: types.ResourcesWithLabels{
				makeDynamicResource("res1", map[string]string{"env": "dev"}),
			},
			onUpdateCalls: types.ResourcesWithLabels{
				makeDynamicResource("res1", map[string]string{"env": "dev"}),
			},
		},
		{
			description: "non-matching updated resource should be removed",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"env": []string{"prod"}},
			}},
			registeredResources: types.ResourcesWithLabels{
				makeDynamicResource("res1", map[string]string{"env": "prod"}),
			},
			newResources: types.ResourcesWithLabels{
				makeDynamicResource("res1", map[string]string{"env": "dev"}),
			},
			onDeleteCalls: types.ResourcesWithLabels{
				makeDynamicResource("res1", map[string]string{"env": "prod"}),
			},
		},
		{
			description: "complex scenario with multiple created/updated/deleted resources",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"env": []string{"prod"}},
			}},
			registeredResources: types.ResourcesWithLabels{
				makeStaticResource("res0", nil),
				makeDynamicResource("res1", map[string]string{"env": "prod"}),
				makeDynamicResource("res2", map[string]string{"env": "prod"}),
				makeDynamicResource("res3", map[string]string{"env": "prod"}),
				makeDynamicResource("res4", map[string]string{"env": "prod"}),
			},
			newResources: types.ResourcesWithLabels{
				makeDynamicResource("res0", map[string]string{"env": "prod"}),
				makeDynamicResource("res2", map[string]string{"env": "prod", "a": "b"}),
				makeDynamicResource("res3", map[string]string{"env": "prod"}),
				makeDynamicResource("res4", map[string]string{"env": "dev"}),
				makeDynamicResource("res5", map[string]string{"env": "prod"}),
				makeDynamicResource("res6", map[string]string{"env": "dev"}),
			},
			onCreateCalls: types.ResourcesWithLabels{
				makeDynamicResource("res5", map[string]string{"env": "prod"}),
			},
			onUpdateCalls: types.ResourcesWithLabels{
				makeDynamicResource("res2", map[string]string{"env": "prod", "a": "b"}),
			},
			onDeleteCalls: types.ResourcesWithLabels{
				makeDynamicResource("res1", map[string]string{"env": "prod"}),
				makeDynamicResource("res4", map[string]string{"env": "prod"}),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			// Reconciler will record all callback calls in these lists.
			var onCreateCalls, onUpdateCalls, onDeleteCalls types.ResourcesWithLabels

			reconciler, err := NewReconciler(ReconcilerConfig{
				Matcher: func(rwl types.ResourceWithLabels) bool {
					return MatchResourceLabels(test.selectors, rwl)
				},
				GetCurrentResources: func() types.ResourcesWithLabelsMap {
					return test.registeredResources.ToMap()
				},
				GetNewResources: func() types.ResourcesWithLabelsMap {
					return test.newResources.ToMap()
				},
				OnCreate: func(ctx context.Context, r types.ResourceWithLabels) error {
					onCreateCalls = append(onCreateCalls, r)
					return nil
				},
				OnUpdate: func(ctx context.Context, r types.ResourceWithLabels) error {
					onUpdateCalls = append(onUpdateCalls, r)
					return nil
				},
				OnDelete: func(ctx context.Context, r types.ResourceWithLabels) error {
					onDeleteCalls = append(onDeleteCalls, r)
					return nil
				},
			})
			require.NoError(t, err)

			// Reconcile and make sure we got all expected callback calls.
			err = reconciler.Reconcile(context.Background())
			require.NoError(t, err)
			require.Equal(t, test.onCreateCalls, onCreateCalls)
			require.Equal(t, test.onUpdateCalls, onUpdateCalls)
			require.Equal(t, test.onDeleteCalls, onDeleteCalls)
		})
	}
}

func makeStaticResource(name string, labels map[string]string) types.ResourceWithLabels {
	return makeResource(name, labels, map[string]string{
		types.OriginLabel: types.OriginConfigFile,
	})
}

func makeDynamicResource(name string, labels map[string]string, opts ...func(*testResource)) types.ResourceWithLabels {
	return makeResource(name, labels, map[string]string{
		types.OriginLabel: types.OriginDynamic,
	}, opts...)
}

func makeResource(name string, labels map[string]string, additionalLabels map[string]string, opts ...func(*testResource)) types.ResourceWithLabels {
	if labels == nil {
		labels = make(map[string]string)
	}
	maps.Copy(labels, additionalLabels)
	r := &testResource{
		Metadata: types.Metadata{
			Name:   name,
			Labels: labels,
		},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

type testResource struct {
	types.ResourceWithLabels
	Metadata types.Metadata
}

func (r *testResource) GetName() string {
	return r.Metadata.Name
}

func (r *testResource) GetKind() string {
	return "TestResource"
}

func (r *testResource) GetMetadata() types.Metadata {
	return r.Metadata
}

func (r *testResource) Origin() string {
	return r.Metadata.Labels[types.OriginLabel]
}

func (r *testResource) GetAllLabels() map[string]string {
	return r.Metadata.Labels
}
