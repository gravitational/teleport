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
	"log/slog"
	"slices"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	libslices "github.com/gravitational/teleport/lib/utils/slices"
)

// ResourceGraphClient is a client for Azure Resource Graph (ARG) VM discovery.
type ResourceGraphClient interface {
	// QueryVMs returns running VMs matching the supplied scope and filters.
	QueryVMs(ctx context.Context, params QueryVMsParams) ([]DiscoveredVM, error)
}

// QueryVMsParams scopes a Resource Graph VM query to a set of subscriptions
// and an optional set of regions, resource groups, and OS types.
//
// QueryVMs does not deduplicate the region, resource-group, or OS-type lists; callers are expected to
// simplify them by trimming, deduping, and collapsing wildcards. Any types.Wildcard anywhere in a list
// subsumes all other entries in that list and causes that filter to match every value even when the
// list also contains entries that would narrow it.
type QueryVMsParams struct {
	// SubscriptionIDs is the set of Azure subscriptions to query, e.g.
	// "11111111-1111-1111-1111-111111111111". QueryVMs requires at least one entry.
	// Empty is an error, not a wildcard: this is the query scope, not a filter.
	// QueryVMs rejects empty or untrimmed entries; duplicates pass through (caller's job).
	SubscriptionIDs []string
	// Regions filters VMs by location. An empty slice or any occurrence of types.Wildcard matches every region.
	Regions []string
	// ResourceGroups filters VMs by resource group. An empty slice or any
	// occurrence of types.Wildcard matches every resource group.
	ResourceGroups []string
	// OSTypes filters VMs by osDisk.osType, e.g. []OSType{OSTypeLinux, OSTypeWindows}.
	// An empty slice or any occurrence of types.Wildcard matches every OS type.
	OSTypes []OSType
}

// OSType is the closed enum of OS family values that QueryVMsParams.OSTypes accepts
// and that DiscoveredVM.OSType reports. Microsoft documents the full set as exactly
// Linux and Windows; callers must use the constants below rather than constructing
// OSType values directly. ARG records osDisk.osType in this canonical case.
type OSType string

// OS-type values accepted by QueryVMsParams.OSTypes and reported by DiscoveredVM.OSType.
// Strict canonical case: validation rejects any other casing or whitespace.
const (
	OSTypeLinux   OSType = "Linux"
	OSTypeWindows OSType = "Windows"
)

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

// argPageSize is the per-page row limit passed to Resource Graph as QueryRequestOptions.Top.
// The SDK names the field "Top" after the underlying REST parameter; we alias it here to make
// the call site read as a page size and to give argMaxPagesPerChunk a real symbol to reference.
const argPageSize int32 = 1000

// argMaxPagesPerChunk bounds the SkipToken pagination loop in a single chunk's worth of Resource Graph results.
// QueryVMs asks for argPageSize rows per page, so this allows up to argPageSize x argMaxPagesPerChunk rows
// (about one million) per chunk before treating pagination as runaway.
//
// Source: https://learn.microsoft.com/en-us/azure/governance/resource-graph/concepts/guidance-for-throttled-requests#pagination
const argMaxPagesPerChunk = 1000

// Resource Graph query vocabulary used by buildVMDiscoveryKQL. Named so the query's external
// dependencies are explicit and a future Microsoft schema rename has one greppable place to update.
//
// Semantic-intent values (type, running power state) define what "discoverable VM" means for this query.
// Path values are schema dependencies on the shape of the Microsoft.Compute/virtualMachines projection returned by ARG.
const (
	argVMType            = "Microsoft.Compute/virtualMachines"
	argRunningPowerState = "PowerState/running"

	argPowerStatePath = "properties.extended.instanceView.powerState.code"
	argOSTypePath     = "properties.storageProfile.osDisk.osType"
	argVMIDPath       = "properties.vmId"
)

// QueryVMs runs the discovery query against Resource Graph and translates the rows to []DiscoveredVM.
//
// Callers are expected to pass simplified inputs satisfying the QueryVMsParams documented contract:
// trimmed, deduped, with wildcards collapsed to []string{types.Wildcard}. QueryVMs defensively rejects
// empty or untrimmed subscription IDs and filter values; deduplication remains the caller's job.
func (c *resourceGraphClient) QueryVMs(ctx context.Context, params QueryVMsParams) ([]DiscoveredVM, error) {
	sanitized, err := sanitizeQueryVMsParams(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	query := buildVMDiscoveryKQL(sanitized)

	var (
		all          []DiscoveredVM
		rawRowsTotal int
	)
	for chunk := range slices.Chunk(sanitized.SubscriptionIDs, argMaxSubscriptionsPerQuery) {
		result, err := c.queryChunk(ctx, query, chunk)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		all = append(all, result.vms...)
		rawRowsTotal += result.rawRowsTotal
	}
	if rawRowsTotal > 0 && len(all) == 0 {
		// Systemic drift: ARG returned rows across the query but none parsed.
		// Evaluated at query level (not per page or per chunk) so one bad page
		// or subscription chunk cannot discard valid VMs found elsewhere.
		return nil, trace.Errorf(
			"resource graph query returned %d rows but none could be parsed; "+
				"likely contract drift (renamed field or shifted type)", rawRowsTotal)
	}

	return all, nil
}

type queryVMsResult struct {
	vms          []DiscoveredVM
	rawRowsTotal int
}

// queryChunk runs a single ARG query against one chunk of subscription IDs and follows SkipToken pagination internally.
// The pagination loop is bounded by argMaxPagesPerChunk, a defense against runaway loops a buggy server or mock could
// otherwise drive. AccessDenied errors are wrapped with ARG-specific remediation guidance.
func (c *resourceGraphClient) queryChunk(ctx context.Context, query string, subscriptionIDs []string) (queryVMsResult, error) {
	subs := libslices.Map(subscriptionIDs, to.Ptr[string])

	var (
		all          []DiscoveredVM
		lastResp     armresourcegraph.ClientResourcesResponse
		skipToken    *string
		rawRowsTotal int
	)

	// Logged once per chunk: the KQL text and scope do not change across the
	// SkipToken loop. Per-page diagnostic (page index, response summary) is
	// emitted inside the loop after each Resources call; the chunk-complete
	// log fires on the success return below with aggregate counts.
	slog.DebugContext(ctx, "Resource Graph query starting",
		"subscription_count", len(subscriptionIDs),
		"query", query)

	for page := range argMaxPagesPerChunk {
		req := armresourcegraph.QueryRequest{
			Query:         to.Ptr(query),
			Subscriptions: subs,
			Options: &armresourcegraph.QueryRequestOptions{
				ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
				Top:          to.Ptr(argPageSize),
				SkipToken:    skipToken,
			},
		}

		resp, err := c.api.Resources(ctx, req, nil)
		if err != nil {
			converted := ConvertResponseError(err)
			if trace.IsAccessDenied(converted) {
				// Resource Graph has no separate authorization gate: ARG derives access from the
				// caller's read permissions on the queried resources. A 403 here means the credential
				// has no read access to any subscription in the chunk. See:
				// https://learn.microsoft.com/en-us/azure/governance/resource-graph/overview#permissions-in-azure-resource-graph
				return queryVMsResult{}, trace.Wrap(converted,
					"resource graph query was denied; ensure the credential has "+
						"Microsoft.Compute/virtualMachines/read (e.g. via the Reader role) "+
						"on the queried subscription(s) or a containing management group scope")
			}
			return queryVMsResult{}, trace.Wrap(converted)
		}
		lastResp = resp

		slog.DebugContext(ctx, "Resource Graph page returned",
			"page", page,
			"summary", resourceGraphResponseSummary(resp))

		rows, err := parseDiscoveredVMs(ctx, resp.Data)
		if err != nil {
			return queryVMsResult{}, trace.Wrap(err)
		}
		// parseDiscoveredVMs returned no error: Data was nil or []any. Track raw row count
		// across pages so QueryVMs can distinguish "no rows ever returned" (empty tenant)
		// from "rows returned but none parsed" (systemic schema drift) across the whole query.
		if data, ok := resp.Data.([]any); ok {
			rawRowsTotal += len(data)
		}

		all = append(all, rows...)
		if resp.SkipToken == nil || *resp.SkipToken == "" {
			if resp.ResultTruncated != nil && *resp.ResultTruncated == armresourcegraph.ResultTruncatedTrue {
				return queryVMsResult{}, trace.Errorf(
					"resource graph response was truncated but did not include a skip token; "+
						"results are incomplete (%s)",
					resourceGraphResponseSummary(resp))
			}
			slog.DebugContext(ctx, "Resource Graph chunk complete",
				"pages", page+1,
				"rows_kept", len(all),
				"raw_rows_total", rawRowsTotal)
			return queryVMsResult{vms: all, rawRowsTotal: rawRowsTotal}, nil
		}
		skipToken = resp.SkipToken
	}

	return queryVMsResult{}, trace.Errorf(
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
		parts = append(parts, fmt.Sprintf("result_truncated=%v", *resp.ResultTruncated))
	}
	if len(parts) == 0 {
		return "response metadata unavailable"
	}
	return strings.Join(parts, ", ")
}

// buildVMDiscoveryKQL composes the KQL (Kusto Query Language) query used by QueryVMs.
// The shape is intentionally fixed: type and power-state predicates are baked in;
// OS, region, and resource-group predicates are caller-controllable.
//
// Single quotes in inputs are doubled according to KQL's escape rule.
//
// The sanitizedParams type is constructed only by sanitizeQueryVMsParams (in
// kqlsafety.go), which allowlists every interpolated literal. Injection safety
// is therefore a compile-time invariant: an unsanitized input cannot reach this
// function. quoteKQL is defense-in-depth on top of that guarantee.
//
// All comparisons use case-insensitive KQL operators (`=~` for equality, `in~` for
// set membership), not `==` or `in`. ARG normalizes some columns to lowercase
// (e.g. `resourceGroup`) and preserves portal casing on others, so case-insensitive
// comparison matches uniformly without the caller needing to know which is which.
//
// Note: when adding a new `| where` clause here, use `=~` / `in~`, not `==` / `in`.
func buildVMDiscoveryKQL(params sanitizedParams) string {
	var sb strings.Builder
	sb.WriteString("Resources")
	sb.WriteString("\n| where type =~ " + quoteKQL(argVMType))
	if pred := osTypesPredicate(params.OSTypes); pred != "" {
		sb.WriteString("\n")
		sb.WriteString(pred)
	}
	sb.WriteString("\n| where tostring(" + argPowerStatePath + ") =~ " + quoteKQL(argRunningPowerState))
	if pred := regionPredicate(params.Regions); pred != "" {
		sb.WriteString("\n")
		sb.WriteString(pred)
	}
	if pred := resourceGroupsPredicate(params.ResourceGroups); pred != "" {
		sb.WriteString("\n")
		sb.WriteString(pred)
	}
	sb.WriteString("\n| project id, name, subscriptionId, resourceGroup, location, tags," +
		"\n          vmId = tostring(" + argVMIDPath + ")," +
		"\n          osType = tostring(" + argOSTypePath + ")")
	return sb.String()
}

// regionPredicate returns a KQL `| where location in~ (...)` clause, or empty string when the
// filter is effectively unset. Uses case-insensitive set membership (in~) because ARG normalizes
// the `location` column to canonical lowercase, but callers may pass region names in their
// display-case form like "EastUS".
//
// Any occurrence of types.Wildcard is treated as "match everything" to avoid
// interpreting unsimplified wildcard-containing input as a region name.
func regionPredicate(regions []string) string {
	if isMatchAll(regions) {
		return ""
	}
	return "| where location in~ (" + strings.Join(libslices.Map(regions, quoteKQL), ", ") + ")"
}

// resourceGroupsPredicate returns a KQL `| where resourceGroup in~ (...)` clause, or empty string
// when the filter is effectively unset. Uses case-insensitive set membership (in~) because ARM
// (Azure Resource Manager) resource path segments are case-insensitive.
//
// Any occurrence of types.Wildcard is treated as "match everything" to avoid
// interpreting unsimplified wildcard-containing input as a resource group name.
func resourceGroupsPredicate(rgs []string) string {
	if isMatchAll(rgs) {
		return ""
	}
	return "| where resourceGroup in~ (" + strings.Join(libslices.Map(rgs, quoteKQL), ", ") + ")"
}

// osTypesPredicate returns a KQL clause filtering VMs by osDisk.osType, or empty string when the
// filter is effectively unset. Uses case-insensitive set membership (in~) for symmetry with
// regionPredicate and resourceGroupsPredicate; the file-wide rationale for case-insensitive
// operators is documented on buildVMDiscoveryKQL.
//
// Any occurrence of types.Wildcard is treated as "match everything" to avoid
// interpreting unsimplified wildcard-containing input as an OS-type name.
func osTypesPredicate(osTypes []OSType) string {
	if isMatchAll(osTypes) {
		return ""
	}
	quoted := libslices.Map(osTypes, func(t OSType) string { return quoteKQL(string(t)) })
	return "| where tostring(" + argOSTypePath + ") in~ (" + strings.Join(quoted, ", ") + ")"
}

// isMatchAll reports whether values is an unset filter or contains an explicit wildcard. Callers
// normally pass values trimmed, deduped, and with wildcards collapsed, but treating wildcard as absorbing
// here prevents unsimplified inputs from silently narrowing results by treating "*" as a value to match.
// Generic over ~string so the same predicate covers []string (Regions, ResourceGroups) and []OSType.
func isMatchAll[S ~string](values []S) bool {
	return len(values) == 0 || slices.Contains(values, S(types.Wildcard))
}

// parseDiscoveredVMs parses Resource Graph QueryResponse.Data into VMs.
// Malformed rows are skipped; outer-shape drift returns an error.
func parseDiscoveredVMs(ctx context.Context, data any) ([]DiscoveredVM, error) {
	if data == nil {
		return nil, nil
	}

	rows, ok := data.([]any)
	if !ok {
		return nil, trace.BadParameter("resource graph response Data has unexpected type %T (expected []any)", data)
	}

	out := make([]DiscoveredVM, 0, len(rows))
	skipped := 0
	for i, row := range rows {
		m, ok := row.(map[string]any)
		if !ok {
			// Per-row at debug so a persistently malformed VM doesn't flood logs
			// every poll cycle. The summary warn below fires once per call.
			slog.DebugContext(ctx, "Skipping Resource Graph row with unexpected type",
				"row", i, "got_type", fmt.Sprintf("%T", row))
			skipped++
			continue
		}
		vm, err := parseDiscoveredVMRow(ctx, m)
		if err != nil {
			slog.DebugContext(ctx, "Skipping malformed Resource Graph row",
				"row", i, "error", err)
			skipped++
			continue
		}
		out = append(out, vm)
	}

	if skipped > 0 {
		// One warn per affected response, regardless of how many rows were bad.
		// Row-level detail is at debug above for operators investigating.
		// Systemic drift (every row failing across the query) is detected in
		// QueryVMs after all chunks complete, not per-page here, so that
		// isolated malformed pages/chunks don't discard valid VMs found elsewhere.
		slog.WarnContext(ctx, "Resource Graph returned malformed rows",
			"skipped", skipped, "kept", len(out))
	}

	return out, nil
}

// parseDiscoveredVMRow extracts a DiscoveredVM from a single ARG response row.
// Identity-field drift errors; tag drift is best-effort: malformed outer tags
// yield an empty map, and malformed inner entries are dropped.
func parseDiscoveredVMRow(ctx context.Context, m map[string]any) (DiscoveredVM, error) {
	id, err := getRequiredARGString(m, "id")
	if err != nil {
		return DiscoveredVM{}, err
	}
	subID, err := getRequiredARGString(m, "subscriptionId")
	if err != nil {
		return DiscoveredVM{}, err
	}
	name, err := getRequiredARGString(m, "name")
	if err != nil {
		return DiscoveredVM{}, err
	}
	vmID, err := getRequiredARGString(m, "vmId")
	if err != nil {
		return DiscoveredVM{}, err
	}
	location, err := getRequiredARGString(m, "location")
	if err != nil {
		return DiscoveredVM{}, err
	}
	rg, err := getRequiredARGString(m, "resourceGroup")
	if err != nil {
		return DiscoveredVM{}, err
	}
	// OS type is not identity: empty when ARG omits the field is fine. Non-string
	// drift propagates to parseDiscoveredVMs's per-row skip path. ARG returns this
	// in canonical case ("Linux" / "Windows") per the documented Azure enum; if a
	// future ARG response drifts to lowercase or an unknown value, it surfaces as
	// OSType(<raw>) for the consumer to see rather than being silently rejected here.
	osType, err := getStringIfPresent(m, "osType")
	if err != nil {
		return DiscoveredVM{}, err
	}
	tags := getStringMap(ctx, m, "tags")

	return DiscoveredVM{
		ID:             id,
		SubscriptionID: subID,
		Name:           name,
		VMID:           vmID,
		Location:       location,
		ResourceGroup:  rg,
		OSType:         OSType(osType),
		Tags:           tags,
	}, nil
}

// getRequiredARGString returns a required string field from an ARG row.
func getRequiredARGString(m map[string]any, key string) (string, error) {
	value, err := getStringIfPresent(m, key)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", trace.BadParameter("resource graph response row missing or empty required field %q", key)
	}

	return value, nil
}

// getStringIfPresent returns the string value at key in an ARG row. Missing or
// nil values return "", nil. Non-string values return BadParameter.
func getStringIfPresent(m map[string]any, key string) (string, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", trace.BadParameter("field %q has unexpected type %T (expected string)", key, v)
	}

	return s, nil
}

// getStringMap extracts a map[string]string from an ARG row's nested object
// field. It is best-effort: malformed shape (missing, nil, or wrong-type outer;
// nil or non-string inner values) is logged at debug and the affected entries
// are dropped, rather than failing the parent row.
//
// Non-string values are dropped, not stringified. The general rule is that
// callers may use tag values as inputs to identity, authorization, or selector
// logic, where a fmt.Sprint'd Go literal (e.g. "map[k:v]") could be
// misinterpreted as a value Azure actually returned. The live caller making
// this concrete today is lib/srv/server/azure_watcher.go, which feeds vm.Tags
// into services.MatchLabels for discovery label matching; dropping non-string
// values enforces "selectors only see literal string values Azure returned" so
// drifted ARG output cannot present unintended selector matches.
func getStringMap(ctx context.Context, m map[string]any, key string) map[string]string {
	raw, ok := m[key]
	if !ok || raw == nil {
		return map[string]string{}
	}
	asMap, ok := raw.(map[string]any)
	if !ok {
		slog.DebugContext(ctx, "Resource Graph row has malformed tags; using empty map",
			"key", key, "got_type", fmt.Sprintf("%T", raw))
		return map[string]string{}
	}
	out := make(map[string]string, len(asMap))
	for k, v := range asMap {
		if v == nil {
			continue
		}
		s, ok := v.(string)
		if !ok {
			slog.DebugContext(ctx, "Dropping non-string Resource Graph tag value",
				"key", key, "tag_key", k, "got_type", fmt.Sprintf("%T", v))
			continue
		}
		out[k] = s
	}

	return out
}
