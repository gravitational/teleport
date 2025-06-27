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
			name:          "nonexistant resource group",
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
