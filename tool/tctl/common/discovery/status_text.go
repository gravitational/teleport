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
	"maps"
	"slices"
	"strings"
	"time"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
)

const (
	summaryStatusNotReporting = "not reporting yet"

	resultKindNotReporting     = "not_reporting"
	resultKindCounts           = "counts"
	resultKindUnsupported      = "unsupported"
	resultKindNoResourceStatus = "no_resource_status"

	resultMessageNotReporting     = "no status reported by a Discovery Service"
	resultMessageUnsupported      = "detailed counts are not available for this resource type"
	resultMessageNoResourceStatus = "no resource status reported for this discovery target"
)

type resourceKey struct {
	cloud        string
	resourceType string
	integration  string
}

func newDiscoverySummary(configs []*discoveryconfig.DiscoveryConfig, cloudProviders cloudProviderConfig) discoverySummary {
	out := make(discoverySummary, 0, len(configs))
	for _, dc := range configs {
		configStatus := summarizeConfigStatus(dc.Status)
		resourcesByKey := make(map[resourceKey]*resourceSummary)
		if cloudProviders.aws {
			addAWSResources(resourcesByKey, dc, configStatus)
		}
		if cloudProviders.azure {
			addAzureResources(resourcesByKey, dc, configStatus)
		}

		resources := make([]resourceSummary, 0, len(resourcesByKey))
		for _, row := range resourcesByKey {
			resource := *row
			slices.SortFunc(resource.Scopes, func(a, b resourceScope) int {
				return cmp.Or(
					cmp.Compare(strings.Join(a.Regions, "\x00"), strings.Join(b.Regions, "\x00")),
					cmp.Compare(strings.Join(a.Subscriptions, "\x00"), strings.Join(b.Subscriptions, "\x00")),
					cmp.Compare(strings.Join(a.ResourceGroups, "\x00"), strings.Join(b.ResourceGroups, "\x00")),
					cmp.Compare(strings.Join(a.MatchTags, "\x00"), strings.Join(b.MatchTags, "\x00")),
				)
			})
			resources = append(resources, resource)
		}
		if len(resources) == 0 {
			continue
		}
		slices.SortFunc(resources, func(a, b resourceSummary) int {
			return cmp.Or(
				cmp.Compare(a.Cloud, b.Cloud),
				cmp.Compare(a.ResourceType, b.ResourceType),
				cmp.Compare(a.Integration, b.Integration),
			)
		})

		out = append(out, configSummary{
			Name:           dc.GetName(),
			DiscoveryGroup: dc.Spec.DiscoveryGroup,
			Status:         configStatus,
			Resources:      resources,
		})
	}

	slices.SortFunc(out, func(a, b configSummary) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return out
}

func addAWSResources(resources map[resourceKey]*resourceSummary, dc *discoveryconfig.DiscoveryConfig, configStatus configStatus) {
	for _, matcher := range dc.Spec.AWS {
		for _, matcherType := range matcher.Types {
			desc := describeAWSResource(matcherType)
			counts := desc.lookupCounts(dc.Status.IntegrationDiscoveredResources[matcher.Integration])
			resource := ensureResource(resources, configStatus, cloudAWS, desc.displayName, matcher.Integration, desc.supportsCounts, counts)
			addResourceScope(resource, matcher.Regions, nil, nil, matcher.Tags)
		}
	}
}

func addAzureResources(resources map[resourceKey]*resourceSummary, dc *discoveryconfig.DiscoveryConfig, configStatus configStatus) {
	for _, matcher := range dc.Spec.Azure {
		for _, matcherType := range matcher.Types {
			desc := describeAzureResource(matcherType)
			counts := desc.lookupCounts(dc.Status.IntegrationDiscoveredResources[matcher.Integration])
			resource := ensureResource(resources, configStatus, cloudAzure, desc.displayName, matcher.Integration, desc.supportsCounts, counts)
			addResourceScope(resource, matcher.Regions, matcher.Subscriptions, matcher.ResourceGroups, matcher.ResourceTags)
		}
	}
}

// ensureResource returns the resource summary for the given grouping, creating
// it on first sight. Resource counts are resolved once, at creation, and are
// identical for every matcher that maps to the same resource; later matchers
// only contribute scope via addResourceScope.
func ensureResource(resources map[resourceKey]*resourceSummary, configStatus configStatus, cloud, resourceType, integration string, supportsCounts bool, counts *discoveryconfigv1.ResourcesDiscoveredSummary) *resourceSummary {
	key := resourceKey{
		cloud:        cloud,
		resourceType: resourceType,
		integration:  integration,
	}
	if resource := resources[key]; resource != nil {
		return resource
	}

	result, lastSync := resolveSummaryResult(configStatus.Reported, supportsCounts, counts)
	resource := &resourceSummary{
		Cloud:        cloud,
		ResourceType: resourceType,
		Source:       summarySource(integration),
		Integration:  integration,
		LastSync:     lastSync,
		Result:       result,
	}
	resources[key] = resource
	return resource
}

func summarizeConfigStatus(status discoveryconfig.Status) configStatus {
	out := configStatus{
		Reported: hasConfigStatus(status),
		State:    summaryStatusNotReporting,
	}
	isError := false
	if !status.LastSyncTime.IsZero() {
		out.LastRun = new(status.LastSyncTime)
	}
	if status.ErrorMessage != nil {
		out.ErrorMessage = *status.ErrorMessage
		isError = true
	}

	if !out.Reported {
		return out
	}
	if isError {
		out.State = "error"
		return out
	}

	switch status.State {
	case discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_ERROR.String():
		out.State = "error"
	case discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_SYNCING.String():
		out.State = "syncing"
	case discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String():
		out.State = "healthy"
	default:
		out.State = "reported"
	}
	return out
}

func hasConfigStatus(status discoveryconfig.Status) bool {
	return (status.State != "" &&
		status.State != discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_UNSPECIFIED.String()) ||
		status.ErrorMessage != nil ||
		!status.LastSyncTime.IsZero() ||
		len(status.IntegrationDiscoveredResources) > 0 ||
		len(status.ServerStatus) > 0
}

func addResourceScope(resource *resourceSummary, regions, subscriptions, resourceGroups []string, tags types.Labels) {
	scope := newResourceScope(regions, subscriptions, resourceGroups, tags)
	if !slices.ContainsFunc(resource.Scopes, func(existing resourceScope) bool {
		return resourceScopesEqual(existing, scope)
	}) {
		resource.Scopes = append(resource.Scopes, scope)
	}
}

func newResourceScope(regions, subscriptions, resourceGroups []string, tags types.Labels) resourceScope {
	scope := resourceScope{
		Regions:        appendUnique(nil, regions...),
		Subscriptions:  appendUnique(nil, subscriptions...),
		ResourceGroups: appendUnique(nil, resourceGroups...),
		MatchTags:      appendUnique(nil, formatMatchTags(tags)),
	}
	slices.Sort(scope.Regions)
	slices.Sort(scope.Subscriptions)
	slices.Sort(scope.ResourceGroups)
	slices.Sort(scope.MatchTags)
	return scope
}

func resourceScopesEqual(a, b resourceScope) bool {
	return slices.Equal(a.Regions, b.Regions) &&
		slices.Equal(a.Subscriptions, b.Subscriptions) &&
		slices.Equal(a.ResourceGroups, b.ResourceGroups) &&
		slices.Equal(a.MatchTags, b.MatchTags)
}

func resolveSummaryResult(reported bool, supportsCounts bool, counts *discoveryconfigv1.ResourcesDiscoveredSummary) (resultSummary, *time.Time) {
	switch {
	case !reported && counts == nil:
		return resultSummary{
			Kind:    resultKindNotReporting,
			Message: resultMessageNotReporting,
		}, nil
	case !supportsCounts:
		return resultSummary{
			Kind:    resultKindUnsupported,
			Message: resultMessageUnsupported,
		}, nil
	case counts == nil:
		return resultSummary{
			Kind:    resultKindNoResourceStatus,
			Message: resultMessageNoResourceStatus,
		}, nil
	default:
		return resultSummary{
			Kind: resultKindCounts,
			Counts: &resultCounts{
				Found:    counts.GetFound(),
				Enrolled: counts.GetEnrolled(),
				Failed:   counts.GetFailed(),
			},
		}, resourceSummarySyncTime(counts)
	}
}

func summarySource(integration string) string {
	if integration == "" {
		return "ambient_credentials"
	}
	return "integration"
}

func resourceSummarySyncTime(counts *discoveryconfigv1.ResourcesDiscoveredSummary) *time.Time {
	switch {
	case counts.GetSyncEnd() != nil:
		return new(counts.GetSyncEnd().AsTime())
	case counts.GetSyncStart() != nil:
		return new(counts.GetSyncStart().AsTime())
	default:
		return nil
	}
}

func formatMatchTags(labels types.Labels) string {
	if matchAllLabels(labels) {
		return "all"
	}

	parts := make([]string, 0, len(labels))
	for _, key := range slices.Sorted(maps.Keys(labels)) {
		parts = append(parts, formatMatchTag(key, labels[key]))
	}
	return strings.Join(parts, ", ")
}

func matchAllLabels(labels types.Labels) bool {
	if len(labels) == 0 {
		return true
	}
	values, ok := labels[types.Wildcard]
	return ok && len(labels) == 1 && slices.Equal([]string(values), []string{types.Wildcard})
}

func formatMatchTag(key string, values []string) string {
	values = slices.Clone(values)
	slices.Sort(values)

	switch len(values) {
	case 0:
		return key
	case 1:
		return key + "=" + values[0]
	default:
		return key + " in (" + strings.Join(values, ", ") + ")"
	}
}
