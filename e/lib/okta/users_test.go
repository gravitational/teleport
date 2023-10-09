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

package okta

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/services"
)

type testUACAccessPoint struct {
	*testAccessPoint

	userState *userloginstate.UserLoginState
}

// GetUserOrLoginState will return the given user or the login state associated with the user.
func (t *testUACAccessPoint) GetUserOrLoginState(ctx context.Context, username string) (services.UserState, error) {
	if t.userState == nil {
		return t.GetUser(username, false)
	}
	return t.userState, nil
}

func TestUserAssignmentCreator(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	ap := &testUACAccessPoint{
		testAccessPoint: newTestAccessPoint(t, clock),
	}
	uac, err := NewUserAssignmentCreator(UserAssignmentCreatorConfig{
		Clock:       clock,
		ClusterName: testClusterName,
		AccessPoint: ap,
	})
	require.NoError(t, err)

	// set app and group page sizes to 1 to make sure we exercise pagination logic.
	uac.groupPageSize = 1
	uac.appPageSize = 1

	testUser := "test-user@test.user"
	testRole := "test-role"

	role, err := types.NewRole(testRole, types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
			GroupLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
			Rules: []types.Rule{
				{
					Resources: []string{
						types.KindAppServer,
						types.KindUserGroup,
					},
					Verbs: []string{
						types.VerbRead, types.VerbList,
					},
				},
			},
		},
	})
	require.NoError(t, err)
	user, err := types.NewUser(testUser)
	require.NoError(t, err)
	user.SetRoles([]string{role.GetName()})

	require.NoError(t, ap.CreateRole(ctx, role))
	user, err = ap.CreateUserWithContext(ctx, user)
	require.NoError(t, err)

	assignments, _, err := ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, assignments)

	// No valid resources, so this should exit quickly and produce nothing.
	require.NoError(t, uac.OnLogin(ctx, user))

	assignments, _, err = ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, assignments)

	app1 := application(t, uac.hash, "app1", "link", types.OriginOkta, testOrgURL)
	_, err = ap.UpsertApplicationServer(ctx, app1)
	require.NoError(t, err)

	group1 := group(t, "group1", types.OriginOkta, testOrgURL)
	require.NoError(t, ap.CreateUserGroup(ctx, group1))

	// Should get an assignment that has two actions: one for the app and one for the group.
	require.NoError(t, uac.OnLogin(ctx, user))

	assignments, _, err = ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)

	expectedName, err := uacAssignmentName(uac.hash, testUser, []string{"group1"}, []string{app1.GetName()})
	require.NoError(t, err)

	expectedAssignment1, err := types.NewOktaAssignment(types.Metadata{
		Name: expectedName,
		Labels: map[string]string{
			teleport.OktaAssignmentSourceLabel: userAssignmentCreatorSource,
		},
	}, types.OktaAssignmentSpecV1{
		User: testUser,
		Targets: []*types.OktaAssignmentTargetV1{
			{
				Type: types.OktaAssignmentTargetV1_GROUP,
				Id:   "group1",
			},
			{
				Type: types.OktaAssignmentTargetV1_APPLICATION,
				Id:   app1.GetName(),
			},
		},
		Status:         types.OktaAssignmentSpecV1_PENDING,
		LastTransition: clock.Now(),
	})
	require.NoError(t, err)

	require.Empty(t, cmp.Diff([]types.OktaAssignment{expectedAssignment1}, assignments))

	// We'll add a new app, which should cause a new assignment to be generated and the
	// old one to be marked as needing cleanup.
	app2 := application(t, uac.hash, "app2", "link", types.OriginOkta, testOrgURL)
	_, err = ap.UpsertApplicationServer(ctx, app2)
	require.NoError(t, err)

	require.NoError(t, uac.OnLogin(ctx, user))

	assignments, _, err = ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)

	expectedName, err = uacAssignmentName(uac.hash, testUser, []string{"group1"}, []string{app1.GetName(), app2.GetName()})
	require.NoError(t, err)

	// Old assignment should be marked as needing cleanup.
	expectedAssignment1.SetCleanupTime(clock.Now())

	expectedAssignment2, err := types.NewOktaAssignment(types.Metadata{
		Name: expectedName,
		Labels: map[string]string{
			teleport.OktaAssignmentSourceLabel: userAssignmentCreatorSource,
		},
	}, types.OktaAssignmentSpecV1{
		User: testUser,
		Targets: []*types.OktaAssignmentTargetV1{
			{
				Type: types.OktaAssignmentTargetV1_GROUP,
				Id:   "group1",
			},
			{
				Type: types.OktaAssignmentTargetV1_APPLICATION,
				Id:   app1.GetName(),
			},
			{
				Type: types.OktaAssignmentTargetV1_APPLICATION,
				Id:   app2.GetName(),
			},
		},
		Status:         types.OktaAssignmentSpecV1_PENDING,
		LastTransition: clock.Now(),
	})
	require.NoError(t, err)

	require.Empty(t, cmp.Diff([]types.OktaAssignment{expectedAssignment1, expectedAssignment2}, assignments,
		cmpopts.SortSlices(assignmentLess)))

	// We'll add in a group and make sure that triggers a second cleanup and another new assignment.
	group2 := group(t, "group2", types.OriginOkta, testOrgURL)
	require.NoError(t, ap.CreateUserGroup(ctx, group2))

	require.NoError(t, uac.OnLogin(ctx, user))

	assignments, _, err = ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)

	expectedName, err = uacAssignmentName(uac.hash, testUser, []string{"group1", "group2"}, []string{app1.GetName(), app2.GetName()})
	require.NoError(t, err)

	// Old assignment should be given a cleanup time.
	expectedAssignment2.SetCleanupTime(clock.Now())

	expectedAssignment3, err := types.NewOktaAssignment(types.Metadata{
		Name: expectedName,
		Labels: map[string]string{
			teleport.OktaAssignmentSourceLabel: userAssignmentCreatorSource,
		},
	}, types.OktaAssignmentSpecV1{
		User: testUser,
		Targets: []*types.OktaAssignmentTargetV1{
			{
				Type: types.OktaAssignmentTargetV1_GROUP,
				Id:   "group1",
			},
			{
				Type: types.OktaAssignmentTargetV1_GROUP,
				Id:   "group2",
			},
			{
				Type: types.OktaAssignmentTargetV1_APPLICATION,
				Id:   app1.GetName(),
			},
			{
				Type: types.OktaAssignmentTargetV1_APPLICATION,
				Id:   app2.GetName(),
			},
		},
		Status:         types.OktaAssignmentSpecV1_PENDING,
		LastTransition: clock.Now(),
	})
	require.NoError(t, err)

	require.Empty(t, cmp.Diff([]types.OktaAssignment{expectedAssignment1, expectedAssignment2, expectedAssignment3}, assignments,
		cmpopts.SortSlices(assignmentLess)))

	// We'll delete the old group and ensure that the old assignment is restored.
	require.NoError(t, ap.DeleteUserGroup(ctx, group2.GetName()))

	require.NoError(t, uac.OnLogin(ctx, user))

	assignments, _, err = ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)

	// Old assignment should be marked as needing cleanup, other assignment should be restored.
	expectedAssignment2.SetCleanupTime(time.Time{})
	expectedAssignment3.SetCleanupTime(clock.Now())

	require.Empty(t, cmp.Diff([]types.OktaAssignment{expectedAssignment1, expectedAssignment2, expectedAssignment3}, assignments,
		cmpopts.SortSlices(assignmentLess)))

	// Create an empty user state, which should cause a cleanup of all assignmentssince it has no permissions.
	ap.userState, err = userloginstate.New(header.Metadata{
		Name: testUser,
	}, userloginstate.Spec{})
	require.NoError(t, err)

	require.NoError(t, uac.OnLogin(ctx, user))

	assignments, _, err = ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)

	// All assignments should have a cleanup time set to now.
	expectedAssignment1.SetCleanupTime(clock.Now())
	expectedAssignment2.SetCleanupTime(clock.Now())

	require.Empty(t, cmp.Diff([]types.OktaAssignment{expectedAssignment1, expectedAssignment2, expectedAssignment3}, assignments,
		cmpopts.SortSlices(assignmentLess)))
}

func TestAssignmentDiff(t *testing.T) {
	t.Parallel()

	testUser := "test-user@test.user"

	tests := []struct {
		name                  string
		newAssignment         types.OktaAssignment
		oldAssignments        types.OktaAssignments
		expectedNewGroups     []string
		expectedNewApps       []string
		expectedRemovedGroups []string
		expectedRemovedApps   []string
	}{
		{
			name: "all new groups and apps",
			newAssignment: assignment(t, "assignment1", testUser, time.Time{}, constants.OktaAssignmentStatusPending, time.Time{}, false,
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
				target(types.OktaAssignmentTargetV1_GROUP, "group2"),
				target(types.OktaAssignmentTargetV1_APPLICATION, "application1"),
				target(types.OktaAssignmentTargetV1_APPLICATION, "application2"),
			),
			expectedNewGroups: []string{
				"group1",
				"group2",
			},
			expectedNewApps: []string{
				"application1",
				"application2",
			},
		},
		{
			name: "some new, some old, some removed",
			newAssignment: assignment(t, "assignment1", testUser, time.Time{}, constants.OktaAssignmentStatusPending, time.Time{}, false,
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
				target(types.OktaAssignmentTargetV1_GROUP, "group2"),
				target(types.OktaAssignmentTargetV1_APPLICATION, "application1"),
				target(types.OktaAssignmentTargetV1_APPLICATION, "application3"),
			),
			oldAssignments: types.OktaAssignments{
				assignment(t, "assignment1", testUser, time.Time{}, constants.OktaAssignmentStatusPending, time.Time{}, false,
					target(types.OktaAssignmentTargetV1_GROUP, "group2"),
					target(types.OktaAssignmentTargetV1_GROUP, "group3"),
					target(types.OktaAssignmentTargetV1_APPLICATION, "application1"),
					target(types.OktaAssignmentTargetV1_APPLICATION, "application2"),
				),
			},
			expectedNewGroups: []string{
				"group1",
			},
			expectedNewApps: []string{
				"application3",
			},
			expectedRemovedGroups: []string{
				"group3",
			},
			expectedRemovedApps: []string{
				"application2",
			},
		},
		{
			name: "all removed",
			oldAssignments: types.OktaAssignments{
				assignment(t, "assignment1", testUser, time.Time{}, constants.OktaAssignmentStatusPending, time.Time{}, false,
					target(types.OktaAssignmentTargetV1_GROUP, "group2"),
					target(types.OktaAssignmentTargetV1_GROUP, "group3"),
					target(types.OktaAssignmentTargetV1_APPLICATION, "application1"),
					target(types.OktaAssignmentTargetV1_APPLICATION, "application2"),
				),
			},
			expectedRemovedGroups: []string{
				"group2",
				"group3",
			},
			expectedRemovedApps: []string{
				"application1",
				"application2",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			newGroups, newApps, removedGroups, removedApps := assignmentDiff(test.newAssignment, test.oldAssignments...)

			require.Equal(t, test.expectedNewGroups, newGroups)
			require.Equal(t, test.expectedNewApps, newApps)
			require.Equal(t, test.expectedRemovedGroups, removedGroups)
			require.Equal(t, test.expectedRemovedApps, removedApps)
		})
	}
}
