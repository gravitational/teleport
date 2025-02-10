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

package authz_test

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/devicetrust/authz"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestIsTLSDeviceVerified(t *testing.T) {
	testIsDeviceVerified(t, "IsTLSDeviceVerified", authz.IsTLSDeviceVerified)
}

func TestIsSSHDeviceVerified(t *testing.T) {
	testIsDeviceVerified(t, "IsSSHDeviceVerified", func(ext *tlsca.DeviceExtensions) bool {
		var ident *sshca.Identity
		if ext != nil {
			ident = &sshca.Identity{
				DeviceID:           ext.DeviceID,
				DeviceAssetTag:     ext.AssetTag,
				DeviceCredentialID: ext.CredentialID,
			}
		}
		return authz.IsSSHDeviceVerified(ident)
	})
}

func testIsDeviceVerified(t *testing.T, name string, fn func(ext *tlsca.DeviceExtensions) bool) {
	tests := []struct {
		name string
		ext  *tlsca.DeviceExtensions
		want bool
	}{
		{
			name: "all extensions",
			ext: &tlsca.DeviceExtensions{
				DeviceID:     "deviceid1",
				AssetTag:     "assettag1",
				CredentialID: "credentialid1",
			},
			want: true,
		},
		{
			name: "nok: nil extensions",
		},
		{
			name: "nok: empty extensions",
			ext:  &tlsca.DeviceExtensions{},
		},
		{
			name: "nok: missing DeviceID",
			ext: &tlsca.DeviceExtensions{
				AssetTag:     "assettag1",
				CredentialID: "credentialid1",
			},
		},
		{
			name: "nok: missing AssetTag",
			ext: &tlsca.DeviceExtensions{
				DeviceID:     "deviceid1",
				CredentialID: "credentialid1",
			},
		},
		{
			name: "nok: missing CredentialID",
			ext: &tlsca.DeviceExtensions{
				DeviceID: "deviceid1",
				AssetTag: "assettag1",
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			got := fn(test.ext)
			if got != test.want {
				t.Errorf("%v(%#v) = %v, want = %v", name, test.ext, got, test.want)
			}
		})
	}
}

func TestVerifyTLSUser(t *testing.T) {
	runVerifyUserTest(t, "VerifyTLSUser", func(dt *types.DeviceTrust, ext *tlsca.DeviceExtensions) error {
		return authz.VerifyTLSUser(dt, tlsca.Identity{
			Username:         "llama",
			DeviceExtensions: *ext,
		})
	})
}

func TestVerifySSHUser(t *testing.T) {
	runVerifyUserTest(t, "VerifySSHUser", func(dt *types.DeviceTrust, ext *tlsca.DeviceExtensions) error {
		return authz.VerifySSHUser(dt, &sshca.Identity{
			DeviceID:           ext.DeviceID,
			DeviceAssetTag:     ext.AssetTag,
			DeviceCredentialID: ext.CredentialID,
		})
	})
}

func runVerifyUserTest(t *testing.T, method string, verify func(dt *types.DeviceTrust, ext *tlsca.DeviceExtensions) error) {
	assertNoErr := func(t *testing.T, err error) {
		assert.NoError(t, err, "%v mismatch", method)
	}
	assertDeniedErr := func(t *testing.T, err error) {
		assert.ErrorContains(t, err, "trusted device", "%v mismatch", method)
		assert.True(t, trace.IsAccessDenied(err), "%v returned an error other than trace.AccessDeniedError: %T", method, err)
	}

	userWithoutExtensions := &tlsca.DeviceExtensions{}
	userWithExtensions := &tlsca.DeviceExtensions{
		DeviceID:     "deviceid1",
		AssetTag:     "assettag1",
		CredentialID: "credentialid1",
	}

	tests := []struct {
		name      string
		buildType string
		dt        *types.DeviceTrust
		ext       *tlsca.DeviceExtensions
		assertErr func(t *testing.T, err error)
	}{
		{
			name:      "OSS dt=nil",
			buildType: modules.BuildOSS,
			dt:        nil, // OK, config absent.
			ext:       userWithoutExtensions,
			assertErr: assertNoErr,
		},
		{
			name:      "OSS mode=off",
			buildType: modules.BuildOSS,
			dt: &types.DeviceTrust{
				Mode: constants.DeviceTrustModeOff, // Valid for OSS.
			},
			ext:       userWithoutExtensions,
			assertErr: assertNoErr,
		},
		{
			name:      "OSS mode=required (Enterprise Auth)",
			buildType: modules.BuildOSS,
			dt: &types.DeviceTrust{
				Mode: constants.DeviceTrustModeRequired,
			},
			ext:       userWithoutExtensions,
			assertErr: assertDeniedErr,
		},
		{
			name:      "Enterprise mode=off",
			buildType: modules.BuildEnterprise,
			dt: &types.DeviceTrust{
				Mode: constants.DeviceTrustModeOff,
			},
			ext:       userWithoutExtensions,
			assertErr: assertNoErr,
		},
		{
			name:      "Enterprise mode=optional without extensions",
			buildType: modules.BuildEnterprise,
			dt: &types.DeviceTrust{
				Mode: constants.DeviceTrustModeOptional,
			},
			ext:       userWithoutExtensions,
			assertErr: assertNoErr,
		},
		{
			name:      "Enterprise mode=optional with extensions",
			buildType: modules.BuildEnterprise,
			dt: &types.DeviceTrust{
				Mode: constants.DeviceTrustModeOptional,
			},
			ext:       userWithExtensions, // Happens if the device is enrolled.
			assertErr: assertNoErr,
		},
		{
			name:      "nok: Enterprise mode=required without extensions",
			buildType: modules.BuildEnterprise,
			dt: &types.DeviceTrust{
				Mode: constants.DeviceTrustModeRequired,
			},
			ext:       userWithoutExtensions,
			assertErr: assertDeniedErr,
		},
		{
			name:      "Enterprise mode=required with extensions",
			buildType: modules.BuildEnterprise,
			dt: &types.DeviceTrust{
				Mode: constants.DeviceTrustModeRequired,
			},
			ext:       userWithExtensions,
			assertErr: assertNoErr,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			modules.SetTestModules(t, &modules.TestModules{
				TestBuildType: test.buildType,
			})

			test.assertErr(t, verify(test.dt, test.ext))
		})
	}
}
