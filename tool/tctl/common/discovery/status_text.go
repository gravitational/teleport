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
	"cmp"
	"slices"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
)

const (
	summaryStatusNotReporting = "not reporting yet"

	resourceKindAWSEC2  = "AWS EC2"
	resourceKindAWSRDS  = "AWS RDS"
	resourceKindAWSEKS  = "AWS EKS"
	resourceKindAzureVM = "Azure VM"
)

func newDiscoverySummary(configs []*discoveryconfig.DiscoveryConfig, cloudProviders cloudProviderConfig) discoverySummary {
	out := make(discoverySummary, 0, len(configs))
	for _, dc := range configs {
		summary := configSummary{
			Name:           dc.GetName(),
			DiscoveryGroup: dc.Spec.DiscoveryGroup,
			State:          dc.Status.State,
			Servers:        buildServerSummaries(dc.Status.ServerStatus, cloudProviders),
		}
		if dc.Status.ErrorMessage != nil {
			summary.ErrorMessage = *dc.Status.ErrorMessage
		}
		if !dc.Status.LastSyncTime.IsZero() {
			summary.LastSyncTime = new(dc.Status.LastSyncTime)
		}

		out = append(out, summary)
	}

	slices.SortFunc(out, func(a, b configSummary) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return out
}

func buildServerSummaries(status map[string]*discoveryconfig.DiscoveryStatusServer, cloudProviders cloudProviderConfig) []serverSummary {
	servers := make([]serverSummary, 0, len(status))
	for serverID, serverStatus := range status {
		server := serverSummary{
			ServerID: serverID,
		}
		if serverStatus != nil && serverStatus.DiscoveryStatusServer != nil {
			if pollInterval := serverStatus.GetPollInterval(); pollInterval != nil {
				server.PollInterval = pollInterval.AsDuration().String()
			}
			if lastUpdate := serverStatus.GetLastUpdate(); lastUpdate != nil {
				server.LastUpdate = new(lastUpdate.AsTime())
			}
			server.Integrations = buildIntegrationSummaries(serverStatus.GetIntegrationSummaries(), cloudProviders)
		}
		servers = append(servers, server)
	}

	slices.SortFunc(servers, func(a, b serverSummary) int {
		return cmp.Compare(a.ServerID, b.ServerID)
	})
	return servers
}

func buildIntegrationSummaries(status map[string]*discoveryconfigv1.DiscoverSummary, cloudProviders cloudProviderConfig) []integrationSummary {
	integrations := make([]integrationSummary, 0, len(status))
	for integrationName, summary := range status {
		integrations = append(integrations, integrationSummary{
			Integration: integrationName,
			Resources:   buildResourceResults(summary, cloudProviders),
		})
	}

	slices.SortFunc(integrations, func(a, b integrationSummary) int {
		return cmp.Compare(a.Integration, b.Integration)
	})
	return integrations
}

func buildResourceResults(summary *discoveryconfigv1.DiscoverSummary, cloudProviders cloudProviderConfig) []resourceResult {
	var resources []resourceResult
	if cloudProviders.aws {
		addResourceResult(&resources, resourceKindAWSEC2, summary.GetAwsEc2())
		addResourceResult(&resources, resourceKindAWSRDS, summary.GetAwsRds())
		addResourceResult(&resources, resourceKindAWSEKS, summary.GetAwsEks())
	}
	if cloudProviders.azure {
		addResourceResult(&resources, resourceKindAzureVM, summary.GetAzureVms())
	}
	return resources
}

func addResourceResult(resources *[]resourceResult, kind string, summary *discoveryconfigv1.ResourceSummary) {
	previous := summary.GetPrevious()
	if previous == nil {
		return
	}
	*resources = append(*resources, newResourceResult(kind, previous))
}

func newResourceResult(kind string, summary *discoveryconfigv1.ResourcesDiscoveredSummary) resourceResult {
	result := resourceResult{
		Kind:     kind,
		Found:    summary.GetFound(),
		Enrolled: summary.GetEnrolled(),
		Failed:   summary.GetFailed(),
	}
	if syncStart := summary.GetSyncStart(); syncStart != nil {
		result.SyncStart = new(syncStart.AsTime())
	}
	if syncEnd := summary.GetSyncEnd(); syncEnd != nil {
		result.SyncEnd = new(syncEnd.AsTime())
	}
	return result
}
