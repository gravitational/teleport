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
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
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
	// GetByVMID returns the Virtual Machine for a given VM ID.
	GetByVMID(ctx context.Context, resourceGroup, vmID string) (*VirtualMachine, error)
	// ListVirtualMachines gets all of the virtual machines in the given resource group.
	ListVirtualMachines(ctx context.Context, resourceGroup string) ([]*armcompute.VirtualMachine, error)
}

// VirtualMachine represents an Azure Virtual Machine.
type VirtualMachine struct {
	// ID resource ID.
	ID string `json:"id,omitempty"`
	// Name resource name.
	Name string `json:"name,omitempty"`
	// Subscription is the Azure subscription the VM is in.
	Subscription string
	// ResourceGroup is the resource group the VM is in.
	ResourceGroup string
	// VMID is the VM's ID.
	VMID string
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

func parseVirtualMachine(vm *armcompute.VirtualMachine) (*VirtualMachine, error) {
	resourceID, err := arm.ParseResourceID(*vm.ID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var identities []Identity
	if vm.Identity != nil {
		if systemAssigned := StringVal(vm.Identity.PrincipalID); systemAssigned != "" {
			identities = append(identities, Identity{ResourceID: systemAssigned})
		}

		for identityID := range vm.Identity.UserAssignedIdentities {
			identities = append(identities, Identity{ResourceID: identityID})
		}
	}

	var vmID string
	if vm.Properties != nil {
		vmID = *vm.Properties.VMID
	}

	return &VirtualMachine{
		ID:            *vm.ID,
		Name:          *vm.Name,
		Subscription:  resourceID.SubscriptionID,
		ResourceGroup: resourceID.ResourceGroupName,
		VMID:          vmID,
		Identities:    identities,
	}, nil
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

	vm, err := parseVirtualMachine(&resp.VirtualMachine)
	return vm, trace.Wrap(err)
}

// GetByVMID returns the Virtual Machine for a given VM ID.
func (c *vmClient) GetByVMID(ctx context.Context, resourceGroup, vmID string) (*VirtualMachine, error) {
	vms, err := c.ListVirtualMachines(ctx, resourceGroup)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, vm := range vms {
		if vm.Properties != nil && *vm.Properties.VMID == vmID {
			result, err := parseVirtualMachine(vm)
			return result, trace.Wrap(err)
		}
	}
	return nil, trace.NotFound("no VM with ID %q", vmID)
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

// RunCommandRequest combines parameters for running a command on an Azure virtual machine.
type RunCommandRequest struct {
	// Region is the region of the VM.
	Region string
	// ResourceGroup is the resource group for the VM.
	ResourceGroup string
	// VMName is the name of the VM.
	VMName string
	// Script is the URI of the script for the virtual machine to execute.
	Script string
	// Parameters is a list of parameters for the script.
	Parameters []string
}

// RunCommandClient is a client for Azure Run Commands.
type RunCommandClient interface {
	Run(ctx context.Context, req RunCommandRequest) error
}

type runCommandClient struct {
	api *armcompute.VirtualMachineRunCommandsClient
}

// NewRunCommandClient creates a new Azure Run Command client by subscription
// and credentials.
func NewRunCommandClient(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (RunCommandClient, error) {
	runCommandAPI, err := armcompute.NewVirtualMachineRunCommandsClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &runCommandClient{
		api: runCommandAPI,
	}, nil
}

// Run runs a command on a virtual machine.
func (c *runCommandClient) Run(ctx context.Context, req RunCommandRequest) error {
	var params []*armcompute.RunCommandInputParameter
	for _, value := range req.Parameters {
		params = append(params, &armcompute.RunCommandInputParameter{
			Value: to.Ptr(value),
		})
	}
	poller, err := c.api.BeginCreateOrUpdate(ctx, req.ResourceGroup, req.VMName, "RunShellScript", armcompute.VirtualMachineRunCommand{
		Location: to.Ptr(req.Region),
		Properties: &armcompute.VirtualMachineRunCommandProperties{
			AsyncExecution: to.Ptr(false),
			Parameters:     params,
			Source: &armcompute.VirtualMachineRunCommandScriptSource{
				Script: to.Ptr(req.Script),
			},
		},
	}, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = poller.PollUntilDone(ctx, nil /* options */)
	return trace.Wrap(err)
}
