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

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
)

func TestOktaAssignments_SetStatus(t *testing.T) {
	testTargets := []*OktaAssignmentTargetV1{
		newOktaAssignmentTarget(OktaAssignmentTargetV1_GROUP, "1"),
	}
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

			assignment := newOktaAssignment(t, "assignment", test.startStatus, testTargets)
			errAssertionFunc(t, assignment.SetStatus(test.nextStatus))
		})
	}
}

func TestOktaAssignmentTargets(t *testing.T) {
	tests := []struct {
		name            string
		targets         []*OktaAssignmentTargetV1
		expectedTargets []OktaAssignmentTarget
	}{
		{
			name: "no duplicates",
			targets: []*OktaAssignmentTargetV1{
				newOktaAssignmentTarget(OktaAssignmentTargetV1_GROUP, "1"),
				newOktaAssignmentTarget(OktaAssignmentTargetV1_GROUP, "2"),
				newOktaAssignmentTarget(OktaAssignmentTargetV1_GROUP, "3"),
				newOktaAssignmentTarget(OktaAssignmentTargetV1_APPLICATION, "1"),
				newOktaAssignmentTarget(OktaAssignmentTargetV1_APPLICATION, "2"),
				newOktaAssignmentTarget(OktaAssignmentTargetV1_APPLICATION, "3"),
			},
			expectedTargets: []OktaAssignmentTarget{
				newOktaAssignmentTarget(OktaAssignmentTargetV1_APPLICATION, "1"),
				newOktaAssignmentTarget(OktaAssignmentTargetV1_APPLICATION, "2"),
				newOktaAssignmentTarget(OktaAssignmentTargetV1_APPLICATION, "3"),
				newOktaAssignmentTarget(OktaAssignmentTargetV1_GROUP, "1"),
				newOktaAssignmentTarget(OktaAssignmentTargetV1_GROUP, "2"),
				newOktaAssignmentTarget(OktaAssignmentTargetV1_GROUP, "3"),
			},
		},
		{
			name: "duplicates",
			targets: []*OktaAssignmentTargetV1{
				newOktaAssignmentTarget(OktaAssignmentTargetV1_GROUP, "1"),
				newOktaAssignmentTarget(OktaAssignmentTargetV1_GROUP, "1"),
				newOktaAssignmentTarget(OktaAssignmentTargetV1_GROUP, "1"),
				newOktaAssignmentTarget(OktaAssignmentTargetV1_APPLICATION, "1"),
				newOktaAssignmentTarget(OktaAssignmentTargetV1_APPLICATION, "1"),
				newOktaAssignmentTarget(OktaAssignmentTargetV1_APPLICATION, "1"),
			},
			expectedTargets: []OktaAssignmentTarget{
				newOktaAssignmentTarget(OktaAssignmentTargetV1_APPLICATION, "1"),
				newOktaAssignmentTarget(OktaAssignmentTargetV1_GROUP, "1"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assignment := newOktaAssignment(t, "assignment", constants.OktaAssignmentStatusPending, test.targets)
			targets := assignment.GetTargets()
			require.Empty(t, cmp.Diff(test.expectedTargets, targets))
		})
	}
}

func newOktaAssignment(t *testing.T, name, status string, targets []*OktaAssignmentTargetV1) OktaAssignment {
	assignment, err := NewOktaAssignment(Metadata{
		Name: name,
	}, OktaAssignmentSpecV1{
		Targets: targets,
		Status:  OktaAssignmentStatusToProto(status),
		User:    "test-user",
	})
	require.NoError(t, err)
	return assignment
}

func newOktaAssignmentTarget(targetType OktaAssignmentTargetV1_OktaAssignmentTargetType, id string) *OktaAssignmentTargetV1 {
	return &OktaAssignmentTargetV1{
		Type: targetType,
		Id:   id,
	}
}

func invalidTransition(startStatus, nextStatus string) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, _ ...interface{}) {
		require.ErrorIs(t, trace.BadParameter("invalid transition: %s -> %s", startStatus, nextStatus), err)
	}
}
