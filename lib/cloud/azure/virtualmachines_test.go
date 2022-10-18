/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package azure

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/stretchr/testify/require"
)

func TestListVirtualMachines(t *testing.T) {
	t.Parallel()
	mockAPI := &ARMVirtualMachinesMock{
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := NewVirtualMachinesClientByAPI(mockAPI)

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
