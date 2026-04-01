package okta

import (
	"context"
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
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/events"
)

func TestAssignmentReconciler(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClockAt(time.Now())
	ctx := context.Background()
	ap := newTestAccessPoint(t, clock)
	oktaClient := newTestOktaClient()
	svc, emitter := newTestService(t, ap, oktaClient)
	svc.clock = clock
	onReconcileCh := make(chan struct{}, 1)
	testUser := userName("test-user@test.user")
	testOktaUserID := oktaUserID("okta-user-id")

	oktaClient.AddUserID(testUser, testOktaUserID)

	const link = "link"
	appName := func(name string) string {
		return mustAppName(t, name, link)
	}

	reconciler := newAssignmentReconciler(svc)
	reconciler.onReconcileCh = onReconcileCh
	reconciler.noAssignmentProcessorLoop = true
	require.NoError(t, reconciler.start(ctx))
	t.Cleanup(func() {
		reconciler.stop()
	})

	waitForResult(t, onReconcileCh, struct{}{}, 1)

	t.Run("finalized assignment should be deleted from backend", func(t *testing.T) {
		cleanedUpAssignment := assignment(t, "cleaned-up-assignment", testUser, clock.Now(), constants.OktaAssignmentStatusSuccessful, clock.Now(), true,
			target(types.OktaAssignmentTargetV1_APPLICATION, appName("cleanedUpApp1")),
			target(types.OktaAssignmentTargetV1_GROUP, "cleanedUpGroup1"),
		)
		_, err := ap.CreateOktaAssignment(ctx, cleanedUpAssignment)
		require.NoError(t, err)

		// 2 event expected:
		// Assignment Creation
		// Assignment Deletion
		waitForResult(t, onReconcileCh, struct{}{}, 2)
		assertOktaAssignments(t, ap, types.OktaAssignments{})

	})

	// This Okta assignment should be recognized.
	assignment1 := assignment(t, "assignment1", testUser, time.Time{}, constants.OktaAssignmentStatusPending, clock.Now(), false,
		target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
		target(types.OktaAssignmentTargetV1_GROUP, "group1"),
	)

	_, err := ap.CreateOktaAssignment(ctx, assignment1)
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
	assertOktaAssignments(t, ap, types.OktaAssignments{assignment1})

	foundAssignment, err := ap.GetOktaAssignment(ctx, assignment1.GetName())
	require.NoError(t, err)
	assertAssignments(t, types.OktaAssignments{assignment1}, types.OktaAssignments{foundAssignment})

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
	waitForResult(t, onReconcileCh, struct{}{}, 4)

	expectAuditEvent(t, emitter, func(event *apievents.OktaAssignmentResult) {
		require.Equal(t, events.OktaAssignmentCleanupEvent, event.GetType())
		require.Equal(t, events.OktaAssignmentCleanupSuccessCode, event.GetCode())
		require.Equal(t, constants.OktaAssignmentStatusSuccessful, event.StartingStatus)
		require.Equal(t, constants.OktaAssignmentStatusSuccessful, event.EndingStatus)
	})

	assertAssignmentDoesntExist(t, ap, assignment1.GetName())
	assertOktaAssignments(t, ap, types.OktaAssignments{})
}

func assertAssignmentDoesntExist(t *testing.T, ap *testAccessPoint, name string) {
	t.Helper()
	ctx := t.Context()
	_, err := ap.GetOktaAssignment(ctx, name)
	require.True(t, trace.IsNotFound(err))
}

func assertOktaAssignments(t *testing.T, ap *testAccessPoint, want types.OktaAssignments) {
	t.Helper()
	ctx := t.Context()
	actual := make([]types.OktaAssignment, 0, len(want))
	for oa, err := range clientutils.Resources(ctx, ap.ListOktaAssignments) {
		require.NoError(t, err)
		actual = append(actual, oa)
	}
	assertAssignments(t, want, actual)
}

func assertAssignments(t *testing.T, want, got types.OktaAssignments) {
	cmpOpts := cmpopts.IgnoreFields(types.Metadata{}, "Revision")
	require.Empty(t, cmp.Diff(want, got, cmpopts.SortSlices(assignmentLess), cmpOpts))
}
