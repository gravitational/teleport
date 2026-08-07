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

func TestIsMatchersEmpty(t *testing.T) {
	for _, tt := range []struct {
		name     string
		config   *DiscoveryConfig
		expected bool
	}{
		{
			name: "empty config",
			config: &DiscoveryConfig{
				Spec: Spec{},
			},
			expected: true,
		},
		{
			name: "has AWS matchers",
			config: &DiscoveryConfig{
				Spec: Spec{
					AWS: []types.AWSMatcher{{
						Types:   []string{"ec2"},
						Regions: []string{"us-west-2"},
					}},
				},
			},
			expected: false,
		},
		{
			name: "has Azure matchers",
			config: &DiscoveryConfig{
				Spec: Spec{
					Azure: []types.AzureMatcher{{
						Types:   []string{"vm"},
						Regions: []string{"europe-west-2"},
					}},
				},
			},
			expected: false,
		},
		{
			name: "has GCP matchers",
			config: &DiscoveryConfig{
				Spec: Spec{
					GCP: []types.GCPMatcher{{
						Types:      []string{"gce"},
						ProjectIDs: []string{"p1"},
					}},
				},
			},
			expected: false,
		},
		{
			name: "has Kube matchers",
			config: &DiscoveryConfig{
				Spec: Spec{
					Kube: []types.KubernetesMatcher{{
						Types: []string{"app"},
					}},
				},
			},
			expected: false,
		},
		{
			name: "has AccessGraph with AWS",
			config: &DiscoveryConfig{
				Spec: Spec{
					AccessGraph: &types.AccessGraphSync{
						AWS: []*types.AccessGraphAWSSync{{
							Integration: "integration1",
							Regions:     []string{"us-west-2"},
						}},
					},
				},
			},
			expected: false,
		},
		{
			name: "has AccessGraph but no syncs",
			config: &DiscoveryConfig{
				Spec: Spec{
					AccessGraph: &types.AccessGraphSync{},
				},
			},
			expected: true,
		},
		{
			name: "has AccessGraph Azure sync",
			config: &DiscoveryConfig{
				Spec: Spec{
					AccessGraph: &types.AccessGraphSync{
						Azure: []*types.AccessGraphAzureSync{{
							Integration:    "integration1",
							SubscriptionID: "sub-id",
						}},
					},
				},
			},
			expected: false,
		},
		{
			name: "has multiple matcher types",
			config: &DiscoveryConfig{
				Spec: Spec{
					AWS: []types.AWSMatcher{{
						Types:   []string{"ec2"},
						Regions: []string{"us-west-2"},
					}},
					Azure: []types.AzureMatcher{{
						Types:   []string{"vm"},
						Regions: []string{"europe-west-2"},
					}},
				},
			},
			expected: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.IsMatchersEmpty()
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestReferencesIntegration(t *testing.T) {
	for _, tt := range []struct {
		name     string
		config   *DiscoveryConfig
		expected bool
	}{
		{
			name:     "empty config",
			config:   &DiscoveryConfig{Spec: Spec{}},
			expected: false,
		},
		{
			name: "AWS matcher on the integration",
			config: &DiscoveryConfig{
				Spec: Spec{
					AWS: []types.AWSMatcher{
						{Integration: "integration2"},
						{Integration: "integration1"},
					},
				},
			},
			expected: true,
		},
		{
			name: "Azure matcher on the integration",
			config: &DiscoveryConfig{
				Spec: Spec{
					Azure: []types.AzureMatcher{{Integration: "integration1"}},
				},
			},
			expected: true,
		},
		{
			name: "AccessGraph AWS sync on the integration",
			config: &DiscoveryConfig{
				Spec: Spec{
					AccessGraph: &types.AccessGraphSync{
						AWS: []*types.AccessGraphAWSSync{{Integration: "integration1"}},
					},
				},
			},
			expected: true,
		},
		{
			name: "AccessGraph Azure sync on the integration",
			config: &DiscoveryConfig{
				Spec: Spec{
					AccessGraph: &types.AccessGraphSync{
						Azure: []*types.AccessGraphAzureSync{{Integration: "integration1"}},
					},
				},
			},
			expected: true,
		},
		{
			name: "only another integration",
			config: &DiscoveryConfig{
				Spec: Spec{
					AWS:   []types.AWSMatcher{{Integration: "integration2"}},
					Azure: []types.AzureMatcher{{Integration: "integration2"}},
					AccessGraph: &types.AccessGraphSync{
						AWS:   []*types.AccessGraphAWSSync{{Integration: "integration2"}},
						Azure: []*types.AccessGraphAzureSync{{Integration: "integration2"}},
					},
				},
			},
			expected: false,
		},
		{
			name: "nil AccessGraph sync entries",
			config: &DiscoveryConfig{
				Spec: Spec{
					AccessGraph: &types.AccessGraphSync{
						AWS:   []*types.AccessGraphAWSSync{nil},
						Azure: []*types.AccessGraphAzureSync{nil},
					},
				},
			},
			expected: false,
		},
		{
			name: "GCP and Kube matchers cannot reference an integration",
			config: &DiscoveryConfig{
				Spec: Spec{
					GCP:  []types.GCPMatcher{{Types: []string{"gce"}}},
					Kube: []types.KubernetesMatcher{{Types: []string{"app"}}},
				},
			},
			expected: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.config.ReferencesIntegration("integration1"))
		})
	}

	t.Run("matchers using ambient credentials reference no integration", func(t *testing.T) {
		config := &DiscoveryConfig{
			Spec: Spec{
				AWS:   []types.AWSMatcher{{Types: []string{"ec2"}}},
				Azure: []types.AzureMatcher{{Types: []string{"vm"}}},
				AccessGraph: &types.AccessGraphSync{
					AWS:   []*types.AccessGraphAWSSync{{Regions: []string{"us-east-1"}}},
					Azure: []*types.AccessGraphAzureSync{{SubscriptionID: "sub-id"}},
				},
			},
		}

		require.False(t, config.ReferencesIntegration(""))
	})
}

func TestHasOtherMatchers(t *testing.T) {
	for _, tt := range []struct {
		name     string
		config   *DiscoveryConfig
		expected bool
	}{
		{
			name:     "empty config",
			config:   &DiscoveryConfig{Spec: Spec{}},
			expected: false,
		},
		{
			name: "AWS matchers on the integration",
			config: &DiscoveryConfig{
				Spec: Spec{
					AWS: []types.AWSMatcher{
						{Integration: "integration1"},
						{Integration: "integration1"},
					},
				},
			},
			expected: false,
		},
		{
			name: "one AWS matcher on another integration",
			config: &DiscoveryConfig{
				Spec: Spec{
					AWS: []types.AWSMatcher{
						{Integration: "integration1"},
						{Integration: "integration2"},
					},
				},
			},
			expected: true,
		},
		{
			name: "AWS matcher with no integration",
			config: &DiscoveryConfig{
				Spec: Spec{
					AWS: []types.AWSMatcher{{Types: []string{"ec2"}}},
				},
			},
			expected: true,
		},
		{
			name: "Azure matcher on another integration",
			config: &DiscoveryConfig{
				Spec: Spec{
					AWS:   []types.AWSMatcher{{Integration: "integration1"}},
					Azure: []types.AzureMatcher{{Integration: "integration2"}},
				},
			},
			expected: true,
		},
		{
			name: "AccessGraph syncs on the integration",
			config: &DiscoveryConfig{
				Spec: Spec{
					AWS: []types.AWSMatcher{{Integration: "integration1"}},
					AccessGraph: &types.AccessGraphSync{
						AWS:   []*types.AccessGraphAWSSync{{Integration: "integration1"}},
						Azure: []*types.AccessGraphAzureSync{{Integration: "integration1"}},
					},
				},
			},
			expected: false,
		},
		{
			name: "AccessGraph AWS sync on another integration",
			config: &DiscoveryConfig{
				Spec: Spec{
					AWS: []types.AWSMatcher{{Integration: "integration1"}},
					AccessGraph: &types.AccessGraphSync{
						AWS: []*types.AccessGraphAWSSync{{Integration: "integration2"}},
					},
				},
			},
			expected: true,
		},
		{
			name: "AccessGraph Azure sync on another integration",
			config: &DiscoveryConfig{
				Spec: Spec{
					AWS: []types.AWSMatcher{{Integration: "integration1"}},
					AccessGraph: &types.AccessGraphSync{
						Azure: []*types.AccessGraphAzureSync{{Integration: "integration2"}},
					},
				},
			},
			expected: true,
		},
		{
			name: "nil AccessGraph AWS sync entry",
			config: &DiscoveryConfig{
				Spec: Spec{
					AccessGraph: &types.AccessGraphSync{
						AWS: []*types.AccessGraphAWSSync{nil},
					},
				},
			},
			expected: true,
		},
		{
			name: "nil AccessGraph Azure sync entry",
			config: &DiscoveryConfig{
				Spec: Spec{
					AccessGraph: &types.AccessGraphSync{
						Azure: []*types.AccessGraphAzureSync{nil},
					},
				},
			},
			expected: true,
		},
		{
			name: "GCP matcher",
			config: &DiscoveryConfig{
				Spec: Spec{
					AWS: []types.AWSMatcher{{Integration: "integration1"}},
					GCP: []types.GCPMatcher{{Types: []string{"gce"}}},
				},
			},
			expected: true,
		},
		{
			name: "Kube matcher",
			config: &DiscoveryConfig{
				Spec: Spec{
					AWS:  []types.AWSMatcher{{Integration: "integration1"}},
					Kube: []types.KubernetesMatcher{{Types: []string{"app"}}},
				},
			},
			expected: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.config.HasOtherMatchers("integration1"))
		})
	}
}
