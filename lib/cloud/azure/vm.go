/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package azure

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

const (
	// virtualScaleSetUniformVMResourceType represents the resource type of uniform
	// virtual scale set VMs.
	virtualScaleSetUniformVMResourceType = "virtualMachineScaleSets/virtualMachines"

	// VirtualMachinesResourceType is the resource type for Azure Virtual Machine Scale Sets.
	VirtualMachineScaleSetsResourceType = "Microsoft.Compute/virtualMachineScaleSets"
)

// virtualMachinesAPI provides an interface for an Azure virtual machine client.
type virtualMachinesAPI interface {
	// Get retrieves information about an Azure virtual machine.
	Get(ctx context.Context, resourceGroupName string, vmName string, options *armcompute.VirtualMachinesClientGetOptions) (armcompute.VirtualMachinesClientGetResponse, error)
	// NewListPager lists Azure virtual Machines.
	NewListPager(resourceGroup string, opts *armcompute.VirtualMachinesClientListOptions) *runtime.Pager[armcompute.VirtualMachinesClientListResponse]
	// NewListAllPager lists Azure virtual machines in any resource group.
	NewListAllPager(opts *armcompute.VirtualMachinesClientListAllOptions) *runtime.Pager[armcompute.VirtualMachinesClientListAllResponse]
}

// scaleSetsAPI provides an interface for an Azure VM scale set client.
type scaleSetsAPI interface {
	// NewListAllPager gets a list of all VM Scale Sets in the subscription, regardless of the associated resource group.
	NewListAllPager(options *armcompute.VirtualMachineScaleSetsClientListAllOptions) *runtime.Pager[armcompute.VirtualMachineScaleSetsClientListAllResponse]
	// NewListPager gets a list of all VM scale sets under a resource group.
	NewListPager(resourceGroupName string, options *armcompute.VirtualMachineScaleSetsClientListOptions) *runtime.Pager[armcompute.VirtualMachineScaleSetsClientListResponse]
}

// scaleSetVMsAPI provides an interface for an Azure VM scale set VMs client.
type scaleSetVMsAPI interface {
	// Get retrieves a virtual machine from a VM scale set.
	Get(ctx context.Context, resourceGroupName string, vmScaleSetName string, instanceID string, options *armcompute.VirtualMachineScaleSetVMsClientGetOptions) (armcompute.VirtualMachineScaleSetVMsClientGetResponse, error)
	// NewListPager gets a list of all virtual machines in a VM scale sets.
	NewListPager(resourceGroupName string, virtualMachineScaleSetName string, options *armcompute.VirtualMachineScaleSetVMsClientListOptions) *runtime.Pager[armcompute.VirtualMachineScaleSetVMsClientListResponse]
}

// VirtualMachinesClient is a client for Azure virtual machines.
type VirtualMachinesClient interface {
	// Get returns the virtual machine (including scale set VMs) for the given
	// resource ID.
	Get(ctx context.Context, resourceID string) (*VirtualMachine, error)
	// GetByVMID returns the virtual machine for a given VM ID.
	GetByVMID(ctx context.Context, vmID string) (*VirtualMachine, error)
	// ListVirtualMachines gets all of the virtual machines in the given resource group.
	ListVirtualMachines(ctx context.Context, resourceGroup string) ([]*armcompute.VirtualMachine, error)
	// ListVirtualMachinesFromUniformVMSS gets all of the virtual machines in the given resource group from all uniform VM Scale Sets.
	ListVirtualMachinesFromUniformVMSS(ctx context.Context, resourceGroup string) ([]*armcompute.VirtualMachineScaleSetVM, error)
}

// VirtualMachine represents an Azure virtual machine.
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

// Identity represents an Azure virtual machine identity.
type Identity struct {
	// ResourceID the identity resource ID.
	ResourceID string
}

type vmClient struct {
	// vmAPI is the Azure virtual machine client.
	vmAPI virtualMachinesAPI
	// scaleSetVMsAPI is the Azure VM scale set VMs client.
	scaleSetVMsAPI scaleSetVMsAPI
	// scaleSetsAPI is the Azure VM scale set client.
	scaleSetsAPI scaleSetsAPI

	logger *slog.Logger
}

// NewVirtualMachinesClient creates a new Azure virtual machines client by
// subscription and credentials.
func NewVirtualMachinesClient(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (VirtualMachinesClient, error) {
	computeAPI, err := armcompute.NewVirtualMachinesClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	scaleSetVMsAPI, err := armcompute.NewVirtualMachineScaleSetVMsClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	scaleSetAPI, err := armcompute.NewVirtualMachineScaleSetsClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := VirtualMachinesClientConfig{
		VirtualMachineAPI: computeAPI,
		ScaleSetsAPI:      scaleSetAPI,
		ScaleSetVMsAPI:    scaleSetVMsAPI,
	}
	return NewVirtualMachinesClientByAPI(config), nil
}

// VirtualMachinesClientConfig combines dependencies for creating a VirtualMachinesClient.
type VirtualMachinesClientConfig struct {
	VirtualMachineAPI virtualMachinesAPI
	ScaleSetsAPI      scaleSetsAPI
	ScaleSetVMsAPI    scaleSetVMsAPI
	Logger            *slog.Logger
}

// NewVirtualMachinesClientByAPI creates a new Azure virtual machines client by
// ARM API client.
func NewVirtualMachinesClientByAPI(config VirtualMachinesClientConfig) VirtualMachinesClient {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &vmClient{
		vmAPI:          config.VirtualMachineAPI,
		scaleSetsAPI:   config.ScaleSetsAPI,
		scaleSetVMsAPI: config.ScaleSetVMsAPI,
		logger:         logger,
	}
}

type vmTypes interface {
	*armcompute.VirtualMachine | *armcompute.VirtualMachineScaleSetVM
}

func parseVirtualMachine[T vmTypes](vm T) (*VirtualMachine, error) {
	var (
		id       string
		name     string
		identity *armcompute.VirtualMachineIdentity
		vmID     *string
	)

	switch v := any(vm).(type) {
	case *armcompute.VirtualMachine:
		id = *v.ID
		name = *v.Name
		identity = v.Identity
		if v.Properties != nil {
			vmID = v.Properties.VMID
		}

	case *armcompute.VirtualMachineScaleSetVM:
		id = *v.ID
		name = *v.Name
		identity = v.Identity
		if v.Properties != nil {
			vmID = v.Properties.VMID
		}
	}

	resourceID, err := arm.ParseResourceID(id)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var identities []Identity
	if identity != nil {
		if systemAssigned := StringVal(identity.PrincipalID); systemAssigned != "" {
			identities = append(identities, Identity{ResourceID: systemAssigned})
		}

		for identityID := range identity.UserAssignedIdentities {
			identities = append(identities, Identity{ResourceID: identityID})
		}
	}

	return &VirtualMachine{
		ID:            id,
		Name:          name,
		Subscription:  resourceID.SubscriptionID,
		ResourceGroup: resourceID.ResourceGroupName,
		VMID:          StringVal(vmID),
		Identities:    identities,
	}, nil
}

// Get returns the virtual machine (including scale set VMs) for the given
// resource ID.
//
// The virtual machine scale set (VMSS) supports two types of orchestration
// modes: uniform and flexible. Both have different resource ID format from the
// instance metadata API. A VM from a uniform VMSS has a different resource ID
// and requires a different API to retrieve its information. Flexible VMSS VMs
// use the same resource ID format as regular VMs and don't require special
// handling.
func (c *vmClient) Get(ctx context.Context, resourceID string) (*VirtualMachine, error) {
	parsedResourceID, err := arm.ParseResourceID(resourceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if parsedResourceID.ResourceType.Type == virtualScaleSetUniformVMResourceType {
		return c.getScaleSetVM(ctx, parsedResourceID)
	}

	resp, err := c.vmAPI.Get(ctx, parsedResourceID.ResourceGroupName, parsedResourceID.Name, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	vm, err := parseVirtualMachine(&resp.VirtualMachine)
	return vm, trace.Wrap(err)
}

// GetByVMID returns the virtual machine for a given VM ID.
func (c *vmClient) GetByVMID(ctx context.Context, vmID string) (*VirtualMachine, error) {
	pager := pagerForListingAllVirtualMachines(c.vmAPI)
	for pager.more() {
		res, err := pager.nextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}

		for _, vm := range res {
			if vm.Properties != nil && *vm.Properties.VMID == vmID {
				result, err := parseVirtualMachine(vm)
				return result, trace.Wrap(err)
			}
		}
	}
	return nil, trace.NotFound("no VM with ID %q", vmID)
}

func (c *vmClient) getScaleSetVM(ctx context.Context, resourceID *arm.ResourceID) (*VirtualMachine, error) {
	if resourceID.Parent == nil {
		return nil, trace.BadParameter("expected resource ID to include scale set as parent resource")
	}

	resp, err := c.scaleSetVMsAPI.Get(ctx, resourceID.ResourceGroupName, resourceID.Parent.Name, resourceID.Name, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := parseVirtualMachine(&resp.VirtualMachineScaleSetVM)
	return result, trace.Wrap(err)
}

type apiPager[T any] struct {
	more     func() bool
	nextPage func(context.Context) ([]*T, error)
}

// newPager wraps an Azure SDK pager into a apiPager. The values function
// extracts the slice of *T from each response page; this keeps the wrapper
// generic over the response type R while remaining type-safe at compile time.
func newAPIPager[T, R any](p *runtime.Pager[R], values func(R) []*T) apiPager[T] {
	return apiPager[T]{
		more: p.More,
		nextPage: func(ctx context.Context) ([]*T, error) {
			res, err := p.NextPage(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return values(res), nil
		},
	}
}

func pagerForListingAllVirtualMachines(api virtualMachinesAPI) apiPager[armcompute.VirtualMachine] {
	return newAPIPager(
		api.NewListAllPager(&armcompute.VirtualMachinesClientListAllOptions{}),
		func(resp armcompute.VirtualMachinesClientListAllResponse) []*armcompute.VirtualMachine {
			return resp.Value
		},
	)
}

func pagerForListingVirtualMachines(api virtualMachinesAPI, resourceGroup string) apiPager[armcompute.VirtualMachine] {
	return newAPIPager(
		api.NewListPager(resourceGroup, &armcompute.VirtualMachinesClientListOptions{}),
		func(resp armcompute.VirtualMachinesClientListResponse) []*armcompute.VirtualMachine {
			return resp.Value
		},
	)
}

func pagerForListingAllScaleSets(api scaleSetsAPI) apiPager[armcompute.VirtualMachineScaleSet] {
	return newAPIPager(
		api.NewListAllPager(&armcompute.VirtualMachineScaleSetsClientListAllOptions{}),
		func(resp armcompute.VirtualMachineScaleSetsClientListAllResponse) []*armcompute.VirtualMachineScaleSet {
			return resp.Value
		},
	)
}

func pagerForListingScaleSets(api scaleSetsAPI, resourceGroup string) apiPager[armcompute.VirtualMachineScaleSet] {
	return newAPIPager(
		api.NewListPager(resourceGroup, &armcompute.VirtualMachineScaleSetsClientListOptions{}),
		func(resp armcompute.VirtualMachineScaleSetsClientListResponse) []*armcompute.VirtualMachineScaleSet {
			return resp.Value
		},
	)
}

// ListVirtualMachines lists all virtual machines in a given resource group
// using the Azure virtual machines API. If resourceGroup is "*", it lists
// all virtual machines in any resource group.
// This method returns regular VMs and VMs from flexible VM Scale Sets. For VMs from uniform VM Scale Sets, use ListVirtualMachinesFromUniformVMSS.
func (c *vmClient) ListVirtualMachines(ctx context.Context, resourceGroup string) ([]*armcompute.VirtualMachine, error) {
	var pager apiPager[armcompute.VirtualMachine]
	if resourceGroup == types.Wildcard {
		pager = pagerForListingAllVirtualMachines(c.vmAPI)
	} else {
		pager = pagerForListingVirtualMachines(c.vmAPI, resourceGroup)
	}
	var virtualMachines []*armcompute.VirtualMachine
	for pager.more() {
		res, err := pager.nextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		virtualMachines = append(virtualMachines, res...)
	}

	return virtualMachines, nil
}

// There are two orchestration modes: uniform and flexible.
// Uniform was created before flexible, so we treat any missing orchestration mode as uniform for backward compatibility.
func vmScaleSetIsFlexible(vmScaleSetProperties *armcompute.VirtualMachineScaleSetProperties) bool {
	return vmScaleSetProperties != nil && StringVal(vmScaleSetProperties.OrchestrationMode) == string(armcompute.OrchestrationModeFlexible)
}

// ListVirtualMachinesFromUniformVMSS lists virtual machines in all VM Scale Sets with uniform orchestration mode, optionally filtered by resource group.
// For listing VMs in Flexible VMSS or regular VMs, use ListVirtualMachines.
func (c *vmClient) ListVirtualMachinesFromUniformVMSS(ctx context.Context, resourceGroup string) ([]*armcompute.VirtualMachineScaleSetVM, error) {
	var pager apiPager[armcompute.VirtualMachineScaleSet]
	if resourceGroup == types.Wildcard {
		pager = pagerForListingAllScaleSets(c.scaleSetsAPI)
	} else {
		pager = pagerForListingScaleSets(c.scaleSetsAPI, resourceGroup)
	}

	var virtualMachines []*armcompute.VirtualMachineScaleSetVM

	for pager.more() {
		scaleSetsPage, err := pager.nextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}

		for _, scaleSet := range scaleSetsPage {
			// Skip Scale Set VMs whose orchestration mode is Flexible, because those are returned in the regular List Virtual Machines API.
			if vmScaleSetIsFlexible(scaleSet.Properties) {
				continue
			}

			scaleSetName := StringVal(scaleSet.Name)

			vmssResourceGroup := resourceGroup
			if vmssResourceGroup == types.Wildcard {
				scaleSetResourceID, err := arm.ParseResourceID(StringVal(scaleSet.ID))
				if err != nil {
					c.logger.WarnContext(ctx, "Azure Scale Set ID is not a valid resource ID", "scale_set_id", StringVal(scaleSet.ID), "error", err)
					continue
				}
				vmssResourceGroup = scaleSetResourceID.ResourceGroupName
			}

			scaleSetVMsPager := c.scaleSetVMsAPI.NewListPager(vmssResourceGroup, scaleSetName, nil)
			for scaleSetVMsPager.More() {
				scaleSetVMsPage, err := scaleSetVMsPager.NextPage(ctx)
				if err != nil {
					return nil, trace.Wrap(ConvertResponseError(err))
				}
				virtualMachines = append(virtualMachines, scaleSetVMsPage.Value...)
			}
		}
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
	// ScaleSetName is the name of the Scale Set this VM belongs too.
	// Only used when the VM is part of a uniform VM Scale Set.
	// Regular VMs and VMs from flexible VM Scale Sets should leave this field empty.
	ScaleSetName string
	// ScaleSetVMInstanceID is the instance ID of this instance in the uniform Scale Set VM.
	// Only used when the VM is part of a uniform VM Scale Set.
	// Regular VMs and VMs from flexible VM Scale Sets should leave this field empty.
	ScaleSetVMInstanceID string
	// Script is the shell script to be executed in the virtual machine.
	Script string
}

// RunCommandResult contains the result of executing a command on an Azure VM.
type RunCommandResult struct {
	// ExecutionState is the execution state of the command (e.g. "Succeeded", "Failed").
	ExecutionState string
	// ExitCode is the exit code of the command.
	ExitCode int32
	// StdOut is the stdout of the command.
	StdOut string
	// StdErr is the stderr of the command.
	StdErr string
}

// Failure returns true if the result is considered a failure.
func (r *RunCommandResult) Failure() bool {
	return r.ExitCode != 0 || r.ExecutionState != string(armcompute.ExecutionStateSucceeded)
}

// RunCommandClient is a client for Azure Run Commands.
type RunCommandClient interface {
	// Run runs Teleport installation command on a virtual machine.
	Run(ctx context.Context, req RunCommandRequest) (*RunCommandResult, error)
}

type runCommandClient struct {
	virtualMachineRunCommandsAPI *armcompute.VirtualMachineRunCommandsClient
	scaleSetVMRunCommandsAPI     *armcompute.VirtualMachineScaleSetVMRunCommandsClient
}

// NewRunCommandClient creates a new Azure Run Command client by subscription
// and credentials.
func NewRunCommandClient(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (RunCommandClient, error) {
	runCommandAPI, err := armcompute.NewVirtualMachineRunCommandsClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	scaleSetVMRunCommandsAPI, err := armcompute.NewVirtualMachineScaleSetVMRunCommandsClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &runCommandClient{
		virtualMachineRunCommandsAPI: runCommandAPI,
		scaleSetVMRunCommandsAPI:     scaleSetVMRunCommandsAPI,
	}, nil
}

// TODO(Tener): make the run command name actual parameter.
const runCommandName = "teleport-install"

// Run runs Teleport installation command on a virtual machine.
func (c *runCommandClient) Run(ctx context.Context, req RunCommandRequest) (*RunCommandResult, error) {
	runCommandTimeout := getRunCommandTimeout()
	// pad the timeout so we can still attempt to collect output if it times out
	ctx, cancel := context.WithTimeout(ctx, runCommandTimeout+time.Minute)
	defer cancel()

	runCommand := armcompute.VirtualMachineRunCommand{
		Location: to.Ptr(req.Region),
		Properties: &armcompute.VirtualMachineRunCommandProperties{
			AsyncExecution: to.Ptr(false),
			Source: &armcompute.VirtualMachineRunCommandScriptSource{
				Script: to.Ptr(req.Script),
			},
			TimeoutInSeconds: to.Ptr(int32(runCommandTimeout.Seconds())),
		},
	}

	if req.ScaleSetName != "" {
		return c.uniformScaleSetVirtualMachineRunCommand(ctx, req, runCommand)
	}
	return c.regularVirtualMachineRunCommand(ctx, req, runCommand)
}

func (c *runCommandClient) regularVirtualMachineRunCommand(ctx context.Context, req RunCommandRequest, runCommand armcompute.VirtualMachineRunCommand) (*RunCommandResult, error) {
	poller, err := c.virtualMachineRunCommandsAPI.BeginCreateOrUpdate(ctx, req.ResourceGroup, req.VMName, runCommandName, runCommand, nil)
	if err != nil {
		return nil, trace.Wrap(ConvertResponseError(err))
	}

	_, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{Frequency: 10 * time.Second})
	if err != nil {
		return nil, trace.Wrap(ConvertResponseError(err))
	}

	// note: we are not guaranteed to receive the output of the command above if the req.Name is not unique.
	// in particular, two discovery services may race, causing the output to be empty: our attempt can be shadowed by a newer one.
	resp, err := c.virtualMachineRunCommandsAPI.GetByVirtualMachine(ctx, req.ResourceGroup, req.VMName, runCommandName, &armcompute.VirtualMachineRunCommandsClientGetByVirtualMachineOptions{
		Expand: to.Ptr("instanceView"),
	})
	if err != nil {
		return nil, trace.Wrap(ConvertResponseError(err))
	}

	if resp.Properties == nil || resp.Properties.InstanceView == nil {
		return nil, trace.BadParameter("unable to query command execution state, failure assumed")
	}
	instanceView := resp.Properties.InstanceView

	return &RunCommandResult{
		ExecutionState: string(fromPtr(instanceView.ExecutionState)),
		ExitCode:       fromPtr(instanceView.ExitCode),
		StdOut:         fromPtr(instanceView.Output),
		StdErr:         fromPtr(instanceView.Error),
	}, nil
}

func (c *runCommandClient) uniformScaleSetVirtualMachineRunCommand(ctx context.Context, req RunCommandRequest, runCommand armcompute.VirtualMachineRunCommand) (*RunCommandResult, error) {
	poller, err := c.scaleSetVMRunCommandsAPI.BeginCreateOrUpdate(ctx, req.ResourceGroup, req.ScaleSetName, req.ScaleSetVMInstanceID, runCommandName, runCommand, nil)
	if err != nil {
		return nil, trace.Wrap(ConvertResponseError(err))
	}

	_, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{Frequency: 10 * time.Second})
	if err != nil {
		return nil, trace.Wrap(ConvertResponseError(err))
	}

	// note: we are not guaranteed to receive the output of the command above if the req.Name is not unique.
	// in particular, two discovery services may race, causing the output to be empty: our attempt can be shadowed by a newer one.
	resp, err := c.scaleSetVMRunCommandsAPI.Get(ctx, req.ResourceGroup, req.ScaleSetName, req.ScaleSetVMInstanceID, runCommandName, &armcompute.VirtualMachineScaleSetVMRunCommandsClientGetOptions{
		Expand: to.Ptr("instanceView"),
	})
	if err != nil {
		return nil, trace.Wrap(ConvertResponseError(err))
	}

	if resp.Properties == nil || resp.Properties.InstanceView == nil {
		return nil, trace.BadParameter("unable to query command execution state, failure assumed")
	}

	instanceView := resp.Properties.InstanceView

	return &RunCommandResult{
		ExecutionState: string(fromPtr(instanceView.ExecutionState)),
		ExitCode:       fromPtr(instanceView.ExitCode),
		StdOut:         fromPtr(instanceView.Output),
		StdErr:         fromPtr(instanceView.Error),
	}, nil
}

func fromPtr[T any](ptr *T) T {
	var out T
	if ptr != nil {
		out = *ptr
	}
	return out
}

func getRunCommandTimeout() time.Duration {
	const timeoutEnv = "TELEPORT_UNSTABLE_AZURE_RUN_COMMAND_TIMEOUT"
	if dur, err := time.ParseDuration(os.Getenv(timeoutEnv)); err == nil {
		// clamp the timeout to a reasonable duration.
		const (
			minTimeout = time.Second * 10
			maxTimeout = time.Minute * 90
		)
		return min(maxTimeout, max(minTimeout, dur))
	}
	return 5 * time.Minute // default to 5m
}
