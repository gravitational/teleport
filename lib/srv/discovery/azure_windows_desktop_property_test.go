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
	"errors"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/server"
)

// azureIdentifierGen produces realistic, safe identifier strings (Azure VMIDs,
// subscription/resource-group/location names)
func azureIdentifierGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-z0-9]{1,8}(-[a-z0-9]{1,8}){0,4}`)
}

// azureVMIDGen produces realistic Azure VM IDs (real Azure VMIDs are GUIDs),
// with enough entropy that two draws colliding is not a practical concern.
func azureVMIDGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`)
}

// azurePrivateIPGen produces IP-shaped strings, most will be invalid, but it is
// sufficient for testing.
func azurePrivateIPGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}`)
}

// nonCollidingTagsGen produces random tags that don't collide with reserved
// label keys.
func nonCollidingTagsGen() *rapid.Generator[map[string]string] {
	tagKeyGen := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_.-]{0,20}`)
	return rapid.MapOfN(tagKeyGen, rapid.String(), 0, 5)
}

// azureVMGen generates a Windows VMs with a valid private IP and non-colliding
// tags.
func azureVMGen() *rapid.Generator[*azure.VirtualMachine] {
	return rapid.Custom(func(t *rapid.T) *azure.VirtualMachine {
		name := azureIdentifierGen().Draw(t, "name")
		return &azure.VirtualMachine{
			ID:               "/subscriptions/x/resourceGroups/x/providers/Microsoft.Compute/virtualMachines/" + name,
			Name:             name,
			Subscription:     azureIdentifierGen().Draw(t, "subscription"),
			ResourceGroup:    azureIdentifierGen().Draw(t, "resource_group"),
			Location:         azureIdentifierGen().Draw(t, "location"),
			VMID:             azureVMIDGen().Draw(t, "vmid"),
			PrimaryPrivateIP: azurePrivateIPGen().Draw(t, "private_ip"),
			Tags:             nonCollidingTagsGen().Draw(t, "tags"),
		}
	})
}

// TestProperty_UpsertAzureWindowsDesktop_NoPrivateIPAlwaysFails ensures that a
// VM with no private IP address is rejected.
func TestProperty_UpsertAzureWindowsDesktop_NoPrivateIPAlwaysFails(t *testing.T) {
	t.Parallel()
	s := newWindowsTestServer(t, &mockAzureRunCommandClient{}, clockwork.NewFakeClockAt(time.Now()))

	rapid.Check(t, func(t *rapid.T) {
		vm := azureVMGen().Draw(t, "vm")
		vm.PrimaryPrivateIP = ""

		err := s.createAzureWindowsDesktop(vm, time.Now())
		require.Error(t, err)
		require.True(t, errors.Is(err, errNoPrimaryPrivateIP))

		_, ok := findDynamicWindowsDesktop(t, s, "azure-windows-"+vm.VMID)
		require.False(t, ok)
	})
}

// TestProperty_UpsertAzureWindowsDesktop_FieldPassThrough ensures that all
// fields from a discovered VM are found on the resulting desktop.
func TestProperty_UpsertAzureWindowsDesktop_FieldPassThrough(t *testing.T) {
	t.Parallel()
	s := newWindowsTestServer(t, &mockAzureRunCommandClient{}, clockwork.NewFakeClockAt(time.Now()))

	rapid.Check(t, func(t *rapid.T) {
		vm := azureVMGen().Draw(t, "vm")
		syncTime := time.Now()

		require.NoError(t, s.createAzureWindowsDesktop(vm, syncTime))

		desktop, ok := findDynamicWindowsDesktop(t, s, "azure-windows-"+vm.VMID)
		require.True(t, ok)

		require.Equal(t, net.JoinHostPort(vm.PrimaryPrivateIP, "3389"), desktop.GetAddr())
		require.True(t, desktop.NonAD())
		require.True(t, syncTime.Add(s.PollInterval*5).Equal(desktop.Expiry()))

		labels := desktop.GetAllLabels()
		require.Equal(t, vm.Subscription, labels[types.SubscriptionIDLabelInternal])
		require.Equal(t, vm.ResourceGroup, labels[types.ResourceGroupLabelInternal])
		require.Equal(t, vm.Location, labels[types.RegionLabelInternal])
		require.Equal(t, vm.VMID, labels[types.VMIDLabelInternal])
		require.Equal(t, s.DiscoveryGroup, labels[types.TeleportInternalDiscoveryGroupName])

		require.Equal(t, types.CloudAzure, labels[types.CloudLabel])
		require.Equal(t, vm.Subscription, labels[types.SubscriptionIDLabel])
		require.Equal(t, vm.ResourceGroup, labels[types.ResourceGroupLabel])
		require.Equal(t, vm.Location, labels[types.RegionLabel])
		require.Equal(t, vm.VMID, labels[types.VMIDLabel])
	})
}

// TestProperty_UpsertAzureWindowsDesktop_TagsPropagateToLabels ensures that
// every tag propagates to the desktop's labels.
func TestProperty_UpsertAzureWindowsDesktop_TagsPropagateToLabels(t *testing.T) {
	t.Parallel()
	s := newWindowsTestServer(t, &mockAzureRunCommandClient{}, clockwork.NewFakeClockAt(time.Now()))

	rapid.Check(t, func(t *rapid.T) {
		vm := azureVMGen().Draw(t, "vm")

		require.NoError(t, s.createAzureWindowsDesktop(vm, time.Now()))

		desktop, ok := findDynamicWindowsDesktop(t, s, "azure-windows-"+vm.VMID)
		require.True(t, ok)

		labels := desktop.GetAllLabels()
		for k, v := range vm.Tags {
			require.Equal(t, v, labels[k], "tag %q should propagate to labels", k)
		}
	})
}

// TestProperty_UpsertAzureWindowsDesktop_NeverPanics ensures that
// upsertAzureWindowsDesktop never panics.
func TestProperty_UpsertAzureWindowsDesktop_NeverPanics(t *testing.T) {
	t.Parallel()
	s := newWindowsTestServer(t, &mockAzureRunCommandClient{}, clockwork.NewFakeClockAt(time.Now()))

	arbitraryVMGen := rapid.Custom(func(t *rapid.T) *azure.VirtualMachine {
		return &azure.VirtualMachine{
			ID:               rapid.String().Draw(t, "id"),
			Name:             rapid.String().Draw(t, "name"),
			Subscription:     rapid.String().Draw(t, "subscription"),
			ResourceGroup:    rapid.String().Draw(t, "resource_group"),
			Location:         rapid.String().Draw(t, "location"),
			VMID:             rapid.String().Draw(t, "vmid"),
			PrimaryPrivateIP: rapid.String().Draw(t, "private_ip"),
			Tags:             rapid.MapOfN(rapid.String(), rapid.String(), 0, 5).Draw(t, "tags"),
		}
	})

	rapid.Check(t, func(t *rapid.T) {
		vm := arbitraryVMGen.Draw(t, "vm")
		require.NotPanics(t, func() {
			_ = s.createAzureWindowsDesktop(vm, time.Now())
		})
	})
}

// azureInstallResultGen produces a random AzureInstallResults
func azureInstallResultGen(vm *azure.VirtualMachine) *rapid.Generator[server.AzureInstallResult] {
	return rapid.Custom(func(t *rapid.T) server.AzureInstallResult {
		var apiErr error
		if rapid.Bool().Draw(t, "has_api_error") {
			apiErr = trace.AccessDenied("synthetic error")
		}

		var cmdResult *azure.RunCommandResult
		if rapid.Bool().Draw(t, "has_command_result") {
			cmdResult = &azure.RunCommandResult{
				ExecutionState: rapid.SampledFrom([]string{"Succeeded", "Failed", ""}).Draw(t, "execution_state"),
				ExitCode:       rapid.Int32Range(-1, 5).Draw(t, "exit_code"),
			}
		}

		return server.AzureInstallResult{
			Instance:      vm,
			APIError:      apiErr,
			CommandResult: cmdResult,
		}
	})
}

// TestProperty_RegisterDynamicWindowsDesktops_FailureNeverRegisters ensures
// that a failed AzureInstallResult never produces a desktop.
func TestProperty_RegisterDynamicWindowsDesktops_FailureNeverRegisters(t *testing.T) {
	t.Parallel()
	s := newWindowsTestServer(t, &mockAzureRunCommandClient{}, clockwork.NewFakeClockAt(time.Now()))

	rapid.Check(t, func(t *rapid.T) {
		vm := azureVMGen().Draw(t, "vm")
		result := azureInstallResultGen(vm).Draw(t, "result")

		if !result.Failure() {
			t.Skip("only exercising the failure branch here")
		}

		err := s.registerDynamicWindowsDesktops(result, time.Now())
		require.Error(t, err)

		_, ok := findDynamicWindowsDesktop(t, s, "azure-windows-"+vm.VMID)
		require.False(t, ok, "a failed result must never produce a desktop")
	})
}

// TestProperty_RegisterDynamicWindowsDesktops_SuccessRegistersMatchingDesktop
// ensures that a successful AzureInstallResult produces a desktop.
func TestProperty_RegisterDynamicWindowsDesktops_SuccessRegistersMatchingDesktop(t *testing.T) {
	t.Parallel()
	s := newWindowsTestServer(t, &mockAzureRunCommandClient{}, clockwork.NewFakeClockAt(time.Now()))

	rapid.Check(t, func(t *rapid.T) {
		vm := azureVMGen().Draw(t, "vm")
		result := azureInstallResultGen(vm).Draw(t, "result")

		if result.Failure() {
			t.Skip("only exercising the success branch here")
		}

		require.NoError(t, s.registerDynamicWindowsDesktops(result, time.Now()))

		desktop, ok := findDynamicWindowsDesktop(t, s, "azure-windows-"+vm.VMID)
		require.True(t, ok)
		require.Equal(t, net.JoinHostPort(vm.PrimaryPrivateIP, "3389"), desktop.GetAddr())
	})
}
