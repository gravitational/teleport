/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

// Package network provides clients for Azure network resources, built on top of
// the generic Azure HTTP query client in lib/cloud/azure/client. It avoids the
// armnetwork SDK package: a VM's private IPs live on its NIC resource, and
// pulling in armnetwork solely to read privateIPAddress would add a large
// dependency for a single field.
package network

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure/client"
)

// armEndpoint is the Azure Resource Manager endpoint for the public cloud.
// TODO(gavin): if/when we support AzureChina/AzureGovernment, this needs to be
// derived from the configured cloud.
const armEndpoint = "https://management.azure.com"

// interfaceAPIVersion is the API version for Azure Network Interfaces
//
// See https://learn.microsoft.com/en-us/rest/api/virtualnetwork/network-interfaces/get?view=rest-virtualnetwork-2025-05-01&tabs=HTTP
const interfaceAPIVersion = "2025-05-01"

// maxListPages bounds the number of pages followed when listing network
// interfaces, as a backstop against a runaway nextLink loop.
const maxListPages = 1000

// computeAPIVersion is the Microsoft.Compute API version used to enumerate VM
// Scale Sets, so uniform-orchestration sets (whose instance NICs are not in the
// flat networkInterfaces list) can be found and queried separately.
//
// See https://learn.microsoft.com/en-us/rest/api/compute/virtual-machine-scale-sets/list-all?view=rest-compute-2026-03-02&tabs=HTTP
const computeAPIVersion = "2026-03-01"

// Interface represents an Azure network interface.
type Interface struct {
	// ID is the resource ID of the network interface.
	ID string
	// Name is the resource name of the network interface.
	Name string
	// PrivateIP is the private IP address of the NIC's primary IP
	// configuration. If no configuration is flagged primary, it is the first
	// full private IP address found; CIDR blocks from secondary IP prefix
	// configurations are skipped. Empty only when the NIC has no usable
	// private IP.
	PrivateIP string
}

// InterfacesClient retrieves Azure network interfaces.
type InterfacesClient interface {
	// Get returns the network interface for the given NIC resource ID. The
	// resource ID is the fully-qualified ARM ID, e.g.
	// /subscriptions/<sub>/resourceGroups/<rg>/providers/Microsoft.Network/networkInterfaces/<name>.
	Get(ctx context.Context, resourceID string) (*Interface, error)
	// List returns the network interfaces in the subscription, grouped by the
	// lower-cased resource ID of the VM each NIC is attached to. Pass the
	// wildcard "*" as the resource group to list the whole subscription. NICs
	// that are not attached to a VM are omitted.
	List(ctx context.Context, subscriptionID, resourceGroup string) (map[string][]*Interface, error)
}

type interfacesClient struct {
	// client is the generic Azure ARM HTTP query client. It handles AAD auth,
	// response decoding, error conversion, and rate-limit retries for us.
	client *client.Client
	logger *slog.Logger
}

// Option configures an InterfacesClient during construction.
type Option func(*interfacesClient)

// WithLogger sets the logger used by the client. Defaults to slog.Default().
func WithLogger(logger *slog.Logger) Option {
	return func(c *interfacesClient) {
		c.logger = logger
	}
}

// NewInterfacesClient creates a new Azure network interfaces client backed by
// the given Azure HTTP query client.
func NewInterfacesClient(c *client.Client, opts ...Option) InterfacesClient {
	nic := &interfacesClient{
		client: c,
		logger: slog.Default().With(teleport.ComponentKey, "azure_network_interfaces_client"),
	}
	for _, opt := range opts {
		opt(nic)
	}
	return nic
}

// Get returns the network interface for the given NIC resource ID.
func (c *interfacesClient) Get(ctx context.Context, resourceID string) (*Interface, error) {
	if _, err := arm.ParseResourceID(resourceID); err != nil {
		return nil, trace.BadParameter("failed to parse network interface resource ID %q: %v", resourceID, err)
	}

	// Endpoint looks like:
	// GET https://management.azure.com/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Network/networkInterfaces/{networkInterfaceName}
	endpoint, err := url.JoinPath(armEndpoint, resourceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	query := req.URL.Query()
	query.Set("api-version", interfaceAPIVersion)
	req.URL.RawQuery = query.Encode()

	c.logger.DebugContext(ctx, "fetching Azure network interface", "resource_id", resourceID)

	raw, err := client.DoRequest[armNetworkInterface](ctx, c.client, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	nic := interfaceFromARM(resourceID, raw)
	c.logger.DebugContext(ctx, "fetched Azure network interface",
		"resource_id", nic.ID,
		"name", nic.Name,
		"private_ip", nic.PrivateIP,
	)
	return nic, nil
}

// List returns the network interfaces in the subscription, grouped by the
// resource ID of the virtual machine each NIC is attached to. NICs that are not
// attached to a virtual machine are omitted. resourceGroup may be a wildcard.
//
// Keys are lower-cased VM resource IDs. Azure is not always consistent about
// the casing of the resource group segment across the compute and network APIs,
// so look up with strings.ToLower(vm.ID).
func (c *interfacesClient) List(ctx context.Context, subscriptionID, resourceGroup string) (map[string][]*Interface, error) {
	if subscriptionID == "" {
		return nil, trace.BadParameter("subscription ID is required to list network interfaces")
	}

	c.logger.DebugContext(ctx, "listing Azure network interfaces",
		"subscription_id", subscriptionID,
		"resource_group", resourceGroup,
	)

	// Standalone VMs and flexible VMSS instances.
	regularNICsByVM, flatErr := c.listStandaloneAndFlexibleNICs(ctx, subscriptionID, resourceGroup)
	// Uniform VMSS instances, whose NICs are absent from the flat list above.
	uniformNICsByVM, uniformErr := c.listUniformVMSSNICs(ctx, subscriptionID, resourceGroup)

	switch {
	case flatErr != nil && uniformErr != nil:
		return nil, trace.NewAggregate(flatErr, uniformErr)
	case flatErr != nil:
		c.logger.WarnContext(ctx, "failed to list standalone and flexible VMSS network interfaces, continuing with uniform VMSS NICs",
			"error", flatErr,
			"subscription_id", subscriptionID,
			"resource_group", resourceGroup,
			"found_nics", len(uniformNICsByVM),
		)
		return uniformNICsByVM, nil
	case uniformErr != nil:
		c.logger.WarnContext(ctx, "failed to list uniform VMSS network interfaces, continuing with standalone and flexible VMSS NICs",
			"error", uniformErr,
			"subscription_id", subscriptionID,
			"resource_group", resourceGroup,
			"found_nics", len(regularNICsByVM),
		)
		return regularNICsByVM, nil
	default:
		for vmID, nics := range uniformNICsByVM {
			regularNICsByVM[vmID] = append(regularNICsByVM[vmID], nics...)
		}

		c.logger.DebugContext(ctx, "listed Azure network interfaces",
			"subscription_id", subscriptionID,
			"resource_group", resourceGroup,
			"found_nics", len(regularNICsByVM),
		)

		return regularNICsByVM, nil
	}
}

// listStandaloneAndFlexibleNICs lists NICs via the regular networkInterfaces API,
// which returns NICs for standalone VMs and flexible VMSS instances.
func (c *interfacesClient) listStandaloneAndFlexibleNICs(ctx context.Context, subscriptionID, resourceGroup string) (map[string][]*Interface, error) {
	first, err := buildURL(
		scopePath(subscriptionID, resourceGroup, "/providers/Microsoft.Network/networkInterfaces"),
		interfaceAPIVersion,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	nicsByVM := make(map[string][]*Interface)
	if err := c.collectNICsByVM(ctx, first, nicsByVM); err != nil {
		return nil, trace.Wrap(err)
	}
	return nicsByVM, nil
}

// listUniformVMSSNICs lists NICs for uniform VM Scale Set instances. Their NICs
// are absent from the regular networkInterfaces list.
//
// Flexible sets are skipped because their instances' NICs are in the regular
// list. A failure to list a single scale set's NICs is logged and skipped
// rather than failing the whole call.
func (c *interfacesClient) listUniformVMSSNICs(
	ctx context.Context,
	subscriptionID,
	resourceGroup string,
) (map[string][]*Interface, error) {
	scaleSets, err := c.listUniformScaleSets(ctx, subscriptionID, resourceGroup)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	nicsByVM := make(map[string][]*Interface)
	for _, scaleSet := range scaleSets {
		first, err := buildURL(scaleSet.ID+"/networkInterfaces", computeAPIVersion)
		if err != nil {
			c.logger.DebugContext(ctx, "skipping VM Scale Set with an unparseable ID", "scale_set_id", scaleSet.ID, "error", err)
			continue
		}
		if err := c.collectNICsByVM(ctx, first, nicsByVM); err != nil {
			c.logger.WarnContext(ctx, "failed to list network interfaces for VM Scale Set", "scale_set_id", scaleSet.ID, "error", err)
			continue
		}
	}
	return nicsByVM, nil
}

// listUniformScaleSets enumerates VM Scale Sets in the scope and returns only
// those with uniform orchestration. Flexible sets' instances appear in the flat
// networkInterfaces list and are handled there.
func (c *interfacesClient) listUniformScaleSets(
	ctx context.Context,
	subscriptionID,
	resourceGroup string,
) ([]armScaleSet, error) {
	first, err := buildURL(
		scopePath(subscriptionID, resourceGroup, "/providers/Microsoft.Compute/virtualMachineScaleSets"),
		computeAPIVersion,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var uniform []armScaleSet
	pages := 0
	for next := first; next != ""; {
		if pages >= maxListPages {
			return nil, trace.LimitExceeded("listing Azure VM Scale Sets exceeded the maximum of %d pages", maxListPages)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		page, err := client.DoRequest[armScaleSetList](ctx, c.client, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		pages++

		for _, scaleSet := range page.Value {
			if scaleSet.isFlexible() {
				continue
			}
			uniform = append(uniform, scaleSet)
		}
		next = page.NextLink
	}
	return uniform, nil
}

// collectNICsByVM pages through a network-interfaces list beginning at
// firstURL, adding each VM-attached NIC to dst keyed by the lower-cased
// resource ID of the VM it is attached to.
func (c *interfacesClient) collectNICsByVM(ctx context.Context, firstURL string, dst map[string][]*Interface) error {
	pages := 0
	for next := firstURL; next != ""; {
		if pages >= maxListPages {
			return trace.LimitExceeded("listing Azure network interfaces exceeded the maximum of %d pages", maxListPages)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		if err != nil {
			return trace.Wrap(err)
		}

		page, err := client.DoRequest[armNetworkInterfaceList](ctx, c.client, req)
		if err != nil {
			return trace.Wrap(err)
		}
		pages++

		for i := range page.Value {
			raw := &page.Value[i]
			vmID := raw.virtualMachineID()
			if vmID == "" {
				// NICs not attached to a VM are skipped.
				continue
			}
			key := strings.ToLower(vmID)
			dst[key] = append(dst[key], interfaceFromARM(raw.ID, raw))
		}
		next = page.NextLink
	}
	return nil
}

// buildURL joins the ARM endpoint with path and sets the api-version query.
func buildURL(path, apiVersion string) (string, error) {
	endpoint, err := url.JoinPath(armEndpoint, path)
	if err != nil {
		return "", trace.Wrap(err)
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", trace.Wrap(err)
	}
	query := u.Query()
	query.Set("api-version", apiVersion)
	u.RawQuery = query.Encode()
	return u.String(), nil
}

// scopePath builds a subscription- or resource-group-scoped path with the given
// provider/type suffix. A wildcard resource group scopes to the whole subscription.
func scopePath(subscriptionID, resourceGroup, providerPath string) string {
	path := fmt.Sprintf("/subscriptions/%s", subscriptionID)
	if resourceGroup != types.Wildcard {
		path += fmt.Sprintf("/resourceGroups/%s", resourceGroup)
	}
	return path + providerPath
}

// armNetworkInterface is the minimal subset of the network interface resource.
//
// See https://learn.microsoft.com/en-us/rest/api/virtualnetwork/network-interfaces/get?view=rest-virtualnetwork-2025-05-01&tabs=HTTP#get-network-interface
type armNetworkInterface struct {
	ID         string                         `json:"id,omitempty"`
	Name       string                         `json:"name,omitempty"`
	Properties *armNetworkInterfaceProperties `json:"properties,omitempty"`
}

// virtualMachineID returns the resource ID of the VM this NIC is attached to,
// or "" if it isn't attached to one.
func (n *armNetworkInterface) virtualMachineID() string {
	if n.Properties == nil || n.Properties.VirtualMachine == nil {
		return ""
	}
	return n.Properties.VirtualMachine.ID
}

// armNetworkInterfaceList is the paged response from the network interfaces
// list API.
type armNetworkInterfaceList struct {
	Value    []armNetworkInterface `json:"value,omitempty"`
	NextLink string                `json:"nextLink,omitempty"`
}

type armNetworkInterfaceProperties struct {
	IPConfigurations []armIPConfiguration `json:"ipConfigurations,omitempty"`
	// VirtualMachine references the VM the NIC is attached to, if any.
	VirtualMachine *armSubResource `json:"virtualMachine,omitempty"`
}

// armSubResource is an ARM reference to another resource by ID.
type armSubResource struct {
	ID string `json:"id,omitempty"`
}

// armScaleSet is the minimal subset of a VM Scale Set resource: enough to find
// its ID and orchestration mode.
type armScaleSet struct {
	ID         string                 `json:"id,omitempty"`
	Name       string                 `json:"name,omitempty"`
	Properties *armScaleSetProperties `json:"properties,omitempty"`
}

type armScaleSetProperties struct {
	// OrchestrationMode is "Uniform" or "Flexible".
	OrchestrationMode string `json:"orchestrationMode,omitempty"`
}

// armScaleSetList is the paged response from the VM Scale Sets list API.
type armScaleSetList struct {
	Value    []armScaleSet `json:"value,omitempty"`
	NextLink string        `json:"nextLink,omitempty"`
}

// isFlexible reports whether the scale set uses flexible orchestration, whose
// instances' NICs are returned by the flat networkInterfaces list.
func (s *armScaleSet) isFlexible() bool {
	return s.Properties != nil && strings.EqualFold(s.Properties.OrchestrationMode, string(armcompute.OrchestrationModeFlexible))
}

type armIPConfiguration struct {
	Properties *armIPConfigurationProperties `json:"properties,omitempty"`
}

type armIPConfigurationProperties struct {
	PrivateIPAddress string `json:"privateIPAddress,omitempty"`
	// Primary indicates whether this is the NIC's primary IP configuration.
	// A NIC can have multiple IP configurations.
	Primary bool `json:"primary,omitempty"`
}

// interfaceFromARM converts an ARM network interface resource into a simplified
// Interface. If the IP configuration has a primary IP address, that will be
// used, otherwise the first IP address found will be used.
func interfaceFromARM(resourceID string, raw *armNetworkInterface) *Interface {
	nic := &Interface{ID: resourceID}
	if raw == nil {
		return nic
	}
	nic.Name = raw.Name
	if raw.Properties == nil {
		return nic
	}
	var firstIP string
	for _, ipConfig := range raw.Properties.IPConfigurations {
		if ipConfig.Properties == nil || ipConfig.Properties.PrivateIPAddress == "" {
			continue
		}
		ip := ipConfig.Properties.PrivateIPAddress
		// If we've found the primary IP config, return it
		if ipConfig.Properties.Primary {
			nic.PrivateIP = ip
			return nic
		}
		// We use the first IP found as a fallback if no primary ip config is marked.
		// The isIPAddress check is to skip secondary IP configs that have a CIDR
		// block rather than a full IP address.
		// See https://learn.microsoft.com/en-us/azure/virtual-network/ip-services/virtual-network-private-ip-address-blocks-portal
		if firstIP == "" && isIPAddress(ip) {
			firstIP = ip
		}
	}
	// None of the configurations were marked as primary, return the first full IP
	// found. This would be an Azure misconfiguration if encountered, but we don't
	// want to fail the request if it occurs.
	// See https://learn.microsoft.com/en-us/azure/virtual-network/ip-services/virtual-network-network-interface-addresses?tabs=nic-address-portal#primary
	nic.PrivateIP = firstIP
	return nic
}

// isIPAddress reports whether s is an IP address rather than a CIDR block.
func isIPAddress(s string) bool {
	_, err := netip.ParseAddr(s)
	return err == nil
}
