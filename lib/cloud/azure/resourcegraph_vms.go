/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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
	"encoding/base64"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	libslices "github.com/gravitational/teleport/lib/utils/slices"
)

// QueryLinuxVMsParams defines parameters when listing all linux virtual machines.
type QueryLinuxVMsParams struct {
	// SubscriptionID scopes the query to a single subscription.
	// Required.
	SubscriptionID string
	// Locations filters VMs by location. An empty slice or a single types.Wildcard item matches every location.
	// Optional.
	Locations []string
	// ResourceGroup filters VMs by resource group. An empty string matches every resource group.
	// Optional.
	ResourceGroup string
}

// sanitize returns a validated and sanitized version of QueryLinuxVMsParams
// - subscription id must be a valid UUID
// - locations are validated against a weak allowlist
// - resource groups are validated against what's documented in the Azure Docs and what is enforced in the Azure Portal UI
func (q *QueryLinuxVMsParams) sanitize() (*validatedQueryLinuxVMsParams, error) {
	if _, err := uuid.Parse(q.SubscriptionID); err != nil {
		return nil, trace.BadParameter("invalid subscription ID: %q", q.SubscriptionID)
	}

	locations, err := sanitizeLocationsFilter(q.Locations)
	if err != nil {
		return nil, trace.Wrap(err, "invalid locations")
	}

	resourceGroup, err := sanitizeResourceGroupFilter(q.ResourceGroup)
	if err != nil {
		return nil, trace.Wrap(err, "invalid resource group")
	}

	return &validatedQueryLinuxVMsParams{
		SubscriptionID: q.SubscriptionID,
		ResourceGroup:  resourceGroup,
		Locations:      locations,
	}, nil
}

func sanitizeLocationsFilter(locations []string) ([]string, error) {
	if len(locations) == 0 || slices.Contains(locations, types.Wildcard) {
		return nil, nil
	}

	sanitized := make([]string, 0, len(locations))
	for _, location := range locations {
		if err := IsValidLocationNameWeak(location); err != nil {
			return nil, trace.BadParameter("invalid location %q: %v", location, err)
		}
		sanitized = append(sanitized, strings.ToLower(location))
	}

	slices.Sort(sanitized)
	uniqLocations := slices.Compact(sanitized)
	return uniqLocations, nil
}

func sanitizeResourceGroupFilter(resourceGroup string) (string, error) {
	if resourceGroup == "" || resourceGroup == types.Wildcard {
		return "", nil
	}

	if err := IsValidResourceGroupName(resourceGroup); err != nil {
		return "", trace.BadParameter("%q: %v", resourceGroup, err)
	}

	return strings.ToLower(resourceGroup), nil
}

// validatedQueryLinuxVMsParams is QueryLinuxVMsParams after being validated.
// Must never be constructed directly. Use QueryLinuxVMsParams.sanitize() to get an instance.
type validatedQueryLinuxVMsParams struct {
	// SubscriptionID is the Azure subscription ID to query.
	SubscriptionID string

	// ResourceGroup is the Azure resource group name to filter the query by.
	// Must be lowercase. If empty, no filter is applied.
	ResourceGroup string

	// Locations is a list of Azure location names to filter the query by.
	// Must be lowercase. If empty, no filter is applied.
	Locations []string
}

// kqlQueryLinuxVMs creates a KQL query for discovering running Linux VMs.
// Optional filters for location and resource group are included based on the provided parameters.
func kqlQueryLinuxVMs(params *validatedQueryLinuxVMsParams) string {
	filters := strings.Join([]string{
		insecureKQLFieldInValuesCondition("location", params.Locations),
		insecureKQLFieldEqualsValueCondition("resourceGroup", params.ResourceGroup),
	}, "\n    ")

	// Ordering is required because:
	// https://learn.microsoft.com/en-us/azure/governance/resource-graph/concepts/paging-results#pagination-limitations-scenario-sorting-by-non-unique-columns
	// > When using skip or first, it's recommended to order results by at least one column with asc or desc. Without sorting, results are random and not repeatable.

	return `
(
  resources |
  where type == 'microsoft.compute/virtualmachines'
    and properties.extended.instanceView.powerState.code == 'PowerState/running'
    and properties.storageProfile.osDisk.osType == 'Linux'
    ` + filters + `
  | project id, name, location, tags, vmId = properties.vmId
) | union (
  computeresources |
  where type == "microsoft.compute/virtualmachinescalesets/virtualmachines"
    and properties.extended.instanceView.powerState.code == 'PowerState/running'
    and properties.storageProfile.osDisk.osType == 'Linux'
    ` + filters + `
  | project id, name, location, tags, vmId = properties.vmId
)
| order by id desc`
}

// insecureKQLFieldInValuesCondition builds a KQL where sub-clause of the form "and field in ('value1', 'value2', ...)" for the given field and values.
// The values are single-quoted without any escaping. Callers of this function must ensure that values are safe to be included in KQL queries without escaping.
// Empty list of values results in a comment indicating that no filter is applied for the field.
func insecureKQLFieldInValuesCondition(field string, values []string) string {
	if len(values) == 0 {
		return "// no filter by " + field
	}
	quotedValues := libslices.Map(values, func(value string) string {
		return "'" + value + "'"
	})
	valueList := strings.Join(quotedValues, ", ")
	return "and " + field + " in (" + valueList + ")"
}

func insecureKQLFieldEqualsValueCondition(field string, value string) string {
	if value == "" {
		return "// no filter by " + field
	}
	return fmt.Sprintf("and %s == '%s'", field, value)
}

func queryRequestWithSkipToken(query string, subscriptionID string, skipToken *string) armresourcegraph.QueryRequest {
	return armresourcegraph.QueryRequest{
		Query:         to.Ptr(query),
		Subscriptions: []*string{to.Ptr(subscriptionID)},
		Options: &armresourcegraph.QueryRequestOptions{
			ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
			Top:          to.Ptr(int32(resourceGraphPageSize)),
			SkipToken:    skipToken,
		},
	}
}

// QueryLinuxVMs runs the discovery query against Resource Graph and translates the rows to []VirtualMachine.
// Only running and Linux VMs are returned, up to a maximum of 10 million resources.
//
// Callers can use params to scope down the query. Empty or wildcard parameters are treated as "match all".
// Required Azure permissions:
// - Microsoft.Compute/virtualMachineScaleSets/read
// - Microsoft.Compute/virtualMachineScaleSets/virtualMachines/read
// - Microsoft.Compute/virtualMachineScaleSets/virtualMachines/runCommand/action
// - Microsoft.Compute/virtualMachineScaleSets/virtualMachines/runCommands/write
// - Microsoft.Compute/virtualMachineScaleSets/virtualMachines/runCommands/read
// - Microsoft.Compute/virtualMachineScaleSets/virtualMachines/runCommands/delete
//
// Some azure docs mention that `Microsoft.ResourceGraph/resources/read` is also required, but we have observed that it's not actually required.
func (c *resourceGraphClient) QueryLinuxVMs(ctx context.Context, params QueryLinuxVMsParams) ([]*VirtualMachine, error) {
	sanitized, err := params.sanitize()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log := c.logger.With("method", "QueryLinuxVMs", "subscription_id", sanitized.SubscriptionID)

	query := kqlQueryLinuxVMs(sanitized)
	log.DebugContext(ctx, "Resource Graph query starting", "query_b64", base64.StdEncoding.EncodeToString([]byte(query)))

	var all []*VirtualMachine
	var skipToken *string
	for page := range resourceGraphMaxPages {
		log := log.With("page", page)

		resp, err := c.resourcesAPI.Resources(ctx, queryRequestWithSkipToken(query, sanitized.SubscriptionID, skipToken), nil)
		if err != nil {
			// We can't detect missing permissions because Azure API returns 0 records instead of an error when permissions are missing.
			return nil, trace.Wrap(ConvertResponseError(err))
		}

		log.DebugContext(ctx, "Resource Graph page returned", "summary", resourceGraphResponseSummary(resp))
		rows, err := collectRows(resp.Data)
		if err != nil {
			if len(all) > 0 {
				log.WarnContext(ctx, "Resource Graph returned invalid Data field, skipping remaining rows", "error", err)
				break
			}

			return nil, trace.Wrap(err)
		}

		parsedVMs := parseVirtualMachines(ctx, log, rows)
		// The entire page is malformed and there's no valid data from previous pages: an error is returned.
		// If there are valid rows from previous pages, continue processing and return the valid data.
		if len(parsedVMs.vms) == 0 && len(rows) > 0 && len(all) == 0 {
			return nil, trace.BadParameter("all rows are malformed, last error: %v", parsedVMs.lastParseError)
		}

		all = append(all, parsedVMs.vms...)
		log.DebugContext(ctx, "Resource Graph page processed", "returned_rows", len(rows), "valid_rows", len(parsedVMs.vms), "current_total", len(all))

		skipToken = resp.SkipToken
		if StringVal(skipToken) != "" {
			if page == resourceGraphMaxPages-1 {
				log.WarnContext(ctx,
					"Reached max allowed page when fetching Resource Graph results, skipping remaining pages to avoid runaway pagination. "+
						"Consider creating multiple matchers with smaller scopes (eg. by resource group). ",
					"page_limit", resourceGraphMaxPages,
					"current_total", len(all))
				break
			}

			continue
		}

		// When the SkipToken is empty, it indicates one of the following: either no more results, or a truncated result
		// Docs: https://learn.microsoft.com/en-us/azure/governance/resource-graph/concepts/work-with-data#paging-results
		if isTruncatedResult(resp) {
			log.WarnContext(ctx, "Azure Resource Graph Resource API returned a truncated result. "+
				"Consider creating multiple matchers with smaller scopes (eg. by resource group). "+
				"Continuing with current parsed VMs", "current_total", len(all), "summary", resourceGraphResponseSummary(resp))
		}

		// No skip token for the next page and the result is not truncated, so we have fetched all results.
		break
	}

	return all, nil
}

func collectRows(data any) ([]any, error) {
	if data == nil {
		return nil, nil
	}

	rows, ok := data.([]any)
	if !ok {
		return nil, trace.BadParameter("Data field has unexpected type %T (expected []any)", data)
	}

	return rows, nil
}

func isTruncatedResult(resp armresourcegraph.ClientResourcesResponse) bool {
	return resp.ResultTruncated != nil && *resp.ResultTruncated == armresourcegraph.ResultTruncatedTrue
}

func resourceGraphResponseSummary(resp armresourcegraph.ClientResourcesResponse) string {
	var parts []string
	if StringVal(resp.SkipToken) != "" {
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

type parsedVirtualMachines struct {
	vms            []*VirtualMachine
	lastParseError error
}

// parseVirtualMachines parses Resource Graph QueryResponse.Data into the VirtualMachine shape.
// It's a best effort parser where malformed rows are skipped with a debug log.
// If all rows are malformed, an error is returned.
func parseVirtualMachines(ctx context.Context, log *slog.Logger, data []any) parsedVirtualMachines {
	if data == nil {
		return parsedVirtualMachines{}
	}
	var lastParseError error

	out := make([]*VirtualMachine, 0, len(data))
	malformedRows := 0
	for i, row := range data {
		log := log.With("row_index", i)
		m, ok := row.(map[string]any)
		if !ok {
			// Per-row at debug so a persistently malformed VM doesn't flood logs
			// every poll cycle. The summary warn below fires once per call.
			log.DebugContext(ctx, "Skipping Resource Graph row with unexpected type", "got_type", fmt.Sprintf("%T", row))
			malformedRows++
			lastParseError = trace.BadParameter("row has unexpected type %T (expected map[string]any)", row)
			continue
		}

		vm, err := parseVirtualMachineRow(ctx, log, m)
		if err != nil {
			log.DebugContext(ctx, "Skipping malformed Resource Graph row", "error", err)
			lastParseError = err
			malformedRows++
			continue
		}

		out = append(out, vm)
	}

	if malformedRows > 0 {
		// One warn per affected response, regardless of how many rows were bad.
		// Row-level detail is at debug above for operators investigating.
		log.WarnContext(ctx, "Skipped some malformed rows", "malformed", malformedRows, "kept", len(out), "last_error", lastParseError)
	}

	return parsedVirtualMachines{
		vms:            out,
		lastParseError: lastParseError,
	}
}

// parseVirtualMachineRow extracts a VirtualMachine from a single ARG response row.
// Identity-field drift errors; tag drift is best-effort: malformed outer tags
// yield an empty map, and malformed inner entries are dropped.
func parseVirtualMachineRow(ctx context.Context, log *slog.Logger, m map[string]any) (*VirtualMachine, error) {
	id, err := queryResultGetString(m, "id")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log = log.With("resource_id", id)

	name, err := queryResultGetString(m, "name")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	vmID, err := queryResultGetString(m, "vmId")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	location, err := queryResultGetString(m, "location")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tags := queryResultGetKeyValueString(ctx, log, m["tags"])

	// Extract meta information from the resource ID.
	// We could get Subscription and Resource Group from dedicated fields, but there's no dedicated field for Uniform Scale Set name or index.
	parsedResourceID, err := arm.ParseResourceID(id)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	subscriptionID := parsedResourceID.SubscriptionID
	resourceGroup := parsedResourceID.ResourceGroupName

	var uniformScaleSetName string
	var uniformScaleSetIndex string
	if parsedResourceID.ResourceType.Type == virtualScaleSetUniformVMResourceType {
		uniformScaleSetIndex = parsedResourceID.Name

		if parsedResourceID.Parent == nil {
			return nil, trace.BadParameter("missing parent resource from uniform scale set VM ID: %s", id)
		}

		uniformScaleSetName = parsedResourceID.Parent.Name
	}

	return &VirtualMachine{
		ID:                          id,
		Subscription:                subscriptionID,
		Name:                        name,
		VMID:                        vmID,
		Location:                    location,
		ResourceGroup:               resourceGroup,
		Tags:                        tags,
		UniformScaleSetName:         uniformScaleSetName,
		UniformScaleSetVMInstanceID: uniformScaleSetIndex,
	}, nil
}
