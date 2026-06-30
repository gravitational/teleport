// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package discovery

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/utils"
)

func TestBuildDiscoveryTextSummariesFromConfigs(t *testing.T) {
	t.Parallel()

	lastRun := time.Date(2026, 1, 15, 11, 58, 0, 0, time.UTC)
	awsConfig, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: "aws-config"},
		discoveryconfig.Spec{
			DiscoveryGroup: "prod",
			AWS: []types.AWSMatcher{
				{
					Types:       []string{types.AWSMatcherEC2},
					Regions:     []string{"eu-west-1"},
					Integration: "aws-prod",
					Tags:        types.Labels{"env": {"prod"}},
					Params: &types.InstallerParams{
						EnrollMode: types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
					},
				},
				{
					Types:       []string{types.AWSMatcherEC2},
					Regions:     []string{"eu-west-2"},
					Integration: "aws-prod",
					Tags:        types.Labels{"env": {"staging"}},
					Params: &types.InstallerParams{
						EnrollMode: types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
					},
				},
			},
		},
	)
	require.NoError(t, err)
	awsConfig.Status = discoveryconfig.Status{
		State:        discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String(),
		LastSyncTime: lastRun,
		IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
			"aws-prod": {
				IntegrationDiscoveredSummary: discoveryconfigv1.IntegrationDiscoveredSummary_builder{
					AwsEc2: discoveryResourcesSummary(10, 8, 2, lastRun),
				}.Build(),
			},
		},
	}

	azureConfig, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: "azure-config"},
		discoveryconfig.Spec{
			DiscoveryGroup: "prod",
			Azure: []types.AzureMatcher{
				{
					Types:          []string{types.AzureMatcherVM},
					Regions:        []string{"eastus"},
					Subscriptions:  []string{"sub-1"},
					ResourceGroups: []string{"rg-1"},
					ResourceTags:   types.Labels{types.Wildcard: {types.Wildcard}},
				},
			},
		},
	)
	require.NoError(t, err)

	summaries := newDiscoverySummary(
		[]*discoveryconfig.DiscoveryConfig{awsConfig, azureConfig},
		cloudProviderConfig{aws: true, azure: true},
	)
	require.Len(t, summaries, 2)

	awsConfigSummary := requireConfigSummary(t, summaries, "aws-config")
	require.Equal(t, "prod", awsConfigSummary.DiscoveryGroup)
	require.Equal(t, "healthy", awsConfigSummary.Status.State)
	require.NotNil(t, awsConfigSummary.Status.LastRun)
	require.Equal(t, lastRun, *awsConfigSummary.Status.LastRun)
	awsResource := requireResourceSummary(t, awsConfigSummary.Resources, cloudAWS, "EC2", "aws-prod")
	require.Equal(t, "counts", awsResource.Result.Kind)
	require.Equal(t, &resultCounts{
		Found:    10,
		Enrolled: 8,
		Failed:   2,
	}, awsResource.Result.Counts)
	require.Equal(t, []resourceScope{
		{
			Regions:   []string{"eu-west-1"},
			MatchTags: []string{"env=prod"},
		},
		{
			Regions:   []string{"eu-west-2"},
			MatchTags: []string{"env=staging"},
		},
	}, awsResource.Scopes)
	require.NotNil(t, awsResource.LastSync)
	require.Equal(t, lastRun, *awsResource.LastSync)

	azureConfigSummary := requireConfigSummary(t, summaries, "azure-config")
	require.Equal(t, "prod", azureConfigSummary.DiscoveryGroup)
	require.Equal(t, summaryStatusNotReporting, azureConfigSummary.Status.State)
	require.Nil(t, azureConfigSummary.Status.LastRun)
	azureResource := requireResourceSummary(t, azureConfigSummary.Resources, cloudAzure, "VM", "")
	require.Equal(t, "not_reporting", azureResource.Result.Kind)
	require.Equal(t, []resourceScope{
		{
			Regions:        []string{"eastus"},
			Subscriptions:  []string{"sub-1"},
			ResourceGroups: []string{"rg-1"},
			MatchTags:      []string{"all"},
		},
	}, azureResource.Scopes)
	require.Nil(t, azureResource.LastSync)

	structured := summaries
	require.Len(t, structured, 2)

	awsStructured := requireConfigSummary(t, structured, "aws-config")
	require.Equal(t, "prod", awsStructured.DiscoveryGroup)
	require.Equal(t, "healthy", awsStructured.Status.State)
	require.NotNil(t, awsStructured.Status.LastRun)
	require.Equal(t, lastRun, *awsStructured.Status.LastRun)
	require.Len(t, awsStructured.Resources, 1)
	require.Equal(t, cloudAWS, awsStructured.Resources[0].Cloud)
	require.Equal(t, "EC2", awsStructured.Resources[0].ResourceType)
	require.Equal(t, "integration", awsStructured.Resources[0].Source)
	require.Equal(t, "aws-prod", awsStructured.Resources[0].Integration)
	require.Equal(t, []resourceScope{
		{
			Regions:   []string{"eu-west-1"},
			MatchTags: []string{"env=prod"},
		},
		{
			Regions:   []string{"eu-west-2"},
			MatchTags: []string{"env=staging"},
		},
	}, awsStructured.Resources[0].Scopes)
	require.Equal(t, resultSummary{
		Kind: "counts",
		Counts: &resultCounts{
			Found:    10,
			Enrolled: 8,
			Failed:   2,
		},
	}, awsStructured.Resources[0].Result)

	azureStructured := requireConfigSummary(t, structured, "azure-config")
	require.Equal(t, summaryStatusNotReporting, azureStructured.Status.State)
	require.Nil(t, azureStructured.Status.LastRun)
	require.Len(t, azureStructured.Resources, 1)
	require.Equal(t, "ambient_credentials", azureStructured.Resources[0].Source)
	require.Empty(t, azureStructured.Resources[0].Integration)
	require.Equal(t, []resourceScope{
		{
			Regions:        []string{"eastus"},
			Subscriptions:  []string{"sub-1"},
			ResourceGroups: []string{"rg-1"},
			MatchTags:      []string{"all"},
		},
	}, azureStructured.Resources[0].Scopes)
	require.Equal(t, "not_reporting", azureStructured.Resources[0].Result.Kind)
	require.Equal(t, "no status reported by a Discovery Service", azureStructured.Resources[0].Result.Message)
}

func TestBuildDiscoveryTextSummariesFromConfigsGlobalErrorWithMultipleScopes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	lastRun := now.Add(-5 * time.Minute)
	errorMessage := "AccessDenied: missing ec2:DescribeInstances permission"
	awsConfig, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: "aws-config"},
		discoveryconfig.Spec{
			DiscoveryGroup: "prod",
			AWS: []types.AWSMatcher{
				{
					Types:       []string{types.AWSMatcherEC2},
					Regions:     []string{"eu-west-1"},
					Integration: "aws-prod",
					Tags:        types.Labels{"env": {"prod"}},
					Params: &types.InstallerParams{
						EnrollMode: types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
					},
				},
				{
					Types:       []string{types.AWSMatcherEC2},
					Regions:     []string{"eu-west-2"},
					Integration: "aws-prod",
					Tags:        types.Labels{"env": {"staging"}},
					Params: &types.InstallerParams{
						EnrollMode: types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
					},
				},
			},
		},
	)
	require.NoError(t, err)
	awsConfig.Status = discoveryconfig.Status{
		State:        discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_ERROR.String(),
		LastSyncTime: lastRun,
		ErrorMessage: &errorMessage,
	}

	summaries := newDiscoverySummary(
		[]*discoveryconfig.DiscoveryConfig{awsConfig},
		cloudProviderConfig{aws: true},
	)
	require.Len(t, summaries, 1)

	awsConfigSummary := requireConfigSummary(t, summaries, "aws-config")
	require.Equal(t, "error", awsConfigSummary.Status.State)
	require.Equal(t, errorMessage, awsConfigSummary.Status.ErrorMessage)
	require.NotNil(t, awsConfigSummary.Status.LastRun)
	require.Equal(t, lastRun, *awsConfigSummary.Status.LastRun)
	require.Len(t, awsConfigSummary.Resources, 1)
	awsResource := requireResourceSummary(t, awsConfigSummary.Resources, cloudAWS, "EC2", "aws-prod")
	require.Equal(t, "no_resource_status", awsResource.Result.Kind)
	require.Equal(t, []resourceScope{
		{
			Regions:   []string{"eu-west-1"},
			MatchTags: []string{"env=prod"},
		},
		{
			Regions:   []string{"eu-west-2"},
			MatchTags: []string{"env=staging"},
		},
	}, awsResource.Scopes)

	var buf bytes.Buffer
	require.NoError(t, summaries.renderText(&buf, now))
	out := buf.String()
	requireNoTrailingWhitespace(t, out)
	require.Equal(t, 1, strings.Count(out, "Discovery config status"))
	require.Equal(t, 1, strings.Count(out, errorMessage))
	require.Contains(t, out, "Status:          error")
	require.Contains(t, out, "Last run:        5 minutes ago")
	require.Contains(t, out, "Last run:        5 minutes ago\nError:")
	require.Contains(t, out, errorMessage+"\n\nAWS EC2 discovery")
	require.Contains(t, out, "Matcher scopes:")
	require.Contains(t, out, "- Regions: eu-west-1; Match tags: env=prod")
	require.Contains(t, out, "- Regions: eu-west-2; Match tags: env=staging")
	require.Contains(t, out, "Result:         no resource status reported for this discovery target")
}

func TestRenderSummaryTextGroupsMultipleResourcesByConfig(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	lastRun := now.Add(-2 * time.Minute)
	awsConfig, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: "aws-config"},
		discoveryconfig.Spec{
			DiscoveryGroup: "prod",
			AWS: []types.AWSMatcher{
				{
					Types:       []string{types.AWSMatcherEC2},
					Regions:     []string{"eu-west-1"},
					Integration: "aws-prod",
					Tags:        types.Labels{"env": {"prod"}},
					Params: &types.InstallerParams{
						EnrollMode: types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
					},
				},
				{
					Types:       []string{types.AWSMatcherRDS},
					Regions:     []string{"eu-west-1"},
					Integration: "aws-prod",
					Tags:        types.Labels{"env": {"prod"}},
				},
			},
		},
	)
	require.NoError(t, err)
	awsConfig.Status = discoveryconfig.Status{
		State:        discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String(),
		LastSyncTime: lastRun,
		IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
			"aws-prod": {
				IntegrationDiscoveredSummary: discoveryconfigv1.IntegrationDiscoveredSummary_builder{
					AwsEc2: discoveryResourcesSummary(10, 8, 2, lastRun),
					AwsRds: discoveryResourcesSummary(5, 4, 1, lastRun),
				}.Build(),
			},
		},
	}

	summaries := newDiscoverySummary(
		[]*discoveryconfig.DiscoveryConfig{awsConfig},
		cloudProviderConfig{aws: true},
	)
	require.Len(t, summaries, 1)
	awsConfigSummary := requireConfigSummary(t, summaries, "aws-config")
	require.Len(t, awsConfigSummary.Resources, 2)
	requireResourceSummary(t, awsConfigSummary.Resources, cloudAWS, "EC2", "aws-prod")
	requireResourceSummary(t, awsConfigSummary.Resources, cloudAWS, "database", "aws-prod")

	var buf bytes.Buffer
	require.NoError(t, summaries.renderText(&buf, now))
	out := buf.String()
	requireNoTrailingWhitespace(t, out)
	require.Equal(t, 1, strings.Count(out, "Discovery config status"))
	require.Contains(t, out, "AWS EC2 discovery")
	require.Contains(t, out, "10 found, 8 enrolled, 2 failed")
	require.Contains(t, out, "10 found, 8 enrolled, 2 failed\n\nAWS database discovery")
	require.Contains(t, out, "AWS database discovery")
	require.Contains(t, out, "5 found, 4 enrolled, 1 failed")
}

func TestSummarizeConfigStatus(t *testing.T) {
	t.Parallel()

	lastRun := time.Date(2026, 1, 15, 11, 58, 0, 0, time.UTC)
	errorMessage := "AccessDenied: missing ec2:DescribeInstances permission"

	tests := []struct {
		name   string
		status discoveryconfig.Status
		want   configStatus
	}{
		{
			name:   "empty status is not reporting",
			status: discoveryconfig.Status{},
			want: configStatus{
				State: summaryStatusNotReporting,
			},
		},
		{
			name: "unspecified state is not reporting",
			status: discoveryconfig.Status{
				State: discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_UNSPECIFIED.String(),
			},
			want: configStatus{
				State: summaryStatusNotReporting,
			},
		},
		{
			name: "last sync time reports status",
			status: discoveryconfig.Status{
				LastSyncTime: lastRun,
			},
			want: configStatus{
				Reported: true,
				State:    "reported",
				LastRun:  &lastRun,
			},
		},
		{
			name: "integration resources report status",
			status: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
					"aws-prod": nil,
				},
			},
			want: configStatus{
				Reported: true,
				State:    "reported",
			},
		},
		{
			name: "running state is healthy",
			status: discoveryconfig.Status{
				State: discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String(),
			},
			want: configStatus{
				Reported: true,
				State:    "healthy",
			},
		},
		{
			name: "syncing state is syncing",
			status: discoveryconfig.Status{
				State: discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_SYNCING.String(),
			},
			want: configStatus{
				Reported: true,
				State:    "syncing",
			},
		},
		{
			name: "error state is error",
			status: discoveryconfig.Status{
				State: discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_ERROR.String(),
			},
			want: configStatus{
				Reported: true,
				State:    "error",
			},
		},
		{
			name: "error message is error",
			status: discoveryconfig.Status{
				State:        discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String(),
				ErrorMessage: &errorMessage,
			},
			want: configStatus{
				Reported:     true,
				State:        "error",
				ErrorMessage: errorMessage,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := summarizeConfigStatus(tt.status)
			require.Equal(t, tt.want.Reported, got.Reported)
			require.Equal(t, tt.want.State, got.State)
			require.Equal(t, tt.want.ErrorMessage, got.ErrorMessage)
			if tt.want.LastRun == nil {
				require.Nil(t, got.LastRun)
				return
			}
			require.NotNil(t, got.LastRun)
			require.Equal(t, *tt.want.LastRun, *got.LastRun)
		})
	}
}

func TestResolveSummaryResult(t *testing.T) {
	t.Parallel()

	lastRun := time.Date(2026, 1, 15, 11, 58, 0, 0, time.UTC)
	errorMessage := "AccessDenied: missing ec2:DescribeInstances permission"

	tests := []struct {
		name        string
		status      configStatus
		supports    bool
		counts      *discoveryconfigv1.ResourcesDiscoveredSummary
		want        resultSummary
		wantLastRun *time.Time
	}{
		{
			name: "error status does not override resource counts",
			status: configStatus{
				Reported:     true,
				ErrorMessage: errorMessage,
			},
			supports: true,
			counts:   discoveryResourcesSummary(10, 8, 2, lastRun),
			want: resultSummary{
				Kind: "counts",
				Counts: &resultCounts{
					Found:    10,
					Enrolled: 8,
					Failed:   2,
				},
			},
			wantLastRun: &lastRun,
		},
		{
			name:     "no reported status",
			supports: true,
			want: resultSummary{
				Kind:    "not_reporting",
				Message: "no status reported by a Discovery Service",
			},
		},
		{
			name: "reported unsupported resource",
			status: configStatus{
				Reported: true,
			},
			want: resultSummary{
				Kind:    "unsupported",
				Message: "detailed counts are not available for this resource type",
			},
		},
		{
			name: "reported supported resource without resource status",
			status: configStatus{
				Reported: true,
			},
			supports: true,
			want: resultSummary{
				Kind:    "no_resource_status",
				Message: "no resource status reported for this discovery target",
			},
		},
		{
			name: "reported counts",
			status: configStatus{
				Reported: true,
			},
			supports: true,
			counts:   discoveryResourcesSummary(10, 8, 2, lastRun),
			want: resultSummary{
				Kind: "counts",
				Counts: &resultCounts{
					Found:    10,
					Enrolled: 8,
					Failed:   2,
				},
			},
			wantLastRun: &lastRun,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, lastRun := resolveSummaryResult(tt.status.Reported, tt.supports, tt.counts)
			require.Equal(t, tt.want, got)
			if tt.wantLastRun == nil {
				require.Nil(t, lastRun)
				return
			}
			require.NotNil(t, lastRun)
			require.Equal(t, *tt.wantLastRun, *lastRun)
		})
	}
}

func TestConfigSummaryStructuredOutputGolden(t *testing.T) {
	t.Parallel()

	lastRun := time.Date(2026, 1, 15, 11, 58, 0, 0, time.UTC)
	errorRun := time.Date(2026, 1, 15, 12, 1, 0, 0, time.UTC)
	summaries := []configSummary{
		{
			Name:           "healthy-config",
			DiscoveryGroup: "prod",
			Status: configStatus{
				Reported: true,
				State:    "healthy",
				LastRun:  &lastRun,
			},
			Resources: []resourceSummary{
				{
					Cloud:        cloudAWS,
					ResourceType: "EC2",
					Source:       "integration",
					Integration:  "aws-prod",
					Scopes: []resourceScope{
						{
							Regions:   []string{"eu-west-1"},
							MatchTags: []string{"env=prod"},
						},
						{
							Regions:   []string{"eu-west-2"},
							MatchTags: []string{"env=staging"},
						},
					},
					LastSync: &lastRun,
					Result: resultSummary{
						Kind: "counts",
						Counts: &resultCounts{
							Found:    10,
							Enrolled: 8,
							Failed:   2,
						},
					},
				},
				{
					Cloud:        cloudAzure,
					ResourceType: "VM",
					Source:       "ambient_credentials",
					Scopes: []resourceScope{
						{
							Regions:        []string{"eastus"},
							Subscriptions:  []string{"sub-1"},
							ResourceGroups: []string{"rg-1"},
							MatchTags:      []string{"all"},
						},
					},
					Result: resultSummary{
						Kind:    "not_reporting",
						Message: "no status reported by a Discovery Service",
					},
				},
			},
		},
		{
			Name:           "error-config",
			DiscoveryGroup: "prod",
			Status: configStatus{
				Reported:     true,
				State:        "error",
				LastRun:      &errorRun,
				ErrorMessage: "AccessDenied: missing ec2:DescribeInstances permission",
			},
			Resources: []resourceSummary{
				{
					Cloud:        cloudAWS,
					ResourceType: "database",
					Source:       "integration",
					Integration:  "aws-prod",
					Scopes: []resourceScope{
						{
							Regions:   []string{"eu-west-1"},
							MatchTags: []string{"env=prod"},
						},
					},
					Result: resultSummary{
						Kind:    "no_resource_status",
						Message: "no resource status reported for this discovery target",
					},
				},
			},
		},
	}

	var jsonBuf bytes.Buffer
	require.NoError(t, utils.WriteJSONArray(&jsonBuf, summaries))
	require.Equal(t, `[
    {
        "name": "healthy-config",
        "discovery_group": "prod",
        "status": {
            "state": "healthy",
            "last_run": "2026-01-15T11:58:00Z"
        },
        "resources": [
            {
                "cloud": "AWS",
                "resource_type": "EC2",
                "source": "integration",
                "integration": "aws-prod",
                "scopes": [
                    {
                        "regions": [
                            "eu-west-1"
                        ],
                        "match_tags": [
                            "env=prod"
                        ]
                    },
                    {
                        "regions": [
                            "eu-west-2"
                        ],
                        "match_tags": [
                            "env=staging"
                        ]
                    }
                ],
                "last_resource_sync": "2026-01-15T11:58:00Z",
                "result": {
                    "kind": "counts",
                    "counts": {
                        "found": 10,
                        "enrolled": 8,
                        "failed": 2
                    }
                }
            },
            {
                "cloud": "Azure",
                "resource_type": "VM",
                "source": "ambient_credentials",
                "scopes": [
                    {
                        "regions": [
                            "eastus"
                        ],
                        "subscriptions": [
                            "sub-1"
                        ],
                        "resource_groups": [
                            "rg-1"
                        ],
                        "match_tags": [
                            "all"
                        ]
                    }
                ],
                "result": {
                    "kind": "not_reporting",
                    "message": "no status reported by a Discovery Service"
                }
            }
        ]
    },
    {
        "name": "error-config",
        "discovery_group": "prod",
        "status": {
            "state": "error",
            "last_run": "2026-01-15T12:01:00Z",
            "error_message": "AccessDenied: missing ec2:DescribeInstances permission"
        },
        "resources": [
            {
                "cloud": "AWS",
                "resource_type": "database",
                "source": "integration",
                "integration": "aws-prod",
                "scopes": [
                    {
                        "regions": [
                            "eu-west-1"
                        ],
                        "match_tags": [
                            "env=prod"
                        ]
                    }
                ],
                "result": {
                    "kind": "no_resource_status",
                    "message": "no resource status reported for this discovery target"
                }
            }
        ]
    }
]
`, jsonBuf.String())

	var yamlBuf bytes.Buffer
	require.NoError(t, utils.WriteYAML(&yamlBuf, summaries))
	require.Equal(t, `discovery_group: prod
name: healthy-config
resources:
- cloud: AWS
  integration: aws-prod
  last_resource_sync: "2026-01-15T11:58:00Z"
  resource_type: EC2
  result:
    counts:
      enrolled: 8
      failed: 2
      found: 10
    kind: counts
  scopes:
  - match_tags:
    - env=prod
    regions:
    - eu-west-1
  - match_tags:
    - env=staging
    regions:
    - eu-west-2
  source: integration
- cloud: Azure
  resource_type: VM
  result:
    kind: not_reporting
    message: no status reported by a Discovery Service
  scopes:
  - match_tags:
    - all
    regions:
    - eastus
    resource_groups:
    - rg-1
    subscriptions:
    - sub-1
  source: ambient_credentials
status:
  last_run: "2026-01-15T11:58:00Z"
  state: healthy
---
discovery_group: prod
name: error-config
resources:
- cloud: AWS
  integration: aws-prod
  resource_type: database
  result:
    kind: no_resource_status
    message: no resource status reported for this discovery target
  scopes:
  - match_tags:
    - env=prod
    regions:
    - eu-west-1
  source: integration
status:
  error_message: 'AccessDenied: missing ec2:DescribeInstances permission'
  last_run: "2026-01-15T12:01:00Z"
  state: error
`, yamlBuf.String())
}

func TestFormatMatchTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		labels types.Labels
		want   string
	}{
		{
			name: "empty labels match all",
			want: "all",
		},
		{
			name: "wildcard labels match all",
			labels: types.Labels{
				types.Wildcard: {types.Wildcard},
			},
			want: "all",
		},
		{
			name: "single value",
			labels: types.Labels{
				"env": {"prod"},
			},
			want: "env=prod",
		},
		{
			name: "multiple values are sorted",
			labels: types.Labels{
				"env": {"staging", "prod"},
			},
			want: "env in (prod, staging)",
		},
		{
			name: "keys are sorted",
			labels: types.Labels{
				"team": {"platform"},
				"env":  {"prod"},
			},
			want: "env=prod, team=platform",
		},
		{
			name: "key with no values",
			labels: types.Labels{
				"env": nil,
			},
			want: "env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, formatMatchTags(tt.labels))
		})
	}
}

func discoveryResourcesSummary(found, enrolled, failed uint64, syncEnd time.Time) *discoveryconfigv1.ResourcesDiscoveredSummary {
	return discoveryconfigv1.ResourcesDiscoveredSummary_builder{
		Found:    found,
		Enrolled: enrolled,
		Failed:   failed,
		SyncEnd:  timestamppb.New(syncEnd),
	}.Build()
}

func requireConfigSummary(t *testing.T, summaries discoverySummary, name string) configSummary {
	t.Helper()
	for _, summary := range summaries {
		if summary.Name == name {
			return summary
		}
	}
	require.FailNowf(t, "config summary not found", "name=%q", name)
	return configSummary{}
}

func requireResourceSummary(t *testing.T, resources []resourceSummary, cloud, resourceType, integration string) resourceSummary {
	t.Helper()
	for _, resource := range resources {
		if resource.Cloud == cloud && resource.ResourceType == resourceType && resource.Integration == integration {
			return resource
		}
	}
	require.FailNowf(t, "resource summary not found", "cloud=%q resource_type=%q integration=%q", cloud, resourceType, integration)
	return resourceSummary{}
}
