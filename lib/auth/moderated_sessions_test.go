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

package auth

import (
	"context"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services/local"
)

// TestUnmoderatedSessionsAllowed tests that we allow creating unmoderated sessions even if the
// license does not support the moderated sessions feature.
func TestUnmoderatedSessionsAllowed(t *testing.T) {
	// Use OSS License (which doesn't support moderated sessions).
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildOSS})

	srv := &Server{
		clock:    clockwork.NewRealClock(),
		Services: &Services{},
	}

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	srv.Services.SessionTrackerService, err = local.NewSessionTrackerService(bk)
	require.NoError(t, err)

	tracker, err := types.NewSessionTracker(types.SessionTrackerSpecV1{
		SessionID: "foo",
	})
	require.NoError(t, err)
	tracker.AddParticipant(types.Participant{})

	_, err = srv.CreateSessionTracker(context.Background(), tracker)
	require.NoError(t, err)
	require.NotNil(t, tracker)
}

// TestModeratedSessionsDisabled makes sure moderated sessions are disabled when the license does not support it.
// Since moderated sessions require trackers, we mediate this in the tracker creation function.
func TestModeratedSessionsDisabled(t *testing.T) {
	// Use OSS License (which doesn't support moderated sessions).
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildOSS})

	srv := &Server{
		clock:    clockwork.NewRealClock(),
		Services: &Services{},
	}

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	srv.Services.SessionTrackerService, err = local.NewSessionTrackerService(bk)
	require.NoError(t, err)

	tracker, err := types.NewSessionTracker(types.SessionTrackerSpecV1{
		SessionID: "foo",
		HostPolicies: []*types.SessionTrackerPolicySet{
			{
				Name:    "foo",
				Version: "5",
				RequireSessionJoin: []*types.SessionRequirePolicy{
					{
						Name: "foo",
					},
				},
			},
		},
	})
	require.NoError(t, err)
	tracker.AddParticipant(types.Participant{})

	tracker, err = srv.CreateSessionTracker(context.Background(), tracker)
	require.Error(t, err)
	require.Nil(t, tracker)
	require.ErrorIs(t, err, ErrRequiresEnterprise)
}

// TestModeratedSessionsEnabled verifies that we can create session trackers with moderation
// requirements when the license supports it.
func TestModeratedSesssionsEnabled(t *testing.T) {
	// Use Enterprise License (which supports moderated sessions).
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})

	srv := &Server{
		clock:    clockwork.NewRealClock(),
		Services: &Services{},
	}

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	srv.Services.SessionTrackerService, err = local.NewSessionTrackerService(bk)
	require.NoError(t, err)

	tracker, err := types.NewSessionTracker(types.SessionTrackerSpecV1{
		SessionID: "foo",
		HostPolicies: []*types.SessionTrackerPolicySet{
			{
				Name:    "foo",
				Version: "5",
				RequireSessionJoin: []*types.SessionRequirePolicy{
					{
						Name: "foo",
					},
				},
			},
		},
	})
	require.NoError(t, err)
	tracker.AddParticipant(types.Participant{})

	_, err = srv.CreateSessionTracker(context.Background(), tracker)
	require.NoError(t, err)
	require.NotNil(t, tracker)
}
