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
	"context"
	"slices"
	"strings"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/utils/clientutils"

	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/trace"
)

type integrationListItem struct {
	Name         string `json:"name" yaml:"name"`
	Type         string `json:"type" yaml:"type"`
	Found        uint64 `json:"found" yaml:"found"`
	Enrolled     uint64 `json:"enrolled" yaml:"enrolled"`
	Failed       uint64 `json:"failed" yaml:"failed"`
	AwaitingJoin uint64 `json:"awaiting_join" yaml:"awaiting_join"`
	OpenTasks    int    `json:"open_tasks" yaml:"open_tasks"`
}

type integrationListOutput struct {
	Total int                   `json:"total" yaml:"total"`
	Items []integrationListItem `json:"items" yaml:"items"`
}

type resourceTypeStatsRow struct {
	ResourceType string `json:"resource_type" yaml:"resource_type"`
	Found        uint64 `json:"found" yaml:"found"`
	Enrolled     uint64 `json:"enrolled" yaml:"enrolled"`
	Failed       uint64 `json:"failed" yaml:"failed"`
}

type integrationDetail struct {
	Name              string                 `json:"name" yaml:"name"`
	Type              string                 `json:"type" yaml:"type"`
	Credentials       map[string]string      `json:"credentials" yaml:"credentials"`
	ResourceTypeStats []resourceTypeStatsRow `json:"resource_type_stats" yaml:"resource_type_stats"`
	DiscoveryConfigs  []configStatus         `json:"discovery_configs" yaml:"discovery_configs"`
	OpenTasks         []taskListItem         `json:"open_tasks" yaml:"open_tasks"`
}

func listIntegrations(ctx context.Context, client discoveryClient) ([]types.Integration, error) {
	items, err := stream.Collect(clientutils.Resources(ctx, client.ListIntegrations))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return items, nil
}

func friendlyIntegrationType(subKind string) string {
	switch subKind {
	case types.IntegrationSubKindAWSOIDC:
		return "AWS OIDC"
	case types.IntegrationSubKindAzureOIDC:
		return "Azure OIDC"
	case types.IntegrationSubKindGitHub:
		return "GitHub"
	case types.IntegrationSubKindAWSRolesAnywhere:
		return "AWS Roles Anywhere"
	default:
		if strings.TrimSpace(subKind) == "" {
			return "Unknown"
		}
		return subKind
	}
}

func integrationCredentialDetails(ig types.Integration) map[string]string {
	creds := map[string]string{}
	switch ig.GetSubKind() {
	case types.IntegrationSubKindAWSOIDC:
		if spec := ig.GetAWSOIDCIntegrationSpec(); spec != nil {
			creds["Role ARN"] = spec.RoleARN
		}
	case types.IntegrationSubKindAzureOIDC:
		if spec := ig.GetAzureOIDCIntegrationSpec(); spec != nil {
			creds["Tenant ID"] = spec.TenantID
			creds["Client ID"] = spec.ClientID
		}
	case types.IntegrationSubKindGitHub:
		if spec := ig.GetGitHubIntegrationSpec(); spec != nil {
			creds["Organization"] = spec.Organization
		}
	case types.IntegrationSubKindAWSRolesAnywhere:
		if spec := ig.GetAWSRolesAnywhereIntegrationSpec(); spec != nil {
			if spec.ProfileSyncConfig != nil {
				creds["Role ARN"] = spec.ProfileSyncConfig.RoleARN
			}
		}
	}
	return creds
}

func buildIntegrationStatsMap(dcs []*discoveryconfig.DiscoveryConfig) map[string]resourcesAggregate {
	statsMap := map[string]resourcesAggregate{}
	for _, dc := range dcs {
		for integrationName, integrationSummary := range dc.Status.IntegrationDiscoveredResources {
			agg := statsMap[integrationName]
			addDiscoveredSummary(&agg, integrationSummary.GetAwsEc2())
			addDiscoveredSummary(&agg, integrationSummary.GetAwsEks())
			addDiscoveredSummary(&agg, integrationSummary.GetAwsRds())
			addDiscoveredSummary(&agg, integrationSummary.GetAzureVms())
			statsMap[integrationName] = agg
		}
	}
	return statsMap
}

func countTasksByIntegration(tasks []*usertasksv1.UserTask) map[string]int {
	counts := map[string]int{}
	for _, task := range tasks {
		counts[task.GetSpec().GetIntegration()]++
	}
	return counts
}

func toIntegrationListItems(integrations []types.Integration, statsMap map[string]resourcesAggregate, taskCountMap map[string]int) []integrationListItem {
	items := make([]integrationListItem, 0, len(integrations))
	for _, ig := range integrations {
		name := ig.GetName()
		stats := statsMap[name]
		items = append(items, integrationListItem{
			Name:         name,
			Type:         friendlyIntegrationType(ig.GetSubKind()),
			Found:        stats.Found,
			Enrolled:     stats.Enrolled,
			Failed:       stats.Failed,
			AwaitingJoin: awaitingJoin(stats),
			OpenTasks:    taskCountMap[name],
		})
	}
	slices.SortFunc(items, func(a, b integrationListItem) int {
		if a.Failed != b.Failed {
			if a.Failed > b.Failed {
				return -1
			}
			return 1
		}
		if a.Found != b.Found {
			if a.Found > b.Found {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return items
}

func perResourceTypeStats(dcs []*discoveryconfig.DiscoveryConfig, integrationName string) []resourceTypeStatsRow {
	type key struct{ name string }
	statsMap := map[key]resourceTypeStatsRow{}
	addRow := func(resourceType string, summary *discoveryconfigv1.ResourcesDiscoveredSummary) {
		if summary == nil {
			return
		}
		if summary.GetFound() == 0 && summary.GetEnrolled() == 0 && summary.GetFailed() == 0 {
			return
		}
		k := key{name: resourceType}
		row := statsMap[k]
		row.ResourceType = resourceType
		row.Found += summary.GetFound()
		row.Enrolled += summary.GetEnrolled()
		row.Failed += summary.GetFailed()
		statsMap[k] = row
	}
	for _, dc := range dcs {
		integrationSummary, ok := dc.Status.IntegrationDiscoveredResources[integrationName]
		if !ok {
			continue
		}
		addRow("EC2", integrationSummary.GetAwsEc2())
		addRow("EKS", integrationSummary.GetAwsEks())
		addRow("RDS", integrationSummary.GetAwsRds())
		addRow("Azure VM", integrationSummary.GetAzureVms())
	}
	rows := make([]resourceTypeStatsRow, 0, len(statsMap))
	for _, row := range statsMap {
		rows = append(rows, row)
	}
	slices.SortFunc(rows, func(a, b resourceTypeStatsRow) int {
		return cmp.Compare(a.ResourceType, b.ResourceType)
	})
	return rows
}

func associatedDiscoveryConfigs(dcs []*discoveryconfig.DiscoveryConfig, integrationName string) []configStatus {
	configs := make([]configStatus, 0)
	for _, dc := range dcs {
		if _, ok := dc.Status.IntegrationDiscoveredResources[integrationName]; !ok {
			continue
		}
		configs = append(configs, configStatus{
			Name:       dc.GetName(),
			Group:      dc.GetDiscoveryGroup(),
			State:      cmp.Or(strings.TrimSpace(dc.Status.State), "UNKNOWN"),
			Matchers:   configMatchersSummary(dc),
			Discovered: dc.Status.DiscoveredResources,
			LastSync:   dc.Status.LastSyncTime.UTC(),
		})
	}
	slices.SortFunc(configs, func(a, b configStatus) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return configs
}

func buildIntegrationDetail(ig types.Integration, dcs []*discoveryconfig.DiscoveryConfig, tasks []*usertasksv1.UserTask) integrationDetail {
	name := ig.GetName()
	return integrationDetail{
		Name:              name,
		Type:              friendlyIntegrationType(ig.GetSubKind()),
		Credentials:       integrationCredentialDetails(ig),
		ResourceTypeStats: perResourceTypeStats(dcs, name),
		DiscoveryConfigs:  associatedDiscoveryConfigs(dcs, name),
		OpenTasks:         toTaskListItems(tasks),
	}
}
