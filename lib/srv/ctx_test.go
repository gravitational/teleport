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
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshca"
)

func TestCheckSFTPAllowed(t *testing.T) {
	srv := newMockServer(t)
	ctx := newTestServerContext(t, srv, nil, nil)

	tests := []struct {
		name                 string
		nodeAllowFileCopying bool
		permit               *decisionpb.SSHAccessPermit
		proxyingPermit       *proxyingPermit
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
			name:                 "proxying role disallowed",
			nodeAllowFileCopying: true,
			proxyingPermit: &proxyingPermit{
				SSHFileCopy: false,
			},
			expectedErr: errRoleFileCopyingNotPermitted,
		},
		{
			name:                 "proxying role allowed",
			nodeAllowFileCopying: true,
			proxyingPermit: &proxyingPermit{
				SSHFileCopy: true,
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
			ctx.Identity.ProxyingPermit = tt.proxyingPermit

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
				MappedRoles:    []string{"role1", "role2"},
				Traits: wrappers.Traits{
					"trait1": []string{"value1", "value2"},
					"trait2": []string{"value3"},
				},
			},
			want: apievents.UserMetadata{
				User:           "alpaca",
				Login:          "alpaca1",
				Impersonator:   "llama",
				AccessRequests: []string{"access-req1", "access-req2"},
				UserKind:       apievents.UserKind_USER_KIND_HUMAN,
				UserRoles:      []string{"role1", "role2"},
				UserTraits: wrappers.Traits{
					"trait1": []string{"value1", "value2"},
					"trait2": []string{"value3"},
				},
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
