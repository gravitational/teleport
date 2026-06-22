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

	summaries := buildConfigSummaries(
		[]*discoveryconfig.DiscoveryConfig{awsConfig, azureConfig},
		cloudProviderConfig{aws: true, azure: true},
	)
	require.Len(t, summaries, 2)

	awsConfigSummary := requireConfigSummary(t, summaries, "aws-config")
	require.Equal(t, "prod", awsConfigSummary.DiscoveryGroup)
	require.Equal(t, "healthy", awsConfigSummary.Status.label)
	require.NotNil(t, awsConfigSummary.Status.lastRun)
	require.Equal(t, lastRun, *awsConfigSummary.Status.lastRun)
	awsResource := requireResourceSummary(t, awsConfigSummary.Resources, cloudAWS, "EC2", "aws-prod")
	require.Equal(t, resultCounts, awsResource.Result.Kind)
	require.Equal(t, uint64(10), awsResource.Result.Found)
	require.Equal(t, uint64(8), awsResource.Result.Enrolled)
	require.Equal(t, uint64(2), awsResource.Result.Failed)
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
	require.Equal(t, summaryStatusNotReporting, azureConfigSummary.Status.label)
	require.Nil(t, azureConfigSummary.Status.lastRun)
	azureResource := requireResourceSummary(t, azureConfigSummary.Resources, cloudAzure, "VM", "")
	require.Equal(t, resultNotReporting, azureResource.Result.Kind)
	require.Equal(t, []resourceScope{
		{
			Regions:        []string{"eastus"},
			Subscriptions:  []string{"sub-1"},
			ResourceGroups: []string{"rg-1"},
			MatchTags:      []string{"all"},
		},
	}, azureResource.Scopes)
	require.Nil(t, azureResource.LastSync)

	structured := buildStructuredSummaries(summaries)
	require.Len(t, structured, 2)

	awsStructured := requireStructuredSummary(t, structured, "aws-config")
	require.Equal(t, "prod", awsStructured.DiscoveryGroup)
	require.Equal(t, "healthy", awsStructured.Status.State)
	require.NotNil(t, awsStructured.Status.LastRun)
	require.Equal(t, lastRun, *awsStructured.Status.LastRun)
	require.Len(t, awsStructured.Resources, 1)
	require.Equal(t, cloudAWS, awsStructured.Resources[0].Cloud)
	require.Equal(t, "EC2", awsStructured.Resources[0].ResourceType)
	require.Equal(t, "integration", awsStructured.Resources[0].Source)
	require.Equal(t, "aws-prod", awsStructured.Resources[0].Integration)
	require.Equal(t, []structuredSummaryScope{
		{
			Regions:   []string{"eu-west-1"},
			MatchTags: []string{"env=prod"},
		},
		{
			Regions:   []string{"eu-west-2"},
			MatchTags: []string{"env=staging"},
		},
	}, awsStructured.Resources[0].Scopes)
	require.Equal(t, structuredSummaryResult{
		Kind: "counts",
		Counts: &structuredSummaryCounts{
			Found:    10,
			Enrolled: 8,
			Failed:   2,
		},
	}, awsStructured.Resources[0].Result)

	azureStructured := requireStructuredSummary(t, structured, "azure-config")
	require.Equal(t, summaryStatusNotReporting, azureStructured.Status.State)
	require.Nil(t, azureStructured.Status.LastRun)
	require.Len(t, azureStructured.Resources, 1)
	require.Equal(t, "ambient_credentials", azureStructured.Resources[0].Source)
	require.Empty(t, azureStructured.Resources[0].Integration)
	require.Equal(t, []structuredSummaryScope{
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

	summaries := buildConfigSummaries(
		[]*discoveryconfig.DiscoveryConfig{awsConfig},
		cloudProviderConfig{aws: true},
	)
	require.Len(t, summaries, 1)

	awsConfigSummary := requireConfigSummary(t, summaries, "aws-config")
	require.Equal(t, "error", awsConfigSummary.Status.label)
	require.Equal(t, errorMessage, awsConfigSummary.Status.errorMessage)
	require.NotNil(t, awsConfigSummary.Status.lastRun)
	require.Equal(t, lastRun, *awsConfigSummary.Status.lastRun)
	require.Len(t, awsConfigSummary.Resources, 1)
	awsResource := requireResourceSummary(t, awsConfigSummary.Resources, cloudAWS, "EC2", "aws-prod")
	require.Equal(t, resultNoResourceStatus, awsResource.Result.Kind)
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
	require.NoError(t, renderSummaryText(&buf, summaries, now))
	out := buf.String()
	require.Equal(t, 1, strings.Count(out, "Discovery config status"))
	require.Equal(t, 1, strings.Count(out, errorMessage))
	require.Contains(t, out, "Status:          error")
	require.Contains(t, out, "Last run:        5 minutes ago")
	require.Contains(t, out, "Matcher scopes:")
	require.Contains(t, out, "- Regions: eu-west-1; Match tags: env=prod")
	require.Contains(t, out, "- Regions: eu-west-2; Match tags: env=staging")
	require.Contains(t, out, "Result:          no resource status reported for this discovery target")
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

	summaries := buildConfigSummaries(
		[]*discoveryconfig.DiscoveryConfig{awsConfig},
		cloudProviderConfig{aws: true},
	)
	require.Len(t, summaries, 1)
	awsConfigSummary := requireConfigSummary(t, summaries, "aws-config")
	require.Len(t, awsConfigSummary.Resources, 2)
	requireResourceSummary(t, awsConfigSummary.Resources, cloudAWS, "EC2", "aws-prod")
	requireResourceSummary(t, awsConfigSummary.Resources, cloudAWS, "database", "aws-prod")

	var buf bytes.Buffer
	require.NoError(t, renderSummaryText(&buf, summaries, now))
	out := buf.String()
	require.Equal(t, 1, strings.Count(out, "Discovery config status"))
	require.Contains(t, out, "AWS EC2 discovery")
	require.Contains(t, out, "10 found, 8 enrolled, 2 failed")
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
		want   configStatusSummary
	}{
		{
			name:   "empty status is not reporting",
			status: discoveryconfig.Status{},
			want: configStatusSummary{
				label: summaryStatusNotReporting,
			},
		},
		{
			name: "unspecified state is not reporting",
			status: discoveryconfig.Status{
				State: discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_UNSPECIFIED.String(),
			},
			want: configStatusSummary{
				label: summaryStatusNotReporting,
			},
		},
		{
			name: "last sync time reports status",
			status: discoveryconfig.Status{
				LastSyncTime: lastRun,
			},
			want: configStatusSummary{
				reported: true,
				label:    "reported",
				lastRun:  &lastRun,
			},
		},
		{
			name: "integration resources report status",
			status: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
					"aws-prod": nil,
				},
			},
			want: configStatusSummary{
				reported: true,
				label:    "reported",
			},
		},
		{
			name: "running state is healthy",
			status: discoveryconfig.Status{
				State: discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String(),
			},
			want: configStatusSummary{
				reported: true,
				label:    "healthy",
			},
		},
		{
			name: "syncing state is syncing",
			status: discoveryconfig.Status{
				State: discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_SYNCING.String(),
			},
			want: configStatusSummary{
				reported: true,
				label:    "syncing",
			},
		},
		{
			name: "error state is error",
			status: discoveryconfig.Status{
				State: discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_ERROR.String(),
			},
			want: configStatusSummary{
				reported: true,
				label:    "error",
			},
		},
		{
			name: "error message is error",
			status: discoveryconfig.Status{
				State:        discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String(),
				ErrorMessage: &errorMessage,
			},
			want: configStatusSummary{
				reported:     true,
				label:        "error",
				errorMessage: errorMessage,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := summarizeConfigStatus(tt.status)
			require.Equal(t, tt.want.reported, got.reported)
			require.Equal(t, tt.want.label, got.label)
			require.Equal(t, tt.want.errorMessage, got.errorMessage)
			if tt.want.lastRun == nil {
				require.Nil(t, got.lastRun)
				return
			}
			require.NotNil(t, got.lastRun)
			require.Equal(t, *tt.want.lastRun, *got.lastRun)
		})
	}
}

func TestResolveSummaryResult(t *testing.T) {
	t.Parallel()

	lastRun := time.Date(2026, 1, 15, 11, 58, 0, 0, time.UTC)
	errorMessage := "AccessDenied: missing ec2:DescribeInstances permission"

	tests := []struct {
		name        string
		status      configStatusSummary
		supports    bool
		counts      *discoveryconfigv1.ResourcesDiscoveredSummary
		want        summaryResult
		wantLastRun *time.Time
	}{
		{
			name: "error status does not override resource counts",
			status: configStatusSummary{
				reported:     true,
				errorMessage: errorMessage,
			},
			supports: true,
			counts:   discoveryResourcesSummary(10, 8, 2, lastRun),
			want: summaryResult{
				Kind:     resultCounts,
				Found:    10,
				Enrolled: 8,
				Failed:   2,
			},
			wantLastRun: &lastRun,
		},
		{
			name:     "no reported status",
			supports: true,
			want: summaryResult{
				Kind: resultNotReporting,
			},
		},
		{
			name: "reported unsupported resource",
			status: configStatusSummary{
				reported: true,
			},
			want: summaryResult{
				Kind: resultUnsupported,
			},
		},
		{
			name: "reported supported resource without resource status",
			status: configStatusSummary{
				reported: true,
			},
			supports: true,
			want: summaryResult{
				Kind: resultNoResourceStatus,
			},
		},
		{
			name: "reported counts",
			status: configStatusSummary{
				reported: true,
			},
			supports: true,
			counts:   discoveryResourcesSummary(10, 8, 2, lastRun),
			want: summaryResult{
				Kind:     resultCounts,
				Found:    10,
				Enrolled: 8,
				Failed:   2,
			},
			wantLastRun: &lastRun,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, lastRun := resolveSummaryResult(tt.status.reported, tt.supports, tt.counts)
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

func requireConfigSummary(t *testing.T, summaries []configSummary, name string) configSummary {
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

func requireStructuredSummary(t *testing.T, summaries []structuredSummary, name string) structuredSummary {
	t.Helper()
	for _, summary := range summaries {
		if summary.Name == name {
			return summary
		}
	}
	require.FailNowf(t, "structured summary not found", "name=%q", name)
	return structuredSummary{}
}
