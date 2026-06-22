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
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func mustQuery(t *testing.T) *ListRunningLinuxVMsQuery {
	t.Helper()
	return mustQueryWith(t, ListRunningLinuxVMsQueryFilters{})
}

func mustQueryWith(t *testing.T, filters ListRunningLinuxVMsQueryFilters) *ListRunningLinuxVMsQuery {
	t.Helper()
	q, err := NewListRunningLinuxVMsQuery(filters)
	require.NoError(t, err)
	return q
}

// vmRow returns one JSON row that unmarshals cleanly into VMMetadata.
func vmRow(name string) string {
	return fmt.Sprintf(
		`{"id":"/subscriptions/sub/%s","name":%q,"location":"eastus","tags":{"env":"prod"},"vmId":"vmid-%s"}`,
		name, name, name,
	)
}

func TestListRunningLinuxVMsQuery(t *testing.T) {
	t.Run("no filters", func(t *testing.T) {
		got, err := mustQuery(t).Query()
		require.NoError(t, err)
		for _, want := range []string{
			"resources |",
			"computeresources |",
			"microsoft.compute/virtualmachines",
			"virtualmachinescalesets/virtualmachines",
			"properties.extended.instanceView.powerState.code == 'PowerState/running'",
			"properties.storageProfile.osDisk.osType == 'Linux'",
			"union",
			"| order by id desc",
			"project id, name, location, tags, vmId = properties.vmId",
			"// all resource groups",
			"// all locations",
		} {
			require.Contains(t, got, want)
		}
	})

	t.Run("resource group filter applied to both union branches", func(t *testing.T) {
		got, err := mustQueryWith(t, ListRunningLinuxVMsQueryFilters{ResourceGroupFilter: "rg_prod-01"}).Query()
		require.NoError(t, err)
		require.Equal(t, 2, strings.Count(got, "resourceGroup =~ 'rg_prod-01'"), got)
	})

	t.Run("locations applied verbatim to both branches", func(t *testing.T) {
		// KQL's in~ operator is case-insensitive, so locations are embedded with
		// their original casing rather than being lowercased.
		got, err := mustQueryWith(t, ListRunningLinuxVMsQueryFilters{LocationsFilter: []string{"EastUS", "WESTEUROPE"}}).Query()
		require.NoError(t, err)
		want := "location in~ ('EastUS', 'WESTEUROPE')"
		require.Equal(t, 2, strings.Count(got, want), got)
	})

	t.Run("filters slice is cloned", func(t *testing.T) {
		locations := []string{"eastus"}
		q, err := NewListRunningLinuxVMsQuery(ListRunningLinuxVMsQueryFilters{LocationsFilter: locations})
		require.NoError(t, err)
		locations[0] = "centralus"
		got, err := q.Query()
		require.NoError(t, err)
		require.NotContains(t, got, "centralus")
	})

	t.Run("Item returns the zero VMMetadata", func(t *testing.T) {
		require.Equal(t, VMMetadata{}, mustQuery(t).Item())
	})

	t.Run("rejects invalid resource group", func(t *testing.T) {
		q, err := NewListRunningLinuxVMsQuery(ListRunningLinuxVMsQueryFilters{ResourceGroupFilter: "bad rg"})
		require.True(t, trace.IsBadParameter(err), "want BadParameter, got %v", err)
		require.Nil(t, q)
		require.Contains(t, err.Error(), "invalid resource group filter")
	})

	t.Run("rejects invalid location", func(t *testing.T) {
		_, err := NewListRunningLinuxVMsQuery(ListRunningLinuxVMsQueryFilters{LocationsFilter: []string{"eastus", "bad-loc"}})
		require.True(t, trace.IsBadParameter(err), "want BadParameter, got %v", err)
		require.Contains(t, err.Error(), "invalid location filter")
	})

	// Security: malicious filter values must be rejected before they can reach
	// the KQL string builder.
	t.Run("rejects KQL injection via resource group", func(t *testing.T) {
		_, err := NewListRunningLinuxVMsQuery(ListRunningLinuxVMsQueryFilters{ResourceGroupFilter: "x' | union (resources) | where '1'=='1"})
		require.True(t, trace.IsBadParameter(err), "injection attempt should be rejected, got %v", err)
	})

	t.Run("rejects KQL injection via location", func(t *testing.T) {
		_, err := NewListRunningLinuxVMsQuery(ListRunningLinuxVMsQueryFilters{LocationsFilter: []string{"eastus') | union (resources"}})
		require.True(t, trace.IsBadParameter(err), "injection attempt should be rejected, got %v", err)
	})
}

func TestVMMetadata_Unmarshal(t *testing.T) {
	var vm VMMetadata
	require.NoError(t, json.Unmarshal([]byte(vmRow("web-1")), &vm))
	require.Equal(t, "/subscriptions/sub/web-1", vm.ID)
	require.Equal(t, "web-1", vm.Name)
	require.Equal(t, "eastus", vm.Location)
	require.Equal(t, "vmid-web-1", vm.VMID)
	require.Equal(t, map[string]string{"env": "prod"}, vm.Tags)
}
