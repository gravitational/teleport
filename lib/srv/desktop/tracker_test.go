/*
Copyright 2021 Gravitational, Inc.

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

package desktop

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestSessionTracker(t *testing.T) {
	ctx := context.Background()
	log := logrus.New()
	log.SetOutput(io.Discard)
	clock := clockwork.NewFakeClockAt(time.Now())

	mockAuthClient := &mockSessiontrackerService{
		clock:    clock,
		trackers: make(map[string]types.SessionTracker),
	}

	s := &WindowsService{
		closeCtx:    ctx,
		clusterName: "test-cluster",
		cfg: WindowsServiceConfig{
			Log: log,
			Heartbeat: HeartbeatConfig{
				HostUUID: "test-host-id",
			},
			Clock:      clock,
			AuthClient: mockAuthClient,
		},
	}

	id := &tlsca.Identity{
		Username:     "foo",
		Impersonator: "bar",
		MFAVerified:  "mfa-id",
		ClientIP:     "127.0.0.1",
	}

	desktop := &types.WindowsDesktopV3{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name:   "test-desktop",
				Labels: map[string]string{"env": "production"},
			},
		},
		Spec: types.WindowsDesktopSpecV3{
			Addr:   "192.168.100.12",
			Domain: "test.example.com",
		},
	}

	userMeta := id.GetUserMetadata()
	userMeta.Login = "Administrator"

	cancelCtx, cancel := context.WithCancel(ctx)
	err := s.trackSession(cancelCtx, id, "Administrator", "sessionID", desktop)
	require.NoError(t, err)

	// Tracker should be created
	tracker, ok := mockAuthClient.trackers["sessionID"]
	require.True(t, ok)
	require.Equal(t, types.SessionState_SessionStateRunning, tracker.GetState())

	// The session tracker expiration should be extended while the session is active
	clock.BlockUntil(1)
	expectedExpiry := tracker.Expiry().Add(defaults.SessionTrackerExpirationUpdateInterval)
	clock.Advance(defaults.SessionTrackerExpirationUpdateInterval)

	trackerExpiryUpdated := func() bool {
		return tracker.Expiry() == expectedExpiry
	}
	require.Eventually(t, trackerExpiryUpdated, time.Second*5, time.Second)

	// Closing ctx should trigger session tracker state to be terminated.
	cancel()
	trackerTerminated := func() bool {
		return tracker.GetState() == types.SessionState_SessionStateTerminated
	}
	require.Eventually(t, trackerTerminated, time.Second*5, time.Second)
}

type mockSessiontrackerService struct {
	auth.ClientI
	clock    clockwork.Clock
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

func (m *mockSessiontrackerService) UpsertSessionTracker(ctx context.Context, tracker types.SessionTracker) error {
	tracker.SetExpiry(m.clock.Now().Add(defaults.SessionTrackerTTL))
	m.trackers[tracker.GetSessionID()] = tracker
	return nil
}
