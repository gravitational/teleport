package okta

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

type testUACAccessPoint struct {
	*testAccessPoint

	userState *userloginstate.UserLoginState
}

// GetUserOrLoginState will return the given user or the login state associated with the user.
func (t *testUACAccessPoint) GetUserOrLoginState(ctx context.Context, username string) (services.UserState, error) {
	if t.userState == nil {
		return t.GetUser(ctx, username, false)
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

	role, err := auth.CreateRole(ctx, ap, testRole, types.RoleSpecV6{
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

	user, err = ap.CreateUser(ctx, user)
	require.NoError(t, err)

	assignments, _, err := ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, assignments)

	// No valid resources, so this should exit quickly and produce nothing.
	require.NoError(t, uac.OnLogin(ctx, user))

	assignments, _, err = ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, assignments)

	app1 := application(t, uac.hash, "app1", "link", types.OriginOkta, testOrgURL, testHostID)
	_, err = ap.UpsertApplicationServer(ctx, app1)
	require.NoError(t, err)
	appDupe := application(t, uac.hash, "app1", "link", types.OriginOkta, testOrgURL, "dummy-host")
	_, err = ap.UpsertApplicationServer(ctx, appDupe)
	require.NoError(t, err)

	// App servers from different hosts will both show up under list resources.
	resources, err := ap.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType: types.KindAppServer,
		Limit:        5,
	})
	require.NoError(t, err)
	require.Len(t, resources.Resources, 2)

	// Should get an assignment that has one action for the app.
	require.NoError(t, uac.OnLogin(ctx, user))

	assignments, _, err = ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)

	expectedName, err := uacAssignmentName(uac.hash, testUser, []string{}, []string{app1.GetName()})
	require.NoError(t, err)

	var allAssignments []types.OktaAssignment
	expectedAssignmentAppOnly, err := types.NewOktaAssignment(types.Metadata{
		Name: expectedName,
		Labels: map[string]string{
			teleport.OktaAssignmentSourceLabel: userAssignmentCreatorSource,
		},
	}, types.OktaAssignmentSpecV1{
		User: testUser,
		Targets: []*types.OktaAssignmentTargetV1{
			{
				Type: types.OktaAssignmentTargetV1_APPLICATION,
				Id:   app1.GetName(),
			},
		},
		Status:         types.OktaAssignmentSpecV1_PENDING,
		LastTransition: clock.Now(),
	})
	require.NoError(t, err)

	allAssignments = append(allAssignments, expectedAssignmentAppOnly)

	cmpOpts := cmp.Options{
		cmpopts.IgnoreFields(
			types.OktaAssignmentV1{},
			"ResourceHeader.Metadata.ID",
			"ResourceHeader.Metadata.Revision"),
		cmpopts.SortSlices(assignmentLess),
	}
	require.Empty(t, cmp.Diff(allAssignments, assignments, cmpOpts))

	require.NoError(t, ap.DeleteApplicationServer(ctx, defaults.Namespace, app1.GetHostID(), app1.GetName()))
	require.NoError(t, ap.DeleteApplicationServer(ctx, defaults.Namespace, appDupe.GetHostID(), appDupe.GetName()))

	group1 := group(t, "group1", types.OriginOkta, testOrgURL)
	require.NoError(t, ap.CreateUserGroup(ctx, group1))

	// Should get an assignment that has one action for the group.
	require.NoError(t, uac.OnLogin(ctx, user))

	assignments, _, err = ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)

	expectedName, err = uacAssignmentName(uac.hash, testUser, []string{group1.GetName()}, []string{})
	require.NoError(t, err)

	expectedAssignmentAppOnly.SetCleanupTime(clock.Now())

	expectedAssignmentGroupOnly, err := types.NewOktaAssignment(types.Metadata{
		Name: expectedName,
		Labels: map[string]string{
			teleport.OktaAssignmentSourceLabel: userAssignmentCreatorSource,
		},
	}, types.OktaAssignmentSpecV1{
		User: testUser,
		Targets: []*types.OktaAssignmentTargetV1{
			{
				Type: types.OktaAssignmentTargetV1_GROUP,
				Id:   group1.GetName(),
			},
		},
		Status:         types.OktaAssignmentSpecV1_PENDING,
		LastTransition: clock.Now(),
	})
	require.NoError(t, err)

	allAssignments = append(allAssignments, expectedAssignmentGroupOnly)

	require.Empty(t, cmp.Diff(allAssignments, assignments, cmpOpts))

	// Re-add application server.
	_, err = ap.UpsertApplicationServer(ctx, app1)
	require.NoError(t, err)

	// Should get an assignment that has two actions: one for the app and one for the group.
	require.NoError(t, uac.OnLogin(ctx, user))

	assignments, _, err = ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)

	expectedName, err = uacAssignmentName(uac.hash, testUser, []string{group1.GetName()}, []string{app1.GetName()})
	require.NoError(t, err)

	expectedAssignmentGroupOnly.SetCleanupTime(clock.Now())

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
				Id:   group1.GetName(),
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

	allAssignments = append(allAssignments, expectedAssignment1)
	require.Empty(t, cmp.Diff(allAssignments, assignments, cmpOpts))

	// We'll add a new app, which should cause a new assignment to be generated and the
	// old one to be marked as needing cleanup.
	app2 := application(t, uac.hash, "app2", "link", types.OriginOkta, testOrgURL, testHostID)
	_, err = ap.UpsertApplicationServer(ctx, app2)
	require.NoError(t, err)

	require.NoError(t, uac.OnLogin(ctx, user))

	assignments, _, err = ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)

	expectedName, err = uacAssignmentName(uac.hash, testUser, []string{group1.GetName()}, []string{app1.GetName(), app2.GetName()})
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
				Id:   group1.GetName(),
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

	allAssignments = append(allAssignments, expectedAssignment2)

	require.Empty(t, cmp.Diff(allAssignments, assignments, cmpOpts))

	// We'll add in a group and make sure that triggers a second cleanup and another new assignment.
	group2 := group(t, "group2", types.OriginOkta, testOrgURL)
	require.NoError(t, ap.CreateUserGroup(ctx, group2))

	require.NoError(t, uac.OnLogin(ctx, user))

	assignments, _, err = ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)

	expectedName, err = uacAssignmentName(uac.hash, testUser, []string{group1.GetName(), "group2"}, []string{app1.GetName(), app2.GetName()})
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
				Id:   group1.GetName(),
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

	allAssignments = append(allAssignments, expectedAssignment3)

	require.Empty(t, cmp.Diff(allAssignments, assignments, cmpOpts))

	// We'll delete the old group and ensure that the old assignment is restored.
	require.NoError(t, ap.DeleteUserGroup(ctx, group2.GetName()))

	require.NoError(t, uac.OnLogin(ctx, user))

	assignments, _, err = ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)

	// Old assignment should be marked as needing cleanup, other assignment should be restored.
	expectedAssignment2.SetCleanupTime(time.Time{})
	expectedAssignment3.SetCleanupTime(clock.Now())

	require.Empty(t, cmp.Diff(allAssignments, assignments, cmpOpts))

	// Lock should cause all assignments to be cleaned up.
	lock, err := types.NewLock("lock", types.LockSpecV2{
		Target: types.LockTarget{
			User: testUser,
		},
	})
	require.NoError(t, err)
	require.NoError(t, ap.UpsertLock(ctx, lock))

	require.NoError(t, uac.OnLogin(ctx, user))

	assignments, _, err = ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)

	expectedAssignment2.SetCleanupTime(clock.Now())

	require.Empty(t, cmp.Diff(allAssignments, assignments, cmpOpts))

	// Delete lock should restore assignments
	require.NoError(t, ap.DeleteLock(ctx, "lock"))

	require.NoError(t, uac.OnLogin(ctx, user))

	assignments, _, err = ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)

	expectedAssignment2.SetCleanupTime(time.Time{})

	require.Empty(t, cmp.Diff(allAssignments, assignments, cmpOpts))

	// Create an empty user state, which should cause a cleanup of all assignments since it has no permissions.
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

	require.Empty(t, cmp.Diff(allAssignments, assignments, cmpOpts))
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
