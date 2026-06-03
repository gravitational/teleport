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
	"time"

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

func TestOktAssignmentIsEqual(t *testing.T) {
	newAssignment := func(changeFns ...func(*OktaAssignmentV1)) *OktaAssignmentV1 {
		assignment := &OktaAssignmentV1{
			ResourceHeader: ResourceHeader{
				Kind:    KindOktaAssignment,
				Version: V1,
				Metadata: Metadata{
					Name: "name",
				},
			},
			Spec: OktaAssignmentSpecV1{
				User: "user",
				Targets: []*OktaAssignmentTargetV1{
					{Id: "1", Type: OktaAssignmentTargetV1_APPLICATION},
					{Id: "2", Type: OktaAssignmentTargetV1_GROUP},
				},
				CleanupTime:    time.Time{},
				Status:         OktaAssignmentSpecV1_PENDING,
				LastTransition: time.Time{},
				Finalized:      true,
			},
		}
		require.NoError(t, assignment.CheckAndSetDefaults())

		for _, fn := range changeFns {
			fn(assignment)
		}

		return assignment
	}
	tests := []struct {
		name     string
		o1       *OktaAssignmentV1
		o2       *OktaAssignmentV1
		expected bool
	}{
		{
			name:     "empty equals",
			o1:       &OktaAssignmentV1{},
			o2:       &OktaAssignmentV1{},
			expected: true,
		},
		{
			name:     "nil equals",
			o1:       nil,
			o2:       (*OktaAssignmentV1)(nil),
			expected: true,
		},
		{
			name:     "one is nil",
			o1:       &OktaAssignmentV1{},
			o2:       (*OktaAssignmentV1)(nil),
			expected: false,
		},
		{
			name:     "populated equals",
			o1:       newAssignment(),
			o2:       newAssignment(),
			expected: true,
		},
		{
			name: "resource header is different",
			o1:   newAssignment(),
			o2: newAssignment(func(o *OktaAssignmentV1) {
				o.ResourceHeader.Kind = "different-kind"
			}),
			expected: false,
		},
		{
			name: "user is different",
			o1:   newAssignment(),
			o2: newAssignment(func(o *OktaAssignmentV1) {
				o.Spec.User = "different-user"
			}),
			expected: false,
		},
		{
			name: "targets different id",
			o1:   newAssignment(),
			o2: newAssignment(func(o *OktaAssignmentV1) {
				o.Spec.Targets = []*OktaAssignmentTargetV1{
					{Id: "2", Type: OktaAssignmentTargetV1_APPLICATION},
					{Id: "2", Type: OktaAssignmentTargetV1_GROUP},
				}
			}),
			expected: false,
		},
		{
			name: "targets different type",
			o1:   newAssignment(),
			o2: newAssignment(func(o *OktaAssignmentV1) {
				o.Spec.Targets = []*OktaAssignmentTargetV1{
					{Id: "1", Type: OktaAssignmentTargetV1_GROUP},
					{Id: "2", Type: OktaAssignmentTargetV1_GROUP},
				}
			}),
			expected: false,
		},
		{
			name: "targets different sizes",
			o1:   newAssignment(),
			o2: newAssignment(func(o *OktaAssignmentV1) {
				o.Spec.Targets = []*OktaAssignmentTargetV1{
					{Id: "1", Type: OktaAssignmentTargetV1_APPLICATION},
				}
			}),
			expected: false,
		},
		{
			name: "targets both nil",
			o1: newAssignment(func(o *OktaAssignmentV1) {
				o.Spec.Targets = nil
			}),
			o2: newAssignment(func(o *OktaAssignmentV1) {
				o.Spec.Targets = nil
			}),
			expected: true,
		},
		{
			name: "targets o1 is nil",
			o1: newAssignment(func(o *OktaAssignmentV1) {
				o.Spec.Targets = nil
			}),
			o2:       newAssignment(),
			expected: false,
		},
		{
			name: "targets o2 is nil",
			o1:   newAssignment(),
			o2: newAssignment(func(o *OktaAssignmentV1) {
				o.Spec.Targets = nil
			}),
			expected: false,
		},
		{
			name: "cleanup time is different",
			o1:   newAssignment(),
			o2: newAssignment(func(o *OktaAssignmentV1) {
				o.Spec.CleanupTime = time.Date(1, 2, 3, 4, 5, 6, 7, time.UTC)
			}),
			expected: false,
		},
		{
			name: "status is different",
			o1:   newAssignment(),
			o2: newAssignment(func(o *OktaAssignmentV1) {
				o.Spec.Status = OktaAssignmentSpecV1_PROCESSING
			}),
			expected: false,
		},
		{
			name: "last transition is different",
			o1:   newAssignment(),
			o2: newAssignment(func(o *OktaAssignmentV1) {
				o.Spec.CleanupTime = time.Date(1, 2, 3, 4, 5, 6, 7, time.UTC)
			}),
			expected: false,
		},
		{
			name: "finalized is different",
			o1:   newAssignment(),
			o2: newAssignment(func(o *OktaAssignmentV1) {
				o.Spec.Finalized = false
			}),
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, test.o1.IsEqual(test.o2))
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

func Test_PluginOktaSyncSettings_SetUserSyncSource(t *testing.T) {
	t.Parallel()

	t.Run("known cases", func(t *testing.T) {
		known := []OktaUserSyncSource{
			OktaUserSyncSourceUnknown,
			OktaUserSyncSourceSamlApp,
			OktaUserSyncSourceOrg,
		}
		for _, userSyncSource := range known {
			syncSettings := &PluginOktaSyncSettings{}
			syncSettings.SetUserSyncSource(userSyncSource)
			require.Equal(t, userSyncSource, syncSettings.GetUserSyncSource())
		}

	})

	t.Run("edge cases", func(t *testing.T) {
		syncSettings := &PluginOktaSyncSettings{}

		// OktaUserSyncSourceUnknown is returned for empty value
		require.Empty(t, syncSettings.UserSyncSource)
		require.Equal(t, OktaUserSyncSourceUnknown, syncSettings.GetUserSyncSource())

		// When "asdf" is set, it doesn't change empty value
		syncSettings.SetUserSyncSource("asdf")
		require.Empty(t, syncSettings.UserSyncSource)
		require.Equal(t, OktaUserSyncSourceUnknown, syncSettings.GetUserSyncSource())

		// When "asdf" is set, it doesn't change set value
		syncSettings.UserSyncSource = string(OktaUserSyncSourceSamlApp)
		syncSettings.SetUserSyncSource("asdf")
		require.Equal(t, string(OktaUserSyncSourceSamlApp), syncSettings.UserSyncSource)
		require.Equal(t, OktaUserSyncSourceSamlApp, syncSettings.GetUserSyncSource())
	})
}

func Test_PluginOktaSyncSettings_SyncEnabledGetters(t *testing.T) {
	t.Run("on nil settings", func(t *testing.T) {
		syncSettings := (*PluginOktaSyncSettings)(nil)

		require.False(t, syncSettings.GetEnableUserSync())
		require.False(t, syncSettings.GetEnableAppGroupSync())
		require.False(t, syncSettings.GetEnableAccessListSync())
		require.False(t, syncSettings.GetEnableBidirectionalSync())
		require.False(t, syncSettings.GetEnableSystemLogExport())
		require.False(t, syncSettings.GetAssignDefaultRoles())
	})

	t.Run("on empty settings", func(t *testing.T) {
		syncSettings := &PluginOktaSyncSettings{}

		require.False(t, syncSettings.GetEnableUserSync())
		require.False(t, syncSettings.GetEnableAppGroupSync())
		require.False(t, syncSettings.GetEnableAccessListSync())
		require.False(t, syncSettings.GetEnableBidirectionalSync())
		require.False(t, syncSettings.GetEnableSystemLogExport())
		require.True(t, syncSettings.GetAssignDefaultRoles())
	})

	t.Run("on user sync enabled", func(t *testing.T) {
		syncSettings := &PluginOktaSyncSettings{
			SyncUsers: true,
		}

		require.True(t, syncSettings.GetEnableUserSync())
		require.True(t, syncSettings.GetEnableAppGroupSync()) // true by default
		require.False(t, syncSettings.GetEnableAccessListSync())
		require.True(t, syncSettings.GetEnableBidirectionalSync())
		require.False(t, syncSettings.GetEnableSystemLogExport())
		require.True(t, syncSettings.GetAssignDefaultRoles())
	})

	t.Run("on user sync enabled with disabled app and group sync", func(t *testing.T) {
		syncSettings := &PluginOktaSyncSettings{
			SyncUsers:                 true,
			DisableAssignDefaultRoles: true,
			DisableSyncAppGroups:      true,
		}

		require.True(t, syncSettings.GetEnableUserSync())
		require.False(t, syncSettings.GetEnableAppGroupSync())
		require.False(t, syncSettings.GetEnableAccessListSync())
		require.False(t, syncSettings.GetEnableBidirectionalSync())
		require.False(t, syncSettings.GetAssignDefaultRoles())
	})

	t.Run("on access list sync enabled", func(t *testing.T) {
		syncSettings := &PluginOktaSyncSettings{
			SyncUsers:       true,
			SyncAccessLists: true,
		}

		require.True(t, syncSettings.GetEnableUserSync())
		require.True(t, syncSettings.GetEnableAppGroupSync())
		require.True(t, syncSettings.GetEnableAccessListSync())
		require.True(t, syncSettings.GetEnableBidirectionalSync()) // true by default
		require.False(t, syncSettings.GetEnableSystemLogExport())
		require.True(t, syncSettings.GetAssignDefaultRoles())
	})

	t.Run("on access list sync enabled with bidirectional sync disabled", func(t *testing.T) {
		syncSettings := &PluginOktaSyncSettings{
			SyncUsers:                true,
			SyncAccessLists:          true,
			DisableBidirectionalSync: true,
			EnableSystemLogExport:    true,
		}

		require.True(t, syncSettings.GetEnableUserSync())
		require.True(t, syncSettings.GetEnableAppGroupSync())
		require.True(t, syncSettings.GetEnableAccessListSync())
		require.False(t, syncSettings.GetEnableBidirectionalSync())
		require.True(t, syncSettings.GetEnableSystemLogExport())
		require.True(t, syncSettings.GetAssignDefaultRoles())
	})
}
