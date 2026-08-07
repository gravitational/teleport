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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/utils"
)

func TestBuildDiscoverySummaryFromServerStatus(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	multiConfig := newTestDiscoveryConfig(t, "multi-config", "prod")
	multiConfig.Status = discoveryconfig.Status{
		State:        discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String(),
		LastSyncTime: now.Add(-2 * time.Minute),
		ServerStatus: map[string]*discoveryconfig.DiscoveryStatusServer{
			"server-b": testServerStatus(now.Add(-3*time.Minute), 10*time.Minute, map[string]*discoveryconfigv1.DiscoverSummary{
				"": testDiscoverSummary(
					withPreviousRDS(discoveryResourcesSummary(3, 2, 1, now.Add(-5*time.Minute), now.Add(-4*time.Minute))),
					withPreviousAzureVM(discoveryResourcesSummary(7, 6, 1, now.Add(-4*time.Minute), now.Add(-3*time.Minute))),
				),
				"azure-prod": testDiscoverSummary(
					withPreviousAzureVM(discoveryResourcesSummary(4, 4, 0, now.Add(-3*time.Minute), now.Add(-2*time.Minute))),
				),
			}),
			"server-a": testServerStatus(now.Add(-time.Minute), 5*time.Minute, map[string]*discoveryconfigv1.DiscoverSummary{
				"": testDiscoverSummary(
					withPreviousEC2(discoveryResourcesSummary(10, 8, 2, now.Add(-5*time.Minute), now.Add(-4*time.Minute-time.Second))),
					withPreviousRDS(discoveryResourcesSummary(5, 4, 1, now.Add(-4*time.Minute), now.Add(-3*time.Minute-time.Second))),
				),
				"aws-prod": testDiscoverSummary(
					withPreviousEC2(discoveryResourcesSummary(8, 7, 1, now.Add(-3*time.Minute), now.Add(-2*time.Minute-time.Second))),
					withCurrentEKS(discoveryResourcesSummary(99, 0, 0, now.Add(-time.Minute), now)),
					withPreviousRDS(discoveryResourcesSummary(6, 6, 0, now.Add(-2*time.Minute), now.Add(-time.Minute-time.Second))),
				),
			}),
		},
	}

	errorMessage := "AccessDenied: missing ec2:DescribeInstances permission"
	noServiceConfig := newTestDiscoveryConfig(t, "empty-config", "prod")
	noServiceConfig.Status = discoveryconfig.Status{
		State:        discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_ERROR.String(),
		LastSyncTime: now.Add(-5 * time.Minute),
		ErrorMessage: &errorMessage,
	}

	summaries := newDiscoverySummary(
		[]*discoveryconfig.DiscoveryConfig{multiConfig, noServiceConfig},
		cloudProviderConfig{aws: true, azure: true},
	)
	require.Len(t, summaries, 2)

	emptySummary := requireConfigSummary(t, summaries, "empty-config")
	require.Equal(t, "prod", emptySummary.DiscoveryGroup)
	require.Equal(t, discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_ERROR.String(), emptySummary.State)
	require.Equal(t, errorMessage, emptySummary.ErrorMessage)
	require.NotNil(t, emptySummary.LastSyncTime)
	require.Equal(t, now.Add(-5*time.Minute), *emptySummary.LastSyncTime)
	require.Empty(t, emptySummary.Servers)

	multiSummary := requireConfigSummary(t, summaries, "multi-config")
	require.Equal(t, discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String(), multiSummary.State)
	require.NotNil(t, multiSummary.LastSyncTime)
	require.Equal(t, now.Add(-2*time.Minute), *multiSummary.LastSyncTime)
	require.Len(t, multiSummary.Servers, 2)
	require.Equal(t, "server-a", multiSummary.Servers[0].ServerID)
	require.Equal(t, "server-b", multiSummary.Servers[1].ServerID)
	require.Equal(t, "5m0s", multiSummary.Servers[0].PollInterval)
	require.NotNil(t, multiSummary.Servers[0].LastUpdate)
	require.Equal(t, now.Add(-time.Minute), *multiSummary.Servers[0].LastUpdate)

	serverA := multiSummary.Servers[0]
	require.Len(t, serverA.Integrations, 2)
	require.Empty(t, serverA.Integrations[0].Integration)
	require.Equal(t, "aws-prod", serverA.Integrations[1].Integration)
	require.Equal(t, []string{resourceKindAWSEC2, resourceKindAWSRDS}, resourceKinds(serverA.Integrations[0].Resources))
	require.Equal(t, []string{resourceKindAWSEC2, resourceKindAWSRDS}, resourceKinds(serverA.Integrations[1].Resources), "current-only EKS summary must be skipped")
	require.Equal(t, resourceResult{
		Kind:      resourceKindAWSEC2,
		Found:     10,
		Enrolled:  8,
		Failed:    2,
		SyncStart: new(now.Add(-5 * time.Minute)),
		SyncEnd:   new(now.Add(-4*time.Minute - time.Second)),
	}, serverA.Integrations[0].Resources[0])

	serverB := multiSummary.Servers[1]
	require.Len(t, serverB.Integrations, 2)
	require.Empty(t, serverB.Integrations[0].Integration)
	require.Equal(t, "azure-prod", serverB.Integrations[1].Integration)
	require.Equal(t, []string{resourceKindAWSRDS, resourceKindAzureVM}, resourceKinds(serverB.Integrations[0].Resources))
	require.Equal(t, []string{resourceKindAzureVM}, resourceKinds(serverB.Integrations[1].Resources))
}

func TestBuildDiscoverySummaryCloudFilter(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	config := newTestDiscoveryConfig(t, "multi-cloud", "prod")
	config.Status = discoveryconfig.Status{
		State: discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String(),
		ServerStatus: map[string]*discoveryconfig.DiscoveryStatusServer{
			"server-a": testServerStatus(now, time.Minute, map[string]*discoveryconfigv1.DiscoverSummary{
				"": testDiscoverSummary(
					withPreviousEC2(discoveryResourcesSummary(10, 8, 2, now.Add(-time.Minute), now)),
					withPreviousAzureVM(discoveryResourcesSummary(7, 6, 1, now.Add(-time.Minute), now)),
				),
			}),
		},
	}

	awsSummary := newDiscoverySummary([]*discoveryconfig.DiscoveryConfig{config}, cloudProviderConfig{aws: true})
	require.Equal(t, []string{resourceKindAWSEC2}, resourceKinds(awsSummary[0].Servers[0].Integrations[0].Resources))

	azureSummary := newDiscoverySummary([]*discoveryconfig.DiscoveryConfig{config}, cloudProviderConfig{azure: true})
	require.Equal(t, []string{resourceKindAzureVM}, resourceKinds(azureSummary[0].Servers[0].Integrations[0].Resources))
}

func TestConfigSummaryStructuredOutputGolden(t *testing.T) {
	t.Parallel()

	lastRun := time.Date(2026, 1, 15, 11, 58, 0, 0, time.UTC)
	lastUpdate := time.Date(2026, 1, 15, 11, 59, 0, 0, time.UTC)
	syncStart := time.Date(2026, 1, 15, 11, 55, 0, 0, time.UTC)
	syncEnd := time.Date(2026, 1, 15, 11, 56, 0, 0, time.UTC)
	errorRun := time.Date(2026, 1, 15, 11, 55, 0, 0, time.UTC)

	summaries := discoverySummary{
		{
			Name:           "healthy-config",
			DiscoveryGroup: "prod",
			State:          discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String(),
			LastSyncTime:   &lastRun,
			Servers: []serverSummary{
				{
					ServerID:     "server-a",
					PollInterval: "5m0s",
					LastUpdate:   &lastUpdate,
					Integrations: []integrationSummary{
						{
							Resources: []resourceResult{
								{
									Kind:      resourceKindAWSEC2,
									Found:     10,
									Enrolled:  8,
									Failed:    2,
									SyncStart: &syncStart,
									SyncEnd:   &syncEnd,
								},
							},
						},
						{
							Integration: "aws-prod",
							Resources: []resourceResult{
								{
									Kind:      resourceKindAWSRDS,
									Found:     5,
									Enrolled:  4,
									Failed:    1,
									SyncStart: &syncStart,
									SyncEnd:   &syncEnd,
								},
							},
						},
					},
				},
			},
		},
		{
			Name:           "error-config",
			DiscoveryGroup: "prod",
			State:          discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_ERROR.String(),
			ErrorMessage:   "AccessDenied: missing ec2:DescribeInstances permission",
			LastSyncTime:   &errorRun,
		},
	}

	var jsonBuf bytes.Buffer
	require.NoError(t, utils.WriteJSONArray(&jsonBuf, summaries))
	require.Equal(t, `[
    {
        "name": "healthy-config",
        "discovery_group": "prod",
        "state": "DISCOVERY_CONFIG_STATE_RUNNING",
        "last_sync_time": "2026-01-15T11:58:00Z",
        "servers": [
            {
                "server_id": "server-a",
                "poll_interval": "5m0s",
                "last_update": "2026-01-15T11:59:00Z",
                "integrations": [
                    {
                        "resources": [
                            {
                                "kind": "AWS EC2",
                                "found": 10,
                                "enrolled": 8,
                                "failed": 2,
                                "sync_start": "2026-01-15T11:55:00Z",
                                "sync_end": "2026-01-15T11:56:00Z"
                            }
                        ]
                    },
                    {
                        "integration": "aws-prod",
                        "resources": [
                            {
                                "kind": "AWS RDS",
                                "found": 5,
                                "enrolled": 4,
                                "failed": 1,
                                "sync_start": "2026-01-15T11:55:00Z",
                                "sync_end": "2026-01-15T11:56:00Z"
                            }
                        ]
                    }
                ]
            }
        ]
    },
    {
        "name": "error-config",
        "discovery_group": "prod",
        "state": "DISCOVERY_CONFIG_STATE_ERROR",
        "error_message": "AccessDenied: missing ec2:DescribeInstances permission",
        "last_sync_time": "2026-01-15T11:55:00Z"
    }
]
`, jsonBuf.String())

	var yamlBuf bytes.Buffer
	require.NoError(t, utils.WriteYAML(&yamlBuf, summaries))
	require.Equal(t, `discovery_group: prod
last_sync_time: "2026-01-15T11:58:00Z"
name: healthy-config
servers:
- integrations:
  - resources:
    - enrolled: 8
      failed: 2
      found: 10
      kind: AWS EC2
      sync_end: "2026-01-15T11:56:00Z"
      sync_start: "2026-01-15T11:55:00Z"
  - integration: aws-prod
    resources:
    - enrolled: 4
      failed: 1
      found: 5
      kind: AWS RDS
      sync_end: "2026-01-15T11:56:00Z"
      sync_start: "2026-01-15T11:55:00Z"
  last_update: "2026-01-15T11:59:00Z"
  poll_interval: 5m0s
  server_id: server-a
state: DISCOVERY_CONFIG_STATE_RUNNING
---
discovery_group: prod
error_message: 'AccessDenied: missing ec2:DescribeInstances permission'
last_sync_time: "2026-01-15T11:55:00Z"
name: error-config
state: DISCOVERY_CONFIG_STATE_ERROR
`, yamlBuf.String())
}

func newTestDiscoveryConfig(t *testing.T, name, discoveryGroup string) *discoveryconfig.DiscoveryConfig {
	t.Helper()

	dc, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{Name: name},
		discoveryconfig.Spec{DiscoveryGroup: discoveryGroup},
	)
	require.NoError(t, err)
	return dc
}

func testServerStatus(lastUpdate time.Time, pollInterval time.Duration, integrationSummaries map[string]*discoveryconfigv1.DiscoverSummary) *discoveryconfig.DiscoveryStatusServer {
	return &discoveryconfig.DiscoveryStatusServer{
		DiscoveryStatusServer: discoveryconfigv1.DiscoveryStatusServer_builder{
			LastUpdate:           timestamppb.New(lastUpdate),
			PollInterval:         durationpb.New(pollInterval),
			IntegrationSummaries: integrationSummaries,
		}.Build(),
	}
}

type discoverSummaryOption func(*discoveryconfigv1.DiscoverSummary_builder)

func testDiscoverSummary(opts ...discoverSummaryOption) *discoveryconfigv1.DiscoverSummary {
	builder := discoveryconfigv1.DiscoverSummary_builder{}
	for _, opt := range opts {
		opt(&builder)
	}
	return builder.Build()
}

func withPreviousEC2(summary *discoveryconfigv1.ResourcesDiscoveredSummary) discoverSummaryOption {
	return func(builder *discoveryconfigv1.DiscoverSummary_builder) {
		builder.AwsEc2 = testResourceSummary(summary, nil)
	}
}

func withPreviousRDS(summary *discoveryconfigv1.ResourcesDiscoveredSummary) discoverSummaryOption {
	return func(builder *discoveryconfigv1.DiscoverSummary_builder) {
		builder.AwsRds = testResourceSummary(summary, nil)
	}
}

func withPreviousAzureVM(summary *discoveryconfigv1.ResourcesDiscoveredSummary) discoverSummaryOption {
	return func(builder *discoveryconfigv1.DiscoverSummary_builder) {
		builder.AzureVms = testResourceSummary(summary, nil)
	}
}

func withCurrentEKS(summary *discoveryconfigv1.ResourcesDiscoveredSummary) discoverSummaryOption {
	return func(builder *discoveryconfigv1.DiscoverSummary_builder) {
		builder.AwsEks = testResourceSummary(nil, summary)
	}
}

func testResourceSummary(previous, current *discoveryconfigv1.ResourcesDiscoveredSummary) *discoveryconfigv1.ResourceSummary {
	return discoveryconfigv1.ResourceSummary_builder{
		Previous: previous,
		Current:  current,
	}.Build()
}

func discoveryResourcesSummary(found, enrolled, failed uint64, syncStart, syncEnd time.Time) *discoveryconfigv1.ResourcesDiscoveredSummary {
	return discoveryconfigv1.ResourcesDiscoveredSummary_builder{
		Found:     found,
		Enrolled:  enrolled,
		Failed:    failed,
		SyncStart: timestamppb.New(syncStart),
		SyncEnd:   timestamppb.New(syncEnd),
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

func resourceKinds(resources []resourceResult) []string {
	kinds := make([]string, 0, len(resources))
	for _, resource := range resources {
		kinds = append(kinds, resource.Kind)
	}
	return kinds
}
