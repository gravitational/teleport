/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestValidateAccessMonitoringRule(t *testing.T) {
	tests := []struct {
		description string
		modifyAMR   func(amr *accessmonitoringrulesv1.AccessMonitoringRule)
		assertErr   require.ErrorAssertionFunc
	}{
		{
			description: "valid AMR",
			assertErr:   require.NoError,
		},
		{
			description: "notification name required",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.Spec.Notification.Name = ""
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "notification plugin name is missing")
			},
		},
		{
			description: "automatic_review integration required",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.Spec.AutomaticReview.Integration = ""
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "automatic_review integration is missing")
			},
		},
		{
			description: "automatic_review decision required",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.Spec.AutomaticReview.Decision = ""
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "automatic_review decision is missing")
			},
		},
		{
			description: "notification or automatic_review required",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.Spec.Notification = nil
				amr.Spec.AutomaticReview = nil
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "notification or automatic_review must be configured")
			},
		},
		{
			description: "allow automatic_review to be nil",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.Spec.AutomaticReview = nil
			},
			assertErr: require.NoError,
		},
		{
			description: "allow notifications to be nil",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.Spec.Notification = nil
			},
			assertErr: require.NoError,
		},
		{
			description: "invalid automatic_review decision",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.Spec.AutomaticReview.Decision = "invalid-decision"
			},
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, `accessMonitoringRule automatic_review decision "invalid-decision" is not supported`)
			},
		},
		{
			description: "invalid desired_state",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.Spec.DesiredState = "invalid-desired-state"
			},
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, `accessMonitoringRule desired_state "invalid-desired-state" is not supported`)
			},
		},
		{
			description: "invalid condition",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.Spec.Condition = "invalid-condition"
			},
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "accessMonitoringRule condition is invalid")
			},
		},
		{
			description: "allow desired_state to be empty",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.Spec.DesiredState = ""
			},
			assertErr: require.NoError,
		},
	}

	validAMR := &accessmonitoringrulesv1.AccessMonitoringRule{
		Kind:     types.KindAccessMonitoringRule,
		Metadata: &headerv1.Metadata{},
		Version:  types.V1,
		Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
			Subjects:     []string{types.KindAccessRequest},
			Condition:    "true",
			DesiredState: types.AccessMonitoringRuleStateReviewed,
			Notification: &accessmonitoringrulesv1.Notification{
				Name: "fakePlugin",
			},
			AutomaticReview: &accessmonitoringrulesv1.AutomaticReview{
				Integration: "fakePlugin",
				Decision:    types.RequestState_APPROVED.String(),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			amr, ok := proto.Clone(validAMR).(*accessmonitoringrulesv1.AccessMonitoringRule)
			require.True(t, ok)

			if test.modifyAMR != nil {
				test.modifyAMR(amr)
			}
			test.assertErr(t, ValidateAccessMonitoringRule(amr))
		})
	}
}
