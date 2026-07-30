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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/slices"
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
	// NewListPager lists Azure network interfaces in the given resource group.
	NewListPager(resourceGroup string, opts *armnetwork.InterfacesClientListOptions) *runtime.Pager[armnetwork.InterfacesClientListResponse]
	// NewListAllPager lists all Azure network interfaces in the subscription.
	NewListAllPager(opts *armnetwork.InterfacesClientListAllOptions) *runtime.Pager[armnetwork.InterfacesClientListAllResponse]
	// NewListVirtualMachineScaleSetNetworkInterfacesPager lists Azure network interfaces in the given resource group and scale set.
	NewListVirtualMachineScaleSetNetworkInterfacesPager(resourceGroup, scaleSetName string, opts *armnetwork.InterfacesClientListVirtualMachineScaleSetNetworkInterfacesOptions) *runtime.Pager[armnetwork.InterfacesClientListVirtualMachineScaleSetNetworkInterfacesResponse]
}

// NetworkInterfacesClient is an interface for listing Azure network interfaces.
type NetworkInterfacesClient interface {
	// ListNetworkInterfaces lists all network interfaces in the given resource group.
	ListNetworkInterfaces(ctx context.Context, resourceGroup string, scaleSetIDs ...string) ([]*NetworkInterface, error)
}

type networkInterfacesClient struct {
	networkInterfacesLister networkInterfacesLister
	logger                  *slog.Logger
	// subscriptionID is the Azure subscription ID to list network interfaces from.
	// This is used to validate that any scale set IDs passed are in the same
	// subscription.
	subscriptionID string
}

func NewNetworkInterfacesClient(subscriptionID string, cred azcore.TokenCredential, options *arm.ClientOptions) (NetworkInterfacesClient, error) {
	networkInterfacesClient, err := armnetwork.NewInterfacesClient(subscriptionID, cred, options)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create Azure network interfaces client")
	}

	config := NetworkInterfacesClientConfig{
		NetworkInterfacesAPI: networkInterfacesClient,
		SubscriptionID:       subscriptionID,
	}
	return NewNetworkInterfacesClientByAPI(config), nil
}

// NetworkInterfacesClientConfig is a configuration struct for creating a
// NetworkInterfacesClient.
type NetworkInterfacesClientConfig struct {
	NetworkInterfacesAPI networkInterfacesLister
	Logger               *slog.Logger
	// SubscriptionID is the Azure subscription ID to list network interfaces from.
	// This is used to validate that any scale set IDs passed are in the same
	// subscription.
	SubscriptionID string
}

func NewNetworkInterfacesClientByAPI(config NetworkInterfacesClientConfig) NetworkInterfacesClient {
	if config.Logger == nil {
		config.Logger = slog.Default().With(teleport.ComponentKey, "azure_networkinterfaces_client")
	}

	return &networkInterfacesClient{
		networkInterfacesLister: config.NetworkInterfacesAPI,
		logger:                  config.Logger,
		subscriptionID:          config.SubscriptionID,
	}
}

// ListNetworkInterfaces lists all network interfaces in the given resource group.
//
// scaleSetIDs is a list of resource IDs for the virtual machine scale sets.
// e.g. "/subscriptions/<sub>/resourceGroups/<rg>/providers/Microsoft.Compute/virtualMachineScaleSets/<vmss>"
// If a scale set ID is not in the same subscription or resource group, it is
// skipped with a warning.
func (c *networkInterfacesClient) ListNetworkInterfaces(ctx context.Context, resourceGroup string, scaleSetIDs ...string) ([]*NetworkInterface, error) {
	standardAndFlexibleNICs, err := c.listStandardAndFlexibleNICs(ctx, resourceGroup)
	if err != nil {
		return nil, trace.Wrap(err, "failed to list standard and flexible VMSS NICs")
	}

	// Currently, we're more concerned with listing standard and flexible VMSS
	// NICs, so if a uniform VMSS NIC listing fails, it is logged and the process
	// continues.
	uniformNICs := c.listUniformNICs(ctx, resourceGroup, scaleSetIDs)

	return append(standardAndFlexibleNICs, uniformNICs...), nil
}

func (c *networkInterfacesClient) listStandardAndFlexibleNICs(ctx context.Context, resourceGroup string) ([]*NetworkInterface, error) {
	var pager apiPager[armnetwork.Interface]
	if resourceGroup == types.Wildcard {
		pager = newAPIPager(
			c.networkInterfacesLister.NewListAllPager(nil),
			func(resp armnetwork.InterfacesClientListAllResponse) []*armnetwork.Interface {
				return resp.InterfaceListResult.Value
			},
		)
	} else {
		pager = newAPIPager(
			c.networkInterfacesLister.NewListPager(resourceGroup, nil),
			func(resp armnetwork.InterfacesClientListResponse) []*armnetwork.Interface {
				return resp.InterfaceListResult.Value
			},
		)
	}
	return c.collectNICs(ctx, pager)
}

func (c *networkInterfacesClient) listUniformNICs(ctx context.Context, resourceGroup string, scaleSetIDs []string) []*NetworkInterface {
	var allNICs []*NetworkInterface
	for _, scaleSetID := range slices.DeduplicateKey(scaleSetIDs, strings.ToLower) {
		id, err := arm.ParseResourceID(scaleSetID)
		if err != nil {
			c.logger.WarnContext(ctx, "Failed to parse scale set ID", "scale_set_id", scaleSetID, "error", err)
			continue
		}

		// Check the resource ID is for a uniform VMSS in the same subscription and resource group.
		isComputeNamespace := strings.EqualFold(id.ResourceType.Namespace, "Microsoft.Compute")
		isVMSSResourceType := strings.EqualFold(id.ResourceType.Type, "virtualMachineScaleSets")
		if !isComputeNamespace || !isVMSSResourceType {
			c.logger.WarnContext(ctx, "Skipping non-uniform scale set ID", "scale_set_id", scaleSetID)
			continue
		}
		if id.SubscriptionID != c.subscriptionID {
			c.logger.WarnContext(ctx, "Skipping scale set ID in a different subscription", "scale_set_id", scaleSetID, "subscription_id", id.SubscriptionID)
			continue
		}
		if resourceGroup != types.Wildcard && !strings.EqualFold(id.ResourceGroupName, resourceGroup) {
			c.logger.WarnContext(ctx, "Skipping scale set ID in a different resource group", "scale_set_id", scaleSetID, "resource_group", resourceGroup)
			continue
		}

		pager := newAPIPager(
			c.networkInterfacesLister.NewListVirtualMachineScaleSetNetworkInterfacesPager(id.ResourceGroupName, id.Name, nil),
			func(resp armnetwork.InterfacesClientListVirtualMachineScaleSetNetworkInterfacesResponse) []*armnetwork.Interface {
				return resp.InterfaceListResult.Value
			},
		)
		nics, err := c.collectNICs(ctx, pager)
		if err != nil {
			c.logger.WarnContext(ctx, "Failed to list NICs from Uniform VMSS", "scale_set_id", scaleSetID, "error", err)
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
				c.logger.DebugContext(ctx, "Skipping Azure Network Interface",
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
