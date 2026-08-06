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
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// ipConfigGen returns a rapid.Generator that produces arbitrary Azure network
// interface IP configurations.
func ipConfigGen() *rapid.Generator[armnetwork.InterfaceIPConfiguration] {
	return rapid.Custom(func(t *rapid.T) armnetwork.InterfaceIPConfiguration {
		return armnetwork.InterfaceIPConfiguration{
			Name: rapid.Ptr(rapid.String(), true).Draw(t, "ipconfig_name"),
			Properties: rapid.Ptr(rapid.Custom(func(t *rapid.T) armnetwork.InterfaceIPConfigurationPropertiesFormat {
				return armnetwork.InterfaceIPConfigurationPropertiesFormat{
					PrivateIPAddress: rapid.Ptr(rapid.String(), true).Draw(t, "private_ip"),
					Primary:          rapid.Ptr(rapid.Bool(), true).Draw(t, "ipconfig_primary"),
				}
			}), true).Draw(t, "ipconfig_props"),
		}
	})
}

// nicGen returns a rapid.Generator that produces arbitrary Azure network
// interfaces.
func nicGen() *rapid.Generator[*armnetwork.Interface] {
	return rapid.Custom(func(t *rapid.T) *armnetwork.Interface {
		return &armnetwork.Interface{
			ID:   rapid.Ptr(rapid.String(), true).Draw(t, "id"),
			Name: rapid.Ptr(rapid.String(), true).Draw(t, "name"),
			Properties: rapid.Ptr(rapid.Custom(func(t *rapid.T) armnetwork.InterfacePropertiesFormat {
				return armnetwork.InterfacePropertiesFormat{
					Primary: rapid.Ptr(rapid.Bool(), true).Draw(t, "primary"),
					VirtualMachine: rapid.Ptr(rapid.Custom(func(t *rapid.T) armnetwork.SubResource {
						return armnetwork.SubResource{
							ID: rapid.Ptr(rapid.String(), true).Draw(t, "vm_id"),
						}
					}), true).Draw(t, "virtual_machine"),
					IPConfigurations: rapid.SliceOfN(rapid.Ptr(ipConfigGen(), true), 0, 5).Draw(t, "ipconfigs"),
				}
			}), true).Draw(t, "props"),
		}
	})
}

// TestProperty_nicFromArmNetworkInterface_NeverPanicsOnArbitraryInput ensures that
// nicFromArmNetworkInterface never panics, even when given arbitrary input.
func TestProperty_nicFromArmNetworkInterface_NeverPanicsOnArbitraryInput(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nicGen := nicGen()

		nic, err := nicFromArmNetworkInterface(nicGen.Draw(t, "nic"))
		require.NoError(t, err)

		if nicGen == nil {
			require.Nil(t, nic)
		} else {
			require.NotNil(t, nic)
		}
	})
}

// TestProperty_nicFromArmNetworkInterface_FieldPassThrough ensures that all fields
// of an Azure network interface are passed through to the Teleport representation
// of the network interface.
func TestProperty_nicFromArmNetworkInterface_FieldPassThrough(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		rawNIC := nicGen().Draw(t, "nic")

		nic, err := nicFromArmNetworkInterface(rawNIC)
		require.NoError(t, err)
		require.NotNil(t, nic)

		if rawNIC.ID != nil {
			require.Equal(t, *rawNIC.ID, nic.ID)
		} else {
			require.Empty(t, nic.ID)
		}
		if rawNIC.Name != nil {
			require.Equal(t, *rawNIC.Name, nic.Name)
		} else {
			require.Empty(t, nic.Name)
		}

		wantPrimary := rawNIC.Properties != nil &&
			rawNIC.Properties.Primary != nil &&
			*rawNIC.Properties.Primary
		require.Equal(t, wantPrimary, nic.Primary)

		wantVMID := ""
		if rawNIC.Properties != nil &&
			rawNIC.Properties.VirtualMachine != nil &&
			rawNIC.Properties.VirtualMachine.ID != nil {
			wantVMID = *rawNIC.Properties.VirtualMachine.ID
		}
		require.Equal(t, wantVMID, nic.AttachedVMID)

		if rawNIC.Properties != nil {
			outIdx := 0
			for _, ipConfig := range rawNIC.Properties.IPConfigurations {
				if ipConfig == nil || ipConfig.Properties == nil {
					continue
				}
				require.Less(t, outIdx, len(nic.IPConfigurations))
				got := nic.IPConfigurations[outIdx]
				outIdx++

				if ipConfig.Properties.Primary != nil {
					require.Equal(t, *ipConfig.Properties.Primary, got.Primary)
				} else {
					require.False(t, got.Primary)
				}
				if ipConfig.Properties.PrivateIPAddress != nil {
					require.Equal(t, *ipConfig.Properties.PrivateIPAddress, got.PrivateIP)
				} else {
					require.Empty(t, got.PrivateIP)
				}
			}
			require.Len(t, nic.IPConfigurations, outIdx)
		}
	})
}

// TestProperty_ListNetworkInterfaces_Aggregation ensures that
// ListNetworkInterfaces correctly aggregates network interfaces from multiple
// sources, including standalone network interfaces and those associated with
// virtual machine scale sets.
func TestProperty_ListNetworkInterfaces_Aggregation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		standalone := rapid.SliceOfN(nicGen(), 0, 5).Draw(t, "standalone")
		scaleSets := rapid.MapOfN(
			rapid.StringMatching(`vmss-[a-z]{1,5}`),
			rapid.SliceOfN(nicGen(), 0, 3),
			0, 4,
		).Draw(t, "scale_sets")

		vmssNICs := make(map[string][]*armnetwork.Interface, len(scaleSets))
		vmssErrs := make(map[string]error)
		var scaleSetIDs []string
		wantCount := len(standalone)
		for name, nics := range scaleSets {
			scaleSetIDs = append(scaleSetIDs, createVMSSID(rgName, name))
			vmssNICs[rgName+"/"+name] = nics
			if rapid.Bool().Draw(t, "fails_"+name) {
				vmssErrs[name] = trace.AccessDenied("unauthorized")
			} else {
				wantCount += len(nics)
			}
		}

		client := NewNetworkInterfacesClientByAPI(NetworkInterfacesClientConfig{
			NetworkInterfacesAPI: &ARMNetworkMock{
				NetworkInterfaces:     map[string][]*armnetwork.Interface{rgName: standalone},
				VMSSNetworkInterfaces: vmssNICs,
				VMSSListErrs:          vmssErrs,
			},
			SubscriptionID: testSubID,
		})

		got, err := client.ListNetworkInterfaces(t.Context(), rgName, scaleSetIDs...)
		require.NoError(t, err)
		require.Len(t, got, wantCount)
	})
}

// TestProperty_collectNICs_PageBoundaries ensures that collectNICs correctly
// aggregates network interfaces across multiple pages of results.
func TestProperty_collectNICs_PageBoundaries(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nics := rapid.SliceOfN(
			rapid.OneOf(rapid.Just[*armnetwork.Interface](nil), nicGen()),
			0, 20,
		).Draw(t, "nics")

		// Split the NICs into pages at arbitrary boundaries.
		var pages [][]*armnetwork.Interface
		for remaining := nics; len(remaining) > 0; {
			n := rapid.IntRange(1, len(remaining)).Draw(t, "page_size")
			pages = append(pages, remaining[:n])
			remaining = remaining[n:]
		}
		if len(pages) == 0 {
			// A pager always serves at least one empty page.
			pages = [][]*armnetwork.Interface{nil}
		}

		pageIdx := 0
		pager := runtime.NewPager(runtime.PagingHandler[armnetwork.InterfacesClientListResponse]{
			More: func(page armnetwork.InterfacesClientListResponse) bool {
				return page.NextLink != nil && *page.NextLink != ""
			},
			Fetcher: func(_ context.Context, _ *armnetwork.InterfacesClientListResponse) (armnetwork.InterfacesClientListResponse, error) {
				page := pages[pageIdx]
				pageIdx++
				var nextLink *string
				if pageIdx < len(pages) {
					nextLink = to.Ptr("nextpage")
				}
				return armnetwork.InterfacesClientListResponse{
					InterfaceListResult: armnetwork.InterfaceListResult{
						Value:    page,
						NextLink: nextLink,
					},
				}, nil
			},
		})

		c := &networkInterfacesClient{logger: logtest.NewLogger()}
		got, err := c.collectNICs(t.Context(), newAPIPager(pager, func(resp armnetwork.InterfacesClientListResponse) []*armnetwork.Interface {
			return resp.InterfaceListResult.Value
		}))
		require.NoError(t, err)

		// Check NICs appear converted and in order, no matter how the input was
		// split into pages. nicFromArmNetworkInterface is used as the oracle to
		// determine the expected output.
		var want []*NetworkInterface
		for _, rawNIC := range nics {
			if rawNIC == nil {
				continue
			}
			nic, err := nicFromArmNetworkInterface(rawNIC)
			require.NoError(t, err)
			want = append(want, nic)
		}
		require.Equal(t, want, got)
	})
}
