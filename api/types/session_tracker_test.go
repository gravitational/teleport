// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestSessionTrackerV1_UpdatePresence(t *testing.T) {
	clock := clockwork.NewFakeClock()
	now := clock.Now().UTC()

	s, err := NewSessionTracker(SessionTrackerSpecV1{
		SessionID: "123",
		Participants: []Participant{
			{
				ID:         "1",
				User:       "llama",
				Cluster:    "teleport-local",
				Mode:       string(SessionPeerMode),
				LastActive: now,
			},
			{
				ID:         "2",
				User:       "fish",
				Cluster:    "teleport-remote",
				Mode:       string(SessionModeratorMode),
				LastActive: now,
			},
			{
				ID:         "3",
				User:       "cat",
				Mode:       string(SessionModeratorMode),
				LastActive: now,
			},
		},
	})
	require.NoError(t, err)

	// Presence cannot be updated for a non-existent user
	err = s.UpdatePresence("alpaca", "", now.Add(time.Hour))
	require.ErrorIs(t, err, trace.NotFound("participant alpaca not found"))

	// Update presence for just the user fish
	require.NoError(t, s.UpdatePresence("fish", "teleport-remote", now.Add(time.Hour)))
	// Try to Update presence for user fish again, but with a different cluster.
	require.Error(t, s.UpdatePresence("fish", "teleport-local", now.Add(time.Hour)))

	// Verify that llama has not been active but that fish was
	for _, participant := range s.GetParticipants() {
		lastActive := now
		if participant.User == "fish" {
			lastActive = lastActive.Add(time.Hour)
		}

		assert.Equal(t, lastActive, participant.LastActive)
	}
}

func TestGetAccessRequestIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ids      []string
		expected []string
	}{
		{
			name:     "returns nil when no access request IDs are set",
			ids:      nil,
			expected: nil,
		},
		{
			name:     "returns the correct access request IDs",
			ids:      []string{"req-1", "req-2", "req-3"},
			expected: []string{"req-1", "req-2", "req-3"},
		},
		{
			name:     "returns single access request ID",
			ids:      []string{"req-only"},
			expected: []string{"req-only"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, err := NewSessionTracker(SessionTrackerSpecV1{
				SessionID:        "session-1",
				AccessRequestIDs: tt.ids,
			})
			require.NoError(t, err)

			require.Equal(t, tt.expected, s.GetAccessRequestIDs())
		})
	}
}

func TestClone(t *testing.T) {
	t.Parallel()

	newTracker := func(t *testing.T) SessionTracker {
		t.Helper()
		s, err := NewSessionTracker(SessionTrackerSpecV1{
			SessionID:        "session-1",
			Kind:             string(SSHSessionKind),
			Hostname:         "node-01",
			AccessRequestIDs: []string{"req-1", "req-2"},
			Participants: []Participant{
				{
					ID:   "p1",
					User: "alice",
					Mode: string(SessionPeerMode),
				},
			},
		})
		require.NoError(t, err)
		return s
	}

	t.Run("returns non-nil", func(t *testing.T) {
		t.Parallel()
		s := newTracker(t)
		require.NotNil(t, s.Clone())
	})

	t.Run("produces an equal-value copy", func(t *testing.T) {
		t.Parallel()
		s := newTracker(t)
		cloned := s.Clone()

		original, ok := s.(*SessionTrackerV1)
		require.True(t, ok)
		copy, ok := cloned.(*SessionTrackerV1)
		require.True(t, ok)
		require.Empty(t, cmp.Diff(original, copy, protocmp.Transform()))
	})

	t.Run("mutating the clone does not affect the original", func(t *testing.T) {
		t.Parallel()
		s := newTracker(t)
		cloned := s.Clone()

		cloned.AddParticipant(Participant{
			ID:   "p2",
			User: "bob",
			Mode: string(SessionObserverMode),
		})

		require.Len(t, s.GetParticipants(), 1, "original should still have 1 participant")
		require.Len(t, cloned.GetParticipants(), 2, "clone should have 2 participants")
	})
}
