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
	"regexp"
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
	// SubscriptionIDs is the set of Azure subscriptions to query. QueryVMs requires at least one entry.
	// Empty is an error, not a wildcard: this is the query scope, not a filter.
	// QueryVMs rejects empty or untrimmed entries; duplicates pass through (caller's job).
	SubscriptionIDs []string
	// Regions filters VMs by location. An empty slice or any occurrence of types.Wildcard matches every region.
	Regions []string
	// ResourceGroups filters VMs by resource group. An empty slice or any
	// occurrence of types.Wildcard matches every resource group.
	ResourceGroups []string
	// OSTypes filters VMs by osDisk.osType, e.g. []string{OSTypeLinux, OSTypeWindows}.
	// An empty slice or any occurrence of types.Wildcard matches every OS type.
	OSTypes []string
	// IncludePrimaryPrivateIP requests that QueryVMs populate DiscoveredVM.PrimaryPrivateIP by
	// issuing a follow-up query against Microsoft.Network/networkInterfaces and applying the
	// primary-of-primary rule on the Go side. Adds one ARG round trip per NIC-ID chunk (chunks
	// are sized to fit ARG's 32 KB query-length limit). Leave false for callers that don't need
	// the IP — e.g. Linux VM discovery using run-command install.
	IncludePrimaryPrivateIP bool
}

// OS-type values accepted by QueryVMsParams.OSTypes. ARG records osDisk.osType in
// canonical case; matching is case-insensitive so callers may pass any casing, but
// these constants give a single source of truth and catch typos at compile time.
const (
	OSTypeLinux   = "Linux"
	OSTypeWindows = "Windows"
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

// argMaxQueryBytes is Resource Graph's documented query-string length limit. chunkNICIDsForQuery
// uses it to size each NIC-query request so its in~ (...) clause never overflows.
//
// Source: https://learn.microsoft.com/en-us/azure/governance/resource-graph/concepts/query-language#limits
const argMaxQueryBytes = 32 * 1024

// Resource Graph query vocabulary used by buildVMDiscoveryKQL and buildNICQuery. Named so the
// queries' external dependencies are explicit and a future Microsoft schema rename has one
// greppable place to update.
//
// Semantic-intent values (type, running power state) define what "discoverable VM" means for these
// queries. Path values are schema dependencies on the shape of the Microsoft.Compute/virtualMachines
// and Microsoft.Network/networkInterfaces projections returned by ARG.
const (
	argVMType            = "Microsoft.Compute/virtualMachines"
	argNICType           = "Microsoft.Network/networkInterfaces"
	argRunningPowerState = "PowerState/running"

	argPowerStatePath       = "properties.extended.instanceView.powerState.code"
	argOSTypePath           = "properties.storageProfile.osDisk.osType"
	argVMIDPath             = "properties.vmId"
	argNICRefsPath          = "properties.networkProfile.networkInterfaces"
	argNICIPConfigsPath     = "properties.ipConfigurations"
	argPrivateIPAddressPath = "properties.privateIPAddress"
	argPrimaryFlagPath      = "properties.primary"
)

// QueryVMs runs the discovery query against Resource Graph and translates the rows to []DiscoveredVM.
//
// Callers are expected to pass simplified inputs satisfying the QueryVMsParams documented contract:
// trimmed, deduped, with wildcards collapsed to []string{types.Wildcard}. QueryVMs defensively rejects
// empty or untrimmed subscription IDs and filter values; deduplication remains the caller's job.
//
// When params.IncludePrimaryPrivateIP is set, QueryVMs issues a second query against
// Microsoft.Network/networkInterfaces scoped exactly to the NIC IDs referenced by the discovered
// VMs, then resolves DiscoveredVM.PrimaryPrivateIP on the Go side. Without the flag, only the
// VM-side query runs.
func (c *resourceGraphClient) QueryVMs(ctx context.Context, params QueryVMsParams) ([]DiscoveredVM, error) {
	if err := validateQueryVMsParams(params); err != nil {
		return nil, trace.Wrap(err)
	}

	query := buildVMDiscoveryKQL(params.Regions, params.ResourceGroups, params.OSTypes, params.IncludePrimaryPrivateIP)

	var (
		all          []DiscoveredVM
		rawRowsTotal int
	)
	for chunk := range slices.Chunk(params.SubscriptionIDs, argMaxSubscriptionsPerQuery) {
		result, err := c.queryVMChunk(ctx, query, chunk)
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

	if !params.IncludePrimaryPrivateIP {
		return all, nil
	}

	nicMap, err := c.fetchNICIPConfigs(ctx, collectNICIDs(all), params.SubscriptionIDs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resolvePrimaryPrivateIPs(all, nicMap)
	return all, nil
}

// nicRef is one entry from a VM's networkProfile.networkInterfaces[]: the NIC's ARM ID
// (lowercased so the NIC-map lookup is case-insensitive) and the VM-side primary flag.
// primary is false when Azure omitted the flag — selectPrimaryPrivateIP treats that as
// "not explicitly primary" and falls back to the implicit-primary rule when appropriate.
type nicRef struct {
	id      string
	primary bool
}

// ipConfigRow is one IP configuration projected from a NIC's properties.ipConfigurations[]
// by the NIC query. primary is the IP-config-level primary flag (false when omitted).
type ipConfigRow struct {
	primary   bool
	privateIP string
}

// queryVMsResult bundles a chunk's worth of parsed VMs plus the raw row count, so QueryVMs
// can distinguish "no rows ever returned" (empty tenant) from "rows returned but none parsed"
// (systemic schema drift) at query level — not per-page, not per-chunk.
type queryVMsResult struct {
	vms          []DiscoveredVM
	rawRowsTotal int
}

// queryVMChunk runs the VM query against one chunk of subscription IDs and follows SkipToken
// pagination internally. The pagination loop is bounded by argMaxPagesPerChunk, a defense against
// runaway loops a buggy server or mock could otherwise drive. AccessDenied errors are wrapped with
// ARG-specific remediation guidance.
func (c *resourceGraphClient) queryVMChunk(ctx context.Context, query string, subscriptionIDs []string) (queryVMsResult, error) {
	subs := libslices.Map(subscriptionIDs, to.Ptr[string])

	var (
		all          []DiscoveredVM
		lastResp     armresourcegraph.ClientResourcesResponse
		skipToken    *string
		rawRowsTotal int
	)

	for range argMaxPagesPerChunk {
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

		rows, err := parseVMRows(ctx, resp.Data)
		if err != nil {
			return queryVMsResult{}, trace.Wrap(err)
		}
		// parseVMRows returned no error → Data was nil or []any. Track raw row count
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
			return queryVMsResult{vms: all, rawRowsTotal: rawRowsTotal}, nil
		}
		skipToken = resp.SkipToken
	}

	return queryVMsResult{}, trace.Errorf(
		"resource graph pagination exceeded the %d-page safety cap; aborting (suspected runaway loop; %s)",
		argMaxPagesPerChunk, resourceGraphResponseSummary(lastResp))
}

// fetchNICIPConfigs runs the NIC query as many times as needed to cover all the NIC IDs, paginates
// each invocation, and merges results into a map keyed by lowercased NIC ARM ID. NIC IDs are
// batched by query-byte budget (chunkNICIDsForQuery); subscription IDs are batched by
// argMaxSubscriptionsPerQuery. For the common case of ≤ 200 subscriptions there is exactly one
// subscription chunk, so total work scales with NIC-ID chunks only.
func (c *resourceGraphClient) fetchNICIPConfigs(ctx context.Context, nicIDs []string, subscriptionIDs []string) (map[string][]ipConfigRow, error) {
	out := make(map[string][]ipConfigRow, len(nicIDs))
	if len(nicIDs) == 0 {
		return out, nil
	}
	prefix, suffix := nicQueryEnvelope()
	for _, nicChunk := range chunkNICIDsForQuery(nicIDs, prefix, suffix) {
		query := buildNICQuery(nicChunk)
		for subChunk := range slices.Chunk(subscriptionIDs, argMaxSubscriptionsPerQuery) {
			chunkResult, err := c.queryNICChunk(ctx, query, subChunk)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			for id, configs := range chunkResult {
				out[id] = configs
			}
		}
	}
	return out, nil
}

// queryNICChunk runs the NIC query against one chunk of subscription IDs and follows SkipToken
// pagination internally. Returns a map keyed by lowercased NIC ARM ID matching the query's tolower
// projection.
func (c *resourceGraphClient) queryNICChunk(ctx context.Context, query string, subscriptionIDs []string) (map[string][]ipConfigRow, error) {
	subs := libslices.Map(subscriptionIDs, to.Ptr[string])

	out := map[string][]ipConfigRow{}
	var (
		lastResp  armresourcegraph.ClientResourcesResponse
		skipToken *string
	)

	for range argMaxPagesPerChunk {
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
				return nil, trace.Wrap(converted,
					"resource graph NIC query was denied; ensure the credential has "+
						"Microsoft.Network/networkInterfaces/read (e.g. via the Reader role) "+
						"on the queried subscription(s) or a containing management group scope")
			}
			return nil, trace.Wrap(converted)
		}
		lastResp = resp

		if err := parseNICs(ctx, resp.Data, out); err != nil {
			return nil, trace.Wrap(err)
		}

		if resp.SkipToken == nil || *resp.SkipToken == "" {
			if resp.ResultTruncated != nil && *resp.ResultTruncated == armresourcegraph.ResultTruncatedTrue {
				return nil, trace.Errorf(
					"resource graph NIC response was truncated but did not include a skip token; "+
						"results are incomplete (%s)",
					resourceGraphResponseSummary(resp))
			}
			return out, nil
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
		parts = append(parts, fmt.Sprintf("result_truncated=%v", *resp.ResultTruncated))
	}
	if len(parts) == 0 {
		return "response metadata unavailable"
	}
	return strings.Join(parts, ", ")
}

// buildVMDiscoveryKQL composes the VM-side KQL query used by QueryVMs. The shape is intentionally
// fixed: type and power-state predicates are baked in; OS, region, and resource-group predicates
// are caller-controllable.
//
// When includePrimaryPrivateIP is true, the projection adds a nicRefs column carrying the VM's
// networkProfile.networkInterfaces array verbatim. QueryVMs uses these references to issue a
// follow-up NIC query against Microsoft.Network/networkInterfaces and resolves PrimaryPrivateIP
// on the Go side via selectPrimaryPrivateIP. The two-query split keeps each query simple and lets
// ARG's indexes serve each lookup independently.
//
// Single quotes in inputs are doubled according to KQL's escape rule.
func buildVMDiscoveryKQL(regions []string, resourceGroups []string, osTypes []string, includePrimaryPrivateIP bool) string {
	var sb strings.Builder
	sb.WriteString("Resources")
	sb.WriteString("\n| where type =~ " + quoteKQL(argVMType))
	if pred := osTypesPredicate(osTypes); pred != "" {
		sb.WriteString("\n")
		sb.WriteString(pred)
	}
	sb.WriteString("\n| where tostring(" + argPowerStatePath + ") =~ " + quoteKQL(argRunningPowerState))
	if pred := regionPredicate(regions); pred != "" {
		sb.WriteString("\n")
		sb.WriteString(pred)
	}
	if pred := resourceGroupsPredicate(resourceGroups); pred != "" {
		sb.WriteString("\n")
		sb.WriteString(pred)
	}
	sb.WriteString("\n| project id, name, subscriptionId, resourceGroup, location, tags," +
		"\n          vmId = tostring(" + argVMIDPath + ")," +
		"\n          osType = tostring(" + argOSTypePath + ")")
	if includePrimaryPrivateIP {
		sb.WriteString("," +
			"\n          nicRefs = " + argNICRefsPath)
	}
	return sb.String()
}

// nicQueryEnvelope returns the fixed prefix and suffix wrapping the in~ (...) clause in the NIC
// query. chunkNICIDsForQuery uses their lengths to size each chunk's ID list to fit under
// argMaxQueryBytes without estimating overhead heuristically.
func nicQueryEnvelope() (prefix, suffix string) {
	prefix = "Resources" +
		"\n| where type =~ " + quoteKQL(argNICType) +
		"\n| where id in~ ("
	suffix = ")" +
		"\n| project id = tolower(tostring(id))," +
		"\n          ipConfigs = " + argNICIPConfigsPath
	return prefix, suffix
}

// buildNICQuery composes the NIC query for a single chunk of NIC ARM IDs. The chunk is expected
// to have passed through chunkNICIDsForQuery, which sized it so the resulting query stays under
// argMaxQueryBytes. NIC IDs come from ARG itself (the projected id column on the VM query's
// nicRefs) rather than caller-supplied input, so they don't need the regex allowlist that filter
// values do — only KQL single-quote escaping via quoteKQL.
func buildNICQuery(ids []string) string {
	prefix, suffix := nicQueryEnvelope()
	var sb strings.Builder
	sb.Grow(len(prefix) + len(suffix) + len(ids)*(len(ids[0])+3))
	sb.WriteString(prefix)
	for i, id := range ids {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(quoteKQL(id))
	}
	sb.WriteString(suffix)
	return sb.String()
}

// chunkNICIDsForQuery splits NIC IDs into chunks each of which, when wrapped by nicQueryEnvelope,
// stays under argMaxQueryBytes. Each ID contributes len(id)+3 bytes to the in~ (...) clause (single
// quotes + comma; the final entry's trailing comma overcounts by one — harmless slack). Chunks
// preserve input order, so callers that need reproducible chunking should sort and dedupe first.
func chunkNICIDsForQuery(ids []string, prefix, suffix string) [][]string {
	budget := argMaxQueryBytes - len(prefix) - len(suffix)
	var chunks [][]string
	var current []string
	size := 0
	for _, id := range ids {
		cost := len(id) + 3 // open quote, close quote, comma separator
		if len(current) > 0 && size+cost > budget {
			chunks = append(chunks, current)
			current = nil
			size = 0
		}
		current = append(current, id)
		size += cost
	}
	if len(current) > 0 {
		chunks = append(chunks, current)
	}
	return chunks
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
// resource path segments are case-insensitive.
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
// filter is effectively unset. Uses case-insensitive set membership (in~) because ARG records
// osType in canonical form ("Linux", "Windows") but callers may pass any casing.
//
// Any occurrence of types.Wildcard is treated as "match everything" to avoid
// interpreting unsimplified wildcard-containing input as an OS-type name.
func osTypesPredicate(osTypes []string) string {
	if isMatchAll(osTypes) {
		return ""
	}
	return "| where tostring(" + argOSTypePath + ") in~ (" + strings.Join(libslices.Map(osTypes, quoteKQL), ", ") + ")"
}

// isMatchAll reports whether values is an unset filter or contains an explicit wildcard. Callers
// normally pass values trimmed, deduped, and with wildcards collapsed, but treating wildcard as absorbing
// here prevents unsimplified inputs from silently narrowing results by treating "*" as a value to match.
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

// Defense-in-depth allowlists for caller-supplied values that flow into KQL string literals.
// Each is a Teleport-supported safe subset of Azure's naming surface, narrow enough to reject any
// character that could break out of a single-quoted KQL string (quote, backslash, newline, null, etc.).
//
// types.Wildcard ("*") is whitelisted by validateKQLValues; the predicate
// helpers absorb wildcards before any KQL is generated.
var (
	// azureRegionPattern is an injection-safety allowlist for region values:
	// alphanumeric only, matching Azure's display and canonical forms.
	azureRegionPattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)
	// azureResourceGroupPattern is our safe subset of Azure resource group
	// names: letters, digits, underscore, hyphen, period, and parenthesis.
	// Length is enforced by Azure; this pattern only blocks unsafe chars.
	azureResourceGroupPattern = regexp.MustCompile(`^[A-Za-z0-9._()\-]+$`)
	// azureOSTypePattern matches Azure VM OS type names. With OSTypeLinux and
	// OSTypeWindows as canonical values, only letter-only inputs are valid;
	// any future single-word OS name will fit.
	azureOSTypePattern = regexp.MustCompile(`^[A-Za-z]+$`)
)

// validateQueryVMsParams enforces the input contract shared by QueryVMs and the ARMResourceGraphMock:
// a non-empty subscription list with no empty or untrimmed entries, and per-field allowlist validation
// for the filter slices. Centralized so the mock cannot drift from production behavior.
func validateQueryVMsParams(params QueryVMsParams) error {
	if len(params.SubscriptionIDs) == 0 {
		return trace.BadParameter("at least one subscription ID is required")
	}

	for _, id := range params.SubscriptionIDs {
		if strings.TrimSpace(id) == "" {
			return trace.BadParameter("subscription ID must not be empty")
		}
		if strings.TrimSpace(id) != id {
			return trace.BadParameter("subscription ID %q must not have leading or trailing whitespace", id)
		}
	}

	if err := validateKQLValues(params.Regions, azureRegionPattern, "region"); err != nil {
		return err
	}
	if err := validateKQLValues(params.ResourceGroups, azureResourceGroupPattern, "resource group"); err != nil {
		return err
	}
	if err := validateKQLValues(params.OSTypes, azureOSTypePattern, "OS type"); err != nil {
		return err
	}

	return nil
}

// validateKQLValues rejects any non-wildcard entry that is empty, untrimmed, or doesn't
// match the supplied pattern. kind is a human-readable label used in error messages.
// types.Wildcard is whitelisted because predicate helpers absorb it before KQL is built.
func validateKQLValues(values []string, pattern *regexp.Regexp, kind string) error {
	for _, v := range values {
		if v == types.Wildcard {
			continue
		}
		if strings.TrimSpace(v) == "" {
			return trace.BadParameter("%s must not be empty", kind)
		}
		if strings.TrimSpace(v) != v {
			return trace.BadParameter("%s %q must not have leading or trailing whitespace", kind, v)
		}
		if !pattern.MatchString(v) {
			return trace.BadParameter("%s %q contains invalid characters; allowed pattern: %s", kind, v, pattern.String())
		}
	}

	return nil
}

// parseVMRows parses VM-query response data into DiscoveredVMs. Each VM carries an internal
// nicRefs field (when present in the projection) that QueryVMs uses to resolve PrimaryPrivateIP
// via the NIC query; nicRefs is not visible outside this package.
// Malformed rows are skipped; outer-shape drift returns an error.
func parseVMRows(ctx context.Context, data any) ([]DiscoveredVM, error) {
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
		vm, err := parseVMRow(ctx, m)
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

// parseVMRow extracts a DiscoveredVM from a single VM-query response row. Identity-field drift
// errors; tag drift and NIC-ref drift are best-effort: malformed shapes yield empty/nil collections.
// The unexported nicRefs field is populated when the row includes the column and left nil otherwise
// (IncludePrimaryPrivateIP=false). PrimaryPrivateIP is left empty for QueryVMs to populate after
// the NIC query resolves it.
func parseVMRow(ctx context.Context, m map[string]any) (DiscoveredVM, error) {
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
	// drift propagates to parseVMRows's per-row skip path.
	osType, err := getStringIfPresent(m, "osType")
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
		OSType:         osType,
		Tags:           getStringMap(ctx, m, "tags"),
		nicRefs:        parseNICRefs(ctx, m, "nicRefs"),
	}, nil
}

// parseNICRefs extracts the nicRefs array projected by buildVMDiscoveryKQL when
// IncludePrimaryPrivateIP is set. Each entry is the raw ARM reference object —
// {id, properties: {primary}} — from the VM's networkProfile.networkInterfaces[].
// Returns nil when the column is absent (IncludePrimaryPrivateIP=false) or the array is empty.
// Best-effort on malformed entries: log at debug and drop the entry; the ID is lowercased so
// downstream map lookups against the NIC query's tolower projection are case-insensitive.
func parseNICRefs(ctx context.Context, m map[string]any, key string) []nicRef {
	raw, ok := m[key]
	if !ok || raw == nil {
		return nil
	}
	asList, ok := raw.([]any)
	if !ok {
		slog.DebugContext(ctx, "Resource Graph row has malformed nicRefs; treating as empty",
			"key", key, "got_type", fmt.Sprintf("%T", raw))
		return nil
	}
	out := make([]nicRef, 0, len(asList))
	for i, entry := range asList {
		em, ok := entry.(map[string]any)
		if !ok {
			slog.DebugContext(ctx, "Dropping non-object nicRefs entry",
				"key", key, "index", i, "got_type", fmt.Sprintf("%T", entry))
			continue
		}
		id, err := getStringIfPresent(em, "id")
		if err != nil || id == "" {
			slog.DebugContext(ctx, "Dropping nicRefs entry with missing or malformed id",
				"index", i, "error", err)
			continue
		}
		out = append(out, nicRef{
			id:      strings.ToLower(id),
			primary: getNestedBool(em, "properties", "primary"),
		})
	}
	return out
}

// parseNICs parses NIC-query response data, populating the supplied map keyed by NIC ARM ID.
// Malformed rows are skipped at debug; outer-shape drift returns an error.
func parseNICs(ctx context.Context, data any, out map[string][]ipConfigRow) error {
	if data == nil {
		return nil
	}
	rows, ok := data.([]any)
	if !ok {
		return trace.BadParameter("resource graph NIC response Data has unexpected type %T (expected []any)", data)
	}
	skipped := 0
	for i, row := range rows {
		m, ok := row.(map[string]any)
		if !ok {
			slog.DebugContext(ctx, "Skipping Resource Graph NIC row with unexpected type",
				"row", i, "got_type", fmt.Sprintf("%T", row))
			skipped++
			continue
		}
		id, configs, err := parseNICRow(ctx, m)
		if err != nil {
			slog.DebugContext(ctx, "Skipping malformed Resource Graph NIC row",
				"row", i, "error", err)
			skipped++
			continue
		}
		out[id] = configs
	}
	if skipped > 0 {
		slog.WarnContext(ctx, "Resource Graph returned malformed NIC rows",
			"skipped", skipped)
	}
	return nil
}

// parseNICRow extracts a NIC ID and its IP configurations from a single NIC-query response row.
// The id field is required (it's the map key). ipConfigs may be empty for NICs in transient states.
func parseNICRow(ctx context.Context, m map[string]any) (string, []ipConfigRow, error) {
	id, err := getRequiredARGString(m, "id")
	if err != nil {
		return "", nil, err
	}
	return id, parseIPConfigList(ctx, m, "ipConfigs"), nil
}

// parseIPConfigList extracts a NIC row's ipConfigs array (NIC-query projection) into ipConfigRow
// values. Each entry is a raw ipConfiguration object whose primary flag and privateIPAddress live
// under a "properties" sub-object. Best-effort on malformed entries: log at debug and drop.
func parseIPConfigList(ctx context.Context, m map[string]any, key string) []ipConfigRow {
	raw, ok := m[key]
	if !ok || raw == nil {
		return nil
	}
	asList, ok := raw.([]any)
	if !ok {
		slog.DebugContext(ctx, "Resource Graph NIC row has malformed ipConfigs; treating as empty",
			"key", key, "got_type", fmt.Sprintf("%T", raw))
		return nil
	}
	out := make([]ipConfigRow, 0, len(asList))
	for i, entry := range asList {
		em, ok := entry.(map[string]any)
		if !ok {
			slog.DebugContext(ctx, "Dropping non-object ipConfigs entry",
				"key", key, "index", i, "got_type", fmt.Sprintf("%T", entry))
			continue
		}
		props, ok := em["properties"].(map[string]any)
		if !ok {
			slog.DebugContext(ctx, "Dropping ipConfigs entry with missing or malformed properties",
				"index", i)
			continue
		}
		privateIP, err := getStringIfPresent(props, "privateIPAddress")
		if err != nil {
			slog.DebugContext(ctx, "Dropping ipConfigs entry with malformed privateIPAddress",
				"index", i, "error", err)
			continue
		}
		out = append(out, ipConfigRow{
			primary:   getBoolIfPresent(props, "primary"),
			privateIP: privateIP,
		})
	}
	return out
}

// selectPrimaryPrivateIP applies the primary-of-primary rule: the privateIPAddress of the primary
// IP configuration on the primary NIC. Inputs are nicRefs from the VM query and a NIC-ID →
// IP-configs map from the NIC query. Returns "" when no unambiguous primary can be chosen.
//
// Resolution:
//
//  1. Pick the primary NIC from nicRefs.
//     - Exactly one ref flagged primary → that NIC.
//     - No ref flagged primary AND only one ref → implicit primary on a single-NIC VM.
//     - Otherwise → "" (no NICs, or ambiguous flagging).
//
//  2. Look up that NIC's IP configs in nicMap.
//     - Missing entry (unindexed by ARG or not in the fetched batch) → "".
//     - Keep only configs whose privateIPAddress is non-empty.
//
//  3. Pick the primary IP config among the survivors.
//     - Exactly one flagged primary → its privateIP.
//     - No flag set AND only one survivor → implicit primary on a single-config NIC.
//     - Otherwise → "".
//
// Ambiguity is not treated as an error: the caller (e.g. Windows desktop registration) is expected
// to check for "" and decide whether to skip the VM or surface a user task.
func selectPrimaryPrivateIP(nicRefs []nicRef, nicMap map[string][]ipConfigRow) string {
	if len(nicRefs) == 0 {
		return ""
	}

	// Step 1: identify the primary NIC.
	var primaryNICID string
	var flagged []nicRef
	for _, r := range nicRefs {
		if r.primary {
			flagged = append(flagged, r)
		}
	}
	switch {
	case len(flagged) == 1:
		primaryNICID = flagged[0].id
	case len(flagged) == 0 && len(nicRefs) == 1:
		primaryNICID = nicRefs[0].id
	default:
		return ""
	}

	// Step 2: resolve the primary IP config on that NIC.
	configs, ok := nicMap[primaryNICID]
	if !ok {
		return ""
	}
	withIP := make([]ipConfigRow, 0, len(configs))
	for _, c := range configs {
		if c.privateIP != "" {
			withIP = append(withIP, c)
		}
	}
	if len(withIP) == 0 {
		return ""
	}
	var primaryConfigs []ipConfigRow
	for _, c := range withIP {
		if c.primary {
			primaryConfigs = append(primaryConfigs, c)
		}
	}
	switch {
	case len(primaryConfigs) == 1:
		return primaryConfigs[0].privateIP
	case len(primaryConfigs) == 0 && len(withIP) == 1:
		return withIP[0].privateIP
	default:
		return ""
	}
}

// collectNICIDs returns the lowercased, deduplicated NIC ARM IDs referenced by all VMs. The result
// feeds chunkNICIDsForQuery; the lowercase form matches the NIC query's tolower(tostring(id))
// projection.
func collectNICIDs(vms []DiscoveredVM) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, vm := range vms {
		for _, ref := range vm.nicRefs {
			if _, ok := seen[ref.id]; ok {
				continue
			}
			seen[ref.id] = struct{}{}
			out = append(out, ref.id)
		}
	}
	return out
}

// resolvePrimaryPrivateIPs walks each VM and sets PrimaryPrivateIP using selectPrimaryPrivateIP
// against the supplied NIC map. Mutates vms in place — the slice is already the caller's working
// copy, and resolution is the final transform before QueryVMs returns it.
func resolvePrimaryPrivateIPs(vms []DiscoveredVM, nicMap map[string][]ipConfigRow) {
	for i := range vms {
		vms[i].PrimaryPrivateIP = selectPrimaryPrivateIP(vms[i].nicRefs, nicMap)
	}
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
// This policy is shaped by how Azure VM tags flow into Teleport's discovery
// label matchers (lib/srv/server/azure_watcher.go calls services.MatchLabels
// on vm.Tags): fabricating selector values via fmt.Sprint of arbitrary
// Go-typed data could let drifted ARG output present unintended selector
// matches. Dropping enforces the rule "selectors only see literal string
// values that Azure actually returned."
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

// getBoolIfPresent returns the bool at key. Missing, nil, or non-bool values return false —
// appropriate for Azure "primary" flags, which are absent rather than explicitly false when
// the resource has no sibling to disambiguate against (single-NIC VM, single-config NIC).
func getBoolIfPresent(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	b, _ := v.(bool)
	return b
}

// getNestedBool returns the bool at m[outer][inner]. Missing outer, wrong-type outer, missing
// inner, or non-bool inner all return false — matching getBoolIfPresent's semantics for the
// implicit-primary case.
func getNestedBool(m map[string]any, outer, inner string) bool {
	sub, ok := m[outer].(map[string]any)
	if !ok {
		return false
	}
	return getBoolIfPresent(sub, inner)
}
