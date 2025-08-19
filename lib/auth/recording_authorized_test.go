/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package auth_test

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestSessionRecordingAuthorized_RBAC(t *testing.T) {
	t.Parallel()

	srv := newTestTLSServerForRecording(t)
	authServer := srv.AuthServer.AuthServer

	session1 := createTestSession(t, authServer, "alice", []string{"alice", "bob"})
	session2 := createTestSession(t, authServer, "bob", []string{"bob", "charlie"})

	testCases := []struct {
		name     string
		username string
		session1 bool // should have access to session1
		session2 bool // should have access to session2
	}{
		{
			name:     "alice has access to session1 only",
			username: "alice",
			session1: true,
			session2: false,
		},
		{
			name:     "bob has access to both sessions",
			username: "bob",
			session1: true,
			session2: true,
		},
		{
			name:     "charlie has access to session2 only",
			username: "charlie",
			session1: false,
			session2: true,
		},
		{
			name:     "dan has no access to any sessions",
			username: "dan",
			session1: false,
			session2: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, role, err := authtest.CreateUserAndRole(
				srv.Auth(),
				tc.username,
				[]string{},
				[]types.Rule{
					{
						Resources: []string{types.KindSession},
						Verbs:     []string{types.VerbRead},
						Where:     "contains(session.participants, user.metadata.name)",
					},
				},
			)
			require.NoError(t, err)

			localUser := authz.LocalUser{
				Username: tc.username,
				Identity: tlsca.Identity{
					Username: tc.username,
					Groups:   []string{role.GetName()},
				},
			}
			authContext, err := authz.ContextForLocalUser(t.Context(), localUser, srv.Auth(), srv.ClusterName(), true /* disableDeviceAuthz */)
			require.NoError(t, err)

			authorizer := &mockAuthorizer{
				ctx: authContext,
			}
			sessionAuth := auth.NewSessionRecordingAuthorizer(authServer, authorizer)

			err = sessionAuth.Authorize(t.Context(), string(session1))
			if tc.session1 {
				require.NoError(t, err, "expected access to session1")
			} else {
				require.Error(t, err, "expected no access to session1")
				assert.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
			}

			err = sessionAuth.Authorize(t.Context(), string(session2))
			if tc.session2 {
				require.NoError(t, err, "expected access to session2")
			} else {
				require.Error(t, err, "expected no access to session2")
				assert.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
			}
		})
	}
}

func newTestTLSServerForRecording(t testing.TB) *authtest.TLSServer {
	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)

	srv, err := as.NewTestTLSServer(func(cfg *authtest.TLSServerConfig) {
		require.NoError(t, err)
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		err := srv.Close()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		require.NoError(t, err)
	})

	return srv
}

func createTestSession(t *testing.T, authServer *auth.Server, user string, participants []string) session.ID {
	id := session.NewID()

	startTime := time.Date(2020, 3, 30, 15, 58, 54, 561*int(time.Millisecond), time.UTC)
	endTime := startTime.Add(time.Minute)
	endEvent := &apievents.SessionEnd{
		Metadata: apievents.Metadata{
			Index: 20,
			Type:  events.SessionEndEvent,
			ID:    string(id),
			Code:  events.SessionEndCode,
			Time:  endTime,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion:   teleport.Version,
			ServerID:        "6a7c593d-345a-431f-9e21-4049be982fa5",
			ServerNamespace: "default",
			ServerLabels:    map[string]string{"env": "prod"},
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: string(id),
		},
		UserMetadata: apievents.UserMetadata{
			User: user,
		},
		EnhancedRecording: true,
		Interactive:       true,
		Participants:      participants,
		StartTime:         startTime,
		EndTime:           endTime,
	}

	err := authServer.EmitAuditEvent(t.Context(), endEvent)
	require.NoError(t, err)

	return id
}
