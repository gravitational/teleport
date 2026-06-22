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
	"context"
	"slices"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/discoveryconfig"
)

type structuredSummary struct {
	Name           string                      `json:"name" yaml:"name"`
	DiscoveryGroup string                      `json:"discovery_group" yaml:"discovery_group"`
	Status         structuredSummaryStatus     `json:"status" yaml:"status"`
	Resources      []structuredSummaryResource `json:"resources" yaml:"resources"`
}

type structuredSummaryStatus struct {
	State        string     `json:"state" yaml:"state"`
	LastRun      *time.Time `json:"last_run,omitempty" yaml:"last_run,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty" yaml:"error_message,omitempty"`
}

type structuredSummaryResource struct {
	Cloud            string                   `json:"cloud" yaml:"cloud"`
	ResourceType     string                   `json:"resource_type" yaml:"resource_type"`
	Source           string                   `json:"source" yaml:"source"`
	Integration      string                   `json:"integration,omitempty" yaml:"integration,omitempty"`
	Scopes           []structuredSummaryScope `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	LastResourceSync *time.Time               `json:"last_resource_sync,omitempty" yaml:"last_resource_sync,omitempty"`
	Result           structuredSummaryResult  `json:"result" yaml:"result"`
}

type structuredSummaryScope struct {
	Regions        []string `json:"regions,omitempty" yaml:"regions,omitempty"`
	Subscriptions  []string `json:"subscriptions,omitempty" yaml:"subscriptions,omitempty"`
	ResourceGroups []string `json:"resource_groups,omitempty" yaml:"resource_groups,omitempty"`
	MatchTags      []string `json:"match_tags,omitempty" yaml:"match_tags,omitempty"`
}

type structuredSummaryResult struct {
	Kind    string                   `json:"kind" yaml:"kind"`
	Counts  *structuredSummaryCounts `json:"counts,omitempty" yaml:"counts,omitempty"`
	Message string                   `json:"message,omitempty" yaml:"message,omitempty"`
}

type structuredSummaryCounts struct {
	Found    uint64 `json:"found" yaml:"found"`
	Enrolled uint64 `json:"enrolled" yaml:"enrolled"`
	Failed   uint64 `json:"failed" yaml:"failed"`
}

func buildStructuredSummaries(configs []configSummary) []structuredSummary {
	out := make([]structuredSummary, 0, len(configs))

	for _, config := range configs {
		summary := structuredSummary{
			Name:           config.Name,
			DiscoveryGroup: config.DiscoveryGroup,
			Status: structuredSummaryStatus{
				State:        config.Status.label,
				LastRun:      config.Status.lastRun,
				ErrorMessage: config.Status.errorMessage,
			},
			Resources: make([]structuredSummaryResource, 0, len(config.Resources)),
		}

		for _, resource := range config.Resources {
			summary.Resources = append(summary.Resources, structuredSummaryResource{
				Cloud:            resource.Cloud,
				ResourceType:     resource.ResourceType,
				Source:           structuredSummarySource(resource.Integration),
				Integration:      resource.Integration,
				Scopes:           structuredSummaryScopes(resource.Scopes),
				LastResourceSync: resource.LastSync,
				Result:           structuredSummaryResultFromResult(resource.Result),
			})
		}
		out = append(out, summary)
	}
	return out
}

func structuredSummarySource(integration string) string {
	if integration == "" {
		return "ambient_credentials"
	}
	return "integration"
}

func structuredSummaryScopes(scopes []resourceScope) []structuredSummaryScope {
	out := make([]structuredSummaryScope, 0, len(scopes))
	for _, scope := range scopes {
		out = append(out, structuredSummaryScope(scope))
	}
	return out
}

func structuredSummaryResultFromResult(result summaryResult) structuredSummaryResult {
	out := structuredSummaryResult{
		Kind: summaryResultKindName(result.Kind),
	}
	if result.Kind == resultCounts {
		out.Counts = &structuredSummaryCounts{
			Found:    result.Found,
			Enrolled: result.Enrolled,
			Failed:   result.Failed,
		}
		return out
	}
	out.Message = formatSummaryResult(result)
	return out
}

func summaryResultKindName(kind summaryResultKind) string {
	switch kind {
	case resultNotReporting:
		return "not_reporting"
	case resultCounts:
		return "counts"
	case resultUnsupported:
		return "unsupported"
	case resultNoResourceStatus:
		return "no_resource_status"
	default:
		return "unknown"
	}
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

func appendUnique(dst []string, values ...string) []string {
	for _, value := range values {
		if value == "" || slices.Contains(dst, value) {
			continue
		}
		dst = append(dst, value)
	}
	return dst
}
