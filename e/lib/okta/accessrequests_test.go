package okta

import (
	"context"
	"crypto"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
)

func TestAccessRequestReconciler(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	ap := newTestAccessPoint(t, clock)
	onReconcileCh := make(chan struct{}, 1)
	onServiceDisconnectedCh := make(chan struct{}, 1)

	reconciler, err := NewAccessRequestReconciler(ctx, &AccessRequestReconcilerConfig{
		Clock:                   clock,
		ClusterName:             testClusterName,
		AccessPoint:             ap,
		OktaClient:              ap,
		onReconcileCh:           onReconcileCh,
		onServiceDisconnectedCh: onServiceDisconnectedCh,
	})
	require.NoError(t, err)
	require.NoError(t, reconciler.Start(ctx))
	t.Cleanup(func() {
		reconciler.Stop()
	})

	waitForResult(t, onReconcileCh, struct{}{}, 1)

	// Reconciler should be empty to start
	require.Empty(t, reconciler.getAccessRequests())
	require.Empty(t, reconciler.getNewAccessRequests())

	user := "test-user"
	roles := []string{"test-role"}

	// This access request should be ignored.
	accessRequest, err := types.NewAccessRequestWithResources(uuid.NewString(), user, roles,
		[]types.ResourceID{{ClusterName: testClusterName, Kind: types.KindRole, Name: "role-request"}})
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

	hash := crypto.SHA256
	appServer := application(t, hash, "app1", "link1", types.OriginOkta, testOrgURL)
	_, err = ap.UpsertApplicationServer(ctx, appServer)
	require.NoError(t, err)

	// This access request should not be registered by the reconciler
	accessRequest, err = types.NewAccessRequestWithResources(uuid.NewString(), user, roles,
		[]types.ResourceID{{ClusterName: testClusterName, Kind: types.KindApp, Name: appServer.GetApp().GetName()}})
	require.NoError(t, err)
	accessRequest.SetState(types.RequestState_DENIED)

	accessRequest, err = ap.CreateAccessRequestV2(ctx, accessRequest)
	require.NoError(t, err)

	waitForResult(t, onReconcileCh, struct{}{}, 1)

	require.Empty(t, reconciler.getAccessRequests())
	require.Empty(t, cmp.Diff(types.ResourcesWithLabelsMap{accessRequest.GetName(): accessRequest}, reconciler.getNewAccessRequests(), cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	require.NoError(t, ap.DeleteAccessRequest(ctx, accessRequest.GetName()))
	waitForResult(t, onReconcileCh, struct{}{}, 1)

	// This access request should also not be registered by the reconciler
	accessRequest, err = types.NewAccessRequestWithResources(uuid.NewString(), user, roles,
		[]types.ResourceID{{ClusterName: "other-cluster-name", Kind: types.KindApp, Name: appServer.GetApp().GetName()}})
	require.NoError(t, err)
	accessRequest.SetState(types.RequestState_APPROVED)

	accessRequest, err = ap.CreateAccessRequestV2(ctx, accessRequest)
	require.NoError(t, err)

	waitForResult(t, onReconcileCh, struct{}{}, 1)

	require.Empty(t, reconciler.getAccessRequests())
	require.Empty(t, cmp.Diff(types.ResourcesWithLabelsMap{accessRequest.GetName(): accessRequest}, reconciler.getNewAccessRequests(), cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	require.NoError(t, ap.DeleteAccessRequest(ctx, accessRequest.GetName()))
	waitForResult(t, onReconcileCh, struct{}{}, 1)

	// This access request should create an Okta assignment, but the Okta service is not connected.
	ap.setServiceCounts(map[types.SystemRole]uint64{})

	// This will stop the reconciler.
	for i := 0; i < maxOktaServiceConnectionFailures; i++ {
		clock.Advance(10 * time.Minute)
		waitForResult(t, onServiceDisconnectedCh, struct{}{}, 1)
	}

	accessRequest, err = types.NewAccessRequestWithResources(uuid.NewString(), user, roles,
		[]types.ResourceID{{ClusterName: testClusterName, Kind: types.KindApp, Name: appServer.GetApp().GetName()}})
	require.NoError(t, err)
	accessRequest.SetState(types.RequestState_APPROVED)

	accessRequest, err = ap.CreateAccessRequestV2(ctx, accessRequest)
	require.NoError(t, err)

	// No reconcile will be triggered because the reconciler will be stopped.
	require.Empty(t, reconciler.getAccessRequests())
	require.Empty(t, reconciler.getNewAccessRequests())

	// We'll reconnect the Okta service and the assignment should be created.
	ap.setServiceCounts(map[types.SystemRole]uint64{types.RoleOkta: 1})
	clock.Advance(10 * time.Minute) // This will restart the reconciler.
	waitForResult(t, onReconcileCh, struct{}{}, 1)
	require.Empty(t, cmp.Diff(types.ResourcesWithLabelsMap{accessRequest.GetName(): accessRequest}, reconciler.getAccessRequests(), cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
	require.Empty(t, cmp.Diff(types.ResourcesWithLabelsMap{accessRequest.GetName(): accessRequest}, reconciler.getNewAccessRequests(), cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))

	foundAssignment := getOktaAssignment(t, ap, accessRequest.GetName())
	expires := accessRequest.GetAccessExpiry()
	require.Empty(t, cmp.Diff(foundAssignment, assignment(t, accessRequest.GetName(), user, expires, constants.OktaAssignmentStatusPending, clock.Now(), false,
		target(types.OktaAssignmentTargetV1_APPLICATION, mustAppName(t, hash, "app1", "link1"))),
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Deny the state after the fact
	_, err = ap.SetAccessRequestState(ctx, types.AccessRequestUpdate{
		RequestID: accessRequest.GetName(),
		State:     types.RequestState_DENIED,
	})
	require.NoError(t, err)
	waitForResult(t, onReconcileCh, struct{}{}, 1)

	cleanupTimeNow := clock.Now()
	foundAssignment = getOktaAssignment(t, ap, accessRequest.GetName())
	require.Empty(t, cmp.Diff(foundAssignment, assignment(t, accessRequest.GetName(), user, cleanupTimeNow, constants.OktaAssignmentStatusPending, clock.Now(), false,
		target(types.OktaAssignmentTargetV1_APPLICATION, mustAppName(t, hash, "app1", "link1"))),
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// This delete shouldn't do anything to the assignment.
	require.NoError(t, ap.DeleteAccessRequest(ctx, accessRequest.GetName()))
	waitForResult(t, onReconcileCh, struct{}{}, 1)

	foundAssignment = getOktaAssignment(t, ap, accessRequest.GetName())
	require.Empty(t, cmp.Diff(foundAssignment, assignment(t, accessRequest.GetName(), user, cleanupTimeNow, constants.OktaAssignmentStatusPending, clock.Now(), false,
		target(types.OktaAssignmentTargetV1_APPLICATION, mustAppName(t, hash, "app1", "link1"))),
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// This access request should create an Okta assignment
	userGroup := group(t, "group1", types.OriginOkta, testOrgURL)
	require.NoError(t, ap.CreateUserGroup(ctx, userGroup))

	accessRequest, err = types.NewAccessRequestWithResources(uuid.NewString(), user, roles,
		[]types.ResourceID{{ClusterName: testClusterName, Kind: types.KindUserGroup, Name: userGroup.GetName()}})
	accessRequest.SetState(types.RequestState_APPROVED)
	require.NoError(t, err)

	accessRequest, err = ap.CreateAccessRequestV2(ctx, accessRequest)
	require.NoError(t, err)

	waitForResult(t, onReconcileCh, struct{}{}, 1)

	foundAssignment = getOktaAssignment(t, ap, accessRequest.GetName())
	expires = accessRequest.GetAccessExpiry()
	require.Empty(t, cmp.Diff(foundAssignment, assignment(t, accessRequest.GetName(), user, expires, constants.OktaAssignmentStatusPending, clock.Now(), false,
		target(types.OktaAssignmentTargetV1_GROUP, userGroup.GetName())),
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	require.NoError(t, ap.DeleteAccessRequest(ctx, accessRequest.GetName()))
	waitForResult(t, onReconcileCh, struct{}{}, 1)

	foundAssignment = getOktaAssignment(t, ap, accessRequest.GetName())
	require.Empty(t, cmp.Diff(foundAssignment, assignment(t, accessRequest.GetName(), user, cleanupTimeNow, constants.OktaAssignmentStatusPending, clock.Now(), false,
		target(types.OktaAssignmentTargetV1_GROUP, userGroup.GetName())),
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))
}

func TestAccessRequestToOktaAssignment(t *testing.T) {
	const accessRequestName = "access_request"
	const user = "user"
	hash := crypto.SHA256
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
				application(t, hash, "app1", "link1", types.OriginOkta, testOrgURL),
				application(t, hash, "app2", "link1", types.OriginOkta, testOrgURL),
				application(t, hash, "app3", "link1", types.OriginDynamic, testOrgURL),
			},
			groupTargets: []types.UserGroup{
				group(t, "group1", types.OriginOkta, testOrgURL),
				group(t, "group2", types.OriginDynamic, testOrgURL),
			},
			assignmentStatus: constants.OktaAssignmentStatusPending,
			expected: assignment(t, accessRequestName, user, expires, constants.OktaAssignmentStatusPending, clock.Now(), false,
				target(types.OktaAssignmentTargetV1_APPLICATION, mustAppName(t, hash, "app1", "link1")),
				target(types.OktaAssignmentTargetV1_APPLICATION, mustAppName(t, hash, "app2", "link1")),
				target(types.OktaAssignmentTargetV1_GROUP, "group1"),
			),
			errAssertionFunc: require.NoError,
		},
		{
			name:              "app not found",
			skipBackendCreate: true,
			appTargets: []types.AppServer{
				application(t, hash, "app1", "link1", types.OriginOkta, testOrgURL),
			},
			assignmentStatus: constants.OktaAssignmentStatusPending,
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
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
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name: "no Okta targets",
			appTargets: []types.AppServer{
				application(t, hash, "app1", "link1", types.OriginDynamic, testOrgURL),
				application(t, hash, "app2", "link1", types.OriginDynamic, testOrgURL),
				application(t, hash, "app3", "link1", types.OriginDynamic, testOrgURL), // This should be skipped
			},
			groupTargets: []types.UserGroup{
				group(t, "group1", types.OriginDynamic, testOrgURL),
				group(t, "group2", types.OriginDynamic, testOrgURL),
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, trace.NotFound("no Okta targets found in access request"), err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			ap := newTestAccessPoint(t, clock)

			reconciler, err := NewAccessRequestReconciler(ctx, &AccessRequestReconcilerConfig{
				Clock:       clock,
				ClusterName: testClusterName,
				AccessPoint: ap,
				OktaClient:  ap,
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

func getOktaAssignment(t *testing.T, ap *testAccessPoint, name string) types.OktaAssignment {
	ctx := context.Background()

	accessRequest, err := ap.GetOktaAssignment(ctx, name)
	require.NoError(t, err)

	return accessRequest
}
