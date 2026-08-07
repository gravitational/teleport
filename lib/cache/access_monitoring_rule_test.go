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

	testResources153(t, p, testFuncs[*accessmonitoringrulesv1.AccessMonitoringRule]{
		newResource: func(name string) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
			return newAccessMonitoringRule(t, name), nil
		},
		create: func(ctx context.Context, i *accessmonitoringrulesv1.AccessMonitoringRule) error {
			_, err := p.accessMonitoringRules.CreateAccessMonitoringRule(ctx, i)
			return err
		},
		list:      p.accessMonitoringRules.ListAccessMonitoringRules,
		cacheGet:  p.cache.GetAccessMonitoringRule,
		cacheList: p.cache.ListAccessMonitoringRules,
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
			rule: accessmonitoringrulesv1.AccessMonitoringRule_builder{
				Kind:    types.KindAccessMonitoringRule,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example-notification-rule",
				}.Build(),
				Spec: accessmonitoringrulesv1.AccessMonitoringRuleSpec_builder{
					Subjects:  []string{types.KindAccessRequest},
					Condition: "true",
					Notification: accessmonitoringrulesv1.Notification_builder{
						Name: "notificationIntegration",
					}.Build(),
				}.Build(),
			}.Build(),
			req: accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest_builder{
				Subjects:         []string{types.KindAccessRequest},
				NotificationName: "notificationIntegration",
			}.Build(),
			expectedRule: true,
		},
		{
			description: "filter by automatic_review integration",
			rule: accessmonitoringrulesv1.AccessMonitoringRule_builder{
				Kind:    types.KindAccessMonitoringRule,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example-automatic-approval-rule",
				}.Build(),
				Spec: accessmonitoringrulesv1.AccessMonitoringRuleSpec_builder{
					Subjects:  []string{types.KindAccessRequest},
					Condition: "true",
					AutomaticReview: accessmonitoringrulesv1.AutomaticReview_builder{
						Integration: "automaticReviewIntegration",
						Decision:    types.RequestState_APPROVED.String(),
					}.Build(),
				}.Build(),
			}.Build(),
			req: accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest_builder{
				Subjects:            []string{types.KindAccessRequest},
				AutomaticReviewName: "automaticReviewIntegration",
			}.Build(),
			expectedRule: true,
		},
		{
			description: "filter by both notification and automatic_review integration",
			rule: accessmonitoringrulesv1.AccessMonitoringRule_builder{
				Kind:    types.KindAccessMonitoringRule,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example-combined-rule",
				}.Build(),
				Spec: accessmonitoringrulesv1.AccessMonitoringRuleSpec_builder{
					Subjects:  []string{types.KindAccessRequest},
					Condition: "true",
					Notification: accessmonitoringrulesv1.Notification_builder{
						Name: "notificationIntegration",
					}.Build(),
					AutomaticReview: accessmonitoringrulesv1.AutomaticReview_builder{
						Integration: "automaticReviewIntegration",
						Decision:    types.RequestState_APPROVED.String(),
					}.Build(),
				}.Build(),
			}.Build(),
			req: accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest_builder{
				Subjects:            []string{types.KindAccessRequest},
				AutomaticReviewName: "automaticReviewIntegration",
				NotificationName:    "notificationIntegration",
			}.Build(),
			expectedRule: true,
		},
		{
			description: "filter by builtin automatic_review rules",
			rule: accessmonitoringrulesv1.AccessMonitoringRule_builder{
				Kind:    types.KindAccessMonitoringRule,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "example-builtin-automatic_approval-rule",
				}.Build(),
				Spec: accessmonitoringrulesv1.AccessMonitoringRuleSpec_builder{
					Subjects:  []string{types.KindAccessRequest},
					Condition: "true",
					Notification: accessmonitoringrulesv1.Notification_builder{
						Name: "notificationIntegration",
					}.Build(),
					AutomaticReview: accessmonitoringrulesv1.AutomaticReview_builder{
						Integration: types.BuiltInAutomaticReview,
						Decision:    types.RequestState_APPROVED.String(),
					}.Build(),
				}.Build(),
			}.Build(),
			req: accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest_builder{
				Subjects:            []string{types.KindAccessRequest},
				AutomaticReviewName: types.BuiltInAutomaticReview,
			}.Build(),
			expectedRule: true,
		},
		{
			description: "no match",
			rule: accessmonitoringrulesv1.AccessMonitoringRule_builder{
				Kind:    types.KindAccessMonitoringRule,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: "no-match-rule",
				}.Build(),
				Spec: accessmonitoringrulesv1.AccessMonitoringRuleSpec_builder{
					Subjects:  []string{types.KindAccessRequest},
					Condition: "true",
					AutomaticReview: accessmonitoringrulesv1.AutomaticReview_builder{
						Integration: types.BuiltInAutomaticReview,
						Decision:    types.RequestState_APPROVED.String(),
					}.Build(),
				}.Build(),
			}.Build(),
			req: accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest_builder{
				Subjects:            []string{types.KindAccessRequest},
				AutomaticReviewName: "automaticReviewIntegration",
			}.Build(),
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
				require.NoError(t, err)
				require.Empty(t, next)
				require.Len(t, results, 1)
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
