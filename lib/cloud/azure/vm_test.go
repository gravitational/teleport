/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestGetVirtualMachine(t *testing.T) {
	ctx := context.Background()
	validResourceID := "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm"

	for _, tc := range []struct {
		desc        string
		resourceID  string
		client      *ARMComputeMock
		assertError require.ErrorAssertionFunc
		assertVM    require.ValueAssertionFunc
	}{
		{
			desc:       "vm with valid user identities",
			resourceID: validResourceID,
			client: &ARMComputeMock{
				GetResult: armcompute.VirtualMachine{
					ID:   to.Ptr(validResourceID),
					Name: to.Ptr("name"),
					Identity: &armcompute.VirtualMachineIdentity{
						PrincipalID: to.Ptr("system assigned"),
						Type:        to.Ptr(armcompute.ResourceIdentityTypeSystemAssigned),
						UserAssignedIdentities: map[string]*armcompute.UserAssignedIdentitiesValue{
							"identity1": {},
							"identity2": {},
						},
					},
				},
			},
			assertError: require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Equal(t, vm.ID, validResourceID)
				require.Equal(t, "name", vm.Name)
				require.ElementsMatch(t, []Identity{
					{ResourceID: "system assigned"},
					{ResourceID: "identity1"},
					{ResourceID: "identity2"},
				}, vm.Identities)
			},
		},
		{
			desc:       "vm without identity",
			resourceID: validResourceID,
			client: &ARMComputeMock{
				GetResult: armcompute.VirtualMachine{
					ID:   to.Ptr(validResourceID),
					Name: to.Ptr("name"),
				},
			},
			assertError: require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Equal(t, vm.ID, validResourceID)
				require.Equal(t, "name", vm.Name)
				require.Empty(t, vm.Identities)
			},
		},
		{
			desc:       "vm with only user managed identities",
			resourceID: validResourceID,
			client: &ARMComputeMock{
				GetResult: armcompute.VirtualMachine{
					ID:   to.Ptr(validResourceID),
					Name: to.Ptr("name"),
					Identity: &armcompute.VirtualMachineIdentity{
						UserAssignedIdentities: map[string]*armcompute.UserAssignedIdentitiesValue{
							"identity1": {},
							"identity2": {},
						},
					},
				},
			},
			assertError: require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Equal(t, vm.ID, validResourceID)
				require.Equal(t, "name", vm.Name)
				require.ElementsMatch(t, []Identity{
					{ResourceID: "identity1"},
					{ResourceID: "identity2"},
				}, vm.Identities)
			},
		},
		{
			desc:        "invalid resource ID",
			resourceID:  "random-id",
			client:      &ARMComputeMock{},
			assertError: require.Error,
			assertVM:    require.Nil,
		},
		{
			desc:       "client error",
			resourceID: validResourceID,
			client: &ARMComputeMock{
				GetErr: fmt.Errorf("client error"),
			},
			assertError: require.Error,
			assertVM:    require.Nil,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			vmClient := NewVirtualMachinesClientByAPI(tc.client, nil /* scaleSetAPI */)

			vm, err := vmClient.Get(ctx, tc.resourceID)
			tc.assertError(t, err)
			tc.assertVM(t, vm)
		})
	}
}

func TestGetScaleSetVirtualMachine(t *testing.T) {
	ctx := context.Background()
	validResourceID := "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets/vmss/virtualMachines/0"

	for _, tc := range []struct {
		desc        string
		resourceID  string
		client      *ARMScaleSetMock
		assertError require.ErrorAssertionFunc
		assertVM    require.ValueAssertionFunc
	}{
		{
			desc:       "vm with valid user identities",
			resourceID: validResourceID,
			client: &ARMScaleSetMock{
				GetResult: armcompute.VirtualMachineScaleSetVM{
					ID:   to.Ptr(validResourceID),
					Name: to.Ptr("name"),
					Identity: &armcompute.VirtualMachineIdentity{
						PrincipalID: to.Ptr("system assigned"),
						Type:        to.Ptr(armcompute.ResourceIdentityTypeSystemAssigned),
						UserAssignedIdentities: map[string]*armcompute.UserAssignedIdentitiesValue{
							"identity1": {},
							"identity2": {},
						},
					},
				},
			},
			assertError: require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Equal(t, vm.ID, validResourceID)
				require.Equal(t, "name", vm.Name)
				require.ElementsMatch(t, []Identity{
					{ResourceID: "system assigned"},
					{ResourceID: "identity1"},
					{ResourceID: "identity2"},
				}, vm.Identities)
			},
		},
		{
			desc:       "vm without identity",
			resourceID: validResourceID,
			client: &ARMScaleSetMock{
				GetResult: armcompute.VirtualMachineScaleSetVM{
					ID:   to.Ptr(validResourceID),
					Name: to.Ptr("name"),
				},
			},
			assertError: require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Equal(t, vm.ID, validResourceID)
				require.Equal(t, "name", vm.Name)
				require.Empty(t, vm.Identities)
			},
		},
		{
			desc:       "vm with only user managed identities",
			resourceID: validResourceID,
			client: &ARMScaleSetMock{
				GetResult: armcompute.VirtualMachineScaleSetVM{
					ID:   to.Ptr(validResourceID),
					Name: to.Ptr("name"),
					Identity: &armcompute.VirtualMachineIdentity{
						UserAssignedIdentities: map[string]*armcompute.UserAssignedIdentitiesValue{
							"identity1": {},
							"identity2": {},
						},
					},
				},
			},
			assertError: require.NoError,
			assertVM: func(t require.TestingT, val any, _ ...any) {
				require.NotNil(t, val)
				vm, ok := val.(*VirtualMachine)
				require.Truef(t, ok, "expected *VirtualMachine, got %T", val)
				require.Equal(t, vm.ID, validResourceID)
				require.Equal(t, "name", vm.Name)
				require.ElementsMatch(t, []Identity{
					{ResourceID: "identity1"},
					{ResourceID: "identity2"},
				}, vm.Identities)
			},
		},
		{
			desc:       "client error",
			resourceID: validResourceID,
			client: &ARMScaleSetMock{
				GetErr: fmt.Errorf("client error"),
			},
			assertError: require.Error,
			assertVM:    require.Nil,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			vmClient := NewVirtualMachinesClientByAPI(nil /* api */, tc.client)

			vm, err := vmClient.Get(ctx, tc.resourceID)
			tc.assertError(t, err)
			tc.assertVM(t, vm)
		})
	}
}

func TestListVirtualMachines(t *testing.T) {
	t.Parallel()
	mockAPI := &ARMComputeMock{
		VirtualMachines: map[string][]*armcompute.VirtualMachine{
			"rg1": {
				{ID: to.Ptr("vm1")},
				{ID: to.Ptr("vm2")},
			},
			"rg2": {
				{ID: to.Ptr("vm3")},
				{ID: to.Ptr("vm4")},
			},
		},
	}
	tests := []struct {
		name          string
		resourceGroup string
		wantIDs       []string
	}{
		{
			name:          "existing resource group",
			resourceGroup: "rg1",
			wantIDs:       []string{"vm1", "vm2"},
		},
		{
			name:          "nonexistent resource group",
			resourceGroup: "rgfake",
			wantIDs:       []string{},
		},
		{
			name:          "all resource groups",
			resourceGroup: types.Wildcard,
			wantIDs:       []string{"vm1", "vm2", "vm3", "vm4"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := NewVirtualMachinesClientByAPI(mockAPI, nil /* scaleSetAPI */)

			vms, err := client.ListVirtualMachines(context.Background(), tc.resourceGroup)
			require.NoError(t, err)
			var vmIDs []string
			for _, vm := range vms {
				vmIDs = append(vmIDs, *vm.ID)
			}
			require.ElementsMatch(t, tc.wantIDs, vmIDs)
		})
	}
}

func TestParsePowerState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		statuses []*armcompute.InstanceViewStatus
		want     PowerState
	}{
		{
			name: "running",
			statuses: []*armcompute.InstanceViewStatus{
				{Code: to.Ptr("ProvisioningState/succeeded")},
				{Code: to.Ptr("PowerState/running")},
			},
			want: PowerStateRunning,
		},
		{
			name: "deallocated",
			statuses: []*armcompute.InstanceViewStatus{
				{Code: to.Ptr("PowerState/deallocated")},
			},
			want: PowerStateDeallocated,
		},
		{
			name: "stopped",
			statuses: []*armcompute.InstanceViewStatus{
				{Code: to.Ptr("PowerState/stopped")},
			},
			want: PowerStateStopped,
		},
		{
			name: "transient suffix returned verbatim",
			statuses: []*armcompute.InstanceViewStatus{
				{Code: to.Ptr("PowerState/starting")},
			},
			want: PowerState("starting"),
		},
		{
			name: "no power-state status",
			statuses: []*armcompute.InstanceViewStatus{
				{Code: to.Ptr("ProvisioningState/succeeded")},
			},
			want: PowerStateUnknown,
		},
		{
			name:     "empty statuses",
			statuses: []*armcompute.InstanceViewStatus{},
			want:     PowerStateUnknown,
		},
		{
			name:     "nil statuses",
			statuses: nil,
			want:     PowerStateUnknown,
		},
		{
			name: "nil status entry",
			statuses: []*armcompute.InstanceViewStatus{
				nil,
				{Code: to.Ptr("PowerState/running")},
			},
			want: PowerStateRunning,
		},
		{
			name: "nil code",
			statuses: []*armcompute.InstanceViewStatus{
				{Code: nil},
			},
			want: PowerStateUnknown,
		},
		{
			name: "first power state wins",
			statuses: []*armcompute.InstanceViewStatus{
				{Code: to.Ptr("PowerState/stopping")},
				{Code: to.Ptr("PowerState/running")},
			},
			want: PowerState("stopping"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParsePowerState(tc.statuses)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestVMOSType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		vm   *armcompute.VirtualMachine
		want string
	}{
		{
			name: "linux",
			vm: &armcompute.VirtualMachine{
				Properties: &armcompute.VirtualMachineProperties{
					StorageProfile: &armcompute.StorageProfile{
						OSDisk: &armcompute.OSDisk{
							OSType: to.Ptr(armcompute.OperatingSystemTypesLinux),
						},
					},
				},
			},
			want: string(armcompute.OperatingSystemTypesLinux),
		},
		{
			name: "windows",
			vm: &armcompute.VirtualMachine{
				Properties: &armcompute.VirtualMachineProperties{
					StorageProfile: &armcompute.StorageProfile{
						OSDisk: &armcompute.OSDisk{
							OSType: to.Ptr(armcompute.OperatingSystemTypesWindows),
						},
					},
				},
			},
			want: string(armcompute.OperatingSystemTypesWindows),
		},
		{
			name: "nil vm",
			vm:   nil,
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := VMOSType(tc.vm)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestFilterLinuxVMs(t *testing.T) {
	t.Parallel()

	linuxVM := &armcompute.VirtualMachine{
		Name: to.Ptr("linux-vm"),
		ID:   to.Ptr("/sub/rg/linux-vm"),
		Properties: &armcompute.VirtualMachineProperties{
			StorageProfile: &armcompute.StorageProfile{
				OSDisk: &armcompute.OSDisk{
					OSType: to.Ptr(armcompute.OperatingSystemTypesLinux),
				},
			},
		},
	}
	windowsVM := &armcompute.VirtualMachine{
		Name: to.Ptr("windows-vm"),
		ID:   to.Ptr("/sub/rg/windows-vm"),
		Properties: &armcompute.VirtualMachineProperties{
			StorageProfile: &armcompute.StorageProfile{
				OSDisk: &armcompute.OSDisk{
					OSType: to.Ptr(armcompute.OperatingSystemTypesWindows),
				},
			},
		},
	}
	unknownOSVM := &armcompute.VirtualMachine{
		Name: to.Ptr("unknown-vm"),
		ID:   to.Ptr("/sub/rg/unknown-vm"),
	}
	tests := []struct {
		name        string
		input       []*armcompute.VirtualMachine
		wantLinux   []*armcompute.VirtualMachine
		wantSkipped []*armcompute.VirtualMachine
	}{
		{
			name:        "nil input",
			input:       nil,
			wantLinux:   nil,
			wantSkipped: nil,
		},
		{
			name:      "nil entry in slice is skipped",
			input:     []*armcompute.VirtualMachine{nil, linuxVM},
			wantLinux: []*armcompute.VirtualMachine{linuxVM},
		},
		{
			name:      "all linux",
			input:     []*armcompute.VirtualMachine{linuxVM},
			wantLinux: []*armcompute.VirtualMachine{linuxVM},
		},
		{
			name:        "all windows",
			input:       []*armcompute.VirtualMachine{windowsVM},
			wantSkipped: []*armcompute.VirtualMachine{windowsVM},
		},
		{
			name:      "unknown OS allowed through",
			input:     []*armcompute.VirtualMachine{unknownOSVM},
			wantLinux: []*armcompute.VirtualMachine{unknownOSVM},
		},
		{
			name: "mixed linux, windows, and unknown",
			input: []*armcompute.VirtualMachine{
				linuxVM, windowsVM, unknownOSVM,
			},
			wantLinux: []*armcompute.VirtualMachine{
				linuxVM, unknownOSVM,
			},
			wantSkipped: []*armcompute.VirtualMachine{windowsVM},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotLinux, gotSkipped := FilterLinuxVMs(tc.input)
			require.Equal(t, tc.wantLinux, gotLinux)
			require.Equal(t, tc.wantSkipped, gotSkipped)
		})
	}
}

func TestGetPowerState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		getResult  armcompute.VirtualMachine
		getErr     error
		wantState  PowerState
		wantErrStr string
	}{
		{
			name: "running",
			getResult: armcompute.VirtualMachine{
				Properties: &armcompute.VirtualMachineProperties{
					InstanceView: &armcompute.VirtualMachineInstanceView{
						Statuses: []*armcompute.InstanceViewStatus{
							{Code: to.Ptr("ProvisioningState/succeeded")},
							{Code: to.Ptr("PowerState/running")},
						},
					},
				},
			},
			wantState: PowerStateRunning,
		},
		{
			name: "deallocated",
			getResult: armcompute.VirtualMachine{
				Properties: &armcompute.VirtualMachineProperties{
					InstanceView: &armcompute.VirtualMachineInstanceView{
						Statuses: []*armcompute.InstanceViewStatus{
							{Code: to.Ptr("PowerState/deallocated")},
						},
					},
				},
			},
			wantState: PowerStateDeallocated,
		},
		{
			name: "transient suffix returned verbatim",
			getResult: armcompute.VirtualMachine{
				Properties: &armcompute.VirtualMachineProperties{
					InstanceView: &armcompute.VirtualMachineInstanceView{
						Statuses: []*armcompute.InstanceViewStatus{
							{Code: to.Ptr("PowerState/starting")},
						},
					},
				},
			},
			wantState: PowerState("starting"),
		},
		{
			name: "no instance view returns unknown",
			getResult: armcompute.VirtualMachine{
				Properties: &armcompute.VirtualMachineProperties{},
			},
			wantState: PowerStateUnknown,
		},
		{
			name:      "nil properties returns unknown",
			getResult: armcompute.VirtualMachine{},
			wantState: PowerStateUnknown,
		},
		{
			name:       "ARM error propagates",
			getErr:     fmt.Errorf("arm boom"),
			wantState:  PowerStateUnknown,
			wantErrStr: "arm boom",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := NewVirtualMachinesClientByAPI(&ARMComputeMock{
				GetResult: tc.getResult,
				GetErr:    tc.getErr,
			}, nil)
			state, err := client.GetPowerState(t.Context(), "rg", "vm")
			if tc.wantErrStr != "" {
				require.ErrorContains(t, err, tc.wantErrStr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantState, state)
		})
	}
}
