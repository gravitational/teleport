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

package azure

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/stretchr/testify/require"
)

// TestNetworkInterfacesClientLiveList exercises the network interfaces client
// against a real Azure subscription, listing the standalone NICs (regular VMs
// and flexible scale set VMs) in a resource group. It is skipped unless
// TELEPORT_TEST_AZURE is set.
//
// Prerequisites:
//   - Authenticate so DefaultAzureCredential can find a credential, e.g. run
//     `az login`, or set AZURE_TENANT_ID / AZURE_CLIENT_ID / AZURE_CLIENT_SECRET.
//     The identity needs Microsoft.Network/networkInterfaces/read in the
//     resource group.
//   - Set AZURE_SUBSCRIPTION_ID to the subscription to list from.
//   - Set AZURE_RESOURCE_GROUP to a resource group with at least one NIC.
//
// Run with:
//
//	TELEPORT_TEST_AZURE=1 \
//	AZURE_SUBSCRIPTION_ID=<sub> \
//	AZURE_RESOURCE_GROUP=<rg> \
//	go test ./lib/cloud/azure/ -run TestNetworkInterfacesClientLiveList -v
func TestNetworkInterfacesClientLiveList(t *testing.T) {
	nicClient, resourceGroup := newLiveNetworkInterfacesClient(t)

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	nics, err := nicClient.ListNetworkInterfaces(ctx, resourceGroup)
	require.NoError(t, err)

	logNICs(t, nics)
}

// TestNetworkInterfacesClientLiveListUniformVMSS exercises the network
// interfaces client against a real Azure subscription, listing the NICs of
// uniform scale set VMs alongside the standalone NICs in the resource group.
// It is skipped unless TELEPORT_TEST_AZURE, TELEPORT_TEST_AZURE_SUBSCRIPTION_ID
// and TELEPORT_TEST_AZURE_SCALE_SET_IDS are set.
// Prerequisites are the same as TestNetworkInterfacesClientLiveList, plus:
//   - Set TELEPORT_TEST_AZURE_SCALE_SET_IDS to a comma-separated list of
//     uniform VM scale set IDs in the resource group, e.g. "/subscriptions/<sub>/resourceGroups/<rg>/providers/Microsoft.Compute/virtualMachineScaleSets/vmss1,/subscriptions/<sub>/resourceGroups/<rg>/providers/Microsoft.Compute/virtualMachineScaleSets/vmss2".
//
// Run with:
//
//	TELEPORT_TEST_AZURE=1 \
//	AZURE_SUBSCRIPTION_ID=<sub> \
//	AZURE_RESOURCE_GROUP=<rg> \
//	TELEPORT_TEST_AZURE_SCALE_SET_IDS=<vmss1_id,vmss2_id> \
//	go test ./lib/cloud/azure/ -run TestNetworkInterfacesClientLiveListUniformVMSS -v
func TestNetworkInterfacesClientLiveListUniformVMSS(t *testing.T) {
	nicClient, resourceGroup := newLiveNetworkInterfacesClient(t)

	var scaleSetIDs []string
	for id := range strings.SplitSeq(os.Getenv("TELEPORT_TEST_AZURE_SCALE_SET_IDS"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			scaleSetIDs = append(scaleSetIDs, id)
		}
	}
	if len(scaleSetIDs) == 0 {
		t.Skip("Set TELEPORT_TEST_AZURE_SCALE_SET_IDS to a comma-separated list of uniform VM scale set IDs.")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	nics, err := nicClient.ListNetworkInterfaces(ctx, resourceGroup, scaleSetIDs...)
	require.NoError(t, err)

	// Uniform VMSS NICs are attached to a scale set VM instance, so they can
	// be identified by their attached VM's resource ID.
	var uniformNICs []*NetworkInterface
	for _, nic := range nics {
		if strings.Contains(strings.ToLower(nic.AttachedVMID), "/virtualmachinescalesets/") {
			uniformNICs = append(uniformNICs, nic)
		}
	}
	require.NotEmpty(t, uniformNICs, "expected the named scale sets to have at least one NIC")

	// A uniform VMSS NIC is a child resource of the scale set VM it is
	// attached to, so its ID must extend its attached VM's ID.
	for _, nic := range uniformNICs {
		require.True(t,
			strings.HasPrefix(strings.ToLower(nic.ID), strings.ToLower(nic.AttachedVMID)+"/"),
			"expected NIC %q to be a child of its attached VM %q", nic.ID, nic.AttachedVMID)
	}

	logNICs(t, uniformNICs)
}

// newLiveNetworkInterfacesClient builds a NetworkInterfacesClient backed by a
// real Azure credential, skipping the test unless the live-test environment
// variables are set.
func newLiveNetworkInterfacesClient(t *testing.T) (NetworkInterfacesClient, string) {
	t.Helper()

	if os.Getenv("TELEPORT_TEST_AZURE") == "" {
		t.Skip("Set TELEPORT_TEST_AZURE to run this test against a real Azure subscription.")
	}
	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if subscriptionID == "" {
		t.Skip("Set AZURE_SUBSCRIPTION_ID to the subscription to list network interfaces from.")
	}
	resourceGroup := os.Getenv("AZURE_RESOURCE_GROUP")
	if resourceGroup == "" {
		t.Skip("Set AZURE_RESOURCE_GROUP to a resource group with at least one network interface.")
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	require.NoError(t, err, "failed to build a default Azure credential; have you run `az login`?")

	nicClient, err := NewNetworkInterfacesClient(subscriptionID, cred, nil)
	require.NoError(t, err)

	return nicClient, resourceGroup
}

// logNICs logs the listed NICs so live-test runs can be verified by eye, and
// sanity-checks the invariants every real Azure NIC satisfies.
func logNICs(t *testing.T, nics []*NetworkInterface) {
	t.Helper()

	t.Logf("listed %d NIC(s)", len(nics))
	for _, nic := range nics {
		require.NotEmpty(t, nic.ID)
		require.NotEmpty(t, nic.Name)
		t.Logf("  NIC %q: primary=%t attached VM=%q", nic.Name, nic.Primary, nic.AttachedVMID)
		for _, ipConfig := range nic.IPConfigurations {
			t.Logf("    ipconfig: primary=%t private IP=%q", ipConfig.Primary, ipConfig.PrivateIP)
		}
	}
}
