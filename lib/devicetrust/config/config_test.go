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

package config_test

import (
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	dtconfig "github.com/gravitational/teleport/lib/devicetrust/config"
	"github.com/gravitational/teleport/lib/modules"
)

func TestValidateConfigAgainstModules(t *testing.T) {
	type testCase struct {
		name        string
		buildType   string
		deviceTrust *types.DeviceTrust
		wantErr     bool
	}

	tests := []testCase{
		{
			name:      "OSS and nil config",
			buildType: modules.BuildOSS,
		},
		{
			name:        "OSS and default config",
			buildType:   modules.BuildOSS,
			deviceTrust: &types.DeviceTrust{},
		},
		{
			name:      "OSS and Mode=off",
			buildType: modules.BuildOSS,
			deviceTrust: &types.DeviceTrust{
				Mode: constants.DeviceTrustModeOff,
			},
		},
		{
			name:      "nok: OSS and Mode=optional",
			buildType: modules.BuildOSS,
			deviceTrust: &types.DeviceTrust{
				Mode: constants.DeviceTrustModeOptional,
			},
			wantErr: true,
		},
		{
			name:      "nok: OSS and Mode=required",
			buildType: modules.BuildOSS,
			deviceTrust: &types.DeviceTrust{
				Mode: constants.DeviceTrustModeRequired,
			},
			wantErr: true,
		},
		{
			name:        "Enterprise and nil config",
			buildType:   modules.BuildEnterprise,
			deviceTrust: nil,
		},
	}

	// All modes are valid for Enterprise.
	for _, mode := range []string{
		"", // aka default config
		constants.DeviceTrustModeOff,
		constants.DeviceTrustModeOptional,
		constants.DeviceTrustModeRequired} {
		tests = append(tests, testCase{
			name:      fmt.Sprintf("Enterprise and Mode=%v", mode),
			buildType: modules.BuildEnterprise,
			deviceTrust: &types.DeviceTrust{
				Mode: mode,
			},
		})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			modules.SetTestModules(t, &modules.TestModules{
				TestBuildType: test.buildType,
			})

			gotErr := dtconfig.ValidateConfigAgainstModules(test.deviceTrust)
			if test.wantErr {
				assert.Error(t, gotErr, "ValidateConfigAgainstModules mismatch")
				assert.True(t, trace.IsBadParameter(gotErr), "gotErr is not a trace.BadParameter error")
			} else {
				assert.NoError(t, gotErr, "ValidateConfigAgainstModules mismatch")
			}
		})
	}
}
