/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
)

func TestOktaAssignments_SetStatus(t *testing.T) {
	tests := []struct {
		startStatus string
		nextStatus  string
		invalid     bool
	}{
		// PENDING transitions
		{startStatus: constants.OktaAssignmentActionStatusPending, nextStatus: constants.OktaAssignmentActionStatusPending, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusPending, nextStatus: constants.OktaAssignmentActionStatusProcessing},
		{startStatus: constants.OktaAssignmentActionStatusPending, nextStatus: constants.OktaAssignmentActionStatusSuccessful, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusPending, nextStatus: constants.OktaAssignmentActionStatusFailed, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusPending, nextStatus: constants.OktaAssignmentActionStatusCleanupPending},
		{startStatus: constants.OktaAssignmentActionStatusPending, nextStatus: constants.OktaAssignmentActionStatusCleanupProcessing, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusPending, nextStatus: constants.OktaAssignmentActionStatusCleanedUp, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusPending, nextStatus: constants.OktaAssignmentActionStatusCleanupFailed, invalid: true},

		// PROCESSING transitions
		{startStatus: constants.OktaAssignmentActionStatusProcessing, nextStatus: constants.OktaAssignmentActionStatusPending, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusProcessing, nextStatus: constants.OktaAssignmentActionStatusProcessing, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusProcessing, nextStatus: constants.OktaAssignmentActionStatusSuccessful},
		{startStatus: constants.OktaAssignmentActionStatusProcessing, nextStatus: constants.OktaAssignmentActionStatusFailed},
		{startStatus: constants.OktaAssignmentActionStatusProcessing, nextStatus: constants.OktaAssignmentActionStatusCleanupPending},
		{startStatus: constants.OktaAssignmentActionStatusProcessing, nextStatus: constants.OktaAssignmentActionStatusCleanupProcessing, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusProcessing, nextStatus: constants.OktaAssignmentActionStatusCleanedUp, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusProcessing, nextStatus: constants.OktaAssignmentActionStatusCleanupFailed, invalid: true},

		// SUCCESSFUL transitions
		{startStatus: constants.OktaAssignmentActionStatusSuccessful, nextStatus: constants.OktaAssignmentActionStatusPending, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusSuccessful, nextStatus: constants.OktaAssignmentActionStatusProcessing, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusSuccessful, nextStatus: constants.OktaAssignmentActionStatusSuccessful, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusSuccessful, nextStatus: constants.OktaAssignmentActionStatusFailed, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusSuccessful, nextStatus: constants.OktaAssignmentActionStatusCleanupPending},
		{startStatus: constants.OktaAssignmentActionStatusSuccessful, nextStatus: constants.OktaAssignmentActionStatusCleanupProcessing, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusSuccessful, nextStatus: constants.OktaAssignmentActionStatusCleanedUp, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusSuccessful, nextStatus: constants.OktaAssignmentActionStatusCleanupFailed, invalid: true},

		// FAILED transitions
		{startStatus: constants.OktaAssignmentActionStatusFailed, nextStatus: constants.OktaAssignmentActionStatusPending, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusFailed, nextStatus: constants.OktaAssignmentActionStatusProcessing},
		{startStatus: constants.OktaAssignmentActionStatusFailed, nextStatus: constants.OktaAssignmentActionStatusSuccessful, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusFailed, nextStatus: constants.OktaAssignmentActionStatusFailed, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusFailed, nextStatus: constants.OktaAssignmentActionStatusCleanupPending},
		{startStatus: constants.OktaAssignmentActionStatusFailed, nextStatus: constants.OktaAssignmentActionStatusCleanupProcessing, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusFailed, nextStatus: constants.OktaAssignmentActionStatusCleanedUp, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusFailed, nextStatus: constants.OktaAssignmentActionStatusCleanupFailed, invalid: true},

		// CLEANUP_PENDING transitions
		{startStatus: constants.OktaAssignmentActionStatusCleanupPending, nextStatus: constants.OktaAssignmentActionStatusPending, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupPending, nextStatus: constants.OktaAssignmentActionStatusProcessing, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupPending, nextStatus: constants.OktaAssignmentActionStatusSuccessful, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupPending, nextStatus: constants.OktaAssignmentActionStatusFailed, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupPending, nextStatus: constants.OktaAssignmentActionStatusCleanupPending, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupPending, nextStatus: constants.OktaAssignmentActionStatusCleanupProcessing},
		{startStatus: constants.OktaAssignmentActionStatusCleanupPending, nextStatus: constants.OktaAssignmentActionStatusCleanedUp, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupPending, nextStatus: constants.OktaAssignmentActionStatusCleanupFailed, invalid: true},

		// CLEANUP_PROCESSING transitions
		{startStatus: constants.OktaAssignmentActionStatusCleanupProcessing, nextStatus: constants.OktaAssignmentActionStatusPending, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupProcessing, nextStatus: constants.OktaAssignmentActionStatusProcessing, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupProcessing, nextStatus: constants.OktaAssignmentActionStatusSuccessful, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupProcessing, nextStatus: constants.OktaAssignmentActionStatusFailed, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupProcessing, nextStatus: constants.OktaAssignmentActionStatusCleanupPending, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupProcessing, nextStatus: constants.OktaAssignmentActionStatusCleanupProcessing, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupProcessing, nextStatus: constants.OktaAssignmentActionStatusCleanedUp},
		{startStatus: constants.OktaAssignmentActionStatusCleanupProcessing, nextStatus: constants.OktaAssignmentActionStatusCleanupFailed},

		// CLEANED_UP transitions
		{startStatus: constants.OktaAssignmentActionStatusCleanedUp, nextStatus: constants.OktaAssignmentActionStatusPending, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanedUp, nextStatus: constants.OktaAssignmentActionStatusProcessing, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanedUp, nextStatus: constants.OktaAssignmentActionStatusSuccessful, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanedUp, nextStatus: constants.OktaAssignmentActionStatusFailed, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanedUp, nextStatus: constants.OktaAssignmentActionStatusCleanupPending, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanedUp, nextStatus: constants.OktaAssignmentActionStatusCleanupProcessing, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanedUp, nextStatus: constants.OktaAssignmentActionStatusCleanedUp, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanedUp, nextStatus: constants.OktaAssignmentActionStatusCleanupFailed, invalid: true},

		// CLEANUP_FAILED transitions
		{startStatus: constants.OktaAssignmentActionStatusCleanupFailed, nextStatus: constants.OktaAssignmentActionStatusPending, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupFailed, nextStatus: constants.OktaAssignmentActionStatusProcessing, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupFailed, nextStatus: constants.OktaAssignmentActionStatusSuccessful, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupFailed, nextStatus: constants.OktaAssignmentActionStatusFailed, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupFailed, nextStatus: constants.OktaAssignmentActionStatusCleanupPending, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupFailed, nextStatus: constants.OktaAssignmentActionStatusCleanupProcessing, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupFailed, nextStatus: constants.OktaAssignmentActionStatusCleanedUp, invalid: true},
		{startStatus: constants.OktaAssignmentActionStatusCleanupFailed, nextStatus: constants.OktaAssignmentActionStatusCleanupFailed, invalid: true},

		// UNKNOWN transitions
		{startStatus: constants.OktaAssignmentActionStatusUnknown, nextStatus: constants.OktaAssignmentActionStatusPending},
		{startStatus: constants.OktaAssignmentActionStatusUnknown, nextStatus: constants.OktaAssignmentActionStatusProcessing},
		{startStatus: constants.OktaAssignmentActionStatusUnknown, nextStatus: constants.OktaAssignmentActionStatusSuccessful},
		{startStatus: constants.OktaAssignmentActionStatusUnknown, nextStatus: constants.OktaAssignmentActionStatusFailed},
		{startStatus: constants.OktaAssignmentActionStatusUnknown, nextStatus: constants.OktaAssignmentActionStatusCleanupPending},
		{startStatus: constants.OktaAssignmentActionStatusUnknown, nextStatus: constants.OktaAssignmentActionStatusCleanupProcessing},
		{startStatus: constants.OktaAssignmentActionStatusUnknown, nextStatus: constants.OktaAssignmentActionStatusCleanedUp},
		{startStatus: constants.OktaAssignmentActionStatusUnknown, nextStatus: constants.OktaAssignmentActionStatusCleanupFailed},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s -> %s", test.startStatus, test.nextStatus), func(t *testing.T) {
			var errAssertionFunc require.ErrorAssertionFunc
			if test.invalid {
				errAssertionFunc = invalidTransition(test.startStatus, test.nextStatus)
			} else {
				errAssertionFunc = require.NoError
			}

			action := newOktaAssignmentAction(t, test.startStatus)
			errAssertionFunc(t, action.SetStatus(test.nextStatus))
		})
	}
}

func newOktaAssignmentAction(t *testing.T, status string) OktaAssignmentAction {
	action := &OktaAssignmentActionV1{
		Target: &OktaAssignmentActionTargetV1{
			Type: OktaAssignmentActionTargetV1_APPLICATION,
			Id:   "dummy",
		},
	}

	require.NoError(t, action.SetStatus(status))
	return action
}

func invalidTransition(startStatus, nextStatus string) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, _ ...interface{}) {
		require.ErrorIs(t, trace.BadParameter("invalid transition: %s -> %s", startStatus, nextStatus), err)
	}
}
