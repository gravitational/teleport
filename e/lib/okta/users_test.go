package okta

import (
	"context"
	"crypto"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/e/lib/okta/common/connected"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

type testUACAccessPoint struct {
	*testAccessPoint

	uac *UserAssignmentCreator

	// user test-user@test.user that is stored in the backend and assigned "test-role" that
	// grants access to all app_server and user_group resources.
	user types.User
}

// GetUserOrLoginState will return the given user or the login state associated with the user.
func (t *testUACAccessPoint) GetUserOrLoginState(ctx context.Context, username string) (services.UserState, error) {
	return t.GetUser(ctx, username, false)
}

func TestUserAssignmentCreator(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	suite := initUACSuite(t, ctx, clock)

	app1 := application(t, "app1", "link", types.OriginOkta, testOrgURL)
	app2 := application(t, "app2", "link", types.OriginOkta, testOrgURL)
	appDupe := application(t, "app1", "link", types.OriginOkta, testOrgURL, withHostID("dummy-host"))
	group1 := group(t, "group1", types.OriginOkta, testOrgURL)
	group2 := group(t, "group2", types.OriginOkta, testOrgURL)

	uac := suite.uac
	user := suite.user
	testUser := user.GetName()
	ap := suite

	// No valid resources, so this should exit quickly and produce nothing.
	require.NoError(t, uac.OnLogin(ctx, user))
	assertEmptyAssignmentList(t, ap)

	upsertAppServer(t, ap, app1)
	upsertAppServer(t, ap, appDupe)

	// The UnifiedResourceCache doesn't take host id into consideration for apps.
	// So even though there exist two apps, the most recent one is the only app
	// that will appear in the cache.
	assertResourceCount(t, ctx, ap, 1)

	// This run should be skipped since no Okta roles or plugins are connected.
	ap.serviceCounts[types.RoleOkta] = 0

	require.NoError(t, uac.OnLogin(ctx, user))

	// Reconnect the service. Should get an assignment that has one action for the app.
	ap.serviceCounts[types.RoleOkta] = 1

	t.Run("test assignment creation", func(t *testing.T) {
		require.NoError(t, uac.OnLogin(ctx, user))
		want := mustNewAssignmentList(t, uac.hash, testUser, clock)([][]types.Resource{
			{app1},
		})
		mustFetchAndAssertAssignments(t, ctx, ap, want)
	})

	t.Run("test appserver deletion", func(t *testing.T) {
		require.NoError(t, ap.DeleteAppServer(ctx, presencev1.DeleteAppServerRequest_builder{HostId: app1.GetHostID(), Name: app1.GetName()}.Build()))
		require.NoError(t, ap.DeleteAppServer(ctx, presencev1.DeleteAppServerRequest_builder{HostId: appDupe.GetHostID(), Name: appDupe.GetName()}.Build()))
		require.NoError(t, ap.CreateUserGroup(ctx, group1))

		assertResourceCount(t, ctx, ap, 0)

		require.NoError(t, uac.OnLogin(ctx, user))
		mustDeleteCleanupAssignments(t, ctx, ap)

		want := mustNewAssignmentList(t, uac.hash, testUser, clock)([][]types.Resource{
			{group1},
		})
		mustFetchAndAssertAssignments(t, ctx, ap, want)
	})

	t.Run("re-add application", func(t *testing.T) {
		upsertAppServer(t, ap, app1)

		assertResourceCount(t, ctx, ap, 1)

		// Should get an assignment that has two actions: one for the app and one for the group.
		require.NoError(t, uac.OnLogin(ctx, user))
		mustDeleteCleanupAssignments(t, ctx, ap)

		want := mustNewAssignmentList(t, uac.hash, testUser, clock)([][]types.Resource{
			{group1, app1},
		})
		mustFetchAndAssertAssignments(t, ctx, ap, want)
	})

	t.Run("Add a new app", func(t *testing.T) {
		// We'll add a new app, which should cause a new assignment to be generated and the
		// old one to be marked as needing cleanup.
		upsertAppServer(t, ap, app2)

		assertResourceCount(t, ctx, ap, 2)

		require.NoError(t, uac.OnLogin(ctx, user))
		mustDeleteCleanupAssignments(t, ctx, ap)

		want := mustNewAssignmentList(t, uac.hash, testUser, clock)([][]types.Resource{
			{group1, app1, app2},
		})
		mustFetchAndAssertAssignments(t, ctx, ap, want)
	})

	t.Run("Add a new group", func(t *testing.T) {
		// We'll add in a group and make sure that triggers a second cleanup and another new assignment.
		require.NoError(t, ap.CreateUserGroup(ctx, group2))

		require.NoError(t, uac.OnLogin(ctx, user))
		mustDeleteCleanupAssignments(t, ctx, ap)

		want := mustNewAssignmentList(t, uac.hash, testUser, clock)([][]types.Resource{
			{group1, group2, app1, app2},
		})
		mustFetchAndAssertAssignments(t, ctx, ap, want)
	})

	t.Run("Remove the group", func(t *testing.T) {
		// We'll delete the old group and ensure that the old assignment is restored.
		require.NoError(t, ap.DeleteUserGroup(ctx, group2.GetName()))

		require.NoError(t, uac.OnLogin(ctx, user))
		mustDeleteCleanupAssignments(t, ctx, ap)

		want := mustNewAssignmentList(t, uac.hash, testUser, clock)([][]types.Resource{
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
		want := mustNewAssignmentList(t, uac.hash, testUser, clock)([][]types.Resource{
			{group1, app1, app2},
		})
		mustFetchAndAssertAssignments(t, ctx, ap, want)
	})

	t.Run("unlock user", func(t *testing.T) {
		require.NoError(t, ap.DeleteLock(ctx, "lock"))

		require.NoError(t, uac.OnLogin(ctx, user))
		mustDeleteCleanupAssignments(t, ctx, ap)

		mustDeleteCleanupAssignments(t, ctx, ap)
		want := mustNewAssignmentList(t, uac.hash, testUser, clock)([][]types.Resource{
			{group1, app1, app2},
		})
		mustFetchAndAssertAssignments(t, ctx, ap, want)
	})
}

func TestUserAssignmentCreator_set_CleanupTime_only_if_needed(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	clock := clockwork.NewFakeClock()
	suite := initUACSuite(t, ctx, clock)

	app1 := application(t, "app1", "link", types.OriginOkta, testOrgURL)
	app2 := application(t, "app2", "link", types.OriginOkta, testOrgURL)
	appDupe := application(t, "app1", "link", types.OriginOkta, testOrgURL, withHostID("dummy-host"))
	group1 := group(t, "group1", types.OriginOkta, testOrgURL)
	group2 := group(t, "group2", types.OriginOkta, testOrgURL)

	uac := suite.uac
	user := suite.user
	ap := suite

	// No resources in the backend - no assignments.
	require.NoError(t, uac.OnLogin(ctx, user))
	assertEmptyAssignmentList(t, ap)

	upsertAppServer(t, ap, app1)
	upsertAppServer(t, ap, app2)
	upsertAppServer(t, ap, appDupe)
	require.NoError(t, ap.CreateUserGroup(ctx, group1))
	require.NoError(t, ap.CreateUserGroup(ctx, group2))

	// The UnifiedResourceCache doesn't take host id into consideration for apps.
	// So even though there exist two apps, the most recent one is the only app
	// that will appear in the cache.
	assertResourceCount(t, ctx, ap, 2)

	time0 := clock.Now()

	// The user is permitted to read all apps and groups so the assignment should have all of
	// them.
	require.NoError(t, uac.OnLogin(ctx, user))
	mustFetchAndAssertAssignments(t, ctx, ap, []types.OktaAssignment{
		new(testAssignmentBuilder).
			Name(user, group1, group2, app1, app2).
			Targets(group1, group2, app1, app2).
			LastTransition(time0).
			Build(t),
	})

	time1 := clock.Now()

	require.NoError(t, ap.DeleteAppServer(ctx, presencev1.DeleteAppServerRequest_builder{HostId: app2.GetHostID(), Name: app2.GetName()}.Build()))
	assertResourceCount(t, ctx, ap, 1)

	require.NoError(t, uac.OnLogin(ctx, user))
	mustFetchAndAssertAssignments(t, ctx, ap, []types.OktaAssignment{
		new(testAssignmentBuilder).
			Name(user, group1, group2, app1, app2).
			Targets(app2).
			LastTransition(time0).
			CleanupTime(time1).
			Build(t),
		new(testAssignmentBuilder).
			Name(user, group1, group2, app1).
			Targets(group1, group2, app1).
			LastTransition(time0).
			Build(t),
	})

	clock.Advance(1 * time.Hour)
	time2 := clock.Now()

	require.NoError(t, ap.DeleteUserGroup(ctx, group1.GetName()))
	assertResourceCount(t, ctx, ap, 1)

	require.NoError(t, uac.OnLogin(ctx, user))
	mustFetchAndAssertAssignments(t, ctx, ap, []types.OktaAssignment{
		new(testAssignmentBuilder).
			Name(user, group1, group2, app1, app2).
			Targets(app2).
			LastTransition(time0).
			CleanupTime(time1).
			Build(t),
		new(testAssignmentBuilder).
			Name(user, group1, group2, app1).
			Targets(group1).
			LastTransition(time1).
			CleanupTime(time2).
			Build(t),
		new(testAssignmentBuilder).
			Name(user, group2, app1).
			Targets(group2, app1).
			LastTransition(time2).
			Build(t),
	})
}

func TestUserAssignmentCreator_calls_backend_only_if_needed(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	clock := clockwork.NewFakeClock()
	suite := initUACSuite(t, ctx, clock)

	app1 := application(t, "app1", "link", types.OriginOkta, testOrgURL)
	app2 := application(t, "app2", "link", types.OriginOkta, testOrgURL)
	group1 := group(t, "group1", types.OriginOkta, testOrgURL)
	group2 := group(t, "group2", types.OriginOkta, testOrgURL)

	uac := suite.uac
	user := suite.user
	ap := suite

	// No resources in the backend - no assignments.
	require.NoError(t, uac.OnLogin(ctx, user))
	assertEmptyAssignmentList(t, ap)

	originalAP := uac.accessPoint
	recordingAP := newRecordingUserAssignmentCreatorAccessPoint(originalAP)
	uac.accessPoint = recordingAP

	expectedCreateCalls := 0
	expectedUpdateCalls := 0

	// Create apps and groups. Expect one assignment to be created.

	upsertAppServer(t, ap, app1)
	upsertAppServer(t, ap, app2)
	require.NoError(t, ap.CreateUserGroup(ctx, group1))
	require.NoError(t, ap.CreateUserGroup(ctx, group2))
	assertResourceCount(t, ctx, ap, 2)

	expectedCreateCalls++

	require.NoError(t, uac.OnLogin(ctx, user))
	require.Len(t, recordingAP.CreateOktaAssignmentCalls, expectedCreateCalls)
	require.Len(t, recordingAP.UpdateOktaAssignmentCalls, expectedUpdateCalls)
	require.NoError(t, uac.OnLogin(ctx, user))
	require.NoError(t, uac.OnLogin(ctx, user))
	require.NoError(t, uac.OnLogin(ctx, user))
	require.Len(t, recordingAP.CreateOktaAssignmentCalls, expectedCreateCalls)
	require.Len(t, recordingAP.UpdateOktaAssignmentCalls, expectedUpdateCalls)

	// Delete a group. Expect the old assignment to be updated with the cleanup time and a new
	// one to be created.

	require.NoError(t, ap.DeleteUserGroup(ctx, group1.GetName()))

	expectedUpdateCalls++ // CleanupTime set, used targets cleaned
	expectedCreateCalls++ // new replacement assignment created

	require.NoError(t, uac.OnLogin(ctx, user))
	require.Len(t, recordingAP.CreateOktaAssignmentCalls, expectedCreateCalls)
	require.Len(t, recordingAP.UpdateOktaAssignmentCalls, expectedUpdateCalls)
	require.NoError(t, uac.OnLogin(ctx, user))
	require.NoError(t, uac.OnLogin(ctx, user))
	require.NoError(t, uac.OnLogin(ctx, user))
	require.Len(t, recordingAP.CreateOktaAssignmentCalls, expectedCreateCalls)
	require.Len(t, recordingAP.UpdateOktaAssignmentCalls, expectedUpdateCalls)

	/* TODO(kopiczko): Uncomment when https://github.com/gravitational/teleport-private/issues/2486 is fixed

	// Unset CleanupTime on the cleanup assignment. Expect to be updated with the CleanupTime,
	// but only once.

	cleanupAssignment := recordingAP.UpdateOktaAssignmentCalls[expectedUpdateCalls-1].Copy()
	cleanupAssignment.SetCleanupTime(time.Time{})                     // reset
	_, err := originalAP.UpdateOktaAssignment(ctx, cleanupAssignment) // use originalAP to bypass recording
	require.NoError(t, err)

	expectedUpdateCalls++ // for CleanupTime set, used targets cleaned

	require.NoError(t, uac.OnLogin(ctx, user))
	require.Len(t, recordingAP.CreateOktaAssignmentCalls, expectedCreateCalls)
	require.Len(t, recordingAP.UpdateOktaAssignmentCalls, expectedUpdateCalls)
	require.NoError(t, uac.OnLogin(ctx, user))
	require.NoError(t, uac.OnLogin(ctx, user))
	require.NoError(t, uac.OnLogin(ctx, user))
	require.Len(t, recordingAP.CreateOktaAssignmentCalls, expectedCreateCalls)
	require.Len(t, recordingAP.UpdateOktaAssignmentCalls, expectedUpdateCalls)

	*/

	/* TODO(kopiczko): Uncomment when https://github.com/gravitational/teleport-private/issues/2486 is fixed

	// Setting CleanupTime time to the future results in updating it to now, so expect one
	// update call.

	cleanupAssignment = recordingAP.UpdateOktaAssignmentCalls[expectedUpdateCalls-1].Copy()
	cleanupAssignment.SetCleanupTime(clock.Now().Add(10 * time.Hour))
	_, err = originalAP.UpdateOktaAssignment(ctx, cleanupAssignment) // use originalAP to bypass recording
	require.NoError(t, err)

	expectedUpdateCalls++ // CleanupTime set from future to now

	require.NoError(t, uac.OnLogin(ctx, user))
	require.Len(t, recordingAP.CreateOktaAssignmentCalls, expectedCreateCalls)
	require.Len(t, recordingAP.UpdateOktaAssignmentCalls, expectedUpdateCalls)
	require.NoError(t, uac.OnLogin(ctx, user))
	require.NoError(t, uac.OnLogin(ctx, user))
	require.NoError(t, uac.OnLogin(ctx, user))
	require.Len(t, recordingAP.CreateOktaAssignmentCalls, expectedCreateCalls)
	require.Len(t, recordingAP.UpdateOktaAssignmentCalls, expectedUpdateCalls)

	*/

	// Setting CleanupTime time to the past doesn't cause the update.

	cleanupAssignment := recordingAP.UpdateOktaAssignmentCalls[expectedUpdateCalls-1].Copy()
	cleanupAssignment.SetCleanupTime(clock.Now().Add(-10 * time.Hour))
	_, err := originalAP.UpdateOktaAssignment(ctx, cleanupAssignment) // use originalAP to bypass recording
	require.NoError(t, err)

	require.Len(t, recordingAP.CreateOktaAssignmentCalls, expectedCreateCalls)
	require.Len(t, recordingAP.UpdateOktaAssignmentCalls, expectedUpdateCalls)
	require.NoError(t, uac.OnLogin(ctx, user))
	require.NoError(t, uac.OnLogin(ctx, user))
	require.NoError(t, uac.OnLogin(ctx, user))
	require.Len(t, recordingAP.CreateOktaAssignmentCalls, expectedCreateCalls)
	require.Len(t, recordingAP.UpdateOktaAssignmentCalls, expectedUpdateCalls)

	// Adding back targets to a cleanup okta_assignment that are "in use" by another
	// okta_assignment results in removing them. Expect 1 update.

	ongoingAssignment := recordingAP.CreateOktaAssignmentCalls[expectedCreateCalls-1].Copy()
	cleanupAssignment = recordingAP.UpdateOktaAssignmentCalls[expectedUpdateCalls-1].Copy()

	// Targets are only updated on the cleanup assignment if the new (replacement)
	// okta_assignment is created. In that case the targets present in the new okta_assignment
	// are removed from all existing cleanup assignments.
	testAppendTargets(t, cleanupAssignment, ongoingAssignment.GetTargets())
	err = ap.DeleteOktaAssignment(ctx, ongoingAssignment.GetName())
	require.NoError(t, err)
	_, err = ap.UpdateOktaAssignment(ctx, cleanupAssignment)
	require.NoError(t, err)

	expectedCreateCalls++ // ongoing assignment re-created
	expectedUpdateCalls++ // removing targets present in the ongoingAssignment

	require.NoError(t, uac.OnLogin(ctx, user))
	require.Len(t, recordingAP.CreateOktaAssignmentCalls, expectedCreateCalls)
	require.Len(t, recordingAP.UpdateOktaAssignmentCalls, expectedUpdateCalls)
	require.NoError(t, uac.OnLogin(ctx, user))
	require.NoError(t, uac.OnLogin(ctx, user))
	require.NoError(t, uac.OnLogin(ctx, user))
	require.Len(t, recordingAP.CreateOktaAssignmentCalls, expectedCreateCalls)
	require.Len(t, recordingAP.UpdateOktaAssignmentCalls, expectedUpdateCalls)

	// Deleting user's standing access sets the cleanup time once.

	denyAllRole, err := types.NewRole("test_deny_all", types.RoleSpecV6{
		Deny: types.RoleConditions{
			AppLabels:   types.Labels{"*": utils.Strings{"*"}},
			GroupLabels: types.Labels{"*": utils.Strings{"*"}},
		},
	})
	require.NoError(t, err)
	_, err = ap.UpsertRole(ctx, denyAllRole)
	require.NoError(t, err)

	user.AddRole(denyAllRole.GetName())
	_, err = ap.UpsertUser(ctx, user)
	require.NoError(t, err)

	expectedUpdateCalls++ // setting cleanup time on the ongoing okta_assignment

	require.NoError(t, uac.OnLogin(ctx, user))
	require.Len(t, recordingAP.CreateOktaAssignmentCalls, expectedCreateCalls)
	require.Len(t, recordingAP.UpdateOktaAssignmentCalls, expectedUpdateCalls)
	require.NoError(t, uac.OnLogin(ctx, user))
	require.NoError(t, uac.OnLogin(ctx, user))
	require.NoError(t, uac.OnLogin(ctx, user))
	require.Len(t, recordingAP.CreateOktaAssignmentCalls, expectedCreateCalls)
	require.Len(t, recordingAP.UpdateOktaAssignmentCalls, expectedUpdateCalls)
}

func testAppendTargets(t *testing.T, a types.OktaAssignment, targets []types.OktaAssignmentTarget) {
	t.Helper()

	var targetsV1 []*types.OktaAssignmentTargetV1
	for _, target := range targets {
		require.IsType(t, &types.OktaAssignmentTargetV1{}, target)
		targetsV1 = append(targetsV1, target.(*types.OktaAssignmentTargetV1))
	}

	require.IsType(t, &types.OktaAssignmentV1{}, a)
	a.(*types.OktaAssignmentV1).Spec.Targets = append(a.(*types.OktaAssignmentV1).Spec.Targets, targetsV1...)
}

type recordingUserAssignmentCreatorAccessPoint struct {
	UserAssignmentCreatorAccessPoint
	CreateOktaAssignmentCalls []types.OktaAssignment
	UpdateOktaAssignmentCalls []types.OktaAssignment
}

func newRecordingUserAssignmentCreatorAccessPoint(underlying UserAssignmentCreatorAccessPoint) *recordingUserAssignmentCreatorAccessPoint {
	return &recordingUserAssignmentCreatorAccessPoint{UserAssignmentCreatorAccessPoint: underlying}
}

func (ap *recordingUserAssignmentCreatorAccessPoint) CreateOktaAssignment(ctx context.Context, assignment types.OktaAssignment) (types.OktaAssignment, error) {
	ap.CreateOktaAssignmentCalls = append(ap.CreateOktaAssignmentCalls, assignment.Copy())
	return ap.UserAssignmentCreatorAccessPoint.CreateOktaAssignment(ctx, assignment)
}

func (ap *recordingUserAssignmentCreatorAccessPoint) UpdateOktaAssignment(ctx context.Context, assignment types.OktaAssignment) (types.OktaAssignment, error) {
	ap.UpdateOktaAssignmentCalls = append(ap.UpdateOktaAssignmentCalls, assignment.Copy())
	return ap.UserAssignmentCreatorAccessPoint.UpdateOktaAssignment(ctx, assignment)
}

func BenchmarkUserAssignmentCreator(b *testing.B) {
	clock := clockwork.NewFakeClock()
	ctx := context.Background()
	suite := initUACSuite(b, ctx, clock)

	for i := range 1234 {
		app1 := application(b, "app"+strconv.Itoa(i), "link", types.OriginOkta, testOrgURL)
		upsertAppServer(b, suite, app1)
	}

	for b.Loop() {
		assert.NoError(b, suite.uac.OnLogin(context.Background(), suite.user))
	}
}

func assertResourceCount(t *testing.T, ctx context.Context, ap *testUACAccessPoint, want int) {
	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		var count int
		for _, err := range ap.uac.resourceCache.AppServers(ctx, services.UnifiedResourcesIterateParams{}) {
			if !assert.NoError(t, err) {
				return
			}
			count++
		}

		assert.Equal(t, want, count)
	}, 10*time.Second, 100*time.Millisecond)
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

type testAssignmentBuilder struct {
	user           string
	name           string
	targets        []*types.OktaAssignmentTargetV1
	lastTransition time.Time
	cleanupTime    time.Time

	buildErrs []error
}

// Name has a separate set of resources form Targets because the OktaAssignment targets may be
// stripped down if the assignment was scheduled for cleanup.
func (b *testAssignmentBuilder) Name(user types.User, targetResources ...types.Resource) *testAssignmentBuilder {
	if len(b.buildErrs) > 0 {
		return b
	}

	var groups, apps []string
	for _, r := range targetResources {
		switch r.GetKind() {
		case types.KindUserGroup:
			groups = append(groups, r.GetName())
		case types.KindAppServer:
			apps = append(apps, r.GetName())
		default:
			b.recordError("building name: unexpected resource kind %q", r.GetKind())
			return b
		}
	}

	name, err := uacAssignmentName(uacNameHash, user.GetName(), groups, apps)
	if err != nil {
		b.recordError("building name: hash error: %v", err)
		return b
	}

	b.user = user.GetName()
	b.name = name

	return b
}

// Targets may be different from those which name is the OktaAssignment name hash is calculated
// with.
func (b *testAssignmentBuilder) Targets(targetResources ...types.Resource) *testAssignmentBuilder {
	if len(b.buildErrs) > 0 {
		return b
	}

	var targets []*types.OktaAssignmentTargetV1
	for _, r := range targetResources {
		var typ types.OktaAssignmentTargetV1_OktaAssignmentTargetType
		switch r.GetKind() {
		case types.KindUserGroup:
			typ = types.OktaAssignmentTargetV1_GROUP
		case types.KindAppServer:
			typ = types.OktaAssignmentTargetV1_APPLICATION
		default:
			b.recordError("building targets: unexpected resource kind %q", r.GetKind())
			return b
		}
		targets = append(targets, &types.OktaAssignmentTargetV1{Type: typ, Id: r.GetName()})
	}

	b.targets = targets

	return b
}

// LastTransition sets LastTransition.
func (b *testAssignmentBuilder) LastTransition(lastTransition time.Time) *testAssignmentBuilder {
	b.lastTransition = lastTransition
	return b
}

// CleanupTime sets CleanupTime.
func (b *testAssignmentBuilder) CleanupTime(cleanupTime time.Time) *testAssignmentBuilder {
	b.cleanupTime = cleanupTime
	return b
}

// Build builds the OktaAssignment.
func (b *testAssignmentBuilder) Build(t *testing.T) types.OktaAssignment {
	t.Helper()

	require.NoError(t, trace.NewAggregate(b.buildErrs...))

	require.NotEmpty(t, b.user, "OktaAssignment user cannot be empty")
	require.NotEmpty(t, b.name, "OktaAssignment name cannot be empty")
	require.False(t, b.lastTransition.IsZero(), "OktaAssignment last transition cannot be zero")

	assignment, err := types.NewOktaAssignment(
		types.Metadata{
			Name: b.name,
			Labels: map[string]string{
				teleport.OktaAssignmentSourceLabel: userAssignmentCreatorSource,
			},
		},
		types.OktaAssignmentSpecV1{
			User:           b.user,
			Targets:        b.targets,
			Status:         types.OktaAssignmentSpecV1_PENDING,
			LastTransition: b.lastTransition,
			CleanupTime:    b.cleanupTime,
		},
	)
	require.NoError(t, err)

	return assignment
}

func (b *testAssignmentBuilder) recordError(format string, a ...any) {
	b.buildErrs = append(b.buildErrs, fmt.Errorf(format, a...))
}

func mustNewAssignmentList(t *testing.T, hash crypto.Hash, user string, clock clocki.FakeClock) func([][]types.Resource) []types.OktaAssignment {
	return func(targetResourceSets [][]types.Resource) (out []types.OktaAssignment) {
		var resourcesForNameHash []types.Resource
		for _, targetResources := range targetResourceSets {
			resourcesForNameHash = append(resourcesForNameHash, targetResources...)
			u := &types.UserV2{Metadata: types.Metadata{Name: user}}
			out = append(out, new(testAssignmentBuilder).
				Name(u, resourcesForNameHash...).
				Targets(targetResources...).
				LastTransition(clock.Now()).
				Build(t))
		}
		return out
	}
}

func initUACSuite(t testing.TB, ctx context.Context, clock clockwork.Clock) *testUACAccessPoint {
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

	urc, err := services.NewUnifiedResourceCache(ctx, services.UnifiedResourceCacheConfig{
		Clock: clock,
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "test",
			Client:    ap,
		},
		ResourceGetter: ap,
	})
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		require.True(t, urc.IsInitialized())
	}, 10*time.Second, 100*time.Millisecond)

	ap.uac, err = NewUserAssignmentCreator(UserAssignmentCreatorConfig{
		Clock:                clock,
		ClusterName:          testClusterName,
		AccessPoint:          ap,
		OktaConnected:        connected,
		UnifiedResourceCache: urc,
	})
	require.NoError(t, err)

	// set app and group page sizes to 1 to make sure we exercise pagination logic.
	ap.uac.groupPageSize = 1
	ap.uac.appPageSize = 1

	testUser := "test-user@test.user"
	testRole := "test-role"

	role, err := authtest.CreateRole(ctx, ap, testRole, types.RoleSpecV6{
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
	t.Parallel()
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
