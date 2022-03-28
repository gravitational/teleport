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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/stretchr/testify/require"
)

// TestUnmoderatedSessionsAllowed tests that we allow creating unmoderated sessions even if the
// moderated sessions feature is disabled via modules.
func TestUnmoderatedSessionsAllowed(t *testing.T) {
	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetTestModules(t, &modules.TestModules{TestFeatures: modules.Features{
		ModeratedSessions: false, // Explicily turn off moderated sessions.
	}})

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	srv, err := local.NewSessionTrackerService(bk)
	require.NoError(t, err)

	tracker, err := srv.CreateSessionTracker(context.Background(), &proto.CreateSessionTrackerRequest{
		ID:        "foo",
		Initiator: &types.Participant{},
	})

	require.NoError(t, err)
	require.NotNil(t, tracker)
}

// TestModeratedSessionsDisabled makes sure moderated sessions can be disabled via modules.
// Since moderated sessions require trackers, we mediate this in the tracker creation function.
func TestModeratedSessionsDisabled(t *testing.T) {
	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetTestModules(t, &modules.TestModules{TestFeatures: modules.Features{
		ModeratedSessions: false, // Explicily turn off moderated sessions.
	}})

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	srv, err := local.NewSessionTrackerService(bk)
	require.NoError(t, err)

	tracker, err := srv.CreateSessionTracker(context.Background(), &proto.CreateSessionTrackerRequest{
		ID:        "foo",
		Initiator: &types.Participant{},
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

	require.Error(t, err)
	require.Nil(t, tracker)
	require.Contains(t, err.Error(), "this Teleport cluster is not licensed for moderated sessions, please contact the cluster administrator")
}

// TestModeratedSessionsEnabled verifies that we can create session trackers with moderation
// requirements when the feature is enabled.
func TestModeratedSesssionsEnabled(t *testing.T) {
	defaultModules := modules.GetModules()
	defer modules.SetModules(defaultModules)
	modules.SetTestModules(t, &modules.TestModules{TestFeatures: modules.Features{
		ModeratedSessions: true,
	}})

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	srv, err := local.NewSessionTrackerService(bk)
	require.NoError(t, err)

	tracker, err := srv.CreateSessionTracker(context.Background(), &proto.CreateSessionTrackerRequest{
		ID:        "foo",
		Initiator: &types.Participant{},
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
	require.NotNil(t, tracker)
}
