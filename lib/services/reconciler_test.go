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

type hasMetadata interface {
	GetMetadata() *headerv1.Metadata
}

// TestReconciler makes sure appropriate callbacks are called during reconciliation.
func TestReconciler(t *testing.T) {
	type updateCall struct{ new, old Reconciled }
	getLabels := func(r Reconciled) map[string]string {
		return any(r).(hasMetadata).GetMetadata().Labels
	}
	forceCompare := func(forceCompare bool) func(Reconciled) {
		return func(r Reconciled) {
			any(r).(*testResourceWithEqual).ForceCompare = forceCompare
		}
	}
	tests := []struct {
		description         string
		selectors           []ResourceMatcher
		registeredResources []Reconciled
		newResources        []Reconciled
		onCreateCalls       []Reconciled
		onUpdateCalls       []updateCall
		onDeleteCalls       []Reconciled
	}{
		{
			description: "new matching resource should be registered",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			newResources:  []Reconciled{makeDynamicResource("res1", nil, false)},
			onCreateCalls: []Reconciled{makeDynamicResource("res1", nil, false)},
		},
		{
			description: "new non-matching resource should not be registered",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"env": []string{"prod"}},
			}},
			newResources: []Reconciled{makeDynamicResource("res1", map[string]string{"env": "dev"}, false)},
		},
		{
			description: "resources that equal don't overwrite each other ",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			registeredResources: []Reconciled{makeDynamicResource("res1", nil, false)},
			newResources: []Reconciled{
				makeDynamicResource("res1", nil, false, func(r Reconciled) {
					any(r).(testResource).Metadata.Labels = map[string]string{"env": "dev"}
				}),
			},
		},
		{
			description: "resources with different origins don't overwrite each other",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			registeredResources: []Reconciled{makeStaticResource("res1", nil, false)},
			newResources:        []Reconciled{makeDynamicResource("res1", nil, false)},
		},
		{
			description: "resource that's no longer present should be removed",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			registeredResources: []Reconciled{makeDynamicResource("res1", nil, false)},
			onDeleteCalls:       []Reconciled{makeDynamicResource("res1", nil, false)},
		},
		{
			description: "resource with updated matching labels should be updated",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			registeredResources: []Reconciled{makeDynamicResource("res1", nil, false)},
			newResources:        []Reconciled{makeDynamicResource("res1", map[string]string{"env": "dev"}, false)},
			onUpdateCalls: []updateCall{
				{
					old: makeDynamicResource("res1", nil, false),
					new: makeDynamicResource("res1", map[string]string{"env": "dev"}, false),
				},
			},
		},
		{
			description: "resource with custom equal forced to true, no update expected",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			registeredResources: []Reconciled{makeDynamicResource("res1", nil, true)},
			newResources:        []Reconciled{makeDynamicResource("res1", map[string]string{"env": "dev"}, true, forceCompare(true))},
		},
		{
			description: "resource with custom equal forced to false, update expected",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"*": []string{"*"}},
			}},
			registeredResources: []Reconciled{makeDynamicResource("res1", nil, true, func(r Reconciled) {
				any(r).(*testResourceWithEqual).ForceCompare = false
			})},
			newResources: []Reconciled{makeDynamicResource("res1", map[string]string{"env": "dev"}, true)},
			onUpdateCalls: []updateCall{
				{
					old: makeDynamicResource("res1", nil, true),
					new: makeDynamicResource("res1", map[string]string{"env": "dev"}, true),
				},
			},
		},
		{
			description: "non-matching updated resource should be removed",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"env": []string{"prod"}},
			}},
			registeredResources: []Reconciled{makeDynamicResource("res1", map[string]string{"env": "prod"}, false)},
			newResources:        []Reconciled{makeDynamicResource("res1", map[string]string{"env": "dev"}, false)},
			onDeleteCalls:       []Reconciled{makeDynamicResource("res1", map[string]string{"env": "prod"}, false)},
		},
		{
			description: "complex scenario with multiple created/updated/deleted resources",
			selectors: []ResourceMatcher{{
				Labels: types.Labels{"env": []string{"prod"}},
			}},
			registeredResources: []Reconciled{
				makeStaticResource("res0", nil, false),
				makeDynamicResource("res1", map[string]string{"env": "prod"}, false),
				makeDynamicResource("res2", map[string]string{"env": "prod"}, false),
				makeDynamicResource("res3", map[string]string{"env": "prod"}, false),
				makeDynamicResource("res4", map[string]string{"env": "prod"}, false),
			},
			newResources: []Reconciled{
				makeDynamicResource("res0", map[string]string{"env": "prod"}, false),
				makeDynamicResource("res2", map[string]string{"env": "prod", "a": "b"}, false),
				makeDynamicResource("res3", map[string]string{"env": "prod"}, false),
				makeDynamicResource("res4", map[string]string{"env": "dev"}, false),
				makeDynamicResource("res5", map[string]string{"env": "prod"}, false),
				makeDynamicResource("res6", map[string]string{"env": "dev"}, false),
			},
			onCreateCalls: []Reconciled{
				makeDynamicResource("res5", map[string]string{"env": "prod"}, false),
			},
			onUpdateCalls: []updateCall{
				{
					new: makeDynamicResource("res2", map[string]string{"env": "prod", "a": "b"}, false),
					old: makeDynamicResource("res2", map[string]string{"env": "prod"}, false),
				},
			},
			onDeleteCalls: []Reconciled{
				makeDynamicResource("res1", map[string]string{"env": "prod"}, false),
				makeDynamicResource("res4", map[string]string{"env": "prod"}, false),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			// Reconciler will record all callback calls in these lists.
			var onCreateCalls, onDeleteCalls []Reconciled
			var onUpdateCalls []updateCall

			reconciler, err := NewReconciler[Reconciled](ReconcilerConfig[Reconciled]{
				Matcher: func(tr Reconciled) bool {
					return MatchResourceLabels(test.selectors, getLabels(tr))
				},
				GetCurrentResources: func() map[string]Reconciled {
					return utils.FromSlice[Reconciled](test.registeredResources, func(t Reconciled) string {
						return t.GetName()
					})
				},
				GetNewResources: func() map[string]Reconciled {
					return utils.FromSlice[Reconciled](test.newResources, func(t Reconciled) string {
						return t.GetName()
					})
				},
				OnCreate: func(ctx context.Context, tr Reconciled) error {
					onCreateCalls = append(onCreateCalls, tr)
					return nil
				},
				OnUpdate: func(ctx context.Context, tr, old Reconciled) error {
					onUpdateCalls = append(onUpdateCalls, updateCall{new: tr, old: old})
					return nil
				},
				OnDelete: func(ctx context.Context, tr Reconciled) error {
					onDeleteCalls = append(onDeleteCalls, tr)
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

func makeStaticResource(name string, labels map[string]string, customEqual bool) Reconciled {
	return makeResource(name, labels, map[string]string{
		types.OriginLabel: types.OriginConfigFile,
	}, customEqual)
}

func makeDynamicResource(name string, labels map[string]string, customEqual bool, opts ...func(Reconciled)) Reconciled {
	return makeResource(name, labels, map[string]string{
		types.OriginLabel: types.OriginDynamic,
	}, customEqual, opts...)
}

func makeResource(name string, labels map[string]string, additionalLabels map[string]string, customEqual bool, opts ...func(Reconciled)) Reconciled {
	if labels == nil {
		labels = make(map[string]string)
	}
	maps.Copy(labels, additionalLabels)
	var r Reconciled
	if customEqual {
		r = &testResourceWithEqual{
			Metadata: &headerv1.Metadata{
				Name:   name,
				Labels: labels,
			},
		}
	} else {
		r = testResource{
			Metadata: &headerv1.Metadata{
				Name:   name,
				Labels: labels,
			},
		}
	}
	for _, opt := range opts {
		opt(r)
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

type testResourceWithEqual struct {
	Metadata     *headerv1.Metadata
	ForceCompare bool
}

func (r *testResourceWithEqual) GetMetadata() *headerv1.Metadata {
	return r.Metadata
}

func (r *testResourceWithEqual) GetName() string {
	return r.Metadata.GetName()
}

func (r *testResourceWithEqual) GetKind() string {
	return "testResourceWithEqual"
}

func (r *testResourceWithEqual) IsEqual(i interface{}) bool {
	return r.ForceCompare
}
