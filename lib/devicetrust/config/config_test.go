// Copyright 2022 Gravitational, Inc
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
