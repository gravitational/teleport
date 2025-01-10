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

package common_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestSession_GetAccessState(t *testing.T) {
	checker := &fakeAccessChecker{}
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{})
	require.NoError(t, err, "NewAuthPreference failed")

	tests := []struct {
		name    string
		session common.Session
		want    services.AccessState
	}{
		{
			name: "default",
			session: common.Session{
				Checker: checker,
			},
			want: services.AccessState{
				EnableDeviceVerification: true, // always set
			},
		},
		{
			name: "mfa verified",
			session: common.Session{
				Identity: tlsca.Identity{
					MFAVerified: "device-id",
				},
				Checker: checker,
			},
			want: services.AccessState{
				MFAVerified:              true,
				EnableDeviceVerification: true,
			},
		},
		{
			name: "device verified",
			session: common.Session{
				Identity: tlsca.Identity{
					DeviceExtensions: tlsca.DeviceExtensions{
						DeviceID:     "deviceid1",
						AssetTag:     "assettag1",
						CredentialID: "credentialid1",
					},
				},
				Checker: checker,
			},
			want: services.AccessState{
				DeviceVerified:           true,
				EnableDeviceVerification: true,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.session.GetAccessState(authPref)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("GetAccessState mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

type fakeAccessChecker struct {
	services.AccessChecker
}

func (c *fakeAccessChecker) GetAccessState(authPref readonly.AuthPreference) services.AccessState {
	return services.AccessState{}
}
