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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/gravitational/trace"
)

// ARMVirtualMachines provides an interface for armcompute.VirtualMachinesClient.
// It is provided so that the client can be mocked.
type ARMVirtualMachines interface {
	NewListPager(resourceGroup string, opts *armcompute.VirtualMachinesClientListOptions) *runtime.Pager[armcompute.VirtualMachinesClientListResponse]
}

// VirtualMachinesClient wraps the Azure VirtualMachines API to fetch virtual machines.
type VirtualMachinesClient struct {
	api ARMVirtualMachines
}

// NewVirtualMachinesClient creates a new Azure virtual machines client by subscription and credentials.
func NewVirtualMachinesClient(subscription string, cred azcore.TokenCredential, opts *arm.ClientOptions) (*VirtualMachinesClient, error) {
	armClient, err := armcompute.NewVirtualMachinesClient(subscription, cred, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewVirtualMachinesClientByAPI(armClient), nil
}

// NewVirtualMachinesClientByAPI creates anAzure virtual machines client with an existing ARM API client.
func NewVirtualMachinesClientByAPI(api ARMVirtualMachines) *VirtualMachinesClient {
	return &VirtualMachinesClient{api: api}
}

// ListVirtualMachines lists all virtual machines in a given resource group using the Azure Virtual Machines API.
func (c *VirtualMachinesClient) ListVirtualMachines(ctx context.Context, resourceGroup string) ([]*armcompute.VirtualMachine, error) {
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
