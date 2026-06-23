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
	"time"

	"github.com/gravitational/trace"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
)

type summary struct {
	Name           string    `json:"name"`
	DiscoveryGroup string    `json:"discovery_group"`
	Cloud          string    `json:"cloud"`
	Integration    string    `json:"integration"`
	ResourceTypes  []string  `json:"resource_types"`
	Regions        []string  `json:"regions,omitempty"`
	State          string    `json:"state"`
	LastSyncTime   time.Time `json:"last_sync_time,omitzero"`
	Found          uint64    `json:"found"`
	Enrolled       uint64    `json:"enrolled"`
	Failed         uint64    `json:"failed"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	ServerCount    int       `json:"server_count"`
}

func buildSummaries(configs []*discoveryconfig.DiscoveryConfig, cfg cloudProviderConfig, integrationFilter string) []summary {
	var summaries []summary
	for _, dc := range configs {
		if cfg.aws {
			summaries = append(summaries, awsSummaries(dc, integrationFilter)...)
		}
		if cfg.azure {
			summaries = append(summaries, azureSummaries(dc, integrationFilter)...)
		}
	}

	// Keep summary output deterministic because rows are assembled from maps.
	slices.SortFunc(summaries, func(a, b summary) int {
		if c := cmp.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Cloud, b.Cloud); c != 0 {
			return c
		}
		return cmp.Compare(a.Integration, b.Integration)
	})
	return summaries
}

func listDiscoveryConfigs(ctx context.Context, clt discoveryClient) ([]*discoveryconfig.DiscoveryConfig, error) {
	var out []*discoveryconfig.DiscoveryConfig
	var nextToken string

	for {
		page, token, err := clt.DiscoveryConfigClient().ListDiscoveryConfigs(ctx, 0, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		out = append(out, page...)
		if token == "" {
			return out, nil
		}
		nextToken = token
	}
}

func awsSummaries(dc *discoveryconfig.DiscoveryConfig, integrationFilter string) []summary {
	return cloudSummaries(dc, dc.Spec.AWS, cloudAWS, integrationFilter, describeAWSResource,
		func(m types.AWSMatcher) (integration string, resourceTypes, regions []string) {
			return m.Integration, m.Types, m.Regions
		})
}

func azureSummaries(dc *discoveryconfig.DiscoveryConfig, integrationFilter string) []summary {
	return cloudSummaries(dc, dc.Spec.Azure, cloudAzure, integrationFilter, describeAzureResource,
		func(m types.AzureMatcher) (integration string, resourceTypes, regions []string) {
			return m.Integration, m.Types, m.Regions
		})
}

// cloudSummaries groups a single cloud's matchers into digest rows keyed by
// integration. AWS and Azure matchers are distinct Go types, so scope extracts
// the grouping fields they have in common.
func cloudSummaries[M any](
	dc *discoveryconfig.DiscoveryConfig,
	matchers []M,
	cloud string,
	integrationFilter string,
	describe func(string) resourceDescriptor,
	scope func(M) (integration string, resourceTypes, regions []string),
) []summary {
	rows := make(map[string]*summary)

	for _, matcher := range matchers {
		integration, resourceTypes, regions := scope(matcher)
		if !matchesIntegration(integration, integrationFilter) {
			continue
		}

		row := rows[integration]
		if row == nil {
			row = baseSummary(dc, cloud, integration)
			rows[integration] = row
		}

		row.ResourceTypes = appendUnique(row.ResourceTypes, resourceTypes...)
		row.Regions = appendUnique(row.Regions, regions...)
	}

	out := make([]summary, 0, len(rows))
	for _, row := range rows {
		addCounts(row, dc.Status.IntegrationDiscoveredResources[row.Integration], describe)
		finalizeSummary(row)
		out = append(out, *row)
	}
	return out
}

func baseSummary(dc *discoveryconfig.DiscoveryConfig, cloud, integration string) *summary {
	var errorMessage string
	if dc.Status.ErrorMessage != nil {
		errorMessage = *dc.Status.ErrorMessage
	}

	return &summary{
		Name:           dc.GetName(),
		DiscoveryGroup: dc.Spec.DiscoveryGroup,
		Cloud:          cloud,
		Integration:    integration,
		State:          dc.Status.State,
		LastSyncTime:   dc.Status.LastSyncTime,
		ErrorMessage:   errorMessage,
		ServerCount:    len(dc.Status.ServerStatus),
	}
}

func matchesIntegration(integration, filter string) bool {
	return filter == "" || integration == filter
}

func appendUnique(dst []string, values ...string) []string {
	for _, value := range values {
		if value == "" || slices.Contains(dst, value) {
			continue
		}
		dst = append(dst, value)
	}
	return dst
}

func finalizeSummary(row *summary) {
	slices.Sort(row.ResourceTypes)
	slices.Sort(row.Regions)
}

// addCounts aggregates resource counts into a digest row. A config can declare
// several matcher types that share one count bucket (e.g. multiple AWS database
// engines all map to aws_rds), so each bucket is counted at most once.
func addCounts(row *summary, counts *discoveryconfig.IntegrationDiscoveredSummary, describe func(string) resourceDescriptor) {
	if counts == nil {
		return
	}

	seen := make(map[string]struct{})
	for _, resourceType := range row.ResourceTypes {
		desc := describe(resourceType)
		if !desc.supportsCounts {
			continue
		}
		if _, ok := seen[desc.bucket]; ok {
			continue
		}
		seen[desc.bucket] = struct{}{}
		addSummaryCounts(row, desc.lookupCounts(counts))
	}
}

func addSummaryCounts(row *summary, counts *discoveryconfigv1.ResourcesDiscoveredSummary) {
	if counts == nil {
		return
	}
	row.Found += counts.GetFound()
	row.Enrolled += counts.GetEnrolled()
	row.Failed += counts.GetFailed()
}
