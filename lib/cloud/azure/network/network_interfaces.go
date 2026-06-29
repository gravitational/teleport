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
	"log/slog"
	"net/http"
	"net/url"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
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

// Interface represents an Azure network interface.
type Interface struct {
	// ID is the resource ID of the network interface.
	ID string
	// Name is the resource name of the network interface.
	Name string
	// PrivateIP is the private IP address of the NIC's primary IP
	// configuration. If no configuration is flagged primary, it is the first
	// private IP address found. Empty only when the NIC has no private IPs.
	PrivateIP string
}

// InterfacesClient retrieves Azure network interfaces.
type InterfacesClient interface {
	// Get returns the network interface for the given NIC resource ID. The
	// resource ID is the fully-qualified ARM ID, e.g.
	// /subscriptions/<sub>/resourceGroups/<rg>/providers/Microsoft.Network/networkInterfaces/<name>.
	Get(ctx context.Context, resourceID string) (*Interface, error)
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

// armNetworkInterface is the minimal subset of the network interface resource.
//
// See https://learn.microsoft.com/en-us/rest/api/virtualnetwork/network-interfaces/get?view=rest-virtualnetwork-2025-05-01&tabs=HTTP#get-network-interface
type armNetworkInterface struct {
	Name       string                         `json:"name,omitempty"`
	Properties *armNetworkInterfaceProperties `json:"properties,omitempty"`
}

type armNetworkInterfaceProperties struct {
	IPConfigurations []armIPConfiguration `json:"ipConfigurations,omitempty"`
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
		if firstIP == "" {
			firstIP = ip
		}
	}
	// None of the configurations were marked as primary, return the first found.
	// This would be a bug if encountered, but we don't want to fail the request
	// if it occurs.
	// See https://learn.microsoft.com/en-us/azure/virtual-network/ip-services/virtual-network-network-interface-addresses?tabs=nic-address-portal#primary
	nic.PrivateIP = firstIP
	return nic
}
