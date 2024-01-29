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
	"bytes"
	"os/user"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/services"
)

// TestDecodeChildError ensures that child error message marshaling
// and unmarshaling returns the original values.
func TestDecodeChildError(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, DecodeChildError(&buf))

	targetErr := trace.NotFound(user.UnknownUserError("test").Error())

	writeChildError(&buf, targetErr)

	require.ErrorIs(t, DecodeChildError(&buf), targetErr)
}

func TestCheckSFTPAllowed(t *testing.T) {
	srv := newMockServer(t)
	ctx := newTestServerContext(t, srv, nil)

	tests := []struct {
		name                 string
		nodeAllowFileCopying bool
		roles                []types.Role
		expectedErr          error
	}{
		{
			name:                 "node disallowed",
			nodeAllowFileCopying: false,
			roles: []types.Role{
				&types.RoleV6{
					Kind: types.KindNode,
				},
			},
			expectedErr: ErrNodeFileCopyingNotPermitted,
		},
		{
			name:                 "node allowed",
			nodeAllowFileCopying: true,
			roles: []types.Role{
				&types.RoleV6{
					Kind: types.KindNode,
				},
			},
			expectedErr: nil,
		},
		{
			name:                 "role disallowed",
			nodeAllowFileCopying: true,
			roles: []types.Role{
				&types.RoleV6{
					Kind: types.KindNode,
					Spec: types.RoleSpecV6{
						Options: types.RoleOptions{
							SSHFileCopy: types.NewBoolOption(false),
						},
					},
				},
			},
			expectedErr: errRoleFileCopyingNotPermitted,
		},
		{
			name:                 "role allowed",
			nodeAllowFileCopying: true,
			roles: []types.Role{
				&types.RoleV6{
					Kind: types.KindNode,
					Spec: types.RoleSpecV6{
						Options: types.RoleOptions{
							SSHFileCopy: types.NewBoolOption(true),
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name:                 "conflicting roles",
			nodeAllowFileCopying: true,
			roles: []types.Role{
				&types.RoleV6{
					Kind: types.KindNode,
					Spec: types.RoleSpecV6{
						Options: types.RoleOptions{
							SSHFileCopy: types.NewBoolOption(true),
						},
					},
				},
				&types.RoleV6{
					Kind: types.KindNode,
					Spec: types.RoleSpecV6{
						Options: types.RoleOptions{
							SSHFileCopy: types.NewBoolOption(false),
						},
					},
				},
			},
			expectedErr: errRoleFileCopyingNotPermitted,
		},
		{
			name:                 "moderated sessions enforced",
			nodeAllowFileCopying: true,
			roles: []types.Role{
				&types.RoleV6{
					Kind: types.KindNode,
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							RequireSessionJoin: []*types.SessionRequirePolicy{
								{
									Name:   "test",
									Filter: `contains(user.roles, "auditor")`,
									Kinds:  []string{string(types.SSHSessionKind)},
									Modes:  []string{string(types.SessionModeratorMode)},
									Count:  3,
								},
							},
						},
					},
				},
			},
			expectedErr: errCannotStartUnattendedSession,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx.AllowFileCopying = tt.nodeAllowFileCopying

			roles := services.NewRoleSet(tt.roles...)

			ctx.Identity.AccessChecker = services.NewAccessCheckerWithRoleSet(
				&services.AccessInfo{
					Roles: roles.RoleNames(),
				},
				"localhost",
				roles,
			)

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
				TeleportUser: "alpaca",
				Login:        "alpaca1",
				Certificate: &ssh.Certificate{
					Permissions: ssh.Permissions{
						Extensions: map[string]string{
							teleport.CertExtensionDeviceID:           "deviceid1",
							teleport.CertExtensionDeviceAssetTag:     "assettag1",
							teleport.CertExtensionDeviceCredentialID: "credentialid1",
						},
					},
				},
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

func TestComputeLockTargets(t *testing.T) {
	t.Run("all locks", func(t *testing.T) {
		const clusterName = "mycluster"
		const serverID = "myserver"
		const mfaDevice = "my-mfa-device-1"
		const trustedDevice = "my-trusted-device-1"
		mappedRoles := []string{"access", "editor"}
		unmappedRoles := []string{"unmapped-role-1", "unmapped-role-2", "access"}
		accessRequests := []string{"access-request-1", "access-request-2"}

		identityCtx := IdentityContext{
			TeleportUser: "llama",
			Impersonator: "alpaca",
			Login:        "camel",
			Certificate: &ssh.Certificate{
				Permissions: ssh.Permissions{
					Extensions: map[string]string{
						teleport.CertExtensionMFAVerified: mfaDevice,
						teleport.CertExtensionDeviceID:    trustedDevice,
					},
				},
			},
			AccessChecker: &fixedRolesChecker{
				roleNames: mappedRoles,
			},
			UnmappedRoles:  unmappedRoles,
			ActiveRequests: accessRequests,
		}

		got := ComputeLockTargets(clusterName, serverID, identityCtx)
		want := []types.LockTarget{
			{User: identityCtx.TeleportUser},
			{Login: identityCtx.Login},
			{Node: serverID, ServerID: serverID},
			{Node: serverID + "." + clusterName, ServerID: serverID + "." + clusterName},
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
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("ComputeLockTargets mismatch (-want +got)\n%s", diff)
		}
	})
}

type fixedRolesChecker struct {
	services.AccessChecker
	roleNames []string
}

func (c *fixedRolesChecker) RoleNames() []string {
	return c.roleNames
}
