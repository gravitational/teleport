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

package cache

import (
	"context"
	"testing"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// TestOktaImportRules tests that CRUD operations on Okta import rule resources are
// replicated from the backend to the cache.
func TestOktaImportRules(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForOkta)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.OktaImportRule]{
		newResource: func(name string) (types.OktaImportRule, error) {
			return types.NewOktaImportRule(
				types.Metadata{
					Name: name,
				},
				types.OktaImportRuleSpecV1{
					Mappings: []*types.OktaImportRuleMappingV1{
						{
							Match: []*types.OktaImportRuleMatchV1{
								{
									AppIDs: []string{"yes"},
								},
							},
							AddLabels: map[string]string{
								"label1": "value1",
							},
						},
						{
							Match: []*types.OktaImportRuleMatchV1{
								{
									GroupIDs: []string{"yes"},
								},
							},
							AddLabels: map[string]string{
								"label1": "value1",
							},
						},
					},
				},
			)
		},
		create: func(ctx context.Context, resource types.OktaImportRule) error {
			_, err := p.okta.CreateOktaImportRule(ctx, resource)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]types.OktaImportRule, error) {
			results, _, err := p.okta.ListOktaImportRules(ctx, 0, "")
			return results, err
		},
		cacheGet: p.cache.GetOktaImportRule,
		cacheList: func(ctx context.Context) ([]types.OktaImportRule, error) {
			results, _, err := p.cache.ListOktaImportRules(ctx, 0, "")
			return results, err
		},
		update: func(ctx context.Context, resource types.OktaImportRule) error {
			_, err := p.okta.UpdateOktaImportRule(ctx, resource)
			return trace.Wrap(err)
		},
		deleteAll: p.okta.DeleteAllOktaImportRules,
	})
}

// TestOktaAssignments tests that CRUD operations on Okta import rule resources are
// replicated from the backend to the cache.
func TestOktaAssignments(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForOkta)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.OktaAssignment]{
		newResource: func(name string) (types.OktaAssignment, error) {
			return types.NewOktaAssignment(
				types.Metadata{
					Name: name,
				},
				types.OktaAssignmentSpecV1{
					User: "test-user@test.user",
					Targets: []*types.OktaAssignmentTargetV1{
						{
							Type: types.OktaAssignmentTargetV1_APPLICATION,
							Id:   "123456",
						},
						{
							Type: types.OktaAssignmentTargetV1_GROUP,
							Id:   "234567",
						},
					},
				},
			)
		},
		create: func(ctx context.Context, resource types.OktaAssignment) error {
			_, err := p.okta.CreateOktaAssignment(ctx, resource)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]types.OktaAssignment, error) {
			results, _, err := p.okta.ListOktaAssignments(ctx, 0, "")
			return results, err
		},
		cacheGet: p.cache.GetOktaAssignment,
		cacheList: func(ctx context.Context) ([]types.OktaAssignment, error) {
			results, _, err := p.cache.ListOktaAssignments(ctx, 0, "")
			return results, err
		},
		update: func(ctx context.Context, resource types.OktaAssignment) error {
			_, err := p.okta.UpdateOktaAssignment(ctx, resource)
			return trace.Wrap(err)
		},
		deleteAll: p.okta.DeleteAllOktaAssignments,
	})
}
