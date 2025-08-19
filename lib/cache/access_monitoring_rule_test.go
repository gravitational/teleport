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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestAccessMonitoringRules(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*accessmonitoringrulesv1.AccessMonitoringRule]{
		newResource: func(name string) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
			return newAccessMonitoringRule(t), nil
		},
		create: func(ctx context.Context, i *accessmonitoringrulesv1.AccessMonitoringRule) error {
			_, err := p.accessMonitoringRules.CreateAccessMonitoringRule(ctx, i)
			return err
		},
		list: func(ctx context.Context) ([]*accessmonitoringrulesv1.AccessMonitoringRule, error) {
			results, _, err := p.accessMonitoringRules.ListAccessMonitoringRules(ctx, 0, "")
			return results, err
		},
		cacheGet: p.cache.GetAccessMonitoringRule,
		cacheList: func(ctx context.Context) ([]*accessmonitoringrulesv1.AccessMonitoringRule, error) {
			results, _, err := p.cache.ListAccessMonitoringRules(ctx, 0, "")
			return results, err
		},
		update: func(ctx context.Context, i *accessmonitoringrulesv1.AccessMonitoringRule) error {
			_, err := p.accessMonitoringRules.UpdateAccessMonitoringRule(ctx, i)
			return err
		},
		deleteAll: p.accessMonitoringRules.DeleteAllAccessMonitoringRules,
	})
}

func TestListAccessMonitoringRulesWithFilter(t *testing.T) {
	t.Parallel()

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
				Metadata: &headerv1.Metadata{
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
				Metadata: &headerv1.Metadata{
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
				Metadata: &headerv1.Metadata{
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
				Metadata: &headerv1.Metadata{
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
				Metadata: &headerv1.Metadata{
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

	ctx := t.Context()
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			p := newTestPack(t, ForAuth)
			t.Cleanup(p.Close)

			_, err := p.accessMonitoringRules.CreateAccessMonitoringRule(ctx, test.rule)
			require.NoError(t, err)

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				results, next, err := p.cache.ListAccessMonitoringRules(ctx, 0, "")
				assert.NoError(t, err)
				assert.Empty(t, next)
				assert.Len(t, results, 1)
			},
				15*time.Second, 100*time.Millisecond)

			rules, _, err := p.cache.ListAccessMonitoringRulesWithFilter(ctx, test.req)
			require.NoError(t, err)
			if test.expectedRule {
				require.Len(t, rules, 1)
				require.Empty(t, cmp.Diff(test.rule, rules[0], protocmp.Transform()))
			} else {
				require.Empty(t, rules)
			}

			require.NoError(t, p.accessMonitoringRules.DeleteAccessMonitoringRule(ctx, test.rule.GetMetadata().GetName()))
		})
	}
}
