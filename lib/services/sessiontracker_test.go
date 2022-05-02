/*
Copyright 2022 Gravitational, Inc.

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

package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestTrackSession(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClockAt(time.Now())

	mockSessionTrackerService := &mockSessiontrackerService{
		trackers: make(map[string]types.SessionTracker),
	}
	sessID := "sessionID"

	// Create a ticker and begin tracking the fake session
	ticker := clock.NewTicker(defaults.SessionTrackerExpirationUpdateInterval)
	closeCtx, cancel := context.WithCancel(ctx)
	doneTracking := make(chan struct{})
	go func() {
		defer close(doneTracking)

		// Prepare a fake session tracker, expiry should automatically be set
		tracker, err := types.NewSessionTracker(types.SessionTrackerSpecV1{
			SessionID: sessID,
			Created:   clock.Now(),
		})
		require.NoError(t, err)
		require.Equal(t, tracker.GetCreated().Add(apidefaults.SessionTrackerTTL), tracker.Expiry())

		err = TrackSession(ctx, mockSessionTrackerService, tracker, ticker, closeCtx.Done())
		require.NoError(t, err)
	}()

	// tracker should be created
	var tracker types.SessionTracker
	trackerCreated := func() bool {
		mockSessionTrackerService.Lock()
		defer mockSessionTrackerService.Unlock()
		tracker = mockSessionTrackerService.trackers[sessID]
		return tracker != nil
	}
	require.Eventually(t, trackerCreated, time.Second, time.Millisecond*100)

	// The session tracker expiration should be extended while ticker is ticking
	expectedExpiry := tracker.Expiry().Add(defaults.SessionTrackerExpirationUpdateInterval)
	clock.Advance(defaults.SessionTrackerExpirationUpdateInterval)

	trackerExpiryUpdated := func() bool {
		mockSessionTrackerService.Lock()
		defer mockSessionTrackerService.Unlock()
		return tracker.Expiry() == expectedExpiry
	}
	require.Eventually(t, trackerExpiryUpdated, time.Second, time.Millisecond*100)

	// Stopping ctx should stop the tracker and update the state to terminated
	cancel()
	select {
	case <-doneTracking:
	case <-time.After(time.Second * 1):
		t.Fatal("timeout waiting for tracking to end")
	}
	require.Equal(t, types.SessionState_SessionStateTerminated, tracker.GetState())
}

type mockSessiontrackerService struct {
	sync.Mutex
	trackers map[string]types.SessionTracker
}

func (m *mockSessiontrackerService) GetActiveSessionTrackers(ctx context.Context) ([]types.SessionTracker, error) {
	return nil, nil
}

func (m *mockSessiontrackerService) GetSessionTracker(ctx context.Context, sessionID string) (types.SessionTracker, error) {
	return nil, nil
}

func (m *mockSessiontrackerService) UpdateSessionTracker(ctx context.Context, req *proto.UpdateSessionTrackerRequest) error {
	switch update := req.Update.(type) {
	case *proto.UpdateSessionTrackerRequest_UpdateExpiry:
		m.Lock()
		defer m.Unlock()
		m.trackers[req.SessionID].SetExpiry(*update.UpdateExpiry.Expires)
	case *proto.UpdateSessionTrackerRequest_UpdateState:
		m.trackers[req.SessionID].SetState(update.UpdateState.State)
	}
	return nil
}

func (m *mockSessiontrackerService) RemoveSessionTracker(ctx context.Context, sessionID string) error {
	return nil
}

func (m *mockSessiontrackerService) UpdatePresence(ctx context.Context, sessionID, user string) error {
	return nil
}

func (m *mockSessiontrackerService) CreateSessionTracker(ctx context.Context, tracker types.SessionTracker) (types.SessionTracker, error) {
	m.Lock()
	defer m.Unlock()
	m.trackers[tracker.GetSessionID()] = tracker
	return tracker, nil
}
