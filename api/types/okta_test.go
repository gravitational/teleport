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
		{startStatus: constants.OktaAssignmentStatusPending, nextStatus: constants.OktaAssignmentStatusPending, invalid: true},
		{startStatus: constants.OktaAssignmentStatusPending, nextStatus: constants.OktaAssignmentStatusProcessing},
		{startStatus: constants.OktaAssignmentStatusPending, nextStatus: constants.OktaAssignmentStatusSuccessful, invalid: true},
		{startStatus: constants.OktaAssignmentStatusPending, nextStatus: constants.OktaAssignmentStatusFailed, invalid: true},

		// PROCESSING transitions
		{startStatus: constants.OktaAssignmentStatusProcessing, nextStatus: constants.OktaAssignmentStatusPending, invalid: true},
		{startStatus: constants.OktaAssignmentStatusProcessing, nextStatus: constants.OktaAssignmentStatusProcessing},
		{startStatus: constants.OktaAssignmentStatusProcessing, nextStatus: constants.OktaAssignmentStatusSuccessful},
		{startStatus: constants.OktaAssignmentStatusProcessing, nextStatus: constants.OktaAssignmentStatusFailed},

		// SUCCESSFUL transitions
		{startStatus: constants.OktaAssignmentStatusSuccessful, nextStatus: constants.OktaAssignmentStatusPending, invalid: true},
		{startStatus: constants.OktaAssignmentStatusSuccessful, nextStatus: constants.OktaAssignmentStatusProcessing},
		{startStatus: constants.OktaAssignmentStatusSuccessful, nextStatus: constants.OktaAssignmentStatusSuccessful, invalid: true},
		{startStatus: constants.OktaAssignmentStatusSuccessful, nextStatus: constants.OktaAssignmentStatusFailed, invalid: true},

		// FAILED transitions
		{startStatus: constants.OktaAssignmentStatusFailed, nextStatus: constants.OktaAssignmentStatusPending, invalid: true},
		{startStatus: constants.OktaAssignmentStatusFailed, nextStatus: constants.OktaAssignmentStatusProcessing},
		{startStatus: constants.OktaAssignmentStatusFailed, nextStatus: constants.OktaAssignmentStatusSuccessful, invalid: true},
		{startStatus: constants.OktaAssignmentStatusFailed, nextStatus: constants.OktaAssignmentStatusFailed, invalid: true},

		// UNKNOWN transitions
		{startStatus: constants.OktaAssignmentStatusUnknown, nextStatus: constants.OktaAssignmentStatusPending},
		{startStatus: constants.OktaAssignmentStatusUnknown, nextStatus: constants.OktaAssignmentStatusProcessing},
		{startStatus: constants.OktaAssignmentStatusUnknown, nextStatus: constants.OktaAssignmentStatusSuccessful},
		{startStatus: constants.OktaAssignmentStatusUnknown, nextStatus: constants.OktaAssignmentStatusFailed},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s -> %s", test.startStatus, test.nextStatus), func(t *testing.T) {
			var errAssertionFunc require.ErrorAssertionFunc
			if test.invalid {
				errAssertionFunc = invalidTransition(test.startStatus, test.nextStatus)
			} else {
				errAssertionFunc = require.NoError
			}

			assignment := newOktaAssignment(t, test.startStatus)
			errAssertionFunc(t, assignment.SetStatus(test.nextStatus))
		})
	}
}

func newOktaAssignment(t *testing.T, status string) OktaAssignment {
	assignment := &OktaAssignmentV1{}

	require.NoError(t, assignment.SetStatus(status))
	return assignment
}

func invalidTransition(startStatus, nextStatus string) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, _ ...interface{}) {
		require.ErrorIs(t, trace.BadParameter("invalid transition: %s -> %s", startStatus, nextStatus), err)
	}
}
