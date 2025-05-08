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
		}

		// Test.
		got := services.LockTargetsFromTLSIdentity(identity)

		want := []types.LockTarget{
			{User: identity.Username},
			{MFADevice: identity.MFAVerified},
			{Device: identity.DeviceExtensions.DeviceID},
		}
		// Insert roles at the start to match `got`s order.
		// The test itself doesn't care about the order, it's just easier to test
		// this way.
		want = append(services.RolesToLockTargets(identity.Groups), want...)
		for _, request := range identity.ActiveRequests {
			want = append(want, types.LockTarget{AccessRequest: request})
		}
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("LockTargetsFromTLSIdentity mismatch (-want +got)\n%s", diff)
		}
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
		mappedRoles := []string{"access", "editor"}
		unmappedRoles := []string{"unmapped-role-1", "unmapped-role-2", "access"}
		accessRequests := []string{"access-request-1", "access-request-2"}

		unmappedIdentity := &sshca.Identity{
			Username:       teleportUser,
			MFAVerified:    mfaDevice,
			DeviceID:       trustedDevice,
			Roles:          unmappedRoles,
			ActiveRequests: accessRequests,
		}

		accessInfo := &services.AccessInfo{
			Username: "llama",
			Roles:    mappedRoles,
		}

		got := services.SSHAccessLockTargets(clusterName, serverID, osLogin, accessInfo, unmappedIdentity)
		want := []types.LockTarget{
			{User: teleportUser},
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
