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

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
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
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "notification plugin name is missing")
			},
		},
		{
			description: "automatic_approval name required",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.Spec.AutomaticApproval.Name = ""
			},
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "automatic_approval plugin name is missing")
			},
		},
		{
			description: "notification or automatic_approval required",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.Spec.Notification = nil
				amr.Spec.AutomaticApproval = nil
			},
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "notification or automatic_approval must be configured")
			},
		},
		{
			description: "allow automatic_approvals to be nil",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.Spec.AutomaticApproval = nil
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
	}

	validAMR := &accessmonitoringrulesv1.AccessMonitoringRule{
		Kind:     types.KindAccessMonitoringRule,
		Metadata: &headerv1.Metadata{},
		Version:  types.V1,
		Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
			Subjects:  []string{types.KindAccessRequest},
			Condition: "true",
			Notification: &accessmonitoringrulesv1.Notification{
				Name: "fakePlugin",
			},
			AutomaticApproval: &accessmonitoringrulesv1.AutomaticApproval{
				Name: "fakePlugin",
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
