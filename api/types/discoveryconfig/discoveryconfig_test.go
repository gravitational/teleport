/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package discoveryconfig

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
)

func requireBadParameter(t require.TestingT, err error, i ...interface{}) {
	require.True(
		t,
		trace.IsBadParameter(err),
		"err should be bad parameter, was: %s", err,
	)
}

func TestNewDiscoveryConfig(t *testing.T) {
	for _, tt := range []struct {
		name       string
		inMetadata header.Metadata
		inSpec     Spec
		expected   *DiscoveryConfig
		errCheck   require.ErrorAssertionFunc
	}{
		{
			name: "valid",
			inMetadata: header.Metadata{
				Name: "my-first-dc",
			},
			inSpec: Spec{
				DiscoveryGroup: "dg1",
			},
			expected: &DiscoveryConfig{
				ResourceHeader: header.ResourceHeader{
					Kind:    types.KindDiscoveryConfig,
					Version: types.V1,
					Metadata: header.Metadata{
						Name: "my-first-dc",
					},
				},
				Spec: Spec{
					DiscoveryGroup: "dg1",
					AWS:            make([]types.AWSMatcher, 0),
					Azure:          make([]types.AzureMatcher, 0),
					GCP:            make([]types.GCPMatcher, 0),
					Kube:           make([]types.KubernetesMatcher, 0),
				},
			},
			errCheck: require.NoError,
		},
		{
			name: "fills in aws matcher default values",
			inMetadata: header.Metadata{
				Name: "my-first-dc",
			},
			inSpec: Spec{
				DiscoveryGroup: "dg1",
				AWS: []types.AWSMatcher{{
					Types:   []string{"ec2"},
					Regions: []string{"eu-west-2"},
					Tags:    types.Labels{"*": []string{"*"}},
				}},
			},
			expected: &DiscoveryConfig{
				ResourceHeader: header.ResourceHeader{
					Kind:    types.KindDiscoveryConfig,
					Version: types.V1,
					Metadata: header.Metadata{
						Name: "my-first-dc",
					},
				},
				Spec: Spec{
					DiscoveryGroup: "dg1",
					AWS: []types.AWSMatcher{{
						Types:   []string{"ec2"},
						Regions: []string{"eu-west-2"},
						Tags:    types.Labels{"*": []string{"*"}},
						SSM: &types.AWSSSM{
							DocumentName: "TeleportDiscoveryInstaller",
						},
						Params: &types.InstallerParams{
							JoinMethod:      "iam",
							JoinToken:       "aws-discovery-iam-token",
							ScriptName:      "default-installer",
							InstallTeleport: true,
							SSHDConfig:      "/etc/ssh/sshd_config",
							EnrollMode:      types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
						},
					}},
					Azure: make([]types.AzureMatcher, 0),
					GCP:   make([]types.GCPMatcher, 0),
					Kube:  make([]types.KubernetesMatcher, 0),
				},
			},

			errCheck: require.NoError,
		},
		{
			name: "fills in azure matcher default values",
			inMetadata: header.Metadata{
				Name: "my-first-dc",
			},
			inSpec: Spec{
				DiscoveryGroup: "dg1",
				Azure: []types.AzureMatcher{{
					Types:   []string{"vm"},
					Regions: []string{"europe-west-2"},
				}},
			},
			expected: &DiscoveryConfig{
				ResourceHeader: header.ResourceHeader{
					Kind:    types.KindDiscoveryConfig,
					Version: types.V1,
					Metadata: header.Metadata{
						Name: "my-first-dc",
					},
				},
				Spec: Spec{
					DiscoveryGroup: "dg1",
					AWS:            make([]types.AWSMatcher, 0),
					Azure: []types.AzureMatcher{{
						Types:          []string{"vm"},
						Regions:        []string{"europe-west-2"},
						Subscriptions:  []string{"*"},
						ResourceGroups: []string{"*"},
						ResourceTags:   types.Labels{"*": []string{"*"}},
						Params: &types.InstallerParams{
							JoinMethod: "azure",
							JoinToken:  "azure-discovery-token",
							ScriptName: "default-installer",
							Azure:      &types.AzureInstallerParams{},
						},
					}},
					GCP:  make([]types.GCPMatcher, 0),
					Kube: make([]types.KubernetesMatcher, 0),
				},
			},
			errCheck: require.NoError,
		},
		{
			name: "fills in azure matcher default values",
			inMetadata: header.Metadata{
				Name: "my-first-dc",
			},
			inSpec: Spec{
				DiscoveryGroup: "dg1",
				GCP: []types.GCPMatcher{{
					Types:      []string{"gce"},
					ProjectIDs: []string{"p1"},
				}},
			},
			expected: &DiscoveryConfig{
				ResourceHeader: header.ResourceHeader{
					Kind:    types.KindDiscoveryConfig,
					Version: types.V1,
					Metadata: header.Metadata{
						Name: "my-first-dc",
					},
				},
				Spec: Spec{
					DiscoveryGroup: "dg1",
					AWS:            make([]types.AWSMatcher, 0),
					Azure:          make([]types.AzureMatcher, 0),
					GCP: []types.GCPMatcher{{
						Types:      []string{"gce"},
						Locations:  []string{"*"},
						ProjectIDs: []string{"p1"},
						Labels:     types.Labels{"*": []string{"*"}},
						Params: &types.InstallerParams{
							JoinMethod: "gcp",
							JoinToken:  "gcp-discovery-token",
							ScriptName: "default-installer",
						},
					}},
					Kube: make([]types.KubernetesMatcher, 0),
				},
			},
			errCheck: require.NoError,
		},
		{
			name: "tag aws sync",
			inMetadata: header.Metadata{
				Name: "my-first-dc",
			},
			inSpec: Spec{
				DiscoveryGroup: "dg1",
				AccessGraph: &types.AccessGraphSync{
					AWS: []*types.AccessGraphAWSSync{
						{
							Integration: "1234",
							AssumeRole: &types.AssumeRole{
								RoleARN: "arn:aws:iam::123456789012:role/teleport",
							},
							Regions: []string{"us-west-2"},
						},
					},
				},
			},
			expected: &DiscoveryConfig{
				ResourceHeader: header.ResourceHeader{
					Kind:    types.KindDiscoveryConfig,
					Version: types.V1,
					Metadata: header.Metadata{
						Name: "my-first-dc",
					},
				},
				Spec: Spec{
					DiscoveryGroup: "dg1",
					AWS:            make([]types.AWSMatcher, 0),
					Azure:          make([]types.AzureMatcher, 0),
					GCP:            make([]types.GCPMatcher, 0),
					Kube:           []types.KubernetesMatcher{},
					AccessGraph: &types.AccessGraphSync{
						AWS: []*types.AccessGraphAWSSync{
							{
								Integration: "1234",
								AssumeRole: &types.AssumeRole{
									RoleARN: "arn:aws:iam::123456789012:role/teleport",
								},
								Regions: []string{"us-west-2"},
							},
						},
					},
				},
			},
			errCheck: require.NoError,
		},
		{
			name: "tag aws sync with cloudtrail logs",
			inMetadata: header.Metadata{
				Name: "my-first-dc",
			},
			inSpec: Spec{
				DiscoveryGroup: "dg1",
				AccessGraph: &types.AccessGraphSync{
					AWS: []*types.AccessGraphAWSSync{
						{
							Integration: "1234",
							AssumeRole: &types.AssumeRole{
								RoleARN: "arn:aws:iam::123456789012:role/teleport",
							},
							Regions: []string{"us-west-2"},
							CloudTrailLogs: &types.AccessGraphAWSSyncCloudTrailLogs{
								SQSQueue: "sqs-queue",
								Region:   "us-west-2",
							},
						},
					},
				},
			},
			expected: &DiscoveryConfig{
				ResourceHeader: header.ResourceHeader{
					Kind:    types.KindDiscoveryConfig,
					Version: types.V1,
					Metadata: header.Metadata{
						Name: "my-first-dc",
					},
				},
				Spec: Spec{
					DiscoveryGroup: "dg1",
					AWS:            make([]types.AWSMatcher, 0),
					Azure:          make([]types.AzureMatcher, 0),
					GCP:            make([]types.GCPMatcher, 0),
					Kube:           []types.KubernetesMatcher{},
					AccessGraph: &types.AccessGraphSync{
						AWS: []*types.AccessGraphAWSSync{
							{
								Integration: "1234",
								AssumeRole: &types.AssumeRole{
									RoleARN: "arn:aws:iam::123456789012:role/teleport",
								},
								Regions: []string{"us-west-2"},
								CloudTrailLogs: &types.AccessGraphAWSSyncCloudTrailLogs{
									SQSQueue: "sqs-queue",
									Region:   "us-west-2",
								},
							},
						},
					},
				},
			},
			errCheck: require.NoError,
		},
		{
			name: "tag aws sync with missing cloudtrail logs fields",
			inMetadata: header.Metadata{
				Name: "my-first-dc",
			},
			inSpec: Spec{
				DiscoveryGroup: "dg1",
				AccessGraph: &types.AccessGraphSync{
					AWS: []*types.AccessGraphAWSSync{
						{
							Integration: "1234",
							AssumeRole: &types.AssumeRole{
								RoleARN: "arn:aws:iam::123456789012:role/teleport",
							},
							Regions:        []string{"us-west-2"},
							CloudTrailLogs: &types.AccessGraphAWSSyncCloudTrailLogs{},
						},
					},
				},
			},
			errCheck: require.Error,
		},
		{
			name: "tag aws sync with invalid region",
			inMetadata: header.Metadata{
				Name: "my-first-dc",
			},
			inSpec: Spec{
				DiscoveryGroup: "dg1",
				AccessGraph: &types.AccessGraphSync{
					AWS: []*types.AccessGraphAWSSync{
						{
							Integration: "1234",
							AssumeRole: &types.AssumeRole{
								RoleARN: "arn:aws:iam::123456789012:role/teleport",
							},
							Regions: []string{"us<random>&-west-2"},
						},
					},
				},
			},
			errCheck: require.Error,
		},
		{
			name: "tag aws sync with empty region",
			inMetadata: header.Metadata{
				Name: "my-first-dc",
			},
			inSpec: Spec{
				DiscoveryGroup: "dg1",
				AccessGraph: &types.AccessGraphSync{
					AWS: []*types.AccessGraphAWSSync{
						{
							Integration: "1234",
							AssumeRole: &types.AssumeRole{
								RoleARN: "arn:aws:iam::123456789012:role/teleport",
							},
							Regions: []string{""},
						},
					},
				},
			},
			errCheck: require.Error,
		},
		{
			name: "tag aws sync with region not set",
			inMetadata: header.Metadata{
				Name: "my-first-dc",
			},
			inSpec: Spec{
				DiscoveryGroup: "dg1",
				AccessGraph: &types.AccessGraphSync{
					AWS: []*types.AccessGraphAWSSync{
						{
							Integration: "1234",
							AssumeRole: &types.AssumeRole{
								RoleARN: "arn:aws:iam::123456789012:role/teleport",
							},
							Regions: nil,
						},
					},
				},
			},
			errCheck: require.Error,
		},
		{
			name: "fills in kube matcher default values",
			inMetadata: header.Metadata{
				Name: "my-first-dc",
			},
			inSpec: Spec{
				DiscoveryGroup: "dg1",
				Kube: []types.KubernetesMatcher{{
					Types: []string{"app"},
				}},
			},
			expected: &DiscoveryConfig{
				ResourceHeader: header.ResourceHeader{
					Kind:    types.KindDiscoveryConfig,
					Version: types.V1,
					Metadata: header.Metadata{
						Name: "my-first-dc",
					},
				},
				Spec: Spec{
					DiscoveryGroup: "dg1",
					AWS:            make([]types.AWSMatcher, 0),
					Azure:          make([]types.AzureMatcher, 0),
					GCP:            make([]types.GCPMatcher, 0),
					Kube: []types.KubernetesMatcher{{
						Types:      []string{"app"},
						Namespaces: []string{"*"},
						Labels:     types.Labels{"*": []string{"*"}},
					}},
				},
			},
			errCheck: require.NoError,
		},
		{
			name: "error when name is not present",
			inMetadata: header.Metadata{
				Name: "",
			},
			inSpec: Spec{
				DiscoveryGroup: "dg1",
			},
			errCheck: requireBadParameter,
		},
		{
			name: "error when discovery group is not present",
			inMetadata: header.Metadata{
				Name: "my-first-dc",
			},
			inSpec: Spec{
				DiscoveryGroup: "",
			},
			errCheck: requireBadParameter,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewDiscoveryConfig(tt.inMetadata, tt.inSpec)
			if tt.errCheck != nil {
				tt.errCheck(t, err)
			}
			if tt.expected != nil {
				require.Equal(t, tt.expected, got)
			}
		})
	}
}
