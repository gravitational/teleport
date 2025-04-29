/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package local

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	v1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestAccessMonitoringRulesCRUD tests backend operations with AccessMonitoringRule resources.
func TestAccessMonitoringRulesCRUD(t *testing.T) {
	ctx := context.Background()

	mem, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)
	t.Cleanup(func() { mem.Close() })

	service, err := NewAccessMonitoringRulesService(mem)
	require.NoError(t, err)

	AccessMonitoringRule1 := &accessmonitoringrulesv1.AccessMonitoringRule{
		Kind:    types.KindAccessMonitoringRule,
		Version: types.V1,
		Metadata: &v1.Metadata{
			Name: "p1",
		},
		Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
			Subjects:  []string{"someSubject"},
			Condition: "someCondition",
		},
	}

	AccessMonitoringRule2 := &accessmonitoringrulesv1.AccessMonitoringRule{
		Kind:    types.KindAccessMonitoringRule,
		Version: types.V1,
		Metadata: &v1.Metadata{
			Name: "p2",
		},
		Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
			Subjects:  []string{"someSubject"},
			Condition: "someCondition",
		},
	}

	// Create both AccessMonitoringRules.
	_, err = service.CreateAccessMonitoringRule(ctx, AccessMonitoringRule1)
	require.NoError(t, err)
	_, err = service.CreateAccessMonitoringRule(ctx, AccessMonitoringRule2)
	require.NoError(t, err)

	// Fetch a specific AccessMonitoringRule.
	rule, err := service.GetAccessMonitoringRule(ctx, AccessMonitoringRule2.Metadata.Name)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(rule, AccessMonitoringRule2,
		cmpopts.IgnoreUnexported(accessmonitoringrulesv1.AccessMonitoringRule{}),
		cmpopts.IgnoreUnexported(accessmonitoringrulesv1.AccessMonitoringRuleSpec{}),
		cmpopts.IgnoreUnexported(v1.Metadata{}),
	))

	// Try to fetch a AccessMonitoringRule that doesn't exist.
	_, err = service.GetAccessMonitoringRule(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Try to create a duplicate AccessMonitoringRule.
	_, err = service.CreateAccessMonitoringRule(ctx, AccessMonitoringRule1)
	require.IsType(t, trace.AlreadyExists(""), err)

	// Delete a AccessMonitoringRule.
	err = service.DeleteAccessMonitoringRule(ctx, AccessMonitoringRule1.Metadata.Name)
	require.NoError(t, err)
	_, err = service.GetAccessMonitoringRule(ctx, AccessMonitoringRule1.Metadata.Name)
	require.IsType(t, trace.NotFound(""), err)

	// Try to delete a AccessMonitoringRule that doesn't exist.
	err = service.DeleteAccessMonitoringRule(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Delete all AccessMonitoringRule.
	err = service.DeleteAllAccessMonitoringRules(ctx)
	require.NoError(t, err)
	_, err = service.GetAccessMonitoringRule(ctx, AccessMonitoringRule1.Metadata.Name)
	require.IsType(t, trace.NotFound(""), err)
	_, err = service.GetAccessMonitoringRule(ctx, AccessMonitoringRule2.Metadata.Name)
	require.IsType(t, trace.NotFound(""), err)
}

func TestListAccessMonitoringRulesWithFilter(t *testing.T) {
	tests := []struct {
		description  string
		rule         *accessmonitoringrulesv1.AccessMonitoringRule
		req          *accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest
		expectedRule bool
	}{
		{
			description: "filter by notification integration",
			rule: &accessmonitoringrulesv1.AccessMonitoringRule{
				Kind:    types.KindAccessMonitoringRule,
				Version: types.V1,
				Metadata: &v1.Metadata{
					Name: "example-notification-rule",
				},
				Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
					Subjects:  []string{types.KindAccessRequest},
					Condition: "true",
					Notification: &accessmonitoringrulesv1.Notification{
						Name: "notificationIntegration",
					},
				},
			},
			req: &accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest{
				Subjects:         []string{types.KindAccessRequest},
				NotificationName: "notificationIntegration",
			},
			expectedRule: true,
		},
		{
			description: "filter by automatic_review integration",
			rule: &accessmonitoringrulesv1.AccessMonitoringRule{
				Kind:    types.KindAccessMonitoringRule,
				Version: types.V1,
				Metadata: &v1.Metadata{
					Name: "example-automatic-approval-rule",
				},
				Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
					Subjects:  []string{types.KindAccessRequest},
					Condition: "true",
					AutomaticReview: &accessmonitoringrulesv1.AutomaticReview{
						Integration: "automaticReviewIntegration",
						Decision:    types.RequestState_APPROVED.String(),
					},
				},
			},
			req: &accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest{
				Subjects:            []string{types.KindAccessRequest},
				AutomaticReviewName: "automaticReviewIntegration",
			},
			expectedRule: true,
		},
		{
			description: "filter by both notification and automatic_review integration",
			rule: &accessmonitoringrulesv1.AccessMonitoringRule{
				Kind:    types.KindAccessMonitoringRule,
				Version: types.V1,
				Metadata: &v1.Metadata{
					Name: "example-combined-rule",
				},
				Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
					Subjects:  []string{types.KindAccessRequest},
					Condition: "true",
					Notification: &accessmonitoringrulesv1.Notification{
						Name: "notificationIntegration",
					},
					AutomaticReview: &accessmonitoringrulesv1.AutomaticReview{
						Integration: "automaticReviewIntegration",
						Decision:    types.RequestState_APPROVED.String(),
					},
				},
			},
			req: &accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest{
				Subjects:            []string{types.KindAccessRequest},
				AutomaticReviewName: "automaticReviewIntegration",
				NotificationName:    "notificationIntegration",
			},
			expectedRule: true,
		},
		{
			description: "filter by builtin automatic_review rules",
			rule: &accessmonitoringrulesv1.AccessMonitoringRule{
				Kind:    types.KindAccessMonitoringRule,
				Version: types.V1,
				Metadata: &v1.Metadata{
					Name: "example-builtin-automatic_approval-rule",
				},
				Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
					Subjects:  []string{types.KindAccessRequest},
					Condition: "true",
					Notification: &accessmonitoringrulesv1.Notification{
						Name: "notificationIntegration",
					},
					AutomaticReview: &accessmonitoringrulesv1.AutomaticReview{
						Integration: types.BuiltInAutomaticReview,
						Decision:    types.RequestState_APPROVED.String(),
					},
				},
			},
			req: &accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest{
				Subjects:            []string{types.KindAccessRequest},
				AutomaticReviewName: types.BuiltInAutomaticReview,
			},
			expectedRule: true,
		},
		{
			description: "no match",
			rule: &accessmonitoringrulesv1.AccessMonitoringRule{
				Kind:    types.KindAccessMonitoringRule,
				Version: types.V1,
				Metadata: &v1.Metadata{
					Name: "no-match-rule",
				},
				Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
					Subjects:  []string{types.KindAccessRequest},
					Condition: "true",
					AutomaticReview: &accessmonitoringrulesv1.AutomaticReview{
						Integration: types.BuiltInAutomaticReview,
						Decision:    types.RequestState_APPROVED.String(),
					},
				},
			},
			req: &accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest{
				Subjects:            []string{types.KindAccessRequest},
				AutomaticReviewName: "automaticReviewIntegration",
			},
			expectedRule: false,
		},
	}

	ctx := context.Background()
	mem, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, mem.Close())
	})

	service, err := NewAccessMonitoringRulesService(mem)
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			_, err = service.CreateAccessMonitoringRule(ctx, test.rule)
			require.NoError(t, err)
			rules, _, err := service.ListAccessMonitoringRulesWithFilter(ctx, test.req)
			require.NoError(t, err)
			if test.expectedRule {
				require.Len(t, rules, 1)
				require.True(t, proto.Equal(test.rule, rules[0]))
			} else {
				require.Empty(t, rules)
			}
			require.NoError(t, service.DeleteAccessMonitoringRule(ctx, test.rule.GetMetadata().GetName()))
		})
	}
}

func TestListAccessMonitoringRules(t *testing.T) {
	const pageSize = 5
	const numAccessMonitoringRules = 2*pageSize + 1
	ctx := context.Background()

	mem, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)
	t.Cleanup(func() { mem.Close() })

	service, err := NewAccessMonitoringRulesService(mem)
	require.NoError(t, err)

	var insertedAccessMonitoringRules []*accessmonitoringrulesv1.AccessMonitoringRule
	for i := 0; i < numAccessMonitoringRules; i++ {
		AccessMonitoringRule := &accessmonitoringrulesv1.AccessMonitoringRule{
			Kind:    types.KindAccessMonitoringRule,
			Version: types.V1,
			Metadata: &v1.Metadata{
				Name: fmt.Sprintf("p%02d", i+1),
			},
			Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
				Subjects:  []string{"someSubject"},
				Condition: "someCondition",
			},
		}
		_, err := service.CreateAccessMonitoringRule(ctx, AccessMonitoringRule)
		require.NoError(t, err)
		insertedAccessMonitoringRules = append(insertedAccessMonitoringRules, AccessMonitoringRule)
	}

	t.Run("paginated", func(t *testing.T) {
		page1, nextKey, err := service.ListAccessMonitoringRules(ctx, pageSize, "")
		require.NoError(t, err)
		require.NotEmpty(t, nextKey)
		require.Len(t, page1, pageSize)

		page2, nextKey, err := service.ListAccessMonitoringRules(ctx, pageSize, nextKey)
		require.NoError(t, err)
		require.NotEmpty(t, nextKey)
		require.Len(t, page2, pageSize)

		page3, nextKey, err := service.ListAccessMonitoringRules(ctx, pageSize, nextKey)
		require.NoError(t, err)
		require.Empty(t, nextKey)
		require.Len(t, page3, 1)

		var fetchedAccessMonitoringRules []*accessmonitoringrulesv1.AccessMonitoringRule
		fetchedAccessMonitoringRules = append(fetchedAccessMonitoringRules, page1...)
		fetchedAccessMonitoringRules = append(fetchedAccessMonitoringRules, page2...)
		fetchedAccessMonitoringRules = append(fetchedAccessMonitoringRules, page3...)

		require.Empty(t, cmp.Diff(insertedAccessMonitoringRules, fetchedAccessMonitoringRules,
			cmpopts.IgnoreUnexported(accessmonitoringrulesv1.AccessMonitoringRule{}),
			cmpopts.IgnoreUnexported(accessmonitoringrulesv1.AccessMonitoringRuleSpec{}),
			cmpopts.IgnoreUnexported(v1.Metadata{}),
		))
	})

	t.Run("single", func(t *testing.T) {
		fetchedAccessMonitoringRules, nextKey, err := service.ListAccessMonitoringRules(ctx, apidefaults.DefaultChunkSize, "")
		require.NoError(t, err)
		require.Empty(t, nextKey)

		require.Empty(t, cmp.Diff(insertedAccessMonitoringRules, fetchedAccessMonitoringRules,
			cmpopts.IgnoreUnexported(accessmonitoringrulesv1.AccessMonitoringRule{}),
			cmpopts.IgnoreUnexported(accessmonitoringrulesv1.AccessMonitoringRuleSpec{}),
			cmpopts.IgnoreUnexported(v1.Metadata{}),
		))
	})
}
