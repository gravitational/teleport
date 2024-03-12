package okta

import (
	"context"
	"crypto"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

func TestAssignmentReconciler(t *testing.T) {
	clock := clockwork.NewFakeClockAt(time.Now())
	ctx := context.Background()
	hash := crypto.SHA256
	ap := newTestAccessPoint(t, clock)
	svc, oktaClient, emitter := newTestService(t, ap)
	svc.SetLeader(true)
	svc.clock = clock
	onReconcileCh := make(chan struct{}, 1)
	testUser := "test-user@test.user"
	testOktaUserID := "okta-user-id"

	oktaClient.addUserID(testUser, testOktaUserID)

	const link = "link"
	appName := func(name string) string {
		return mustAppName(t, hash, name, link)
	}

	reconciler := newAssignmentReconciler(ctx, testClusterName, svc)
	reconciler.onReconcileCh = onReconcileCh
	reconciler.noAssignmentProcessorLoop = true
	require.NoError(t, reconciler.start(ctx))
	t.Cleanup(func() {
		reconciler.stop()
	})

	waitForResult(t, onReconcileCh, struct{}{}, 1)

	// Reconciler should be empty to start
	require.Empty(t, reconciler.getAssignments())
	require.Empty(t, reconciler.getNewAssignments())

	// Create the cleaned up resources in the backend.
	require.NoError(t, ap.CreateUserGroup(ctx, group(t, "cleanedUpGroup1", types.OriginOkta, testOrgURL)))
	_, err := ap.UpsertApplicationServer(ctx,
		application(t, hash, "cleanedUpApp1", link, types.OriginOkta, testOrgURL, testHostID))
	require.NoError(t, err)
	oktaClient.addGroupToMapping("cleanedUpGroup1")
	oktaClient.addApplicationToMapping("okta-app-1")

	// This Okta assignment should not be operated on by the processor.
	startTime := clock.Now()
	cleanedUpAssignment := assignment(t, "cleaned-up-assignment", testUser, startTime, constants.OktaAssignmentStatusSuccessful, clock.Now(), true,
		target(types.OktaAssignmentTargetV1_APPLICATION, appName("cleanedUpApp1")),
		target(types.OktaAssignmentTargetV1_GROUP, "cleanedUpGroup1"),
	)
	_, err = ap.CreateOktaAssignment(ctx, cleanedUpAssignment)
	require.NoError(t, err)

	// 1 event expected:
	// Creation of the above assignment.
	// No further actions on this for now.
	waitForResult(t, onReconcileCh, struct{}{}, 1)

	require.Empty(t, cmp.Diff(types.OktaAssignments{cleanedUpAssignment}, reconciler.getAssignments(),
		cmpopts.SortSlices(assignmentLess), cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")),
	)
	require.Empty(t, cmp.Diff(types.OktaAssignments{cleanedUpAssignment}, reconciler.getNewAssignments(),
		cmpopts.SortSlices(assignmentLess), cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")),
	)

	// Create the actual resources in the backend.
	require.NoError(t, ap.CreateUserGroup(ctx, group(t, "group1", types.OriginOkta, testOrgURL)))
	_, err = ap.UpsertApplicationServer(ctx,
		application(t, hash, "app1", "link", types.OriginOkta, testOrgURL, testHostID))
	require.NoError(t, err)
	oktaClient.addGroupToMapping("group1")
	oktaClient.addApplicationToMapping("okta-app-2")

	// This Okta assignment should be recognized.
	assignment1 := assignment(t, "assignment1", testUser, time.Time{}, constants.OktaAssignmentStatusPending, clock.Now(), false,
		target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
		target(types.OktaAssignmentTargetV1_GROUP, "group1"),
	)

	oktaClient.addApplicationToMapping("app1")
	oktaClient.addGroupToMapping("group1")

	_, err = ap.CreateOktaAssignment(ctx, assignment1)
	require.NoError(t, err)

	// 3 events expected:
	// assignment created
	// assignment -> PROCESSING
	// assignment -> SUCCESSFUL
	waitForResult(t, onReconcileCh, struct{}{}, 3)

	expectAuditEvent(t, emitter, func(event *apievents.OktaAssignmentResult) {
		require.Equal(t, events.OktaAssignmentProcessEvent, event.GetType())
		require.Equal(t, events.OktaAssignmentProcessSuccessCode, event.GetCode())
		require.Equal(t, constants.OktaAssignmentStatusPending, event.StartingStatus)
		require.Equal(t, constants.OktaAssignmentStatusSuccessful, event.EndingStatus)
	})

	assignment1 = assignment(t, "assignment1", testUser, time.Time{}, constants.OktaAssignmentStatusSuccessful, clock.Now(), false,
		target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
		target(types.OktaAssignmentTargetV1_GROUP, "group1"),
	)

	require.Empty(t, cmp.Diff(types.OktaAssignments{cleanedUpAssignment, assignment1}, reconciler.getAssignments(),
		cmpopts.SortSlices(assignmentLess), cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")),
	)
	require.Empty(t, cmp.Diff(types.OktaAssignments{cleanedUpAssignment, assignment1}, reconciler.getNewAssignments(),
		cmpopts.SortSlices(assignmentLess), cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")),
	)

	foundAssignment, err := ap.GetOktaAssignment(ctx, assignment1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(foundAssignment, assignment1,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")),
	)

	// Update should be recognized.
	cleanupTime := clock.Now()
	lastTransition := clock.Now().Add(-5 * time.Second)
	foundAssignment.SetCleanupTime(cleanupTime)
	foundAssignment.SetLastTransition(lastTransition)
	clock.Advance(10 * time.Minute)
	_, err = ap.UpdateOktaAssignment(ctx, foundAssignment)
	require.NoError(t, err)

	// 3 events expected:
	// app1 -> object update (the test update above)
	// app1 -> PROCESSING
	// app1 -> SUCCESSFUL (cleaned up)
	waitForResult(t, onReconcileCh, struct{}{}, 3)

	expectAuditEvent(t, emitter, func(event *apievents.OktaAssignmentResult) {
		require.Equal(t, events.OktaAssignmentCleanupEvent, event.GetType())
		require.Equal(t, events.OktaAssignmentCleanupSuccessCode, event.GetCode())
		require.Equal(t, constants.OktaAssignmentStatusSuccessful, event.StartingStatus)
		require.Equal(t, constants.OktaAssignmentStatusSuccessful, event.EndingStatus)
	})

	assignment1 = assignment(t, "assignment1", testUser, cleanupTime, constants.OktaAssignmentStatusSuccessful, clock.Now(), true,
		target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
		target(types.OktaAssignmentTargetV1_GROUP, "group1"),
	)

	require.Empty(t, cmp.Diff(types.OktaAssignments{cleanedUpAssignment, assignment1}, reconciler.getAssignments(),
		cmpopts.SortSlices(assignmentLess), cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")),
	)
	require.Empty(t, cmp.Diff(types.OktaAssignments{cleanedUpAssignment, assignment1}, reconciler.getNewAssignments(),
		cmpopts.SortSlices(assignmentLess), cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")),
	)

	foundAssignment, err = ap.GetOktaAssignment(ctx, assignment1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(foundAssignment, assignment1,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")),
	)

	// This delete be recognized.
	require.NoError(t, ap.DeleteOktaAssignment(ctx, assignment1.GetName()))

	// 1 event expected:
	// Deletion of the Okta assignment.
	waitForResult(t, onReconcileCh, struct{}{}, 1)

	require.Empty(t, cmp.Diff(types.OktaAssignments{cleanedUpAssignment}, reconciler.getAssignments(),
		cmpopts.SortSlices(assignmentLess), cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")),
	)
	require.Empty(t, cmp.Diff(types.OktaAssignments{cleanedUpAssignment}, reconciler.getNewAssignments(),
		cmpopts.SortSlices(assignmentLess), cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")),
	)

	_, err = ap.GetOktaAssignment(ctx, assignment1.GetName())
	require.True(t, trace.IsNotFound(err))
}
