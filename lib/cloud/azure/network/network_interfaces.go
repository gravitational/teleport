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
	"net/http"
	"net/url"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/gravitational/trace"

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
	// PrivateIPs are the private IP addresses across all of the NIC's IP
	// configurations, in the order Azure returns them.
	PrivateIPs []string
	// PrimaryPrivateIP is the private IP address of the NIC's primary IP
	// configuration. Empty if no configuration is flagged primary.
	PrimaryPrivateIP string
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
}

// NewInterfacesClient creates a new Azure network interfaces client backed by
// the given Azure HTTP query client.
func NewInterfacesClient(c *client.Client) InterfacesClient {
	return &interfacesClient{client: c}
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

	raw, err := client.DoRequest[armNetworkInterface](ctx, c.client, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return interfaceFromARM(resourceID, raw), nil
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

func interfaceFromARM(resourceID string, raw *armNetworkInterface) *Interface {
	nic := &Interface{ID: resourceID}
	if raw == nil {
		return nic
	}
	nic.Name = raw.Name
	if raw.Properties == nil {
		return nic
	}
	for _, ipConfig := range raw.Properties.IPConfigurations {
		if ipConfig.Properties == nil || ipConfig.Properties.PrivateIPAddress == "" {
			continue
		}
		nic.PrivateIPs = append(nic.PrivateIPs, ipConfig.Properties.PrivateIPAddress)
		if ipConfig.Properties.Primary {
			nic.PrimaryPrivateIP = ipConfig.Properties.PrivateIPAddress
		}
	}
	return nic
}
