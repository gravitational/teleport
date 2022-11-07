// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v3"
	"github.com/gravitational/trace"
)

// armCompute provides an interface for an Azure Virtual Machine client.
type armCompute interface {
	// Get retrieves information about an Azure Virtual Machine.
	Get(ctx context.Context, resourceGroupName string, vmName string, options *armcompute.VirtualMachinesClientGetOptions) (armcompute.VirtualMachinesClientGetResponse, error)
	// NewListPagers lists Azure Virtual Machines.
	NewListPager(resourceGroup string, opts *armcompute.VirtualMachinesClientListOptions) *runtime.Pager[armcompute.VirtualMachinesClientListResponse]
}

// VirtualMachinesClient is a client for Azure Virtual Machines.
type VirtualMachinesClient interface {
	// Get returns the Virtual Machine for the given resource ID.
	Get(ctx context.Context, resourceID string) (*VirtualMachine, error)
	// ListVirtualMachines gets all of the virtual machines in the given resource group.
	ListVirtualMachines(ctx context.Context, resourceGroup string) ([]*armcompute.VirtualMachine, error)
}

// VirtualMachine represents an Azure Virtual Machine.
type VirtualMachine struct {
	// ID resource ID.
	ID string `json:"id,omitempty"`
	// Name resource name.
	Name string `json:"name,omitempty"`
	// Identities are the identities associated with the resource.
	Identities []Identity
}

// Identitiy represents an Azure Virtual Machine identity.
type Identity struct {
	// ResourceID the identity resource ID.
	ResourceID string
}

type vmClient struct {
	// api is the Azure Virtual Machine client.
	api armCompute
}

// NewVirtualMachinesClient creates a new Azure Virtual Machines client by
// subscription and credentials.
func NewVirtualMachinesClient(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (VirtualMachinesClient, error) {
	computeAPI, err := armcompute.NewVirtualMachinesClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewVirtualMachinesClientByAPI(computeAPI), nil
}

// NewVirtualMachinesClientByAPI creates a new Azure Virtual Machines client by
// ARM API client.
func NewVirtualMachinesClientByAPI(api armCompute) VirtualMachinesClient {
	return &vmClient{
		api: api,
	}
}

// Get returns the Virtual Machine for the given resource ID.
func (c *vmClient) Get(ctx context.Context, resourceID string) (*VirtualMachine, error) {
	parsedResourceID, err := arm.ParseResourceID(resourceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := c.api.Get(ctx, parsedResourceID.ResourceGroupName, parsedResourceID.Name, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var identities []Identity
	if resp.Identity != nil {
		identities = append(identities, Identity{ResourceID: *resp.Identity.PrincipalID})
		for identityID := range resp.Identity.UserAssignedIdentities {
			identities = append(identities, Identity{ResourceID: identityID})
		}
	}

	return &VirtualMachine{
		ID:         *resp.ID,
		Name:       *resp.Name,
		Identities: identities,
	}, nil
}

// ListVirtualMachines lists all virtual machines in a given resource group using the Azure Virtual Machines API.
func (c *vmClient) ListVirtualMachines(ctx context.Context, resourceGroup string) ([]*armcompute.VirtualMachine, error) {
	pagerOpts := &armcompute.VirtualMachinesClientListOptions{}
	pager := c.api.NewListPager(resourceGroup, pagerOpts)
	var virtualMachines []*armcompute.VirtualMachine
	for pager.More() {
		res, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		virtualMachines = append(virtualMachines, res.Value...)
	}

	return virtualMachines, nil
}
