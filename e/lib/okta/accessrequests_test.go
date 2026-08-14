package okta

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta/common/connected"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

func TestAccessRequestReconciler(t *testing.T) {
	t.Parallel()
	ctx, clock, ap, reconciler := setupTestAccessRequestReconciler(t)
	onReconcileCh := reconciler.onReconcileCh
	onServiceDisconnectedCh := reconciler.onServiceDisconnectedCh

	waitForResult(t, onReconcileCh, struct{}{}, 1)

	// Reconciler should be empty to start
	require.Empty(t, reconciler.accessRequests.CopyAsMap())
	require.Empty(t, reconciler.newAccessRequests.CopyAsMap())

	user := userName("test-user")
	roles := []string{"test-role"}

	// This access request should be ignored.
	accessRequest, err := types.NewAccessRequestWithResources(uuid.NewString(), string(user), roles,
		[]types.ResourceAccessID{{Id: types.ResourceID{ClusterName: testClusterName, Kind: types.KindRole, Name: "role-request"}}})
	require.NoError(t, err)

	accessRequest, err = ap.CreateAccessRequestV2(ctx, accessRequest)
	require.NoError(t, err)

	waitForResult(t, onReconcileCh, struct{}{}, 1)

	// There should be no assignments created.
	assignments, _, err := ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, assignments)

	require.NoError(t, ap.DeleteAccessRequest(ctx, accessRequest.GetName()))
	waitForResult(t, onReconcileCh, struct{}{}, 1)

	appServer := application(t, "app1", "link1", types.OriginOkta, testOrgURL)
	_, err = ap.UpsertApplicationServer(ctx, appServer)
	require.NoError(t, err)

	// This access request should not be registered by the reconciler
	accessRequest, err = types.NewAccessRequestWithResources(uuid.NewString(), string(user), roles,
		[]types.ResourceAccessID{{Id: types.ResourceID{ClusterName: testClusterName, Kind: types.KindApp, Name: appServer.GetApp().GetName()}}})
	require.NoError(t, err)
	accessRequest.SetState(types.RequestState_DENIED)

	accessRequest, err = ap.CreateAccessRequestV2(ctx, accessRequest)
	require.NoError(t, err)

	waitForResult(t, onReconcileCh, struct{}{}, 1)

	require.Empty(t, reconciler.accessRequests.CopyAsMap())
	require.Empty(t, cmp.Diff(map[string]types.AccessRequest{accessRequest.GetName(): accessRequest}, reconciler.newAccessRequests.CopyAsMap(), cmpopts.IgnoreFields(types.Metadata{}, "Revision"), cmpopts.EquateEmpty()))

	require.NoError(t, ap.DeleteAccessRequest(ctx, accessRequest.GetName()))
	waitForResult(t, onReconcileCh, struct{}{}, 1)

	// This access request should also not be registered by the reconciler
	accessRequest, err = types.NewAccessRequestWithResources(uuid.NewString(), string(user), roles,
		[]types.ResourceAccessID{{Id: types.ResourceID{ClusterName: "other-cluster-name", Kind: types.KindApp, Name: appServer.GetApp().GetName()}}})
	require.NoError(t, err)
	accessRequest.SetState(types.RequestState_APPROVED)

	accessRequest, err = ap.CreateAccessRequestV2(ctx, accessRequest)
	require.NoError(t, err)

	waitForResult(t, onReconcileCh, struct{}{}, 1)

	require.Empty(t, reconciler.accessRequests.CopyAsMap())
	require.Empty(t, cmp.Diff(map[string]types.AccessRequest{accessRequest.GetName(): accessRequest}, reconciler.newAccessRequests.CopyAsMap(), cmpopts.IgnoreFields(types.Metadata{}, "Revision"), cmpopts.EquateEmpty()))

	require.NoError(t, ap.DeleteAccessRequest(ctx, accessRequest.GetName()))
	waitForResult(t, onReconcileCh, struct{}{}, 1)

	// This access request should create an Okta assignment, but the Okta service is not connected.
	ap.setServiceCounts(map[types.SystemRole]uint64{})

	// This will stop the reconciler.
	for range maxOktaServiceConnectionFailures {
		clock.Advance(10 * time.Minute)
		waitForResult(t, onServiceDisconnectedCh, struct{}{}, 1)
	}

	accessRequest, err = types.NewAccessRequestWithResources(uuid.NewString(), string(user), roles,
		[]types.ResourceAccessID{{Id: types.ResourceID{ClusterName: testClusterName, Kind: types.KindApp, Name: appServer.GetApp().GetName()}}})
	require.NoError(t, err)
	accessRequest.SetState(types.RequestState_APPROVED)

	accessRequest, err = ap.CreateAccessRequestV2(ctx, accessRequest)
	require.NoError(t, err)

	// No reconcile will be triggered because the reconciler will be stopped.
	require.Empty(t, reconciler.accessRequests.CopyAsMap())
	require.Empty(t, reconciler.newAccessRequests.CopyAsMap())

	// We'll reconnect the Okta service and the assignment should be created.
	ap.setServiceCounts(map[types.SystemRole]uint64{types.RoleOkta: 1})
	clock.Advance(10 * time.Minute) // This will restart the reconciler.
	waitForResult(t, onReconcileCh, struct{}{}, 1)
	require.Empty(t, cmp.Diff(map[string]types.AccessRequest{accessRequest.GetName(): accessRequest}, reconciler.accessRequests.CopyAsMap(), cmpopts.IgnoreFields(types.Metadata{}, "Revision"), cmpopts.EquateEmpty()))
	require.Empty(t, cmp.Diff(map[string]types.AccessRequest{accessRequest.GetName(): accessRequest}, reconciler.newAccessRequests.CopyAsMap(), cmpopts.IgnoreFields(types.Metadata{}, "Revision"), cmpopts.EquateEmpty()))

	foundAssignment := getOktaAssignment(t, ap, accessRequest.GetName())
	expires := accessRequest.GetAccessExpiry()
	require.Empty(t, cmp.Diff(foundAssignment, assignment(t, accessRequest.GetName(), user, expires, constants.OktaAssignmentStatusPending, clock.Now(), false,
		target(types.OktaAssignmentTargetV1_APPLICATION, mustAppName(t, "app1", "link1"))),
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// This delete shouldn't do anything to the assignment.
	cleanupTimeNow := clock.Now()
	require.NoError(t, ap.DeleteAccessRequest(ctx, accessRequest.GetName()))
	waitForResult(t, onReconcileCh, struct{}{}, 1)

	foundAssignment = getOktaAssignment(t, ap, accessRequest.GetName())
	require.Empty(t, cmp.Diff(foundAssignment, assignment(t, accessRequest.GetName(), user, cleanupTimeNow, constants.OktaAssignmentStatusPending, clock.Now(), false,
		target(types.OktaAssignmentTargetV1_APPLICATION, mustAppName(t, "app1", "link1"))),
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		cmpopts.EquateEmpty(),
	))

	// This access request should create an Okta assignment
	userGroup := group(t, "group1", types.OriginOkta, testOrgURL)
	require.NoError(t, ap.CreateUserGroup(ctx, userGroup))

	accessRequest, err = types.NewAccessRequestWithResources(uuid.NewString(), string(user), roles,
		[]types.ResourceAccessID{{Id: types.ResourceID{ClusterName: testClusterName, Kind: types.KindUserGroup, Name: userGroup.GetName()}}})
	accessRequest.SetState(types.RequestState_APPROVED)
	require.NoError(t, err)

	accessRequest, err = ap.CreateAccessRequestV2(ctx, accessRequest)
	require.NoError(t, err)

	waitForResult(t, onReconcileCh, struct{}{}, 1)

	foundAssignment = getOktaAssignment(t, ap, accessRequest.GetName())
	expires = accessRequest.GetAccessExpiry()
	require.Empty(t, cmp.Diff(foundAssignment, assignment(t, accessRequest.GetName(), user, expires, constants.OktaAssignmentStatusPending, clock.Now(), false,
		target(types.OktaAssignmentTargetV1_GROUP, userGroup.GetName())),
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		cmpopts.EquateEmpty(),
	))

	require.NoError(t, ap.DeleteAccessRequest(ctx, accessRequest.GetName()))
	waitForResult(t, onReconcileCh, struct{}{}, 1)

	foundAssignment = getOktaAssignment(t, ap, accessRequest.GetName())
	require.Empty(t, cmp.Diff(foundAssignment, assignment(t, accessRequest.GetName(), user, cleanupTimeNow, constants.OktaAssignmentStatusPending, clock.Now(), false,
		target(types.OktaAssignmentTargetV1_GROUP, userGroup.GetName())),
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		cmpopts.EquateEmpty(),
	))
}

func TestAccessRequestReconciler_idempotency(t *testing.T) {
	t.Parallel()
	ctx, _, ap, reconciler := setupTestAccessRequestReconciler(t)
	onReconcileCh := reconciler.onReconcileCh

	waitForResult(t, onReconcileCh, struct{}{}, 1)

	user := userName("test-user")
	roles := []string{"test-role"}

	// There should be no assignments created.
	assignments, _, err := ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, assignments)

	// Create approved AccessRequest
	appServer := application(t, "app1", "link1", types.OriginOkta, testOrgURL)
	_, err = ap.UpsertApplicationServer(ctx, appServer)
	require.NoError(t, err)

	// Create approved access request to Okta app
	accessRequest, err := types.NewAccessRequestWithResources(uuid.NewString(), string(user), roles,
		[]types.ResourceAccessID{{Id: types.ResourceID{ClusterName: testClusterName, Kind: types.KindApp, Name: appServer.GetApp().GetName()}}})
	require.NoError(t, err)
	accessRequest.SetState(types.RequestState_APPROVED)

	accessRequest, err = ap.CreateAccessRequestV2(ctx, accessRequest)
	require.NoError(t, err)

	// Wait for the access request creation to be reconciled
	waitForResult(t, onReconcileCh, struct{}{}, 1)

	// Now the AccessRequest is reconciled - we should have 1 corresponding OktaAssignment
	assignments, _, err = ap.ListOktaAssignments(ctx, 0, "")
	require.NoError(t, err)
	require.Len(t, assignments, 1)

	// Let's delete the created Okta assignment outside the reconciliation loop. This may
	// happen in e.g. HA setup where another reconciler did it first.
	err = ap.DeleteOktaAssignment(ctx, assignments[0].GetName())
	require.NoError(t, err)

	// There still should be access request registered in the reconciler
	require.Len(t, reconciler.accessRequests.CopyAsMap(), 1)

	// Let's delete the access request now
	err = ap.DeleteAccessRequest(ctx, accessRequest.GetName())
	require.NoError(t, err)

	// Wait for the access request deletion to be reconciled
	waitForResult(t, onReconcileCh, struct{}{}, 1)

	// The access request should be de-registered in the reconciler, even though the
	// corresponding Okta assignment doesn't exist in the backend
	require.Empty(t, reconciler.accessRequests.CopyAsMap())
}

func TestAccessRequestToOktaAssignment(t *testing.T) {
	t.Parallel()
	const accessRequestName = "access_request"
	const user = "user"
	clock := clockwork.NewFakeClock()
	expires := clock.Now()

	tests := []struct {
		name              string
		skipBackendCreate bool
		appTargets        []types.AppServer
		groupTargets      []types.UserGroup
		assignmentStatus  string
		expected          types.OktaAssignment
		errAssertionFunc  require.ErrorAssertionFunc
	}{
		{
			name: "Okta targets",
			appTargets: []types.AppServer{
				application(t, "app1", "link1", types.OriginOkta, testOrgURL),
				application(t, "app2", "link1", types.OriginOkta, testOrgURL),
				application(t, "app3", "link1", types.OriginDynamic, testOrgURL),
			},
			groupTargets: []types.UserGroup{
				group(t, "group1", types.OriginOkta, testOrgURL),
				group(t, "group2", types.OriginDynamic, testOrgURL),
			},
			assignmentStatus: constants.OktaAssignmentStatusPending,
			expected: assignment(t, accessRequestName, user, expires, constants.OktaAssignmentStatusPending, clock.Now(), false,
				target(types.OktaAssignmentTargetV1_APPLICATION, mustAppName(t, "app1", "link1")),
				target(types.OktaAssignmentTargetV1_APPLICATION, mustAppName(t, "app2", "link1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			),
			errAssertionFunc: require.NoError,
		},
		{
			name:              "app not found",
			skipBackendCreate: true,
			appTargets: []types.AppServer{
				application(t, "app1", "link1", types.OriginOkta, testOrgURL),
			},
			assignmentStatus: constants.OktaAssignmentStatusPending,
			errAssertionFunc: func(tt require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name:              "group not found",
			skipBackendCreate: true,
			groupTargets: []types.UserGroup{
				group(t, "group1", types.OriginOkta, testOrgURL),
			},
			assignmentStatus: constants.OktaAssignmentStatusPending,
			errAssertionFunc: func(tt require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name: "no Okta targets",
			appTargets: []types.AppServer{
				application(t, "app1", "link1", types.OriginDynamic, testOrgURL),
				application(t, "app2", "link1", types.OriginDynamic, testOrgURL),
				application(t, "app3", "link1", types.OriginDynamic, testOrgURL), // This should be skipped
			},
			groupTargets: []types.UserGroup{
				group(t, "group1", types.OriginDynamic, testOrgURL),
				group(t, "group2", types.OriginDynamic, testOrgURL),
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(t, trace.NotFound("no Okta targets found in access request"), err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			ap := newTestAccessPoint(t, clock)
			connected, err := connected.New(connected.Config{
				DisableCache:    true,
				ConnectedGetter: ap,
				Plugins:         ap,
			})
			require.NoError(t, err)

			reconciler, err := NewAccessRequestReconciler(ctx, &AccessRequestReconcilerConfig{
				Clock:         clock,
				ClusterName:   testClusterName,
				LockWatcher:   newLockWatcher(t, ap),
				AccessPoint:   ap,
				OktaConnected: connected,
				OktaClient:    ap,
			})
			require.NoError(t, err)
			require.NoError(t, reconciler.Start(ctx))
			t.Cleanup(func() {
				reconciler.Stop()
			})

			accessRequest, err := types.NewAccessRequest(accessRequestName, user, "role")
			require.NoError(t, err)
			accessRequest.SetAccessExpiry(expires)

			var requestedResourceIDs []types.ResourceID
			for _, appServer := range test.appTargets {
				if !test.skipBackendCreate {
					_, err = ap.UpsertApplicationServer(ctx, appServer)
					require.NoError(t, err)
				}
				requestedResourceIDs = append(requestedResourceIDs, types.ResourceID{
					ClusterName: testClusterName,
					Kind:        types.KindApp,
					Name:        appServer.GetName(),
				})
			}
			for _, userGroup := range test.groupTargets {
				if !test.skipBackendCreate {
					err = ap.CreateUserGroup(ctx, userGroup)
					require.NoError(t, err)
				}
				requestedResourceIDs = append(requestedResourceIDs, types.ResourceID{
					ClusterName: testClusterName,
					Kind:        userGroup.GetKind(),
					Name:        userGroup.GetName(),
				})
			}

			accessRequest.SetRequestedResourceIDs(requestedResourceIDs)

			oktaAssignment, err := reconciler.accessRequestToOktaAssignment(ctx, accessRequest, test.assignmentStatus)
			test.errAssertionFunc(t, err)
			require.Empty(t, cmp.Diff(test.expected, oktaAssignment))
		})
	}
}

func TestOnLogin(t *testing.T) {
	t.Parallel()

	now := time.Now()
	user1, err := types.NewUser("user1")
	require.NoError(t, err)
	user2, err := types.NewUser("user2")
	require.NoError(t, err)

	// Access requests must be UUIDs, so we'll pre-define them here
	// for later referencing.
	arNames := make([]string, 2)
	for i := range arNames {
		arNames[i] = uuid.NewString()
	}

	// Run multiple cycles where we create/delete locks and run on-login.
	type lockAndOnLoginCycle struct {
		locks    []types.Lock
		expected []types.OktaAssignment
	}

	tests := []struct {
		name             string
		oktaServiceCount int
		accessRequests   []types.AccessRequest
		expected         []types.OktaAssignment
		cycles           []lockAndOnLoginCycle
	}{
		{
			name:             "no assignments",
			oktaServiceCount: 1,
			cycles:           []lockAndOnLoginCycle{},
		},
		{
			name:             "access requests, no locks",
			oktaServiceCount: 1,
			accessRequests: []types.AccessRequest{
				accessRequest(t, arNames[0], user1.GetName(), []string{"role1", "role2"}, now.Add(time.Hour),
					resourceID(types.KindUserGroup, "group1")),
				accessRequest(t, arNames[1], user2.GetName(), []string{"role1", "role2"}, now.Add(time.Hour),
					resourceID(types.KindUserGroup, "group2")),
			},
			expected: []types.OktaAssignment{
				assignment(t, arNames[0], userName(user1.GetName()), now.Add(time.Hour), constants.OktaAssignmentStatusPending,
					now, false, target(types.OktaAssignmentTargetV1_GROUP, "group1")),
				assignment(t, arNames[1], userName(user2.GetName()), now.Add(time.Hour), constants.OktaAssignmentStatusPending,
					now, false, target(types.OktaAssignmentTargetV1_GROUP, "group2")),
			},
			cycles: []lockAndOnLoginCycle{
				{
					expected: []types.OktaAssignment{
						assignment(t, arNames[0], userName(user1.GetName()), now.Add(time.Hour), constants.OktaAssignmentStatusPending,
							now, false, target(types.OktaAssignmentTargetV1_GROUP, "group1")),
						assignment(t, arNames[1], userName(user2.GetName()), now.Add(time.Hour), constants.OktaAssignmentStatusPending,
							now, false, target(types.OktaAssignmentTargetV1_GROUP, "group2")),
					},
				},
			},
		},
		{
			name:             "access requests, locks",
			oktaServiceCount: 1,
			accessRequests: []types.AccessRequest{
				accessRequest(t, arNames[0], user1.GetName(), []string{"role1", "role2"}, now.Add(time.Hour),
					resourceID(types.KindUserGroup, "group1")),
				accessRequest(t, arNames[1], user2.GetName(), []string{"role1", "role2"}, now.Add(time.Hour),
					resourceID(types.KindUserGroup, "group2")),
			},
			expected: []types.OktaAssignment{
				assignment(t, arNames[0], userName(user1.GetName()), now.Add(time.Hour), constants.OktaAssignmentStatusPending,
					now, false, target(types.OktaAssignmentTargetV1_GROUP, "group1")),
				assignment(t, arNames[1], userName(user2.GetName()), now.Add(time.Hour), constants.OktaAssignmentStatusPending,
					now, false, target(types.OktaAssignmentTargetV1_GROUP, "group2")),
			},
			cycles: []lockAndOnLoginCycle{
				{
					locks: []types.Lock{
						lock(t, "lock1", types.LockTarget{User: user1.GetName()}),
						lock(t, "lock2", types.LockTarget{AccessRequest: arNames[1]}),
					},
					expected: []types.OktaAssignment{
						assignment(t, arNames[0], userName(user1.GetName()), now, constants.OktaAssignmentStatusPending,
							now, false, target(types.OktaAssignmentTargetV1_GROUP, "group1")),
						assignment(t, arNames[1], userName(user2.GetName()), now, constants.OktaAssignmentStatusPending,
							now, false, target(types.OktaAssignmentTargetV1_GROUP, "group2")),
					},
				},
			},
		},
		{
			name:             "access requests, locks, but no connected Okta service",
			oktaServiceCount: 0,
			accessRequests: []types.AccessRequest{
				accessRequest(t, arNames[0], user1.GetName(), []string{"role1", "role2"}, now.Add(time.Hour),
					resourceID(types.KindUserGroup, "group1")),
				accessRequest(t, arNames[1], user2.GetName(), []string{"role1", "role2"}, now.Add(time.Hour),
					resourceID(types.KindUserGroup, "group2")),
			},
			expected: []types.OktaAssignment{
				assignment(t, arNames[0], userName(user1.GetName()), now.Add(time.Hour), constants.OktaAssignmentStatusPending,
					now, false, target(types.OktaAssignmentTargetV1_GROUP, "group1")),
				assignment(t, arNames[1], userName(user2.GetName()), now.Add(time.Hour), constants.OktaAssignmentStatusPending,
					now, false, target(types.OktaAssignmentTargetV1_GROUP, "group2")),
			},
			cycles: []lockAndOnLoginCycle{
				{
					locks: []types.Lock{
						lock(t, "lock1", types.LockTarget{User: user1.GetName()}),
						lock(t, "lock2", types.LockTarget{AccessRequest: arNames[1]}),
					},
					expected: []types.OktaAssignment{
						assignment(t, arNames[0], userName(user1.GetName()), now.Add(time.Hour), constants.OktaAssignmentStatusPending,
							now, false, target(types.OktaAssignmentTargetV1_GROUP, "group1")),
						assignment(t, arNames[1], userName(user2.GetName()), now.Add(time.Hour), constants.OktaAssignmentStatusPending,
							now, false, target(types.OktaAssignmentTargetV1_GROUP, "group2")),
					},
				},
			},
		},
		{
			name:             "access requests, delete locks",
			oktaServiceCount: 1,
			accessRequests: []types.AccessRequest{
				accessRequest(t, arNames[0], user1.GetName(), []string{"role1", "role2"}, now.Add(time.Hour),
					resourceID(types.KindUserGroup, "group1")),
				accessRequest(t, arNames[1], user2.GetName(), []string{"role1", "role2"}, now.Add(time.Hour),
					resourceID(types.KindUserGroup, "group2")),
			},
			expected: []types.OktaAssignment{
				assignment(t, arNames[0], userName(user1.GetName()), now.Add(time.Hour), constants.OktaAssignmentStatusPending,
					now, false, target(types.OktaAssignmentTargetV1_GROUP, "group1")),
				assignment(t, arNames[1], userName(user2.GetName()), now.Add(time.Hour), constants.OktaAssignmentStatusPending,
					now, false, target(types.OktaAssignmentTargetV1_GROUP, "group2")),
			},
			cycles: []lockAndOnLoginCycle{
				{
					locks: []types.Lock{
						lock(t, "lock1", types.LockTarget{User: user1.GetName()}),
						lock(t, "lock2", types.LockTarget{AccessRequest: arNames[1]}),
					},
					expected: []types.OktaAssignment{
						assignment(t, arNames[0], userName(user1.GetName()), now, constants.OktaAssignmentStatusPending,
							now, false, target(types.OktaAssignmentTargetV1_GROUP, "group1")),
						assignment(t, arNames[1], userName(user2.GetName()), now, constants.OktaAssignmentStatusPending,
							now, false, target(types.OktaAssignmentTargetV1_GROUP, "group2")),
					},
				},
				{
					expected: []types.OktaAssignment{
						assignment(t, arNames[0], userName(user1.GetName()), now.Add(time.Hour), constants.OktaAssignmentStatusPending,
							now, false, target(types.OktaAssignmentTargetV1_GROUP, "group1")),
						assignment(t, arNames[1], userName(user2.GetName()), now.Add(time.Hour), constants.OktaAssignmentStatusPending,
							now, false, target(types.OktaAssignmentTargetV1_GROUP, "group2")),
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			clock := clockwork.NewFakeClockAt(now)
			ap := newTestAccessPoint(t, clock)
			onReconcileCh := make(chan struct{}, 1)
			lockWatcher := newLockWatcher(t, ap)

			// Create basic roles and user groups for use by the access requests.
			role, err := types.NewRole("role1", types.RoleSpecV6{})
			require.NoError(t, err)
			_, err = ap.CreateRole(ctx, role)
			require.NoError(t, err)

			role, err = types.NewRole("role2", types.RoleSpecV6{})
			require.NoError(t, err)
			_, err = ap.CreateRole(ctx, role)
			require.NoError(t, err)

			userGroup := group(t, "group1", types.OriginOkta, testOrgURL)
			require.NoError(t, ap.CreateUserGroup(ctx, userGroup))

			userGroup = group(t, "group2", types.OriginOkta, testOrgURL)
			require.NoError(t, ap.CreateUserGroup(ctx, userGroup))

			// Set the service count to 1 to make sure the reconciler is active.
			ap.setServiceCounts(map[types.SystemRole]uint64{types.RoleOkta: 1})

			connected, err := connected.New(connected.Config{
				DisableCache:    true,
				ConnectedGetter: ap,
				Plugins:         ap,
			})
			require.NoError(t, err)

			// Start a new reconciler for the test.
			reconciler, err := NewAccessRequestReconciler(ctx, &AccessRequestReconcilerConfig{
				Clock:         clock,
				ClusterName:   testClusterName,
				AccessPoint:   ap,
				LockWatcher:   lockWatcher,
				OktaConnected: connected,
				OktaClient:    ap,
				onReconcileCh: onReconcileCh,
			})
			require.NoError(t, err)
			require.NoError(t, reconciler.Start(ctx))
			t.Cleanup(func() {
				reconciler.Stop()
			})

			// Create all the access requests.
			count := 0
			for _, accessRequest := range test.accessRequests {
				count++
				require.NoError(t, ap.CreateAccessRequest(ctx, accessRequest))
				waitForResult(t, onReconcileCh, struct{}{}, 1)
				ap.SetAccessRequestState(ctx, types.AccessRequestUpdate{
					RequestID: accessRequest.GetName(),
					State:     types.RequestState_APPROVED,
				})
				waitForResult(t, onReconcileCh, struct{}{}, 1)
			}

			// Wait for the reconciler to see the access requests.
			require.EventuallyWithT(t, func(t *assert.CollectT) {
				require.Len(t, reconciler.accessRequests.CopyAsMap(), count)
			}, 5*time.Second, 10*time.Millisecond)

			cmpOpts := []cmp.Option{
				cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
				cmpopts.SortSlices(func(a1, a2 types.OktaAssignment) bool {
					return a1.GetName() < a2.GetName()
				}),
			}

			// Make sure the Okta assignments reflect the access requests.
			assignments, _, err := ap.ListOktaAssignments(ctx, 0, "")
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(test.expected, assignments, cmpOpts...))

			// Update the service counts.
			ap.setServiceCounts(map[types.SystemRole]uint64{types.RoleOkta: uint64(test.oktaServiceCount)})

			cycleCount := 0
			// Lock, run on login, and then test the Okta assignments.
			for _, cycle := range test.cycles {
				// Delete all of the locks so that we have a fresh set of locks in the cycle
				require.NoError(t, ap.DeleteAllLocks(ctx), "cycle %d", cycleCount)
				require.Eventually(t, func() bool {
					return len(lockWatcher.GetCurrent()) == 0
				}, 5*time.Second, 10*time.Millisecond, "cycle %d: lock watcher did not empty", cycleCount)

				// Create the locks.
				for _, lock := range cycle.locks {
					// Make sure each lock shows up in the watcher.
					require.NoError(t, ap.UpsertLock(ctx, lock), "cycle %d", cycleCount)
					require.Eventually(t, func() bool {
						for _, currentLock := range lockWatcher.GetCurrent() {
							if currentLock.GetName() == lock.GetName() && cmp.Diff(currentLock.Target(), lock.Target()) == "" {
								return true
							}

						}

						return false
					}, 5*time.Second, 10*time.Millisecond,
						"cycle %d: lock %s did not appear in lock watcher with the appropriate target %s", cycleCount, lock.GetName(), lock.Target().String())
				}

				// Run on-login, which should modify any assignments if locks are present/removed.
				require.NoError(t, reconciler.OnLogin(ctx, user1), "cycle %d", cycleCount)
				require.NoError(t, reconciler.OnLogin(ctx, user2), "cycle %d", cycleCount)

				// Make sure the new okta assignments reflect what should appear this cycle.
				assignments, _, err = ap.ListOktaAssignments(ctx, 0, "")
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(cycle.expected, assignments, cmpOpts...), "cycle %d", cycleCount)

				cycleCount++
			}
		})
	}
}

func setupTestAccessRequestReconciler(t *testing.T) (context.Context, clocki.FakeClock, *testAccessPoint, *AccessRequestReconciler) {
	t.Helper()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	ap := newTestAccessPoint(t, clock)
	onReconcileCh := make(chan struct{}, 1)
	onServiceDisconnectedCh := make(chan struct{}, 1)
	connected, err := connected.New(connected.Config{
		DisableCache:    true,
		ConnectedGetter: ap,
		Plugins:         ap,
	})
	require.NoError(t, err)

	reconciler, err := NewAccessRequestReconciler(ctx, &AccessRequestReconcilerConfig{
		Clock:                   clock,
		ClusterName:             testClusterName,
		AccessPoint:             ap,
		LockWatcher:             newLockWatcher(t, ap),
		OktaConnected:           connected,
		OktaClient:              ap,
		onReconcileCh:           onReconcileCh,
		onServiceDisconnectedCh: onServiceDisconnectedCh,
	})
	require.NoError(t, err)
	require.NoError(t, reconciler.Start(ctx))
	t.Cleanup(func() {
		reconciler.Stop()
	})

	return ctx, clock, ap, reconciler
}

func getOktaAssignment(t *testing.T, ap *testAccessPoint, name string) types.OktaAssignment {
	t.Helper()

	ctx := context.Background()

	accessRequest, err := ap.GetOktaAssignment(ctx, name)
	require.NoError(t, err)

	return accessRequest
}

func accessRequest(t *testing.T, name, user string, roles []string, expiry time.Time, resourceIDs ...types.ResourceAccessID) types.AccessRequest {
	t.Helper()

	accessRequest, err := types.NewAccessRequestWithResources(name, user, roles, resourceIDs)
	require.NoError(t, err)
	accessRequest.SetAccessExpiry(expiry)

	return accessRequest
}

func resourceID(kind, name string) types.ResourceAccessID {
	return types.ResourceAccessID{
		Id: types.ResourceID{
			ClusterName: testClusterName,
			Kind:        kind,
			Name:        name,
		},
	}
}

func lock(t *testing.T, name string, target types.LockTarget) types.Lock {
	t.Helper()

	lock, err := types.NewLock(name, types.LockSpecV2{
		Target: target,
	})
	require.NoError(t, err)

	return lock
}
