// Copyright 2021 Gravitational, Inc
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

package auth

import (
	"context"
	"sort"
	"testing"
	"time"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestAPIServer_activeSessions_whereConditions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tlsServer := newTestTLSServer(t)
	authServer := tlsServer.Auth()

	// - "admin" has permissions to access all active sessions
	// - "alpaca" has permissions to access only their own active sessions
	// Each user is assigned its corresponding role, plus whatever extra
	// permissions are needed to run the scenario.
	const admin = "admin"
	const alpaca = "alpaca"
	alpacaRole := services.RoleForUser(&types.UserV2{Metadata: types.Metadata{Name: alpaca}})
	alpacaRole.SetLogins(types.Allow, []string{alpaca})
	alpacaRole.SetRules(types.Allow, append(alpacaRole.GetRules(types.Allow), types.Rule{
		Resources: []string{"ssh_session"},
		// Allow all ssh_session verbs, deny rule below takes precedence.
		Verbs: []string{"*"},
	}))
	alpacaRole.SetRules(types.Deny, append(alpacaRole.GetRules(types.Deny), types.Rule{
		Resources: []string{"ssh_session"},
		Verbs:     []string{"list", "read", "update", "delete"},
		Where:     "!contains(ssh_session.participants, user.metadata.name)",
	}))
	_, err := CreateUser(authServer, alpaca, alpacaRole)
	require.NoError(t, err)

	// Prepare clients.
	adminClient, err := tlsServer.NewClient(TestAdmin())
	require.NoError(t, err)
	alpacaClient, err := tlsServer.NewClient(TestUser(alpaca))
	require.NoError(t, err)

	// Prepare one session per user.
	createSession := func(clt ClientI, user string) session.ID {
		id := session.NewID()
		now := time.Now()

		// Create initial session.
		require.NoError(t, clt.CreateSession(ctx, session.Session{
			ID:        id,
			Namespace: apidefaults.Namespace,
			TerminalParams: session.TerminalParams{
				W: 100,
				H: 100,
			},
			Login:      user,
			Created:    now,
			LastActive: now,
		}))

		// Add parties, must be done via update.
		// Usually the Node does this, in the test we are taking a shortcut and
		// using admin due to its powerful permissions.
		require.NoError(t, adminClient.UpdateSession(ctx, session.UpdateRequest{
			ID:        id,
			Namespace: apidefaults.Namespace,
			Parties: &[]session.Party{
				{ID: session.NewID(), User: user},
			},
		}))
		return id
	}
	adminSessionID := createSession(adminClient, admin)
	alpacaSessionID := createSession(alpacaClient, alpaca)

	t.Run("GetSessions respects role conditions", func(t *testing.T) {
		tests := []struct {
			name    string
			clt     ClientI
			wantIDs []session.ID
		}{
			{
				name:    admin,
				clt:     adminClient,
				wantIDs: []session.ID{adminSessionID, alpacaSessionID},
			},
			{
				name:    alpaca,
				clt:     alpacaClient,
				wantIDs: []session.ID{alpacaSessionID},
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				sessions, err := test.clt.GetSessions(ctx, apidefaults.Namespace)
				require.NoError(t, err)

				got := make([]session.ID, len(sessions))
				for i, s := range sessions {
					got[i] = s.ID
				}
				want := test.wantIDs
				sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
				sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
				if diff := cmp.Diff(test.wantIDs, got); diff != "" {
					t.Errorf("GetSessions() mismatch (-want +got):\n%s", diff)
				}
			})
		}
	})

	// Helper functions used by test cases below.
	getSession := func(clt ClientI) func(id session.ID) error {
		return func(id session.ID) error {
			_, err := clt.GetSession(ctx, apidefaults.Namespace, id)
			return err
		}
	}
	updateSession := func(clt ClientI) func(id session.ID) error {
		return func(id session.ID) error {
			return clt.UpdateSession(ctx, session.UpdateRequest{
				ID:             id,
				Namespace:      apidefaults.Namespace,
				TerminalParams: &session.TerminalParams{W: 150, H: 150},
			})
		}
	}
	deleteSession := func(clt ClientI) func(id session.ID) error {
		return func(id session.ID) error {
			return clt.UpdateSession(ctx, session.UpdateRequest{
				ID:             id,
				Namespace:      apidefaults.Namespace,
				TerminalParams: &session.TerminalParams{W: 150, H: 150},
			})
		}
	}

	t.Run("users can't interact with denied sessions", func(t *testing.T) {
		clt := alpacaClient
		sessionID := adminSessionID
		tests := []struct {
			name string
			fn   func(id session.ID) error
		}{
			{
				name: "GetSession",
				fn:   getSession(clt),
			},
			{
				name: "UpdateSession",
				fn:   updateSession(clt),
			},
			{
				name: "DeleteSession",
				fn:   deleteSession(clt),
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				err := test.fn(sessionID)
				require.True(t, trace.IsAccessDenied(err), "unexpected err: %v (want access denied)", err)
			})
		}
	})

	t.Run("users can interact with allowed sessions", func(t *testing.T) {
		tests := []struct {
			name      string
			fn        func(session.ID) error
			sessionID session.ID
		}{
			{
				name:      "admin reads own session",
				fn:        getSession(adminClient),
				sessionID: adminSessionID,
			},
			{
				name:      "admin updates own session",
				fn:        updateSession(adminClient),
				sessionID: adminSessionID,
			},
			{
				name:      "admin deletes own session",
				fn:        deleteSession(adminClient),
				sessionID: adminSessionID,
			},
			{
				name:      "admin reads alpaca session",
				fn:        getSession(adminClient),
				sessionID: alpacaSessionID,
			},
			{
				name:      "admin updates alpaca session",
				fn:        updateSession(adminClient),
				sessionID: alpacaSessionID,
			},

			{
				name:      "alpaca reads own session",
				fn:        getSession(alpacaClient),
				sessionID: alpacaSessionID,
			},
			{
				name:      "alpaca updates own session",
				fn:        updateSession(alpacaClient),
				sessionID: alpacaSessionID,
			},
			{
				name:      "alpaca deletes own session",
				fn:        deleteSession(alpacaClient),
				sessionID: alpacaSessionID,
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				require.NoError(t, test.fn(test.sessionID))
			})
		}
	})
}
