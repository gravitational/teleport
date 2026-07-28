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
	"log/slog"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils/slices"
	"github.com/gravitational/trace"
)

// IPConfiguration represents an Azure network interface IP configuration.
type IPConfiguration struct {
	// Primary indicates whether this IP configuration is the primary one for the network interface.
	Primary bool
	// PrivateIP is the private IP address assigned to this IP configuration. This
	// field may be in CIDR notation, if this isn't the Primary IP configuration.
	PrivateIP string
}

// NetworkInterface represents an Azure network interface.
type NetworkInterface struct {
	// ID is the resource ID of the network interface.
	ID string
	// Name is the resource name of the network interface.
	Name string
	// Primary indicates whether this network interface is the primary one for the
	// virtual machine or virtual machine scale set instance.
	Primary bool
	// AttachedVMID is the resource ID of the virtual machine the network
	// interface is attached to. Empty for network interfaces not attached to any
	// virtual machine. Resource IDs are not casing-stable across Azure APIs, so
	// compare case-insensitively when matching against VM resource IDs.
	AttachedVMID string
	// IPConfigurations is a list of IP configurations associated with the network
	// interface.
	IPConfigurations []IPConfiguration
}

// networkInterfacesLister is satisfied by *armnetwork.InterfacesClient.
type networkInterfacesLister interface {
	// NewListPager list Azure network interfaces in the given resource group.
	NewListPager(resourceGroup string, opts *armnetwork.InterfacesClientListOptions) *runtime.Pager[armnetwork.InterfacesClientListResponse]
	// NewListVirtualMachineScaleSetNetworkInterfacesPager lists Azure network interfaces in the given resource group and scale set.
	NewListVirtualMachineScaleSetNetworkInterfacesPager(resourceGroup, scaleSetName string, opts *armnetwork.InterfacesClientListVirtualMachineScaleSetNetworkInterfacesOptions) *runtime.Pager[armnetwork.InterfacesClientListVirtualMachineScaleSetNetworkInterfacesResponse]
}

// NetworkInterfacesClient is an interface for listing Azure network interfaces.
type NetworkInterfacesClient interface {
	// ListNetworkInterfaces lists all network interfaces in the given resource group.
	// Wildcard resource group names are not supported.
	ListNetworkInterfaces(ctx context.Context, resourceGroup string, scaleSetNames []string) ([]*NetworkInterface, error)
}

type networkInterfacesClient struct {
	networkInterfacesLister networkInterfacesLister
	logger                  *slog.Logger
}

func NewNetworkInterfacesClient(subscriptionID string, cred azcore.TokenCredential, options *arm.ClientOptions) (NetworkInterfacesClient, error) {
	networkInterfacesClient, err := armnetwork.NewInterfacesClient(subscriptionID, cred, options)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create Azure network interfaces client")
	}

	config := NetworkInterfacesClientConfig{
		NetworkInterfacesAPI: networkInterfacesClient,
	}
	return NewNetworkInterfacesClientByAPI(config), nil
}

// NetworkInterfacesClientConfig is a configuration struct for creating a
// NetworkInterfacesClient.
type NetworkInterfacesClientConfig struct {
	NetworkInterfacesAPI networkInterfacesLister
	Logger               *slog.Logger
}

func NewNetworkInterfacesClientByAPI(config NetworkInterfacesClientConfig) NetworkInterfacesClient {
	if config.Logger == nil {
		config.Logger = slog.Default().With(teleport.ComponentKey, "azure_networkinterfaces_client")
	}

	return &networkInterfacesClient{
		networkInterfacesLister: config.NetworkInterfacesAPI,
		logger:                  config.Logger,
	}
}

// ListNetworkInterfaces lists all network interfaces in the given resource group.
//
// Wildcard resource group names are not currently supported. As this call
// is likely a follow up to a ListVM call, the caller will already be able to
// parse their required resource group names rather than searching the entire
// subscription for network interfaces.
func (c *networkInterfacesClient) ListNetworkInterfaces(ctx context.Context, resourceGroup string, scaleSetNames []string) ([]*NetworkInterface, error) {
	if resourceGroup == "*" {
		return nil, trace.BadParameter("wildcard resource group names are not supported")
	}

	standardAndFlexibleNICs, err := c.listStandardAndFlexibleNICs(ctx, resourceGroup)
	if err != nil {
		return nil, trace.Wrap(err, "failed to list standard and flexible VMSS NICs")
	}

	// Currently, we're more concerned with listing standard and flexible VMSS
	// NICs, so if a uniform VMSS NIC listing fails, it is logged and the process
	// continues.
	uniformNICs := c.listUniformNICs(ctx, resourceGroup, scaleSetNames)

	return append(standardAndFlexibleNICs, uniformNICs...), nil
}

func (c *networkInterfacesClient) listStandardAndFlexibleNICs(ctx context.Context, resourceGroup string) ([]*NetworkInterface, error) {
	pager := newAPIPager(
		c.networkInterfacesLister.NewListPager(resourceGroup, nil),
		func(resp armnetwork.InterfacesClientListResponse) []*armnetwork.Interface {
			return resp.InterfaceListResult.Value
		},
	)
	return c.collectNICs(ctx, pager)
}

func (c *networkInterfacesClient) listUniformNICs(ctx context.Context, resourceGroup string, scaleSetNames []string) []*NetworkInterface {
	var allNICs []*NetworkInterface
	for _, scaleSetName := range slices.DeduplicateKey(scaleSetNames, strings.ToLower) {
		pager := newAPIPager(
			c.networkInterfacesLister.NewListVirtualMachineScaleSetNetworkInterfacesPager(resourceGroup, scaleSetName, nil),
			func(resp armnetwork.InterfacesClientListVirtualMachineScaleSetNetworkInterfacesResponse) []*armnetwork.Interface {
				return resp.InterfaceListResult.Value
			},
		)
		nics, err := c.collectNICs(ctx, pager)
		if err != nil {
			c.logger.WarnContext(ctx, "failed to list NICs from Uniform VMSS", "scale_set_name", scaleSetName, "error", err)
			continue
		}
		allNICs = append(allNICs, nics...)
	}
	return allNICs
}

func (c *networkInterfacesClient) collectNICs(ctx context.Context, pager apiPager[armnetwork.Interface]) ([]*NetworkInterface, error) {
	var nics []*NetworkInterface
	for pager.more() {
		res, err := pager.nextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		for _, rawNIC := range res {
			if rawNIC == nil {
				continue
			}
			nic, err := nicFromArmNetworkInterface(rawNIC)
			if err != nil {
				c.logger.DebugContext(ctx, "skipping Azure Network Interface",
					"resource_id", StringVal(rawNIC.ID),
					"error", err,
				)
				continue
			}
			nics = append(nics, nic)
		}
	}
	return nics, nil
}

func nicFromArmNetworkInterface(nic *armnetwork.Interface) (*NetworkInterface, error) {
	if nic == nil {
		return nil, trace.BadParameter("nil armnetwork.Interface")
	}

	var primary bool
	var attachedVMID string
	ipConfigs := []IPConfiguration{}
	if nic.Properties != nil {
		primary = BoolVal(nic.Properties.Primary)
		if nic.Properties.VirtualMachine != nil {
			attachedVMID = StringVal(nic.Properties.VirtualMachine.ID)
		}
		if nic.Properties.IPConfigurations != nil {
			for _, ipConfig := range nic.Properties.IPConfigurations {
				if ipConfig != nil && ipConfig.Properties != nil {
					ipConfigs = append(ipConfigs, IPConfiguration{
						Primary:   BoolVal(ipConfig.Properties.Primary),
						PrivateIP: StringVal(ipConfig.Properties.PrivateIPAddress),
					})
				}
			}
		}
	}

	return &NetworkInterface{
		ID:               StringVal(nic.ID),
		Name:             StringVal(nic.Name),
		Primary:          primary,
		AttachedVMID:     attachedVMID,
		IPConfigurations: ipConfigs,
	}, nil
}

// BoolVal converts a pointer of a bool to a bool value.
func BoolVal(b *bool) bool {
	if b != nil {
		return *b
	}
	return false
}
