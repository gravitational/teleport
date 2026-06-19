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
	"strings"
	"time"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
)

// summaryResultKind classifies what the "Result:" line of a summary block
// reports. It is derived once, during finalization, from the config status and
// the resource-level counts, so rendering is a simple switch.
type summaryResultKind int

const (
	// resultNotReporting means no Discovery Service has reported status yet.
	resultNotReporting summaryResultKind = iota
	// resultCounts means resource discovery counts are available.
	resultCounts
	// resultError means the discovery config reported an error.
	resultError
	// resultUnsupported means this resource type does not report detailed counts.
	resultUnsupported
	// resultNoResourceStatus means status was reported, but not for this resource.
	resultNoResourceStatus
)

// summaryResult is the resolved "Result:" line for a summary block.
type summaryResult struct {
	Kind     summaryResultKind
	Found    uint64
	Enrolled uint64
	Failed   uint64
	Message  string
}

// summaryBlock is a single rendered block of the text summary output: one
// cloud/resource-type/integration grouping for a discovery config.
type summaryBlock struct {
	Name           string
	DiscoveryGroup string
	Cloud          string
	ResourceType   string
	Integration    string
	Regions        []string
	Subscriptions  []string
	ResourceGroups []string
	MatchTags      []string
	Status         string
	LastRun        *time.Time
	Result         summaryResult

	// The fields below capture the inputs needed to resolve Result during
	// finalization. They are not rendered directly.
	hasStatus      bool
	supportsCounts bool
	counts         *discoveryconfigv1.ResourcesDiscoveredSummary
	errorMessage   string
}

type summaryBlockKey struct {
	name         string
	cloud        string
	resourceType string
	integration  string
}

func buildSummaryBlocks(configs []*discoveryconfig.DiscoveryConfig, cloudProviders cloudProviderConfig, integrationFilter string) []summaryBlock {
	rows := make(map[summaryBlockKey]*summaryBlock)
	for _, dc := range configs {
		if cloudProviders.aws {
			addAWSBlocks(rows, dc, integrationFilter)
		}
		if cloudProviders.azure {
			addAzureBlocks(rows, dc, integrationFilter)
		}
	}

	out := make([]summaryBlock, 0, len(rows))
	for _, row := range rows {
		finalizeBlock(row)
		out = append(out, *row)
	}

	slices.SortFunc(out, func(a, b summaryBlock) int {
		if c := cmp.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Cloud, b.Cloud); c != 0 {
			return c
		}
		if c := cmp.Compare(a.ResourceType, b.ResourceType); c != 0 {
			return c
		}
		return cmp.Compare(a.Integration, b.Integration)
	})
	return out
}

func addAWSBlocks(rows map[summaryBlockKey]*summaryBlock, dc *discoveryconfig.DiscoveryConfig, integrationFilter string) {
	for _, matcher := range dc.Spec.AWS {
		if !matchesIntegration(matcher.Integration, integrationFilter) {
			continue
		}

		for _, matcherType := range matcher.Types {
			desc := describeAWSResource(matcherType)
			counts := desc.lookupCounts(dc.Status.IntegrationDiscoveredResources[matcher.Integration])
			row := ensureBlock(rows, dc, cloudAWS, desc.displayName, matcher.Integration, desc.supportsCounts, counts)
			addBlockScope(row, matcher.Regions, nil, nil, matcher.Tags)
		}
	}
}

func addAzureBlocks(rows map[summaryBlockKey]*summaryBlock, dc *discoveryconfig.DiscoveryConfig, integrationFilter string) {
	for _, matcher := range dc.Spec.Azure {
		if !matchesIntegration(matcher.Integration, integrationFilter) {
			continue
		}

		for _, matcherType := range matcher.Types {
			desc := describeAzureResource(matcherType)
			counts := desc.lookupCounts(dc.Status.IntegrationDiscoveredResources[matcher.Integration])
			row := ensureBlock(rows, dc, cloudAzure, desc.displayName, matcher.Integration, desc.supportsCounts, counts)
			addBlockScope(row, matcher.Regions, matcher.Subscriptions, matcher.ResourceGroups, matcher.ResourceTags)
		}
	}
}

// ensureBlock returns the block for the given grouping, creating it on first
// sight. Status and resource counts are resolved once, at creation, and are
// identical for every matcher that maps to the same block; later matchers only
// contribute scope (regions, tags, etc.) via addBlockScope.
func ensureBlock(rows map[summaryBlockKey]*summaryBlock, dc *discoveryconfig.DiscoveryConfig, cloud, resourceType, integration string, supportsCounts bool, counts *discoveryconfigv1.ResourcesDiscoveredSummary) *summaryBlock {
	key := summaryBlockKey{
		name:         dc.GetName(),
		cloud:        cloud,
		resourceType: resourceType,
		integration:  integration,
	}
	if row := rows[key]; row != nil {
		return row
	}

	row := &summaryBlock{
		Name:           dc.GetName(),
		DiscoveryGroup: dc.Spec.DiscoveryGroup,
		Cloud:          cloud,
		ResourceType:   resourceType,
		Integration:    integration,
		supportsCounts: supportsCounts,
		counts:         counts,
	}
	applyConfigStatus(row, dc.Status)
	rows[key] = row
	return row
}

func applyConfigStatus(row *summaryBlock, status discoveryconfig.Status) {
	row.hasStatus = hasConfigStatus(status)
	row.Status = humanConfigStatus(status, row.hasStatus)
	if !status.LastSyncTime.IsZero() {
		lastRun := status.LastSyncTime
		row.LastRun = &lastRun
	}
	if status.ErrorMessage != nil {
		row.errorMessage = *status.ErrorMessage
	}
}

func hasConfigStatus(status discoveryconfig.Status) bool {
	return (status.State != "" &&
		status.State != discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_UNSPECIFIED.String()) ||
		status.ErrorMessage != nil ||
		!status.LastSyncTime.IsZero() ||
		len(status.IntegrationDiscoveredResources) > 0 ||
		len(status.ServerStatus) > 0
}

func humanConfigStatus(status discoveryconfig.Status, hasStatus bool) string {
	if !hasStatus {
		return "not reporting yet"
	}
	if status.ErrorMessage != nil {
		return "error"
	}

	switch status.State {
	case discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_ERROR.String():
		return "error"
	case discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_SYNCING.String():
		return "syncing"
	case discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String():
		return "healthy"
	default:
		return "reported"
	}
}

func addBlockScope(row *summaryBlock, regions, subscriptions, resourceGroups []string, tags types.Labels) {
	row.Regions = appendUnique(row.Regions, regions...)
	row.Subscriptions = appendUnique(row.Subscriptions, subscriptions...)
	row.ResourceGroups = appendUnique(row.ResourceGroups, resourceGroups...)
	row.MatchTags = appendUnique(row.MatchTags, formatMatchTags(tags))
}

// finalizeBlock sorts the merged scope and resolves the block's Result exactly
// once, after every matcher for the block has been folded in.
func finalizeBlock(row *summaryBlock) {
	slices.Sort(row.Regions)
	slices.Sort(row.Subscriptions)
	slices.Sort(row.ResourceGroups)
	slices.Sort(row.MatchTags)
	row.Result = resolveResult(row)
}

func resolveResult(row *summaryBlock) summaryResult {
	switch {
	case row.Status == "error":
		return summaryResult{Kind: resultError, Message: row.errorMessage}
	case !row.hasStatus && row.counts == nil:
		return summaryResult{Kind: resultNotReporting}
	case !row.supportsCounts:
		return summaryResult{Kind: resultUnsupported}
	case row.counts == nil:
		return summaryResult{Kind: resultNoResourceStatus}
	default:
		// Resource-level counts carry a more precise sync time than the
		// config-level status, so prefer it for the "Last run:" line.
		if syncTime := resourceSummarySyncTime(row.counts); syncTime != nil {
			row.LastRun = syncTime
		}
		return summaryResult{
			Kind:     resultCounts,
			Found:    row.counts.GetFound(),
			Enrolled: row.counts.GetEnrolled(),
			Failed:   row.counts.GetFailed(),
		}
	}
}

func resourceSummarySyncTime(counts *discoveryconfigv1.ResourcesDiscoveredSummary) *time.Time {
	switch {
	case counts.GetSyncEnd() != nil:
		t := counts.GetSyncEnd().AsTime()
		return &t
	case counts.GetSyncStart() != nil:
		t := counts.GetSyncStart().AsTime()
		return &t
	default:
		return nil
	}
}

func formatMatchTags(labels types.Labels) string {
	if len(labels) == 0 {
		return "all"
	}

	keys := labelKeys(labels)
	slices.Sort(keys)
	if len(keys) == 1 && keys[0] == types.Wildcard && slices.Equal([]string(labels[types.Wildcard]), []string{types.Wildcard}) {
		return "all"
	}

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		values := append([]string(nil), labels[key]...)
		slices.Sort(values)
		switch len(values) {
		case 0:
			parts = append(parts, key)
		case 1:
			parts = append(parts, key+"="+values[0])
		default:
			parts = append(parts, key+" in ("+strings.Join(values, ", ")+")")
		}
	}
	return strings.Join(parts, ", ")
}

func labelKeys(labels types.Labels) []string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	return keys
}
