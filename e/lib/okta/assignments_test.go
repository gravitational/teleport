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
	onWatcherEventCh := make(chan struct{}, 1)
	testUser := userName("test-user@test.user")
	testOktaUserID := oktaUserID("okta-user-id")

	oktaClient.AddUserID(testUser, testOktaUserID)

	const link = "link"
	appName := func(name string) string {
		return mustAppName(t, name, link)
	}

	reconciler := newAssignmentReconciler(svc)
	reconciler.testOnWatchEventCh = onWatcherEventCh
	reconciler.testNoAssignmentProcessorLoop = true
	require.NoError(t, reconciler.start(ctx))
	t.Cleanup(func() {
		reconciler.stop()
	})

	// OpInit event comes first. This confirms the watcher is watching before we create the
	// first assignment.
	waitForResult(t, onWatcherEventCh, struct{}{}, 1)

	// Create a pending assignment.
	assignment1 := assignment(t, "assignment1", testUser, time.Time{}, constants.OktaAssignmentStatusPending, clock.Now(), false,
		target(types.OktaAssignmentTargetV1_APPLICATION, appName("app1")),
		target(types.OktaAssignmentTargetV1_GROUP, "group1"),
	)
	_, err := ap.CreateOktaAssignment(ctx, assignment1)
	require.NoError(t, err)

	waitForResult(t, onWatcherEventCh, struct{}{}, 1)

	expectAuditEvent(t, emitter, func(event *apievents.OktaAssignmentResult) {
		require.Equal(t, events.OktaAssignmentProcessEvent, event.GetType())
		require.Equal(t, events.OktaAssignmentProcessSuccessCode, event.GetCode())
		require.Equal(t, constants.OktaAssignmentStatusPending, event.StartingStatus)
		require.Equal(t, constants.OktaAssignmentStatusSuccessful, event.EndingStatus)
	})

	// Check the assignment was reprocessed and is in successful state now.
	require.NoError(t, assignment1.SetStatus(constants.OktaAssignmentStatusProcessing))
	require.NoError(t, assignment1.SetStatus(constants.OktaAssignmentStatusSuccessful))
	for _, target := range assignment1.GetTargets() {
		require.NoError(t, target.RecordStatus(
			clock.Now(),
			constants.OktaAssignmentTargetOpProvision,
			constants.OktaAssignmentTargetOutcomeSuccessful,
		))
	}
	assertOktaAssignments(t, ap, types.OktaAssignments{assignment1})

	// Set CleanupTime to past and set LastTransition to time before the CleanupTime.
	cleanupTime := clock.Now()
	clock.Advance(10 * time.Minute)
	assignment1.SetCleanupTime(cleanupTime)
	assignment1.SetLastTransition(cleanupTime.Add(-5 * time.Second))
	_, err = ap.UpdateOktaAssignment(ctx, assignment1)
	require.NoError(t, err)

	waitForResult(t, onWatcherEventCh, struct{}{}, 1)

	expectAuditEvent(t, emitter, func(event *apievents.OktaAssignmentResult) {
		require.Equal(t, events.OktaAssignmentCleanupEvent, event.GetType())
		require.Equal(t, events.OktaAssignmentCleanupSuccessCode, event.GetCode())
		require.Equal(t, constants.OktaAssignmentStatusSuccessful, event.StartingStatus)
		require.Equal(t, constants.OktaAssignmentStatusSuccessful, event.EndingStatus)
	})

	// Make sure the assignment is successfully cleaned up and finalized.
	assignments, _, err := ap.ListOktaAssignments(ctx, 100, "")
	require.NoError(t, err)
	require.Len(t, assignments, 1)
	require.Equal(t, assignment1.GetName(), assignments[0].GetName())
	require.False(t, assignments[0].GetCleanupTime().IsZero())
	require.True(t, assignments[0].IsFinalized())
	require.Equal(t, constants.OktaAssignmentStatusSuccessful, assignments[0].GetStatus())
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
