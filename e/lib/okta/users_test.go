package okta

import (
	"context"
	"crypto"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/e/lib/okta/common/connected"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

type testUACAccessPoint struct {
	*testAccessPoint

	userState *userloginstate.UserLoginState
	uac       *UserAssignmentCreator
	user      types.User
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
	suite := initUACSuite(t, ctx, clock)

	app1 := application(t, suite.uac.hash, "app1", "link", types.OriginOkta, testOrgURL, testHostID)
	app2 := application(t, suite.uac.hash, "app2", "link", types.OriginOkta, testOrgURL, testHostID)
	appDupe := application(t, suite.uac.hash, "app1", "link", types.OriginOkta, testOrgURL, "dummy-host")
	group1 := group(t, "group1", types.OriginOkta, testOrgURL)
	group2 := group(t, "group2", types.OriginOkta, testOrgURL)

	uac := suite.uac
	user := suite.user
	testUser := user.GetName()
	ap := suite

	// No valid resources, so this should exit quickly and produce nothing.
	require.NoError(t, uac.OnLogin(ctx, user))
	assertEmptyAssignmentList(t, ap)

	for _, v := range []types.AppServer{app1, appDupe} {
		mustUpsertApplicationServer(t, ctx, ap, v)
	}
	assertResourceCount(t, ctx, ap, 2)

	// This run should be skipped since no Okta roles or plugins are connected.
	ap.serviceCounts[types.RoleOkta] = 0

	require.NoError(t, uac.OnLogin(ctx, user))

	// Reconnect the service. Should get an assignment that has one action for the app.
	ap.serviceCounts[types.RoleOkta] = 1

	t.Run("test assignment creation", func(t *testing.T) {
		require.NoError(t, uac.OnLogin(ctx, user))
		want := mustCreateAssignmentList(t, uac.hash, testUser, clock)([][]types.Resource{
			{app1},
		})
		mustFetchAndAssertAssignments(t, ctx, ap, want)
	})

	t.Run("test appserver deletion", func(t *testing.T) {
		require.NoError(t, ap.DeleteApplicationServer(ctx, defaults.Namespace, app1.GetHostID(), app1.GetName()))
		require.NoError(t, ap.DeleteApplicationServer(ctx, defaults.Namespace, appDupe.GetHostID(), appDupe.GetName()))
		require.NoError(t, ap.CreateUserGroup(ctx, group1))

		require.NoError(t, uac.OnLogin(ctx, user))
		mustDeleteCleanupAssignments(t, ctx, ap)

		want := mustCreateAssignmentList(t, uac.hash, testUser, clock)([][]types.Resource{
			{group1},
		})
		mustFetchAndAssertAssignments(t, ctx, ap, want)
	})

	t.Run("re-add application", func(t *testing.T) {
		mustUpsertApplicationServer(t, ctx, ap, app1)

		// Should get an assignment that has two actions: one for the app and one for the group.
		require.NoError(t, uac.OnLogin(ctx, user))
		mustDeleteCleanupAssignments(t, ctx, ap)

		want := mustCreateAssignmentList(t, uac.hash, testUser, clock)([][]types.Resource{
			{group1, app1},
		})
		mustFetchAndAssertAssignments(t, ctx, ap, want)
	})

	t.Run("Add a new app", func(t *testing.T) {
		// We'll add a new app, which should cause a new assignment to be generated and the
		// old one to be marked as needing cleanup.
		mustUpsertApplicationServer(t, ctx, ap, app2)

		require.NoError(t, uac.OnLogin(ctx, user))
		mustDeleteCleanupAssignments(t, ctx, ap)

		want := mustCreateAssignmentList(t, uac.hash, testUser, clock)([][]types.Resource{
			{group1, app1, app2},
		})
		mustFetchAndAssertAssignments(t, ctx, ap, want)
	})

	t.Run("Add a new group", func(t *testing.T) {
		// We'll add in a group and make sure that triggers a second cleanup and another new assignment.
		require.NoError(t, ap.CreateUserGroup(ctx, group2))

		require.NoError(t, uac.OnLogin(ctx, user))
		mustDeleteCleanupAssignments(t, ctx, ap)

		want := mustCreateAssignmentList(t, uac.hash, testUser, clock)([][]types.Resource{
			{group1, group2, app1, app2},
		})
		mustFetchAndAssertAssignments(t, ctx, ap, want)
	})

	t.Run("Remove the group", func(t *testing.T) {
		// We'll delete the old group and ensure that the old assignment is restored.
		require.NoError(t, ap.DeleteUserGroup(ctx, group2.GetName()))

		require.NoError(t, uac.OnLogin(ctx, user))
		mustDeleteCleanupAssignments(t, ctx, ap)

		want := mustCreateAssignmentList(t, uac.hash, testUser, clock)([][]types.Resource{
			{group1, app1, app2},
		})
		mustFetchAndAssertAssignments(t, ctx, ap, want)
	})

	t.Run("lock user", func(t *testing.T) {
		lock, err := types.NewLock("lock", types.LockSpecV2{
			Target: types.LockTarget{
				User: testUser,
			},
		})
		require.NoError(t, err)
		require.NoError(t, ap.UpsertLock(ctx, lock))

		require.NoError(t, uac.OnLogin(ctx, user))
		mustDeleteCleanupAssignments(t, ctx, ap)
		want := mustCreateAssignmentList(t, uac.hash, testUser, clock)([][]types.Resource{
			{group1, app1, app2},
		})
		mustFetchAndAssertAssignments(t, ctx, ap, want)
	})

	t.Run("unlock user", func(t *testing.T) {
		require.NoError(t, ap.DeleteLock(ctx, "lock"))

		require.NoError(t, uac.OnLogin(ctx, user))
		mustDeleteCleanupAssignments(t, ctx, ap)

		mustDeleteCleanupAssignments(t, ctx, ap)
		want := mustCreateAssignmentList(t, uac.hash, testUser, clock)([][]types.Resource{
			{group1, app1, app2},
		})
		mustFetchAndAssertAssignments(t, ctx, ap, want)
	})
}

func assertResourceCount(t *testing.T, ctx context.Context, ap *testUACAccessPoint, want int) {
	resources, err := ap.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType: types.KindAppServer,
		Limit:        5,
	})
	require.NoError(t, err)
	require.Len(t, resources.Resources, want)
}

func mustUpsertApplicationServer(t *testing.T, ctx context.Context, ap *testUACAccessPoint, app1 types.AppServer) {
	_, err := ap.UpsertApplicationServer(ctx, app1)
	require.NoError(t, err)
}

func TestAssignmentDiff(t *testing.T) {
	t.Parallel()

	testUser := userName("test-user@test.user")

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

func assertEmptyAssignmentList(t *testing.T, ap *testUACAccessPoint) {
	assignments, _, err := ap.ListOktaAssignments(context.Background(), 0, "")
	require.NoError(t, err)
	require.Empty(t, assignments)
}

func mustFetchAndAssertAssignments(t *testing.T, ctx context.Context, ap *testUACAccessPoint, want []types.OktaAssignment) {
	assignments, _, err := ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)
	assertAssignments(t, want, assignments)
}

func mustDeleteCleanupAssignments(t *testing.T, ctx context.Context, ap *testUACAccessPoint) {
	assignments, _, err := ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)
	for _, item := range assignments {
		if !item.GetCleanupTime().IsZero() {
			err := ap.DeleteOktaAssignment(ctx, item.GetName())
			require.NoError(t, err)
		}
	}
}

func mustCreateAssignmentList(t *testing.T, hash crypto.Hash, user string, clock clockwork.FakeClock) func([][]types.Resource) []types.OktaAssignment {
	return func(itemsTargets [][]types.Resource) []types.OktaAssignment {
		var groups []string
		var apps []string
		var out []types.OktaAssignment

		for _, v := range itemsTargets {
			var targets []*types.OktaAssignmentTargetV1
			for _, item := range v {
				switch t := item.(type) {
				case types.UserGroup:
					targets = append(targets, &types.OktaAssignmentTargetV1{
						Type: types.OktaAssignmentTargetV1_GROUP,
						Id:   t.GetName(),
					})
					groups = append(groups, t.GetName())
				case types.AppServer:
					targets = append(targets, &types.OktaAssignmentTargetV1{
						Type: types.OktaAssignmentTargetV1_APPLICATION,
						Id:   t.GetName(),
					})
					apps = append(apps, t.GetName())
				}
			}
			name, err := uacAssignmentName(hash, user, groups, apps)
			require.NoError(t, err)
			item, err := types.NewOktaAssignment(types.Metadata{
				Name: name,
				Labels: map[string]string{
					teleport.OktaAssignmentSourceLabel: userAssignmentCreatorSource,
				},
			}, types.OktaAssignmentSpecV1{
				User:           user,
				Targets:        targets,
				Status:         types.OktaAssignmentSpecV1_PENDING,
				LastTransition: clock.Now(),
			})
			require.NoError(t, err)
			out = append(out, item)
		}
		return out
	}
}

func initUACSuite(t *testing.T, ctx context.Context, clock clockwork.Clock) *testUACAccessPoint {
	ap := &testUACAccessPoint{
		testAccessPoint: newTestAccessPoint(t, clock),
	}
	ap.serviceCounts[types.RoleOkta] = 1
	connected, err := connected.New(connected.Config{
		DisableCache:    true,
		ConnectedGetter: ap,
		Plugins:         ap,
	})
	require.NoError(t, err)

	ap.uac, err = NewUserAssignmentCreator(UserAssignmentCreatorConfig{
		Clock:         clock,
		ClusterName:   testClusterName,
		AccessPoint:   ap,
		OktaConnected: connected,
	})
	require.NoError(t, err)

	// set app and group page sizes to 1 to make sure we exercise pagination logic.
	ap.uac.groupPageSize = 1
	ap.uac.appPageSize = 1

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
	// / Update the user so that it's now an SSO user.
	// Should get an assignment that has one action for the app./
	user.SetCreatedBy(types.CreatedBy{Connector: &types.ConnectorRef{}})
	user, err = ap.CreateUser(ctx, user)
	require.NoError(t, err)
	ap.user = user
	return ap
}

func TestRemovedUsedTargetsFromOldOldAnOldAssignments(t *testing.T) {
	tests := []struct {
		name        string
		usedTargets []types.OktaAssignmentTarget
		old         types.OktaAssignment
		want        types.OktaAssignment
	}{
		{
			name:        "empty used targets empty old assignment",
			usedTargets: []types.OktaAssignmentTarget{},
			old: &types.OktaAssignmentV1{
				Spec: types.OktaAssignmentSpecV1{
					Targets: []*types.OktaAssignmentTargetV1{},
				},
			},
			want: &types.OktaAssignmentV1{
				Spec: types.OktaAssignmentSpecV1{
					Targets: []*types.OktaAssignmentTargetV1{},
				},
			},
		},
		{
			name: "used app and group",
			usedTargets: []types.OktaAssignmentTarget{
				&types.OktaAssignmentTargetV1{Type: types.OktaAssignmentTargetV1_GROUP, Id: "group1"},
				&types.OktaAssignmentTargetV1{Type: types.OktaAssignmentTargetV1_APPLICATION, Id: "app2"},
			},
			old: &types.OktaAssignmentV1{
				Spec: types.OktaAssignmentSpecV1{
					Targets: []*types.OktaAssignmentTargetV1{
						{Type: types.OktaAssignmentTargetV1_GROUP, Id: "group1"},
						{Type: types.OktaAssignmentTargetV1_GROUP, Id: "group2"},
						{Type: types.OktaAssignmentTargetV1_APPLICATION, Id: "app1"},
						{Type: types.OktaAssignmentTargetV1_APPLICATION, Id: "app2"},
					},
				},
			},
			want: &types.OktaAssignmentV1{
				Spec: types.OktaAssignmentSpecV1{
					Targets: []*types.OktaAssignmentTargetV1{
						{Type: types.OktaAssignmentTargetV1_GROUP, Id: "group2"},
						{Type: types.OktaAssignmentTargetV1_APPLICATION, Id: "app1"},
					},
				},
			},
		},
		{
			name: "remove all apps and groups",
			usedTargets: []types.OktaAssignmentTarget{
				&types.OktaAssignmentTargetV1{Type: types.OktaAssignmentTargetV1_GROUP, Id: "group1"},
				&types.OktaAssignmentTargetV1{Type: types.OktaAssignmentTargetV1_GROUP, Id: "group2"},
				&types.OktaAssignmentTargetV1{Type: types.OktaAssignmentTargetV1_APPLICATION, Id: "app1"},
				&types.OktaAssignmentTargetV1{Type: types.OktaAssignmentTargetV1_APPLICATION, Id: "app2"},
			},
			old: &types.OktaAssignmentV1{
				Spec: types.OktaAssignmentSpecV1{
					Targets: []*types.OktaAssignmentTargetV1{
						{Type: types.OktaAssignmentTargetV1_GROUP, Id: "group1"},
						{Type: types.OktaAssignmentTargetV1_GROUP, Id: "group2"},
						{Type: types.OktaAssignmentTargetV1_APPLICATION, Id: "app1"},
						{Type: types.OktaAssignmentTargetV1_APPLICATION, Id: "app2"},
					},
				},
			},
			want: &types.OktaAssignmentV1{
				Spec: types.OktaAssignmentSpecV1{
					Targets: nil,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := removedUsedTargetsFromOldAssignment(tc.usedTargets, tc.old)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(tc.old, tc.want))
		})
	}
}
