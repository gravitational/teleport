// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package services_test

import (
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestValidateSessionRecordingConfig(t *testing.T) {
	cases := []struct {
		name      string
		spec      types.SessionRecordingConfigSpecV2
		params    types.SignatureAlgorithmSuiteParams
		cloud     bool
		fips      bool
		expectErr error
	}{
		{
			name: "valid config",
			spec: types.SessionRecordingConfigSpecV2{
				Mode: "node",
			},
			fips:      true,
			cloud:     true,
			expectErr: nil,
		},
		{
			name: "valid config: encryption",
			spec: types.SessionRecordingConfigSpecV2{
				Mode: "node",
				Encryption: &types.SessionRecordingEncryptionConfig{
					Enabled: true,
				},
			},
			expectErr: nil,
		},
		{
			name: "valid config: manual encryption",
			spec: types.SessionRecordingConfigSpecV2{
				Mode: "node",
				Encryption: &types.SessionRecordingEncryptionConfig{
					Enabled: true,
					ManualKeyManagement: &types.ManualKeyManagementConfig{
						Enabled: true,
						ActiveKeys: []*types.KeyLabel{
							{
								Type:  "aws_kms",
								Label: "test",
							},
						},
					},
				},
			},
			expectErr: nil,
		},
		{
			name: "invalid config: session mode",
			spec: types.SessionRecordingConfigSpecV2{
				Mode: "invalid",
			},
			expectErr: trace.BadParameter("session recording mode must be one of %v; got %q", strings.Join(types.SessionRecordingModes, ","), "invalid"),
		},
		{
			name: "invalid config: encryption with FIPS",
			spec: types.SessionRecordingConfigSpecV2{
				Mode: "node",
				Encryption: &types.SessionRecordingEncryptionConfig{
					Enabled: true,
				},
			},
			fips:      true,
			expectErr: trace.BadParameter("non-FIPS compliant session recording setting"),
		},
		{
			name: "invalid config: manual encryption in cloud",
			spec: types.SessionRecordingConfigSpecV2{
				Mode: "node",
				Encryption: &types.SessionRecordingEncryptionConfig{
					Enabled: true,
					ManualKeyManagement: &types.ManualKeyManagementConfig{
						Enabled: true,
					},
				},
			},
			cloud:     true,
			expectErr: trace.BadParameter(`"manual_key_management" configuration is unsupported in Teleport Cloud`),
		},
		{
			name: "invalid config: manual encryption without active keys",
			spec: types.SessionRecordingConfigSpecV2{
				Mode: "node",
				Encryption: &types.SessionRecordingEncryptionConfig{
					Enabled: true,
					ManualKeyManagement: &types.ManualKeyManagementConfig{
						Enabled: true,
					},
				},
			},
			expectErr: trace.BadParameter("at least one active key must be configured when using manually managed encryption keys"),
		},
		{
			name: "invalid config: invalid manual key type",
			spec: types.SessionRecordingConfigSpecV2{
				Mode: "node",
				Encryption: &types.SessionRecordingEncryptionConfig{
					Enabled: true,
					ManualKeyManagement: &types.ManualKeyManagementConfig{
						Enabled: true,
						ActiveKeys: []*types.KeyLabel{
							{
								Type:  "unsupported",
								Label: "test",
							},
						},
					},
				},
			},
			expectErr: trace.BadParameter("invalid key type \"unsupported\" found for active manually managed key"),
		},
		{
			name: "invalid config: invalid manual rotated key type",
			spec: types.SessionRecordingConfigSpecV2{
				Mode: "node",
				Encryption: &types.SessionRecordingEncryptionConfig{
					Enabled: true,
					ManualKeyManagement: &types.ManualKeyManagementConfig{
						Enabled: true,
						ActiveKeys: []*types.KeyLabel{
							{
								Type:  "pkcs11",
								Label: "test",
							},
						},
						RotatedKeys: []*types.KeyLabel{
							{
								Type:  "unsupported",
								Label: "test",
							},
						},
					},
				},
			},
			expectErr: trace.BadParameter("invalid key type \"unsupported\" found for rotated manually managed key"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := services.ValidateSessionRecordingConfig(&types.SessionRecordingConfigV2{Spec: c.spec}, c.fips, c.cloud)
			if c.expectErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, c.expectErr.Error())
			}
		})
	}
}
