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

package services_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestLockTargetsFromTLSIdentity(t *testing.T) {
	t.Run("all locks", func(t *testing.T) {
		identity := tlsca.Identity{
			Username:       "llama",
			Groups:         []string{"access", "editor"},
			MFAVerified:    "mfa-device-id",
			ActiveRequests: []string{"access-request-1", "access-request-2"},
			DeviceExtensions: tlsca.DeviceExtensions{
				DeviceID: "trusted-device-id",
			},
			JoinToken:     "example",
			BotInstanceID: "a-b-c-d",
		}

		// Test.
		got := make(map[types.LockTarget]struct{})
		for lockTarget := range services.LockTargetsFromTLSIdentity(identity) {
			got[lockTarget] = struct{}{}
		}

		want := map[types.LockTarget]struct{}{
			{User: identity.Username}:                    struct{}{},
			{MFADevice: identity.MFAVerified}:            struct{}{},
			{Device: identity.DeviceExtensions.DeviceID}: struct{}{},
			{JoinToken: "example"}:                       struct{}{},
			{BotInstanceID: "a-b-c-d"}:                   struct{}{},
		}
		for _, role := range identity.Groups {
			want[types.LockTarget{Role: role}] = struct{}{}
		}
		for _, request := range identity.ActiveRequests {
			want[types.LockTarget{AccessRequest: request}] = struct{}{}
		}

		require.Empty(t, cmp.Diff(want, got, protocmp.Transform()), "LockTargetsFromTLSIdentity mismatch (-want +got)")
	})
}

func TestSSHAccessLockTargets(t *testing.T) {
	t.Run("all locks", func(t *testing.T) {
		const clusterName = "mycluster"
		const serverID = "myserver"
		const mfaDevice = "my-mfa-device-1"
		const trustedDevice = "my-trusted-device-1"
		const osLogin = "camel"
		const teleportUser = "llama"
		const joinToken = "some-join-token"
		const botInstanceID = "a-b-c-d"
		mappedRoles := []string{"access", "editor"}
		unmappedRoles := []string{"unmapped-role-1", "unmapped-role-2", "access"}
		accessRequests := []string{"access-request-1", "access-request-2"}

		unmappedIdentity := &sshca.Identity{
			Username:       teleportUser,
			MFAVerified:    mfaDevice,
			DeviceID:       trustedDevice,
			Roles:          unmappedRoles,
			ActiveRequests: accessRequests,
			JoinToken:      joinToken,
			BotInstanceID:  botInstanceID,
		}

		accessInfo := &services.AccessInfo{
			Username: "llama",
			Roles:    mappedRoles,
		}

		got := make(map[types.LockTarget]struct{})
		for _, lockTarget := range services.SSHAccessLockTargets(clusterName, serverID, osLogin, accessInfo, unmappedIdentity) {
			got[lockTarget] = struct{}{}
		}

		want := map[types.LockTarget]struct{}{
			{User: teleportUser}:                     struct{}{},
			{ServerID: serverID}:                     struct{}{},
			{ServerID: serverID + "." + clusterName}: struct{}{},
			{MFADevice: mfaDevice}:                   struct{}{},
			{Device: trustedDevice}:                  struct{}{},
			{JoinToken: joinToken}:                   struct{}{},
			{BotInstanceID: botInstanceID}:           struct{}{},
			{Login: osLogin}:                         struct{}{},
		}
		for _, role := range mappedRoles {
			want[types.LockTarget{Role: role}] = struct{}{}
		}
		for _, role := range unmappedRoles {
			want[types.LockTarget{Role: role}] = struct{}{}
		}
		for _, request := range accessRequests {
			want[types.LockTarget{AccessRequest: request}] = struct{}{}
		}

		require.Empty(t, cmp.Diff(want, got, protocmp.Transform()), "SSHAccessLockTargets mismatch (-want +got)")
	})
}
