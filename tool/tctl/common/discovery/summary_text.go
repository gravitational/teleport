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

// summaryResultKind classifies what the "Result:" line of a resource summary
// reports. It is derived from the config status and the resource-level counts,
// so rendering is a simple switch.
type summaryResultKind int

const (
	// resultNotReporting means no Discovery Service has reported status yet.
	resultNotReporting summaryResultKind = iota
	// resultCounts means resource discovery counts are available.
	resultCounts
	// resultUnsupported means this resource type does not report detailed counts.
	resultUnsupported
	// resultNoResourceStatus means status was reported, but not for this resource.
	resultNoResourceStatus
)

// summaryResult is the resolved "Result:" line for a resource summary.
type summaryResult struct {
	Kind     summaryResultKind
	Found    uint64
	Enrolled uint64
	Failed   uint64
}

const summaryStatusNotReporting = "not reporting yet"

// configStatusSummary is the user-facing interpretation of a discovery config's
// raw status. It keeps status-state decisions separate from resource-count
// decisions.
type configStatusSummary struct {
	reported     bool
	label        string
	lastRun      *time.Time
	errorMessage string
}

type configSummary struct {
	Name           string
	DiscoveryGroup string
	Status         configStatusSummary
	Resources      []resourceSummary
}

// resourceSummary is a single rendered resource section: one
// cloud/resource-type/integration grouping for a discovery config.
type resourceSummary struct {
	Cloud        string
	ResourceType string
	Integration  string
	Scopes       []resourceScope
	LastSync     *time.Time
	Result       summaryResult
}

type resourceScope struct {
	Regions        []string
	Subscriptions  []string
	ResourceGroups []string
	MatchTags      []string
}

type resourceKey struct {
	cloud        string
	resourceType string
	integration  string
}

type summaryMatcherScope struct {
	integration    string
	resourceTypes  []string
	regions        []string
	subscriptions  []string
	resourceGroups []string
	tags           types.Labels
}

func buildConfigSummaries(configs []*discoveryconfig.DiscoveryConfig, cloudProviders cloudProviderConfig) []configSummary {
	out := make([]configSummary, 0, len(configs))
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
			finalizeResourceSummary(&resource)
			resources = append(resources, resource)
		}
		if len(resources) == 0 {
			continue
		}
		slices.SortFunc(resources, compareResourceSummaries)

		out = append(out, configSummary{
			Name:           dc.GetName(),
			DiscoveryGroup: dc.Spec.DiscoveryGroup,
			Status:         configStatus,
			Resources:      resources,
		})
	}

	slices.SortFunc(out, compareConfigSummaries)
	return out
}

func compareConfigSummaries(a, b configSummary) int {
	return cmp.Compare(a.Name, b.Name)
}

func compareResourceSummaries(a, b resourceSummary) int {
	if c := cmp.Compare(a.Cloud, b.Cloud); c != 0 {
		return c
	}
	if c := cmp.Compare(a.ResourceType, b.ResourceType); c != 0 {
		return c
	}
	return cmp.Compare(a.Integration, b.Integration)
}

func addAWSResources(resources map[resourceKey]*resourceSummary, dc *discoveryconfig.DiscoveryConfig, configStatus configStatusSummary) {
	addCloudResources(resources, dc, configStatus, dc.Spec.AWS, cloudAWS, describeAWSResource,
		func(matcher types.AWSMatcher) summaryMatcherScope {
			return summaryMatcherScope{
				integration:   matcher.Integration,
				resourceTypes: matcher.Types,
				regions:       matcher.Regions,
				tags:          matcher.Tags,
			}
		})
}

func addAzureResources(resources map[resourceKey]*resourceSummary, dc *discoveryconfig.DiscoveryConfig, configStatus configStatusSummary) {
	addCloudResources(resources, dc, configStatus, dc.Spec.Azure, cloudAzure, describeAzureResource,
		func(matcher types.AzureMatcher) summaryMatcherScope {
			return summaryMatcherScope{
				integration:    matcher.Integration,
				resourceTypes:  matcher.Types,
				regions:        matcher.Regions,
				subscriptions:  matcher.Subscriptions,
				resourceGroups: matcher.ResourceGroups,
				tags:           matcher.ResourceTags,
			}
		})
}

func addCloudResources[M any](
	resources map[resourceKey]*resourceSummary,
	dc *discoveryconfig.DiscoveryConfig,
	configStatus configStatusSummary,
	matchers []M,
	cloud string,
	describe func(string) resourceDescriptor,
	scope func(M) summaryMatcherScope,
) {
	for _, matcher := range matchers {
		matcherScope := scope(matcher)

		for _, matcherType := range matcherScope.resourceTypes {
			desc := describe(matcherType)
			counts := desc.lookupCounts(dc.Status.IntegrationDiscoveredResources[matcherScope.integration])
			resource := ensureResource(resources, configStatus, cloud, desc.displayName, matcherScope.integration, desc.supportsCounts, counts)
			addResourceScope(resource, matcherScope.regions, matcherScope.subscriptions, matcherScope.resourceGroups, matcherScope.tags)
		}
	}
}

// ensureResource returns the resource summary for the given grouping, creating
// it on first sight. Resource counts are resolved once, at creation, and are
// identical for every matcher that maps to the same resource; later matchers
// only contribute scope via addResourceScope.
func ensureResource(resources map[resourceKey]*resourceSummary, configStatus configStatusSummary, cloud, resourceType, integration string, supportsCounts bool, counts *discoveryconfigv1.ResourcesDiscoveredSummary) *resourceSummary {
	key := resourceKey{
		cloud:        cloud,
		resourceType: resourceType,
		integration:  integration,
	}
	if resource := resources[key]; resource != nil {
		return resource
	}

	result, lastSync := resolveSummaryResult(configStatus.reported, supportsCounts, counts)
	resource := &resourceSummary{
		Cloud:        cloud,
		ResourceType: resourceType,
		Integration:  integration,
		LastSync:     lastSync,
		Result:       result,
	}
	resources[key] = resource
	return resource
}

func summarizeConfigStatus(status discoveryconfig.Status) configStatusSummary {
	out := configStatusSummary{
		reported: hasConfigStatus(status),
		label:    summaryStatusNotReporting,
	}
	isError := false
	if !status.LastSyncTime.IsZero() {
		out.lastRun = new(status.LastSyncTime)
	}
	if status.ErrorMessage != nil {
		out.errorMessage = *status.ErrorMessage
		isError = true
	}

	if !out.reported {
		return out
	}
	if isError {
		out.label = "error"
		return out
	}

	switch status.State {
	case discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_ERROR.String():
		out.label = "error"
	case discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_SYNCING.String():
		out.label = "syncing"
	case discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String():
		out.label = "healthy"
	default:
		out.label = "reported"
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

func finalizeResourceSummary(resource *resourceSummary) {
	slices.SortFunc(resource.Scopes, compareResourceScopes)
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

func compareResourceScopes(a, b resourceScope) int {
	if c := cmp.Compare(strings.Join(a.Regions, "\x00"), strings.Join(b.Regions, "\x00")); c != 0 {
		return c
	}
	if c := cmp.Compare(strings.Join(a.Subscriptions, "\x00"), strings.Join(b.Subscriptions, "\x00")); c != 0 {
		return c
	}
	if c := cmp.Compare(strings.Join(a.ResourceGroups, "\x00"), strings.Join(b.ResourceGroups, "\x00")); c != 0 {
		return c
	}
	return cmp.Compare(strings.Join(a.MatchTags, "\x00"), strings.Join(b.MatchTags, "\x00"))
}

func resolveSummaryResult(reported bool, supportsCounts bool, counts *discoveryconfigv1.ResourcesDiscoveredSummary) (summaryResult, *time.Time) {
	switch {
	case !reported && counts == nil:
		return summaryResult{Kind: resultNotReporting}, nil
	case !supportsCounts:
		return summaryResult{Kind: resultUnsupported}, nil
	case counts == nil:
		return summaryResult{Kind: resultNoResourceStatus}, nil
	default:
		return summaryResult{
			Kind:     resultCounts,
			Found:    counts.GetFound(),
			Enrolled: counts.GetEnrolled(),
			Failed:   counts.GetFailed(),
		}, resourceSummarySyncTime(counts)
	}
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
