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

package srv

import (
	"context"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils"
)

func TestCheckSFTPAllowed(t *testing.T) {
	srv := newMockServer(t)
	ctx := newTestServerContext(t, srv, nil, nil)

	tests := []struct {
		name                 string
		nodeAllowFileCopying bool
		permit               *decisionpb.SSHAccessPermit
		sessionPolicies      []*types.SessionRequirePolicy
		expectedErr          error
	}{
		{
			name:                 "node disallowed",
			nodeAllowFileCopying: false,
			permit: &decisionpb.SSHAccessPermit{
				SshFileCopy: true,
			},
			expectedErr: ErrNodeFileCopyingNotPermitted,
		},
		{
			name:                 "node allowed",
			nodeAllowFileCopying: true,
			permit: &decisionpb.SSHAccessPermit{
				SshFileCopy: true,
			},
			expectedErr: nil,
		},
		{
			name:                 "role disallowed",
			nodeAllowFileCopying: true,
			permit: &decisionpb.SSHAccessPermit{
				SshFileCopy: false,
			},
			expectedErr: errRoleFileCopyingNotPermitted,
		},
		{
			name:                 "role allowed",
			nodeAllowFileCopying: true,
			permit: &decisionpb.SSHAccessPermit{
				SshFileCopy: true,
			},
			expectedErr: nil,
		},
		{
			name:                 "moderated sessions enforced",
			nodeAllowFileCopying: true,
			permit: &decisionpb.SSHAccessPermit{
				SshFileCopy: true,
			},
			sessionPolicies: []*types.SessionRequirePolicy{
				{
					Name:   "test",
					Filter: `contains(user.roles, "auditor")`,
					Kinds:  []string{string(types.SSHSessionKind)},
					Modes:  []string{string(types.SessionModeratorMode)},
					Count:  3,
				},
			},
			expectedErr: errCannotStartUnattendedSession,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx.AllowFileCopying = tt.nodeAllowFileCopying

			sessionJoiningRoles := services.NewRoleSet(&types.RoleV6{
				Kind: types.KindRole,
				Metadata: types.Metadata{
					Name: "test",
				},
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						RequireSessionJoin: tt.sessionPolicies,
					},
				},
			})

			ctx.Identity.UnstableSessionJoiningAccessChecker = services.NewAccessCheckerWithRoleSet(
				&services.AccessInfo{
					Roles: sessionJoiningRoles.RoleNames(),
				},
				"localhost",
				sessionJoiningRoles,
			)

			ctx.Identity.AccessPermit = tt.permit

			err := ctx.CheckSFTPAllowed(nil)
			if tt.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.expectedErr.Error())
			}
		})
	}
}

func TestIdentityContext_GetUserMetadata(t *testing.T) {
	tests := []struct {
		name  string
		idCtx IdentityContext
		want  apievents.UserMetadata
	}{
		{
			name: "user metadata",
			idCtx: IdentityContext{
				TeleportUser:   "alpaca",
				Impersonator:   "llama",
				Login:          "alpaca1",
				ActiveRequests: []string{"access-req1", "access-req2"},
			},
			want: apievents.UserMetadata{
				User:           "alpaca",
				Login:          "alpaca1",
				Impersonator:   "llama",
				AccessRequests: []string{"access-req1", "access-req2"},
				UserKind:       apievents.UserKind_USER_KIND_HUMAN,
			},
		},
		{
			name: "device metadata",
			idCtx: IdentityContext{
				UnmappedIdentity: &sshca.Identity{
					Username:           "alpaca",
					DeviceID:           "deviceid1",
					DeviceAssetTag:     "assettag1",
					DeviceCredentialID: "credentialid1",
				},
				TeleportUser: "alpaca",
				Login:        "alpaca1",
			},
			want: apievents.UserMetadata{
				User:  "alpaca",
				Login: "alpaca1",
				TrustedDevice: &apievents.DeviceMetadata{
					DeviceId:     "deviceid1",
					AssetTag:     "assettag1",
					CredentialId: "credentialid1",
				},
				UserKind: apievents.UserKind_USER_KIND_HUMAN,
			},
		},
		{
			name: "bot metadata",
			idCtx: IdentityContext{
				TeleportUser:  "bot-alpaca",
				Login:         "alpaca1",
				BotName:       "alpaca",
				BotInstanceID: "123-123-123",
			},
			want: apievents.UserMetadata{
				User:          "bot-alpaca",
				Login:         "alpaca1",
				UserKind:      apievents.UserKind_USER_KIND_BOT,
				BotName:       "alpaca",
				BotInstanceID: "123-123-123",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.idCtx.GetUserMetadata()
			want := test.want
			if !proto.Equal(&got, &want) {
				t.Errorf("GetUserMetadata mismatch (-want +got):\n%s", cmp.Diff(want, got))
			}
		})
	}
}

func TestSSHAccessLockTargets(t *testing.T) {
	t.Run("all locks", func(t *testing.T) {
		const clusterName = "mycluster"
		const serverID = "myserver"
		const mfaDevice = "my-mfa-device-1"
		const trustedDevice = "my-trusted-device-1"
		const osLogin = "camel"
		const username = "llama"
		mappedRoles := []string{"access", "editor"}
		unmappedRoles := []string{"unmapped-role-1", "unmapped-role-2", "access"}
		accessRequests := []string{"access-request-1", "access-request-2"}

		unmappedIdentity := &sshca.Identity{
			Username:       username,
			MFAVerified:    mfaDevice,
			DeviceID:       trustedDevice,
			Roles:          unmappedRoles,
			ActiveRequests: accessRequests,
		}

		accessInfo := &services.AccessInfo{
			Username: username,
			Roles:    mappedRoles,
		}

		got := services.SSHAccessLockTargets(clusterName, serverID, osLogin, accessInfo, unmappedIdentity)
		want := []types.LockTarget{
			{User: username},
			{ServerID: serverID},
			{ServerID: serverID + "." + clusterName},
			{MFADevice: mfaDevice},
			{Device: trustedDevice},
		}
		for _, role := range mappedRoles {
			want = append(want, types.LockTarget{Role: role})
		}
		for _, role := range unmappedRoles[:len(unmappedRoles)-1] /* skip duplicate role */ {
			want = append(want, types.LockTarget{Role: role})
		}
		for _, request := range accessRequests {
			want = append(want, types.LockTarget{AccessRequest: request})
		}
		want = append(want, types.LockTarget{Login: osLogin})
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("SSHAccessLockTargets mismatch (-want +got)\n%s", diff)
		}
	})
}

func TestCreateOrJoinSession(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := newMockServer(t)
	registry, err := NewSessionRegistry(SessionRegistryConfig{
		clock:                 srv.clock,
		Srv:                   srv,
		SessionTrackerService: srv.auth,
	})
	require.NoError(t, err)

	runningSessionID := rsession.NewID()
	sess, _, err := newSession(ctx, runningSessionID, registry, newTestServerContext(t, srv, nil, nil), newMockSSHChannel(), sessionTypeInteractive)
	require.NoError(t, err)

	t.Cleanup(sess.Stop)

	registry.sessions[runningSessionID] = sess

	tests := []struct {
		name              string
		sessionID         string
		expectedErr       bool
		wantSameSessionID bool
	}{
		{
			name: "no session ID",
		},
		{
			name:              "new session ID",
			sessionID:         string(rsession.NewID()),
			wantSameSessionID: false,
		},
		{
			name:              "existing session ID",
			sessionID:         runningSessionID.String(),
			wantSameSessionID: true,
		},
		{
			name:              "existing session ID in Windows format",
			sessionID:         "{" + runningSessionID.String() + "}",
			wantSameSessionID: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			parsedSessionID := new(rsession.ID)
			var err error
			if tt.sessionID != "" {
				parsedSessionID, err = rsession.ParseID(tt.sessionID)
				require.NoError(t, err)
			}

			scx := newTestServerContext(t, srv, nil, nil)
			if tt.sessionID != "" {
				scx.SetEnv(sshutils.SessionEnvVar, tt.sessionID)
			}

			err = scx.CreateOrJoinSession(ctx, registry)
			if tt.expectedErr {
				require.True(t, trace.IsNotFound(err))
			} else {
				require.NoError(t, err)
			}

			sessID := scx.GetSessionID()
			require.False(t, sessID.IsZero())
			if tt.wantSameSessionID {
				require.Equal(t, parsedSessionID.String(), sessID.String())
				require.Equal(t, *parsedSessionID, scx.GetSessionID())
			} else {
				require.NotEqual(t, parsedSessionID.String(), sessID.String())
				require.NotEqual(t, *parsedSessionID, scx.GetSessionID())
			}
		})
	}
}
