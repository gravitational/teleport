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
		expectErr error
	}{
		{
			name: "valid config",
			spec: types.SessionRecordingConfigSpecV2{
				Mode: "node",
			},
			params: types.SignatureAlgorithmSuiteParams{
				FIPS:  true,
				Cloud: true,
			},
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
			params: types.SignatureAlgorithmSuiteParams{
				FIPS: true,
			},
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
			params: types.SignatureAlgorithmSuiteParams{
				Cloud: true,
			},
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
			err := services.ValidateSessionRecordingConfig(&types.SessionRecordingConfigV2{Spec: c.spec}, c.params)
			if c.expectErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, c.expectErr.Error())
			}
		})
	}
}
