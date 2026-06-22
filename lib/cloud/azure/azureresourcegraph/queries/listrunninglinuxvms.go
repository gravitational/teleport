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

package queries

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/cloud/azure"
)

// NewListRunningLinuxVMsQuery creates a TypedQuery that lists running Linux VMs
// and VM scale set instances, optionally filtered by resource group and Azure
// locations.
//
// Filters are validated before the query is created so values can be safely
// embedded in the generated KQL query.
func NewListRunningLinuxVMsQuery(filters ListRunningLinuxVMsQueryFilters) (*ListRunningLinuxVMsQuery, error) {
	if filters.ResourceGroupFilter != "" {
		if err := azure.IsValidResourceGroupName(filters.ResourceGroupFilter); err != nil {
			return nil, trace.Wrap(err, "invalid resource group filter")
		}
	}

	if len(filters.LocationsFilter) > 0 {
		for _, loc := range filters.LocationsFilter {
			if err := azure.IsValidLocationNameWeak(loc); err != nil {
				return nil, trace.Wrap(err, "invalid location filter")
			}
		}
	}

	return &ListRunningLinuxVMsQuery{
		resourceGroupFilter: filters.ResourceGroupFilter,
		locationsFilter:     slices.Clone(filters.LocationsFilter),
	}, nil
}

// ListRunningLinuxVMsQuery is a TypedQuery that returns metadata for running
// Linux virtual machines from Azure Resource Graph.
type ListRunningLinuxVMsQuery struct {
	resourceGroupFilter string
	locationsFilter     []string
}

// ListRunningLinuxVMsQueryFilters defines the optional filters that can be applied to the ListRunningLinuxVMsQuery.
// Empty values mean that the field is not used as a filter in the query.
type ListRunningLinuxVMsQueryFilters struct {
	// ResourceGroupFilter is an optional filter to limit the query to a specific Azure resource group.
	ResourceGroupFilter string
	// LocationsFilter is an optional filter to limit the query to specific Azure locations (regions).
	LocationsFilter []string
}

// Query constructs the KQL query string for listing running Linux VMs.
//
// The generated query includes both standalone VMs and VM scale set instances,
// applying any configured resource group and location filters to both branches.
func (q *ListRunningLinuxVMsQuery) Query() (string, error) {
	var extraFilters string
	if q.resourceGroupFilter != "" {
		extraFilters += fmt.Sprintf(" and resourceGroup =~ '%s'\n", q.resourceGroupFilter)
	} else {
		extraFilters += "    // all resource groups\n"
	}

	if len(q.locationsFilter) > 0 {
		quotedLocations := make([]string, len(q.locationsFilter))
		for i, loc := range q.locationsFilter {
			quotedLocations[i] = fmt.Sprintf("'%s'", loc)
		}
		extraFilters += fmt.Sprintf(" and location in~ (%s)\n", strings.Join(quotedLocations, ", "))
	} else {
		extraFilters += "    // all locations\n"
	}

	// Ensure the query returns the expected columns so that rows can be unmarshaled into VMMetadata.
	return `
(
  resources |
  where type == 'microsoft.compute/virtualmachines'
    and properties.extended.instanceView.powerState.code == 'PowerState/running'
    and properties.storageProfile.osDisk.osType == 'Linux'
    ` + extraFilters + `
  | project id, name, location, tags, vmId = properties.vmId
) | union (
  computeresources |
  where type == "microsoft.compute/virtualmachinescalesets/virtualmachines"
    and properties.extended.instanceView.powerState.code == 'PowerState/running'
    and properties.storageProfile.osDisk.osType == 'Linux'
    ` + extraFilters + `
  | project id, name, location, tags, vmId = properties.vmId
)
| order by id desc`, nil
}

// Item returns the zero VMMetadata value used for type inference.
func (q *ListRunningLinuxVMsQuery) Item() VMMetadata {
	return VMMetadata{}
}

// VMMetadata represents Linux VM metadata retrieved by ListRunningLinuxVMsQuery.
type VMMetadata struct {
	// ID is the unique identifier of the resource in Azure.
	ID string `json:"id"`
	// Name is the name of the VM.
	Name string `json:"name"`
	// Location is the Azure region where the VM is deployed.
	Location string `json:"location"`
	// Tags are the key-value pairs associated with the VM.
	Tags map[string]string `json:"tags"`
	// VMID is the Azure VM identifier from properties.vmId.
	VMID string `json:"vmId"`
}
