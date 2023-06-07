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

package local

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestSessionTrackerStorage tests backend operations with tracker resources.
func TestSessionTrackerStorage(t *testing.T) {
	ctx := context.Background()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	sid := uuid.New().String()
	srv, err := NewSessionTrackerService(bk)
	require.NoError(t, err)

	tracker, err := types.NewSessionTracker(types.SessionTrackerSpecV1{
		SessionID:   sid,
		Kind:        types.KindSSHSession,
		Hostname:    "hostname",
		ClusterName: "cluster",
		Login:       "root",
		Participants: []types.Participant{
			{
				ID:   uuid.New().String(),
				User: "eve",
				Mode: string(types.SessionPeerMode),
			},
		},
		Expires: time.Now().UTC().Add(24 * time.Hour),
	})
	require.NoError(t, err)

	_, err = srv.CreateSessionTracker(ctx, tracker)
	require.NoError(t, err)

	bobID := uuid.New().String()

	req := &proto.UpdateSessionTrackerRequest{
		SessionID: sid,
		Update: &proto.UpdateSessionTrackerRequest_AddParticipant{
			AddParticipant: &proto.SessionTrackerAddParticipant{
				Participant: &types.Participant{
					ID:   bobID,
					User: "bob",
					Mode: string(types.SessionObserverMode),
				},
			},
		},
	}

	err = srv.UpdateSessionTracker(ctx, req)
	require.NoError(t, err)

	req = &proto.UpdateSessionTrackerRequest{
		SessionID: sid,
		Update: &proto.UpdateSessionTrackerRequest_RemoveParticipant{
			RemoveParticipant: &proto.SessionTrackerRemoveParticipant{
				ParticipantID: bobID,
			},
		},
	}

	err = srv.UpdateSessionTracker(ctx, req)
	require.NoError(t, err)

	sessions, err := srv.GetActiveSessionTrackers(ctx)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	tracker = sessions[0]
	require.Len(t, tracker.GetParticipants(), 1)

	sessions, err = srv.GetActiveSessionTrackersWithFilter(ctx, &types.SessionTrackerFilter{
		Kind: types.KindWindowsDesktop,
	})
	require.NoError(t, err)
	require.Len(t, sessions, 0)
	sessions, err = srv.GetActiveSessionTrackersWithFilter(ctx, &types.SessionTrackerFilter{
		Kind: types.KindSSHSession,
	})
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	tracker = sessions[0]
	require.Len(t, tracker.GetParticipants(), 1)

	err = srv.RemoveSessionTracker(ctx, sid)
	require.NoError(t, err)

	tracker, err = srv.GetSessionTracker(ctx, sid)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))
	require.Nil(t, tracker)
}

func TestSessionTrackerImplicitExpiry(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)

	id := uuid.New().String()
	id2 := uuid.New().String()
	srv, err := NewSessionTrackerService(bk)
	require.NoError(t, err)

	tracker1, err := types.NewSessionTracker(types.SessionTrackerSpecV1{
		SessionID:   id,
		Kind:        types.KindSSHSession,
		Hostname:    "hostname",
		ClusterName: "cluster",
		Login:       "foo",
		Participants: []types.Participant{
			{
				ID:   uuid.New().String(),
				User: "eve",
				Mode: string(types.SessionPeerMode),
			},
		},
		Expires: clock.Now().UTC().Add(time.Second),
	})
	require.NoError(t, err)

	_, err = srv.CreateSessionTracker(ctx, tracker1)
	require.NoError(t, err)

	tracker2, err := types.NewSessionTracker(types.SessionTrackerSpecV1{
		SessionID:   id2,
		Kind:        types.KindSSHSession,
		Hostname:    "hostname",
		ClusterName: "cluster",
		Login:       "foo",
		Participants: []types.Participant{
			{
				ID:   uuid.New().String(),
				User: "eve",
				Mode: string(types.SessionPeerMode),
			},
		},
		Expires: clock.Now().UTC().Add(24 * time.Hour),
	})
	require.NoError(t, err)

	_, err = srv.CreateSessionTracker(ctx, tracker2)
	require.NoError(t, err)

	// Advance the clock to expire tracker1
	clock.Advance(time.Second)

	sessions, err := srv.GetActiveSessionTrackers(ctx)
	require.NoError(t, err)

	// Verify that we only get one session and that it's `id2` since we expect that
	// `id` is filtered out due to its expiry.
	require.Len(t, sessions, 1)

	require.Equal(t, sessions[0].GetSessionID(), id2)
}

// TestSessionTrackerTermination ensures that Session Trackers which transition to terminated
// have their TTL lowered to terminatedTTL instead of lingering around for an entire hour
// after the session is completed.
func TestSessionTrackerTermination(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	// Setup the backend and service
	bk, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)

	srv, err := NewSessionTrackerService(bk)
	require.NoError(t, err)

	// Create our tracker with an expiry 1h from now
	id := uuid.New().String()
	tracker, err := types.NewSessionTracker(types.SessionTrackerSpecV1{
		SessionID:   id,
		Kind:        types.KindSSHSession,
		Hostname:    "hostname",
		ClusterName: "cluster",
		Login:       "foo",
		Participants: []types.Participant{
			{
				ID:   uuid.New().String(),
				User: "eve",
				Mode: string(types.SessionPeerMode),
			},
		},
		Expires: clock.Now().UTC().Add(time.Hour),
	})
	require.NoError(t, err)

	_, err = srv.CreateSessionTracker(ctx, tracker)
	require.NoError(t, err)

	// Validate the tracker exists
	sessions, err := srv.GetActiveSessionTrackers(ctx)
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	// Set the tracker to terminated
	require.NoError(t, srv.UpdateSessionTracker(ctx, &proto.UpdateSessionTrackerRequest{
		SessionID: id,
		Update: &proto.UpdateSessionTrackerRequest_UpdateState{UpdateState: &proto.SessionTrackerUpdateState{
			State: types.SessionState_SessionStateTerminated,
		}},
	}))

	// Try to change the state to running, this should fail because the tracker
	// is now in a terminal state
	err = srv.UpdateSessionTracker(ctx, &proto.UpdateSessionTrackerRequest{
		SessionID: id,
		Update: &proto.UpdateSessionTrackerRequest_UpdateState{UpdateState: &proto.SessionTrackerUpdateState{
			State: types.SessionState_SessionStateRunning,
		}},
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))

	// Validate that the tracker still exists
	sessions, err = srv.GetActiveSessionTrackers(ctx)
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	// Advance the clock to now + terminatedTTL to validate
	// that the tracker was removed prior to the original hour expiry
	clock.Advance(terminatedTTL)

	// Validate that no trackers exist
	sessions, err = srv.GetActiveSessionTrackers(ctx)
	require.NoError(t, err)
	require.Empty(t, sessions)
}
