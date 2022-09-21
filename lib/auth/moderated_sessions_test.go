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
	"github.com/gravitational/trace"
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
	require.True(t, trace.IsAccessDenied(err))
	require.Nil(t, tracker)
	require.Contains(t, err.Error(), "Moderated Sessions are only supported in Teleport Enterprise")
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
