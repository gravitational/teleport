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
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/stretchr/testify/require"
)

// TestSessionTrackerStorage tests backend operations with tracker resources.
func TestSessionTrackerStorage(t *testing.T) {
	ctx := context.Background()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	id := uuid.New().String()
	srv, err := NewSessionTrackerService(bk)
	require.NoError(t, err)

	session, err := srv.CreateSessionTracker(ctx, &proto.CreateSessionTrackerRequest{
		Namespace:   defaults.Namespace,
		ID:          id,
		Type:        types.KindSSHSession,
		Hostname:    "hostname",
		ClusterName: "cluster",
		Login:       "root",
		Initiator: &types.Participant{
			ID:   uuid.New().String(),
			User: "eve",
			Mode: string(types.SessionPeerMode),
		},
		Expires: time.Now().UTC().Add(24 * time.Hour),
	})
	require.NoError(t, err)

	bobID := uuid.New().String()
	err = srv.UpdateSessionTracker(ctx, &proto.UpdateSessionTrackerRequest{
		SessionID: id,
		Update: &proto.UpdateSessionTrackerRequest_AddParticipant{
			AddParticipant: &proto.SessionTrackerAddParticipant{
				Participant: &types.Participant{
					ID:   bobID,
					User: "bob",
					Mode: string(types.SessionObserverMode),
				},
			},
		},
	})
	require.NoError(t, err)

	err = srv.UpdateSessionTracker(ctx, &proto.UpdateSessionTrackerRequest{
		SessionID: id,
		Update: &proto.UpdateSessionTrackerRequest_RemoveParticipant{
			RemoveParticipant: &proto.SessionTrackerRemoveParticipant{
				ParticipantID: bobID,
			},
		},
	})
	require.NoError(t, err)

	sessions, err := srv.GetActiveSessionTrackers(ctx)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.Len(t, session.GetParticipants(), 1)

	err = srv.RemoveSessionTracker(ctx, session.GetSessionID())
	require.NoError(t, err)

	session, err = srv.GetSessionTracker(ctx, session.GetSessionID())
	require.Error(t, err)
	require.Nil(t, session)
}

func TestSessionTrackerImplicitExpiry(t *testing.T) {
	ctx := context.Background()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	id := uuid.New().String()
	id2 := uuid.New().String()
	srv, err := NewSessionTrackerService(bk)
	require.NoError(t, err)

	req1 := proto.CreateSessionTrackerRequest{
		Namespace:   defaults.Namespace,
		ID:          id,
		Type:        types.KindSSHSession,
		Hostname:    "hostname",
		ClusterName: "cluster",
		Login:       "foo",
		Initiator: &types.Participant{
			ID:   uuid.New().String(),
			User: "eve",
			Mode: string(types.SessionPeerMode),
		},
		Expires: time.Now().UTC().Add(time.Second),
	}

	req2 := req1
	req2.ID = id2
	req2.Expires = time.Now().UTC().Add(24 * time.Hour)

	_, err = srv.CreateSessionTracker(ctx, &req1)
	require.NoError(t, err)

	_, err = srv.CreateSessionTracker(ctx, &req2)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		sessions, err := srv.GetActiveSessionTrackers(ctx)
		require.NoError(t, err)
		return len(sessions) == 1 && sessions[0].GetSessionID() == id2
	}, time.Minute, time.Second)
}
