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

package server

import (
	"cmp"
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// TestAzureFetcherGetInstancesWindowsLive exercises the full Windows VM discovery
// path — listing VMs and resolving each VM's private IP from its NIC — against a
// real Azure subscription. It is skipped unless TELEPORT_TEST_AZURE is set. No
// fakes are injected, so it builds and calls the real virtual machines and
// network interfaces clients.
//
// Prerequisites:
//   - Authenticate so DefaultAzureCredential can find a credential, e.g. run
//     `az login`, or set AZURE_TENANT_ID / AZURE_CLIENT_ID / AZURE_CLIENT_SECRET.
//     The identity needs Microsoft.Compute/virtualMachines/read and
//     Microsoft.Network/networkInterfaces/read on the target resources.
//   - Set AZURE_SUBSCRIPTION_ID to a subscription that has at least one Windows VM.
//   - Optionally set AZURE_RESOURCE_GROUP and AZURE_REGION to narrow the search;
//     both default to the wildcard (whole subscription / all regions).
//
// Run with:
//
//	TELEPORT_TEST_AZURE=1 \
//	AZURE_SUBSCRIPTION_ID=<sub> \
//	AZURE_RESOURCE_GROUP=<rg> \
//	AZURE_REGION=<region> \
//	go test ./lib/srv/server/ -run TestAzureFetcherGetInstancesWindowsLive -v
func TestAzureFetcherGetInstancesWindowsLive(t *testing.T) {
	if os.Getenv("TELEPORT_TEST_AZURE") == "" {
		t.Skip("Set TELEPORT_TEST_AZURE to run this test against a real Azure subscription.")
	}
	subscription := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if subscription == "" {
		t.Skip("Set AZURE_SUBSCRIPTION_ID to the subscription to search for Windows VMs.")
	}
	resourceGroup := cmp.Or(os.Getenv("AZURE_RESOURCE_GROUP"), types.Wildcard)
	region := cmp.Or(os.Getenv("AZURE_REGION"), types.Wildcard)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()

	clients, err := azure.NewClients()
	require.NoError(t, err, "failed to build Azure clients, have you run `az login`?")

	fetcher := newAzureInstanceFetcher(azureFetcherConfig{
		Matcher: types.AzureMatcher{
			Types:        []string{types.AzureMatcherWindowsVM},
			Regions:      []string{region},
			ResourceTags: types.Labels{"*": []string{"*"}},
		},
		MatcherType:   types.AzureMatcherWindowsVM,
		Subscription:  subscription,
		ResourceGroup: resourceGroup,
		AzureClientGetter: func(context.Context, string) (azure.Clients, error) {
			return clients, nil
		},
		// InterfacesClientGetter is intentionally left unset so the fetcher builds
		// and uses the real Azure network interfaces client.
		Logger: logtest.NewLogger(),
	})

	instances, err := fetcher.GetInstances(ctx, false)
	require.NoError(t, err)

	var total int
	for _, group := range instances {
		for _, vm := range group.Instances {
			total++
			t.Logf("Windows VM %q (region=%q, resourceGroup=%q, vmID=%q): private IP=%q",
				vm.Name, vm.Location, vm.ResourceGroup, vm.VMID, vm.PrimaryPrivateIP)
		}
	}
	t.Logf("Discovered %d Windows VM(s) in subscription %q (resourceGroup=%q, region=%q)",
		total, subscription, resourceGroup, region)
}
