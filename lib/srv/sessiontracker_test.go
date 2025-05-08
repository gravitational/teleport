/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package srv

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type failingTrackerService struct {
	services.SessionTrackerService
	updated     chan struct{}
	updateError chan error
}

func (f *failingTrackerService) CreateSessionTracker(ctx context.Context, s types.SessionTracker) (types.SessionTracker, error) {
	return s, nil
}

func (f *failingTrackerService) UpdateSessionTracker(ctx context.Context, req *proto.UpdateSessionTrackerRequest) error {
	f.updated <- struct{}{}

	return <-f.updateError
}

func waitForUpdate(t *testing.T, svc *failingTrackerService, done chan error) {
	t.Helper()
	select {
	case <-svc.updated:
	case err := <-done:
		t.Fatal("Update loop terminated early", err.Error())
	case <-time.After(10 * time.Second):
		t.Fatal("Timed out waiting for session tracker update")
	}
}

func TestSessionTracker_UpdateRetry(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	clock := clockwork.NewFakeClock()

	svc := &failingTrackerService{
		updated:     make(chan struct{}),
		updateError: make(chan error),
	}
	spec := types.SessionTrackerSpecV1{
		Created:   clock.Now(),
		SessionID: "session",
		Expires:   clock.Now().Add(10 * time.Hour),
	}
	tracker, err := NewSessionTracker(ctx, spec, svc)
	require.NoError(t, err)
	done := make(chan error)

	updateError := trace.ConnectionProblem(context.DeadlineExceeded, "connection refused")

	// start the update loop
	go func() {
		done <- tracker.UpdateExpirationLoop(ctx, clock)
	}()

	// Walk through a few attempts to update the session tracker. Even iterations
	// will fail and force the retry mechanism to kick in. Odd iterations update
	// session trackers successfully on first attempt.
	for i := 0; i < 4; i++ {
		clock.BlockUntil(1)

		// advance the clock to fire the ticker
		clock.Advance(sessionTrackerExpirationUpdateInterval)

		// wait for update to be called
		waitForUpdate(t, svc, done)

		// send back an error on even iterations
		if i%2 == 0 {
			svc.updateError <- updateError

			// wait for the retry to be engaged
			clock.BlockUntil(1)

			// advance far enough for the retry to fire
			clock.Advance(65 * time.Second)

			// wait for the update to be called again
			waitForUpdate(t, svc, done)
		}

		svc.updateError <- nil
	}

	// advance the clock for one last update attempt
	clock.BlockUntil(1)
	clock.Advance(sessionTrackerExpirationUpdateInterval)

	// wait for update to be called and return an error to
	// get in the retry path
	waitForUpdate(t, svc, done)
	svc.updateError <- updateError

	// advance far enough for the retry to fire
	clock.BlockUntil(1)
	clock.Advance(65 * time.Second)

	// wait for update to be called from the retry loop and return an error
	waitForUpdate(t, svc, done)
	// update the clock to make the tracker stale and abort the retry loop
	clock.Advance(10 * time.Hour)
	svc.updateError <- updateError

	// ensure the update loop ends
	select {
	case err := <-done:
		require.ErrorIs(t, err, updateError)
	case <-time.After(10 * time.Second):
		t.Fatal("Timed out waiting for update loop to terminate")
	}
}

func TestSessionTracker(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clock := clockwork.NewFakeClock()

	mockService := &mockSessiontrackerService{
		trackers: make(map[string]types.SessionTracker),
	}

	sessID := "sessionID"
	trackerSpec := types.SessionTrackerSpecV1{
		Created:   clock.Now(),
		SessionID: sessID,
	}

	// Create a new session tracker
	tracker, err := NewSessionTracker(ctx, trackerSpec, mockService)
	require.NoError(t, err)
	require.NotNil(t, tracker)
	require.Equal(t, tracker.tracker, mockService.trackers[sessID])

	t.Run("UpdateExpirationLoop", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		done := make(chan error)

		// Start update expiration goroutine
		go func() {
			done <- tracker.UpdateExpirationLoop(cancelCtx, clock)
		}()

		// wait until the ticker has been created
		clock.BlockUntil(1)

		// lock expiry and advance clock
		tracker.trackerCond.L.Lock()
		clock.Advance(sessionTrackerExpirationUpdateInterval)
		expectedExpiry := tracker.tracker.Expiry().Add(sessionTrackerExpirationUpdateInterval)

		// wait for expiration to get updated
		tracker.trackerCond.Wait()
		tracker.trackerCond.L.Unlock()
		require.Equal(t, expectedExpiry, tracker.tracker.Expiry())
		require.Equal(t, tracker.tracker, mockService.trackers[sessID])

		// canceling the goroutine's ctx should halt the update loop
		cancel()
		err := <-done
		require.NoError(t, err)
	})

	t.Run("State", func(t *testing.T) {
		stateUpdate := make(chan types.SessionState)
		go func() {
			stateUpdate <- tracker.WaitForStateUpdate(types.SessionState_SessionStatePending)
		}()

		err = tracker.UpdateState(ctx, types.SessionState_SessionStatePending)
		require.NoError(t, err)
		require.Equal(t, types.SessionState_SessionStatePending, tracker.GetState())
		require.Equal(t, tracker.tracker, mockService.trackers[sessID])

		err = tracker.UpdateState(ctx, types.SessionState_SessionStateRunning)
		require.NoError(t, err)
		require.Equal(t, types.SessionState_SessionStateRunning, tracker.GetState())
		require.Equal(t, tracker.tracker, mockService.trackers[sessID])

		// WaitForStateUpdate should ignore the pending update and then catch the running update
		require.Equal(t, types.SessionState_SessionStateRunning, <-stateUpdate)
	})

	t.Run("Participants", func(t *testing.T) {
		participantID := "userID"

		p := &types.Participant{ID: participantID}
		err = tracker.AddParticipant(ctx, p)
		require.NoError(t, err)
		require.Equal(t, []types.Participant{*p}, tracker.GetParticipants())
		require.Equal(t, tracker.tracker, mockService.trackers[sessID])

		err = tracker.RemoveParticipant(ctx, participantID)
		require.NoError(t, err)
		require.Empty(t, tracker.GetParticipants())
		require.Equal(t, tracker.tracker, mockService.trackers[sessID])
	})

	t.Run("Close", func(t *testing.T) {
		// Closing the tracker should update the state to terminated
		err = tracker.Close(ctx)
		require.NoError(t, err)
		require.Equal(t, types.SessionState_SessionStateTerminated, tracker.GetState())
		require.Equal(t, tracker.tracker, mockService.trackers[sessID])
	})
}

type mockSessiontrackerService struct {
	trackers map[string]types.SessionTracker
}

func (m *mockSessiontrackerService) GetActiveSessionTrackers(ctx context.Context) ([]types.SessionTracker, error) {
	return nil, trace.NotImplemented("")
}

func (m *mockSessiontrackerService) GetActiveSessionTrackersWithFilter(ctx context.Context, filter *types.SessionTrackerFilter) ([]types.SessionTracker, error) {
	return nil, trace.NotImplemented("")
}

func (m *mockSessiontrackerService) GetSessionTracker(ctx context.Context, sessionID string) (types.SessionTracker, error) {
	return nil, trace.NotImplemented("")
}

func (m *mockSessiontrackerService) UpdateSessionTracker(ctx context.Context, req *proto.UpdateSessionTrackerRequest) error {
	// m.trackers[req.SessionID] will be updated as a pointer reference
	return nil
}

func (m *mockSessiontrackerService) RemoveSessionTracker(ctx context.Context, sessionID string) error {
	return trace.NotImplemented("")
}

func (m *mockSessiontrackerService) UpdatePresence(ctx context.Context, sessionID, user string) error {
	return trace.NotImplemented("")
}

func (m *mockSessiontrackerService) CreateSessionTracker(ctx context.Context, tracker types.SessionTracker) (types.SessionTracker, error) {
	m.trackers[tracker.GetSessionID()] = tracker
	return tracker, nil
}
