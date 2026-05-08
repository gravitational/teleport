/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package azure

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	libslices "github.com/gravitational/teleport/lib/utils/slices"
)

// ResourceGraphClient is a client for Azure Resource Graph VM discovery.
type ResourceGraphClient interface {
	// QueryVMs returns running Linux VMs matching the supplied scope and filters.
	QueryVMs(ctx context.Context, params QueryVMsParams) ([]DiscoveredVM, error)
}

// QueryVMsParams scopes a Resource Graph VM query to a set of subscriptions
// and an optional set of regions and resource groups.
//
// Region and resource-group filters are expected to be trimmed and deduped by the caller, usually via
// services.SimplifyAzureMatchers. QueryVMs still treats types.Wildcard defensively as match-all if it
// appears anywhere in either filter, so an unsimplified list such as []string{"*", "rg1"} cannot narrow
// discovery by treating "*" as a literal ARG value.
type QueryVMsParams struct {
	// SubscriptionIDs is the set of Azure subscriptions to query. QueryVMs requires at least one entry;
	// ensuring each entry is non-empty is the upstream simplifier's responsibility.
	SubscriptionIDs []string
	// Regions filters VMs by location. An empty slice or any occurrence of types.Wildcard matches every region.
	Regions []string
	// ResourceGroups filters VMs by resource group. An empty slice or any
	// occurrence of types.Wildcard matches every resource group.
	ResourceGroups []string
}

// argResourcesAPI is the slice of armresourcegraph.Client we depend on, extracted as an interface
// so unit tests can fake the SDK without spinning up a real ARG client.
type argResourcesAPI interface {
	Resources(ctx context.Context, query armresourcegraph.QueryRequest, options *armresourcegraph.ClientResourcesOptions) (armresourcegraph.ClientResourcesResponse, error)
}

type resourceGraphClient struct {
	api argResourcesAPI
}

// NewResourceGraphClient returns a ResourceGraphClient backed by the official
// Azure SDK's armresourcegraph.Client.
func NewResourceGraphClient(cred azcore.TokenCredential, options *arm.ClientOptions) (ResourceGraphClient, error) {
	client, err := armresourcegraph.NewClient(cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &resourceGraphClient{api: client}, nil
}

// argMaxSubscriptionsPerQuery caps the subscription scope of a single Resource Graph request.
// Keep this comfortably below Microsoft's recommended grouping size and tenant subscription limits.
//
// Source: https://learn.microsoft.com/en-us/azure/governance/resource-graph/concepts/guidance-for-throttled-requests
const argMaxSubscriptionsPerQuery = 200

// argMaxPagesPerChunk bounds the SkipToken pagination loop in a single chunk's worth of Resource Graph results.
// QueryVMs asks for Top=1000, so this allows up to a million rows per chunk before treating pagination as runaway.
//
// Source: https://learn.microsoft.com/en-us/azure/governance/resource-graph/concepts/guidance-for-throttled-requests#pagination
const argMaxPagesPerChunk = 1000

// QueryVMs runs the discovery query against Resource Graph and translates the rows to []DiscoveredVM.
//
// Callers are expected to pass simplified inputs as produced by services.SimplifyAzureMatchers:
// trimmed, deduped, with wildcards collapsed to []string{types.Wildcard}. The only input check here is
// that the subscription list is non-empty; everything else is the matcher-layer normalizer's job.
func (c *resourceGraphClient) QueryVMs(ctx context.Context, params QueryVMsParams) ([]DiscoveredVM, error) {
	if len(params.SubscriptionIDs) == 0 {
		return nil, trace.BadParameter("at least one subscription ID is required")
	}

	query := buildVMDiscoveryKQL(params.Regions, params.ResourceGroups)

	var all []DiscoveredVM
	for chunk := range slices.Chunk(params.SubscriptionIDs, argMaxSubscriptionsPerQuery) {
		rows, err := c.queryChunk(ctx, query, chunk)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		all = append(all, rows...)
	}
	return all, nil
}

// queryChunk runs a single ARG query against one chunk of subscription IDs and follows SkipToken
// pagination internally. The pagination loop is bounded by argMaxPagesPerChunk and aborts if the server
// returns a non-advancing SkipToken; both are defenses against runaway loops a buggy server or mock could
// otherwise drive. AccessDenied errors are wrapped with ARG-specific remediation guidance.
func (c *resourceGraphClient) queryChunk(ctx context.Context, query string, subscriptionIDs []string) ([]DiscoveredVM, error) {
	subs := libslices.Map(subscriptionIDs, to.Ptr[string])

	var (
		all       []DiscoveredVM
		lastResp  armresourcegraph.ClientResourcesResponse
		skipToken *string
	)

	for page := range argMaxPagesPerChunk {
		req := armresourcegraph.QueryRequest{
			Query:         to.Ptr(query),
			Subscriptions: subs,
			Options: &armresourcegraph.QueryRequestOptions{
				ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
				Top:          to.Ptr[int32](1000),
				SkipToken:    skipToken,
			},
		}

		resp, err := c.api.Resources(ctx, req, nil)
		if err != nil {
			converted := ConvertResponseError(err)
			if trace.IsAccessDenied(converted) {
				// Note: Resource Graph has no separate authorization gate.
				return nil, trace.Wrap(converted,
					"resource graph query was denied; ensure the credential has "+
						"Microsoft.Compute/virtualMachines/read (e.g. via the Reader role) "+
						"on the queried subscription(s) or a containing management group scope")
			}
			return nil, trace.Wrap(converted)
		}
		lastResp = resp

		rows, err := parseDiscoveredVMs(resp.Data)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		all = append(all, rows...)
		if resp.SkipToken == nil || *resp.SkipToken == "" {
			return all, nil
		}

		// Defend against a buggy server (or mock) that hands back the same SkipToken twice;
		// without this guard the loop would spin forever.
		if skipToken != nil && *skipToken == *resp.SkipToken {
			return nil, trace.Errorf(
				"resource graph returned a non-advancing SkipToken at page %d; aborting (%s)",
				page, resourceGraphResponseSummary(resp))
		}
		skipToken = resp.SkipToken
	}

	return nil, trace.Errorf(
		"resource graph pagination exceeded the %d-page safety cap; aborting (suspected runaway loop; %s)",
		argMaxPagesPerChunk, resourceGraphResponseSummary(lastResp))
}

func resourceGraphResponseSummary(resp armresourcegraph.ClientResourcesResponse) string {
	var parts []string
	if resp.SkipToken != nil && *resp.SkipToken != "" {
		parts = append(parts, fmt.Sprintf("skip_token=%q", *resp.SkipToken))
	}
	if resp.Count != nil {
		parts = append(parts, fmt.Sprintf("count=%d", *resp.Count))
	}
	if resp.TotalRecords != nil {
		parts = append(parts, fmt.Sprintf("total_records=%d", *resp.TotalRecords))
	}
	if resp.ResultTruncated != nil {
		parts = append(parts, fmt.Sprintf("result_truncated=%s", *resp.ResultTruncated))
	}
	if len(parts) == 0 {
		return "response metadata unavailable"
	}
	return strings.Join(parts, ", ")
}

// buildVMDiscoveryKQL composes the KQL query used by QueryVMs. The shape is intentionally fixed:
// the type, OS, and power-state predicates are baked in; only region and resource-group predicates
// are caller-controllable.
//
// Single quotes in inputs are doubled according to KQL's escape rule.
func buildVMDiscoveryKQL(regions []string, resourceGroups []string) string {
	var sb strings.Builder
	sb.WriteString("Resources")
	sb.WriteString("\n| where type =~ 'Microsoft.Compute/virtualMachines'")
	sb.WriteString("\n| where tostring(properties.storageProfile.osDisk.osType) =~ 'Linux'")
	sb.WriteString("\n| where tostring(properties.extended.instanceView.powerState.code) =~ 'PowerState/running'")
	if pred := regionPredicate(regions); pred != "" {
		sb.WriteString("\n")
		sb.WriteString(pred)
	}
	if pred := resourceGroupsPredicate(resourceGroups); pred != "" {
		sb.WriteString("\n")
		sb.WriteString(pred)
	}
	sb.WriteString("\n| project id, name, subscriptionId, resourceGroup, location, tags," +
		"\n          vmId = tostring(properties.vmId)")
	return sb.String()
}

// regionPredicate returns a KQL `| where location in~ (...)` clause, or empty string when the
// filter is effectively unset. Uses case-insensitive set membership (in~) because ARG normalizes
// the `location` column to canonical lowercase, but operators may configure matchers with
// display-cased values like "EastUS".
//
// Any occurrence of types.Wildcard is treated as "match everything" to avoid
// interpreting unsimplified wildcard-containing input as a literal ARG value.
func regionPredicate(regions []string) string {
	if isMatchAll(regions) {
		return ""
	}
	return "| where location in~ (" + strings.Join(libslices.Map(regions, quoteKQL), ", ") + ")"
}

// resourceGroupsPredicate returns a KQL `| where resourceGroup in~ (...)` clause, or empty string
// when the filter is effectively unset. Uses case-insensitive set membership (in~) because ARM
// resource path segments are case-insensitive.
//
// Any occurrence of types.Wildcard is treated as "match everything" to avoid
// interpreting unsimplified wildcard-containing input as a literal ARG value.
func resourceGroupsPredicate(rgs []string) string {
	if isMatchAll(rgs) {
		return ""
	}
	return "| where resourceGroup in~ (" + strings.Join(libslices.Map(rgs, quoteKQL), ", ") + ")"
}

// isMatchAll reports whether values is an unset filter or contains an explicit wildcard. Callers
// normally pass values simplified by services.SimplifyAzureMatchers, but treating wildcard as absorbing
// here prevents unsimplified inputs from silently narrowing discovery by treating "*" as a literal ARG value.
func isMatchAll(values []string) bool {
	return len(values) == 0 || slices.Contains(values, types.Wildcard)
}

// quoteKQL escapes single quotes in s and wraps the result in single quotes to produce a KQL string literal.
func quoteKQL(s string) string {
	return "'" + escapeKQL(s) + "'"
}

// escapeKQL escapes single quotes in a KQL string literal by doubling them.
func escapeKQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// parseDiscoveredVMs parses Resource Graph QueryResponse.Data into VMs.
// Missing, empty, or wrongly typed required fields return an error.
func parseDiscoveredVMs(data any) ([]DiscoveredVM, error) {
	if data == nil {
		return nil, nil
	}
	rows, ok := data.([]any)
	if !ok {
		return nil, trace.BadParameter("resource graph response Data has unexpected type %T (expected []any)", data)
	}
	out := make([]DiscoveredVM, 0, len(rows))
	for i, row := range rows {
		m, ok := row.(map[string]any)
		if !ok {
			return nil, trace.BadParameter("resource graph response row %d has unexpected type %T (expected map[string]any)", i, row)
		}
		vm, err := parseDiscoveredVMRow(m)
		if err != nil {
			return nil, trace.Wrap(err, "parsing resource graph row %d", i)
		}
		out = append(out, vm)
	}
	return out, nil
}

// parseDiscoveredVMRow extracts a DiscoveredVM from a single ARG response row.
// Missing, empty, or wrongly typed required fields return an error.
func parseDiscoveredVMRow(m map[string]any) (DiscoveredVM, error) {
	fields := utils.Fields(m)
	id, err := getRequiredARGString(fields, "id")
	if err != nil {
		return DiscoveredVM{}, err
	}
	subID, err := getRequiredARGString(fields, "subscriptionId")
	if err != nil {
		return DiscoveredVM{}, err
	}
	name, err := getRequiredARGString(fields, "name")
	if err != nil {
		return DiscoveredVM{}, err
	}
	vmID, err := getRequiredARGString(fields, "vmId")
	if err != nil {
		return DiscoveredVM{}, err
	}
	location, err := getRequiredARGString(fields, "location")
	if err != nil {
		return DiscoveredVM{}, err
	}
	rg, err := getRequiredARGString(fields, "resourceGroup")
	if err != nil {
		return DiscoveredVM{}, err
	}
	tags, err := fields.GetStringMap("tags")
	if err != nil {
		return DiscoveredVM{}, err
	}
	return DiscoveredVM{
		ID:             id,
		SubscriptionID: subID,
		Name:           name,
		VMID:           vmID,
		Location:       location,
		ResourceGroup:  rg,
		Tags:           tags,
	}, nil
}

// getRequiredARGString returns a required string field from an ARG row.
func getRequiredARGString(fields utils.Fields, key string) (string, error) {
	value, err := fields.GetStringIfPresent(key)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", trace.BadParameter("resource graph response row missing or empty required field %q", key)
	}
	return value, nil
}
