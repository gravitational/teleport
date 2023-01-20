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

package common_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/stretchr/testify/require"
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

func (c *fakeAccessChecker) GetAccessState(authPref types.AuthPreference) services.AccessState {
	return services.AccessState{}
}
