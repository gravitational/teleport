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

package types

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestAWSMatcherCheckAndSetDefaults(t *testing.T) {
	isBadParameterErr := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	for _, tt := range []struct {
		name     string
		in       *AWSMatcher
		errCheck require.ErrorAssertionFunc
		expected *AWSMatcher
	}{
		{
			name: "valid",
			in: &AWSMatcher{
				Types:   []string{"ec2"},
				Regions: []string{"eu-west-2"},
				Tags: Labels{
					"*": []string{"*"},
				},
				Params: &InstallerParams{
					JoinMethod:      JoinMethodIAM,
					JoinToken:       IAMInviteTokenName,
					InstallTeleport: true,
					ScriptName:      DefaultInstallerScriptName,
					SSHDConfig:      SSHDConfigPath,
				},
				SSM: &AWSSSM{DocumentName: AWSInstallerDocument},
				AssumeRole: &AssumeRole{
					RoleARN: "arn:aws:iam:us-west-2:123456789012:role/MyRole001",
				},
			},
			errCheck: require.NoError,
			expected: &AWSMatcher{
				Types:   []string{"ec2"},
				Regions: []string{"eu-west-2"},
				Tags: Labels{
					"*": []string{"*"},
				},
				Params: &InstallerParams{
					JoinMethod:      "iam",
					JoinToken:       "aws-discovery-iam-token",
					InstallTeleport: true,
					ScriptName:      "default-installer",
					SSHDConfig:      "/etc/ssh/sshd_config",
					EnrollMode:      InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
				},
				SSM: &AWSSSM{DocumentName: "TeleportDiscoveryInstaller"},
				AssumeRole: &AssumeRole{
					RoleARN: "arn:aws:iam:us-west-2:123456789012:role/MyRole001",
				},
			},
		},
		{
			name: "default values",
			in: &AWSMatcher{
				Types:   []string{"ec2"},
				Regions: []string{"eu-west-2"},
			},
			errCheck: require.NoError,
			expected: &AWSMatcher{
				Types:   []string{"ec2"},
				Regions: []string{"eu-west-2"},
				Tags: Labels{
					"*": []string{"*"},
				},
				Params: &InstallerParams{
					JoinMethod:      "iam",
					JoinToken:       "aws-discovery-iam-token",
					InstallTeleport: true,
					ScriptName:      "default-installer",
					SSHDConfig:      "/etc/ssh/sshd_config",
					EnrollMode:      InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
				},
				SSM: &AWSSSM{DocumentName: "TeleportDiscoveryInstaller"},
			},
		},
		{
			name: "default values for agentless mode",
			in: &AWSMatcher{
				Types:   []string{"ec2"},
				Regions: []string{"eu-west-2"},
				Params: &InstallerParams{
					InstallTeleport: false,
				},
			},
			errCheck: require.NoError,
			expected: &AWSMatcher{
				Types:   []string{"ec2"},
				Regions: []string{"eu-west-2"},
				Tags: Labels{
					"*": []string{"*"},
				},
				Params: &InstallerParams{
					JoinMethod:      "iam",
					JoinToken:       "aws-discovery-iam-token",
					InstallTeleport: false,
					ScriptName:      "default-agentless-installer",
					SSHDConfig:      "/etc/ssh/sshd_config",
					EnrollMode:      InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
				},
				SSM: &AWSSSM{DocumentName: "TeleportAgentlessDiscoveryInstaller"},
			},
		},
		{
			name: "wildcard is invalid for types",
			in: &AWSMatcher{
				Types:   []string{"*"},
				Regions: []string{"eu-west-2"},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "wildcard is invalid for regions",
			in: &AWSMatcher{
				Types:   []string{"ec2", "rds"},
				Regions: []string{"*"},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "invalid type",
			in: &AWSMatcher{
				Types:   []string{"ec2", "rds", "xpto"},
				Regions: []string{"eu-west-2"},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "invalid region",
			in: &AWSMatcher{
				Types:   []string{"ec2", "rds", "xpto"},
				Regions: []string{"pt-nope-4"},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "invalid join method",
			in: &AWSMatcher{
				Types:   []string{"ec2"},
				Regions: []string{"eu-west-2"},
				Params: &InstallerParams{
					JoinMethod: "ec2",
				},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "no type",
			in: &AWSMatcher{
				Types:   []string{},
				Regions: []string{"eu-west-2"},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "no region",
			in: &AWSMatcher{
				Types:   []string{"ec2"},
				Regions: []string{},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "invalid assume role arn",
			in: &AWSMatcher{
				Types:   []string{"ec2"},
				Regions: []string{"eu-west-2"},
				AssumeRole: &AssumeRole{
					RoleARN: "arn:aws:not valid",
				},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "external id was set but assume role is missing",
			in: &AWSMatcher{
				Types:   []string{"ec2"},
				Regions: []string{"eu-west-2"},
				AssumeRole: &AssumeRole{
					ExternalID: "id123",
				},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "external id was set, assume role is missing but type allows it",
			in: &AWSMatcher{
				Types:   []string{"redshift-serverless"},
				Regions: []string{"eu-west-2"},
				AssumeRole: &AssumeRole{
					ExternalID: "id123",
				},
			},
			errCheck: require.NoError,
			expected: &AWSMatcher{
				Types:   []string{"redshift-serverless"},
				Regions: []string{"eu-west-2"},
				Tags: Labels{
					"*": []string{"*"},
				},
				Params: &InstallerParams{
					JoinMethod:      "iam",
					JoinToken:       "aws-discovery-iam-token",
					InstallTeleport: true,
					ScriptName:      "default-installer",
					SSHDConfig:      "/etc/ssh/sshd_config",
					EnrollMode:      InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
				},
				AssumeRole: &AssumeRole{
					ExternalID: "id123",
				},
				SSM: &AWSSSM{DocumentName: "TeleportDiscoveryInstaller"},
			},
		},
		{
			name: "enroll mode and integration are not set, defaults to script enroll mode",
			in: &AWSMatcher{
				Types:   []string{"ec2"},
				Regions: []string{"eu-west-2"},
			},
			errCheck: require.NoError,
			expected: &AWSMatcher{
				Types:   []string{"ec2"},
				Regions: []string{"eu-west-2"},
				Tags: Labels{
					"*": []string{"*"},
				},
				Params: &InstallerParams{
					JoinMethod:      "iam",
					JoinToken:       "aws-discovery-iam-token",
					InstallTeleport: true,
					ScriptName:      "default-installer",
					SSHDConfig:      "/etc/ssh/sshd_config",
					EnrollMode:      InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
				},
				SSM: &AWSSSM{DocumentName: "TeleportDiscoveryInstaller"},
			},
		},
		{
			name: "enroll mode not set but integration is set, defaults to eice enroll mode",
			in: &AWSMatcher{
				Types:       []string{"ec2"},
				Regions:     []string{"eu-west-2"},
				Integration: "my-integration",
			},
			errCheck: require.NoError,
			expected: &AWSMatcher{
				Types:   []string{"ec2"},
				Regions: []string{"eu-west-2"},
				Tags: Labels{
					"*": []string{"*"},
				},
				Integration: "my-integration",
				Params: &InstallerParams{
					JoinMethod:      "iam",
					JoinToken:       "aws-discovery-iam-token",
					InstallTeleport: true,
					ScriptName:      "default-installer",
					SSHDConfig:      "/etc/ssh/sshd_config",
					EnrollMode:      InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_EICE,
				},
				SSM: &AWSSSM{DocumentName: "TeleportDiscoveryInstaller"},
			},
		},
		{
			name: "non-integration/ambient credentials do not support EICE",
			in: &AWSMatcher{
				Types:   []string{"ec2"},
				Regions: []string{"eu-west-2"},
				Params: &InstallerParams{
					EnrollMode: InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_EICE,
				},
			},
			errCheck: isBadParameterErr,
		},
		{
			name: "integration can be used with Script mode",
			in: &AWSMatcher{
				Types:       []string{"ec2"},
				Regions:     []string{"eu-west-2"},
				Integration: "my-integration",
				Params: &InstallerParams{
					InstallTeleport: true,
					EnrollMode:      InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
				},
			},
			errCheck: require.NoError,
			expected: &AWSMatcher{
				Types:   []string{"ec2"},
				Regions: []string{"eu-west-2"},
				Tags: Labels{
					"*": []string{"*"},
				},
				Integration: "my-integration",
				Params: &InstallerParams{
					JoinMethod:      "iam",
					JoinToken:       "aws-discovery-iam-token",
					InstallTeleport: true,
					ScriptName:      "default-installer",
					SSHDConfig:      "/etc/ssh/sshd_config",
					EnrollMode:      InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
				},
				SSM: &AWSSSM{DocumentName: "TeleportDiscoveryInstaller"},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.CheckAndSetDefaults()
			tt.errCheck(t, err)
			if tt.expected != nil {
				require.Equal(t, tt.expected, tt.in)
			}
		})
	}
}
