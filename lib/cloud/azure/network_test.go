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
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestListNetworkInterfaces(t *testing.T) {
	t.Parallel()

	const (
		scaleSet1 = "vmss1"
		scaleSet2 = "vmss2"
	)

	regularNIC := &armnetwork.Interface{
		ID:   to.Ptr(createNICID(rgName, "nic1")),
		Name: to.Ptr("nic1"),
		Properties: &armnetwork.InterfacePropertiesFormat{
			Primary:        to.Ptr(true),
			VirtualMachine: &armnetwork.SubResource{ID: to.Ptr(createVMID(rgName, "vm1"))},
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr("ipconfig1"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						PrivateIPAddress: to.Ptr("10.0.0.1"),
						Primary:          to.Ptr(true),
					},
				},
			},
		},
	}
	wantRegularNIC := &NetworkInterface{
		ID:           createNICID(rgName, "nic1"),
		Name:         "nic1",
		Primary:      true,
		AttachedVMID: createVMID(rgName, "vm1"),
		IPConfigurations: []IPConfiguration{{
			Primary:   true,
			PrivateIP: "10.0.0.1",
		}},
	}

	multiIPConfigNIC := &armnetwork.Interface{
		ID:   to.Ptr(createNICID(rgName, "nic2")),
		Name: to.Ptr("nic2"),
		Properties: &armnetwork.InterfacePropertiesFormat{
			Primary: to.Ptr(false),
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr("ipconfig1"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						PrivateIPAddress: to.Ptr("10.0.0.2"),
						Primary:          to.Ptr(true),
					},
				},
				{
					Name: to.Ptr("ipconfig2"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						PrivateIPAddress: to.Ptr("10.0.0.128/25"),
						Primary:          to.Ptr(false),
					},
				},
				{
					// IP configuration without properties is skipped.
					Name: to.Ptr("ipconfig3"),
				},
			},
		},
	}
	wantMultiIPConfigNIC := &NetworkInterface{
		ID:   createNICID(rgName, "nic2"),
		Name: "nic2",
		IPConfigurations: []IPConfiguration{
			{
				Primary:   true,
				PrivateIP: "10.0.0.2",
			},
			{
				Primary:   false,
				PrivateIP: "10.0.0.128/25",
			},
		},
	}

	// A NIC not attached to any VM has no Primary flag and may have no
	// properties at all.
	detachedNIC := &armnetwork.Interface{
		ID:   to.Ptr(createNICID(rgName, "nic-detached")),
		Name: to.Ptr("nic-detached"),
	}
	wantDetachedNIC := &NetworkInterface{
		ID:               createNICID(rgName, "nic-detached"),
		Name:             "nic-detached",
		IPConfigurations: []IPConfiguration{},
	}

	scaleSet1NIC := &armnetwork.Interface{
		ID:   to.Ptr(createVMSSNICID(rgName, scaleSet1, "0", "vmss1-nic")),
		Name: to.Ptr("vmss1-nic"),
		Properties: &armnetwork.InterfacePropertiesFormat{
			Primary:        to.Ptr(true),
			VirtualMachine: &armnetwork.SubResource{ID: to.Ptr(createVMSSVMID(rgName, scaleSet1, "0"))},
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr("ipconfig1"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						PrivateIPAddress: to.Ptr("10.0.1.1"),
						Primary:          to.Ptr(true),
					},
				},
			},
		},
	}
	wantScaleSet1NIC := &NetworkInterface{
		ID:           createVMSSNICID(rgName, scaleSet1, "0", "vmss1-nic"),
		Name:         "vmss1-nic",
		Primary:      true,
		AttachedVMID: createVMSSVMID(rgName, scaleSet1, "0"),
		IPConfigurations: []IPConfiguration{{
			Primary:   true,
			PrivateIP: "10.0.1.1",
		}},
	}

	scaleSet2NIC := &armnetwork.Interface{
		ID:   to.Ptr(createVMSSNICID(rgName, scaleSet2, "0", "vmss2-nic")),
		Name: to.Ptr("vmss2-nic"),
		Properties: &armnetwork.InterfacePropertiesFormat{
			Primary:        to.Ptr(true),
			VirtualMachine: &armnetwork.SubResource{ID: to.Ptr(createVMSSVMID(rgName, scaleSet2, "0"))},
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr("ipconfig1"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						PrivateIPAddress: to.Ptr("10.0.2.1"),
						Primary:          to.Ptr(true),
					},
				},
			},
		},
	}
	wantScaleSet2NIC := &NetworkInterface{
		ID:           createVMSSNICID(rgName, scaleSet2, "0", "vmss2-nic"),
		Name:         "vmss2-nic",
		Primary:      true,
		AttachedVMID: createVMSSVMID(rgName, scaleSet2, "0"),
		IPConfigurations: []IPConfiguration{{
			Primary:   true,
			PrivateIP: "10.0.2.1",
		}},
	}

	for _, tc := range []struct {
		desc          string
		nicAPI        *ARMNetworkMock
		scaleSetNames []string
		resourceGroup string
		want          []*NetworkInterface
		expectErr     require.ErrorAssertionFunc
	}{
		{
			desc: "NICs from regular VMs, no scale set NICs",
			nicAPI: &ARMNetworkMock{
				NetworkInterfaces: map[string][]*armnetwork.Interface{
					rgName: {regularNIC},
				},
			},
			resourceGroup: rgName,
			want:          []*NetworkInterface{wantRegularNIC},
			expectErr:     require.NoError,
		},
		{
			desc:          "resource group without NICs returns nothing",
			nicAPI:        &ARMNetworkMock{},
			resourceGroup: rgName,
			expectErr:     require.NoError,
		},
		{
			desc: "converts secondary IP configurations and detached NICs",
			nicAPI: &ARMNetworkMock{
				NetworkInterfaces: map[string][]*armnetwork.Interface{
					rgName: {multiIPConfigNIC, detachedNIC},
				},
			},
			resourceGroup: rgName,
			want:          []*NetworkInterface{wantMultiIPConfigNIC, wantDetachedNIC},
			expectErr:     require.NoError,
		},
		{
			desc: "combines standalone and uniform scale set NICs",
			nicAPI: &ARMNetworkMock{
				NetworkInterfaces: map[string][]*armnetwork.Interface{
					rgName: {regularNIC},
				},
				VMSSNetworkInterfaces: map[string][]*armnetwork.Interface{
					rgName + "/" + scaleSet1: {scaleSet1NIC},
					rgName + "/" + scaleSet2: {scaleSet2NIC},
				},
			},
			resourceGroup: rgName,
			scaleSetNames: []string{scaleSet1, scaleSet2},
			want:          []*NetworkInterface{wantRegularNIC, wantScaleSet1NIC, wantScaleSet2NIC},
			expectErr:     require.NoError,
		},
		{
			desc: "scale set NICs are only listed for named scale sets",
			nicAPI: &ARMNetworkMock{
				NetworkInterfaces: map[string][]*armnetwork.Interface{
					rgName: {regularNIC},
				},
				VMSSNetworkInterfaces: map[string][]*armnetwork.Interface{
					rgName + "/" + scaleSet1: {scaleSet1NIC},
				},
			},
			resourceGroup: rgName,
			want:          []*NetworkInterface{wantRegularNIC},
			expectErr:     require.NoError,
		},
		{
			desc: "failure listing one scale set does not fail the rest",
			nicAPI: &ARMNetworkMock{
				NetworkInterfaces: map[string][]*armnetwork.Interface{
					rgName: {regularNIC},
				},
				VMSSNetworkInterfaces: map[string][]*armnetwork.Interface{
					rgName + "/" + scaleSet1: {scaleSet1NIC},
					rgName + "/" + scaleSet2: {scaleSet2NIC},
				},
				VMSSListErrs: map[string]error{
					scaleSet1: trace.AccessDenied("unauthorized"),
				},
			},
			resourceGroup: rgName,
			scaleSetNames: []string{scaleSet1, scaleSet2},
			want:          []*NetworkInterface{wantRegularNIC, wantScaleSet2NIC},
			expectErr:     require.NoError,
		},
		{
			desc: "nil NICs in the response are skipped",
			nicAPI: &ARMNetworkMock{
				NetworkInterfaces: map[string][]*armnetwork.Interface{
					rgName: {nil, regularNIC},
				},
			},
			resourceGroup: rgName,
			want:          []*NetworkInterface{wantRegularNIC},
			expectErr:     require.NoError,
		},
		{
			desc:          "auth error fails the whole listing",
			nicAPI:        &ARMNetworkMock{NoAuth: true},
			resourceGroup: rgName,
			expectErr: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
			},
		},
		{
			desc:          "wildcard resource group is rejected",
			nicAPI:        &ARMNetworkMock{},
			resourceGroup: "*",
			expectErr: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err), "expected BadParameter error, got %v", err)
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			client := NewNetworkInterfacesClientByAPI(NetworkInterfacesClientConfig{
				NetworkInterfacesAPI: tc.nicAPI,
			})

			nics, err := client.ListNetworkInterfaces(
				t.Context(),
				tc.resourceGroup,
				tc.scaleSetNames,
			)
			tc.expectErr(t, err)
			require.ElementsMatch(t, tc.want, nics)
		})
	}
}

func createNICID(resourceGroup, nicName string) string {
	return "/subscriptions/" + testSubID + "/resourceGroups/" + resourceGroup + "/providers/Microsoft.Network/networkInterfaces/" + nicName
}

func createVMID(resourceGroup, vmName string) string {
	return "/subscriptions/" + testSubID + "/resourceGroups/" + resourceGroup + "/providers/Microsoft.Compute/virtualMachines/" + vmName
}

func createVMSSVMID(resourceGroup, scaleSetName, instanceID string) string {
	return "/subscriptions/" + testSubID + "/resourceGroups/" + resourceGroup + "/providers/Microsoft.Compute/virtualMachineScaleSets/" + scaleSetName + "/virtualMachines/" + instanceID
}

func createVMSSNICID(resourceGroup, scaleSetName, instanceID, nicName string) string {
	return createVMSSVMID(resourceGroup, scaleSetName, instanceID) + "/networkInterfaces/" + nicName
}
