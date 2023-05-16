// Copyright 2023 Gravitational, Inc
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

package services_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
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
