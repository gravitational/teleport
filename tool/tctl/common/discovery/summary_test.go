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

	blocks := buildSummaryBlocks(
		[]*discoveryconfig.DiscoveryConfig{awsConfig, azureConfig},
		cloudProviderConfig{aws: true, azure: true},
		"",
	)
	require.Len(t, blocks, 2)

	awsSummary := requireTextSummary(t, blocks, cloudAWS, "EC2", "aws-prod")
	require.Equal(t, "healthy", awsSummary.Status)
	require.Equal(t, resultCounts, awsSummary.Result.Kind)
	require.Equal(t, uint64(10), awsSummary.Result.Found)
	require.Equal(t, uint64(8), awsSummary.Result.Enrolled)
	require.Equal(t, uint64(2), awsSummary.Result.Failed)
	require.ElementsMatch(t, []string{"eu-west-1", "eu-west-2"}, awsSummary.Regions)
	require.ElementsMatch(t, []string{"env=prod", "env=staging"}, awsSummary.MatchTags)
	require.NotNil(t, awsSummary.LastRun)
	require.Equal(t, lastRun, *awsSummary.LastRun)

	azureSummary := requireTextSummary(t, blocks, cloudAzure, "VM", "")
	require.Equal(t, "not reporting yet", azureSummary.Status)
	require.Equal(t, resultNotReporting, azureSummary.Result.Kind)
	require.ElementsMatch(t, []string{"eastus"}, azureSummary.Regions)
	require.ElementsMatch(t, []string{"sub-1"}, azureSummary.Subscriptions)
	require.ElementsMatch(t, []string{"rg-1"}, azureSummary.ResourceGroups)
	require.ElementsMatch(t, []string{"all"}, azureSummary.MatchTags)
	require.Nil(t, azureSummary.LastRun)
}

func discoveryResourcesSummary(found, enrolled, failed uint64, syncEnd time.Time) *discoveryconfigv1.ResourcesDiscoveredSummary {
	return discoveryconfigv1.ResourcesDiscoveredSummary_builder{
		Found:    found,
		Enrolled: enrolled,
		Failed:   failed,
		SyncEnd:  timestamppb.New(syncEnd),
	}.Build()
}

func requireTextSummary(t *testing.T, blocks []summaryBlock, cloud, resourceType, integration string) summaryBlock {
	t.Helper()
	for _, block := range blocks {
		if block.Cloud == cloud && block.ResourceType == resourceType && block.Integration == integration {
			return block
		}
	}
	require.FailNowf(t, "summary not found", "cloud=%q resource_type=%q integration=%q", cloud, resourceType, integration)
	return summaryBlock{}
}
