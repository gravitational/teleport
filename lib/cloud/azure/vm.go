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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
)

// virtualScaleSetUniformVMResourceType represents the resource type of uniform
// virtual scale set VMs.
const virtualScaleSetUniformVMResourceType = "virtualMachineScaleSets/virtualMachines"

// virtualMachinesLister provides an interface for an Azure virtual machine client.
type virtualMachinesLister interface {
	// Get retrieves information about an Azure virtual machine.
	Get(ctx context.Context, resourceGroupName string, vmName string, options *armcompute.VirtualMachinesClientGetOptions) (armcompute.VirtualMachinesClientGetResponse, error)
	// NewListPager lists Azure virtual Machines.
	NewListPager(resourceGroup string, opts *armcompute.VirtualMachinesClientListOptions) *runtime.Pager[armcompute.VirtualMachinesClientListResponse]
	// NewListAllPager lists Azure virtual machines in any resource group.
	NewListAllPager(opts *armcompute.VirtualMachinesClientListAllOptions) *runtime.Pager[armcompute.VirtualMachinesClientListAllResponse]
}

// scaleSetsLister provides an interface for an Azure VM scale set client.
type scaleSetsLister interface {
	// NewListAllPager gets a list of all VM Scale Sets in the subscription, regardless of the associated resource group.
	NewListAllPager(options *armcompute.VirtualMachineScaleSetsClientListAllOptions) *runtime.Pager[armcompute.VirtualMachineScaleSetsClientListAllResponse]
	// NewListPager gets a list of all VM scale sets under a resource group.
	NewListPager(resourceGroupName string, options *armcompute.VirtualMachineScaleSetsClientListOptions) *runtime.Pager[armcompute.VirtualMachineScaleSetsClientListResponse]
}

// scaleSetVMsLister provides an interface for an Azure VM scale set VMs client.
type scaleSetVMsLister interface {
	// Get retrieves a virtual machine from a VM scale set.
	Get(ctx context.Context, resourceGroupName string, vmScaleSetName string, instanceID string, options *armcompute.VirtualMachineScaleSetVMsClientGetOptions) (armcompute.VirtualMachineScaleSetVMsClientGetResponse, error)
	// NewListPager gets a list of all virtual machines in a VM scale set.
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
	ListVirtualMachines(ctx context.Context, resourceGroup string) ([]*VirtualMachine, error)
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
	// Location is the Azure region containing the VM, e.g. "eastus".
	Location string
	// VMID is the VM's ID.
	VMID string
	// UniformScaleSetName is the name of the Virtual Machine Scale Set. Empty string if the VM is not part of a uniform Scale Set.
	UniformScaleSetName string
	// UniformScaleSetVMInstanceID is the instance ID of the Virtual Machine Scale Set VM. Empty string if the VM is not part of a uniform Scale Set.
	// This is a unique identifier for the VM within its Scale Set, e.g. "0", "1".
	UniformScaleSetVMInstanceID string
	// Identities are the identities associated with the resource.
	Identities []Identity
	// Tags are the VM tags, e.g. {"env": "prod"}. Empty map (not nil) when the VM has no tags.
	Tags map[string]string
}

// Identity represents an Azure virtual machine identity.
type Identity struct {
	// ResourceID the identity resource ID.
	ResourceID string
}

type vmClient struct {
	// virtualMachinesLister is the Azure virtual machine client.
	virtualMachinesLister virtualMachinesLister
	// scaleSetVMsLister is the Azure VM scale set VMs client.
	scaleSetVMsLister scaleSetVMsLister
	// scaleSetsLister is the Azure VM scale set client.
	scaleSetsLister scaleSetsLister

	logger *slog.Logger
}

// NewVirtualMachinesClient creates a new Azure virtual machines client by
// subscription and credentials.
func NewVirtualMachinesClient(subscription string, cred azcore.TokenCredential, options *arm.ClientOptions) (VirtualMachinesClient, error) {
	computeAPI, err := armcompute.NewVirtualMachinesClient(subscription, cred, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	scaleSetVMsLister, err := armcompute.NewVirtualMachineScaleSetVMsClient(subscription, cred, options)
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
		ScaleSetVMsAPI:    scaleSetVMsLister,
	}
	return NewVirtualMachinesClientByAPI(config), nil
}

// VirtualMachinesClientConfig combines dependencies for creating a VirtualMachinesClient.
type VirtualMachinesClientConfig struct {
	VirtualMachineAPI virtualMachinesLister
	ScaleSetsAPI      scaleSetsLister
	ScaleSetVMsAPI    scaleSetVMsLister
	Logger            *slog.Logger
}

// NewVirtualMachinesClientByAPI creates a new Azure virtual machines client by
// ARM API client.
func NewVirtualMachinesClientByAPI(config VirtualMachinesClientConfig) VirtualMachinesClient {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default().With(teleport.ComponentKey, "azure_virtualmachines_client")
	}

	return &vmClient{
		virtualMachinesLister: config.VirtualMachineAPI,
		scaleSetsLister:       config.ScaleSetsAPI,
		scaleSetVMsLister:     config.ScaleSetVMsAPI,
		logger:                logger,
	}
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

	resp, err := c.virtualMachinesLister.Get(ctx, parsedResourceID.ResourceGroupName, parsedResourceID.Name, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	vm, err := virtualMachineFromARMComputeVirtualMachine(&resp.VirtualMachine)
	return vm, trace.Wrap(err)
}

// GetByVMID returns the virtual machine for a given VM ID.
func (c *vmClient) GetByVMID(ctx context.Context, vmID string) (*VirtualMachine, error) {
	pager := newAPIPager(
		c.virtualMachinesLister.NewListAllPager(&armcompute.VirtualMachinesClientListAllOptions{}),
		func(resp armcompute.VirtualMachinesClientListAllResponse) []*armcompute.VirtualMachine {
			return resp.Value
		},
	)
	for pager.more() {
		res, err := pager.nextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}

		for _, vm := range res {
			if vm.Properties != nil && *vm.Properties.VMID == vmID {
				result, err := virtualMachineFromARMComputeVirtualMachine(vm)
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

	resp, err := c.scaleSetVMsLister.Get(ctx, resourceID.ResourceGroupName, resourceID.Parent.Name, resourceID.Name, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	scaleSetName := resourceID.Parent.Name
	resourceGroup := resourceID.ResourceGroupName
	subscriptionID := resourceID.SubscriptionID

	result, err := virtualMachineFromARMComputeVirtualMachineScaleSetVM(&resp.VirtualMachineScaleSetVM, subscriptionID, resourceGroup, scaleSetName)
	return result, trace.Wrap(err)
}

type apiPager[T any] struct {
	more     func() bool
	nextPage func(context.Context) ([]*T, error)
}

// newAPIPager wraps an Azure SDK pager into a apiPager. The values function
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

// ListVirtualMachines lists all virtual machines in a given resource group
// using the Azure virtual machines API. If resourceGroup is "*", it lists
// all virtual machines in any resource group.
// It includes regular VMs and VMs from VM Scale Sets.
func (c *vmClient) ListVirtualMachines(ctx context.Context, resourceGroup string) ([]*VirtualMachine, error) {
	regularVMs, listVMsErr := c.listARMVirtualMachinesAsVirtualMachines(ctx, resourceGroup)

	uniformScaleSetVirtualMachines, uniformVMSSListVMsErr := c.listVirtualMachinesFromUniformVMSS(ctx, resourceGroup)

	switch {
	case listVMsErr != nil && uniformVMSSListVMsErr != nil:
		return nil, trace.NewAggregate(listVMsErr, uniformVMSSListVMsErr)

	case listVMsErr != nil:
		c.logger.WarnContext(ctx, "failed to call ListVirtualMachines API, continuing with uniform VMSS VMs", "error", listVMsErr)
		return uniformScaleSetVirtualMachines, nil

	case uniformVMSSListVMsErr != nil:
		c.logger.WarnContext(ctx, "failed to list VMs from uniform VMSS, continuing with regular VMs", "error", uniformVMSSListVMsErr)
		return regularVMs, nil

	default:
		return append(regularVMs, uniformScaleSetVirtualMachines...), nil
	}
}

// vmScaleSetIsFlexible reports whether the Scale Set was created with flexible orchestration mode.
func vmScaleSetIsFlexible(vmScaleSetProperties *armcompute.VirtualMachineScaleSetProperties) bool {
	return vmScaleSetProperties != nil && StringVal(vmScaleSetProperties.OrchestrationMode) == string(armcompute.OrchestrationModeFlexible)
}

func (c *vmClient) listARMVirtualMachinesAsVirtualMachines(ctx context.Context, resourceGroup string) ([]*VirtualMachine, error) {
	var allVMs []*VirtualMachine

	var pager apiPager[armcompute.VirtualMachine]
	if resourceGroup == types.Wildcard {
		pager = newAPIPager(
			c.virtualMachinesLister.NewListAllPager(&armcompute.VirtualMachinesClientListAllOptions{}),
			func(resp armcompute.VirtualMachinesClientListAllResponse) []*armcompute.VirtualMachine {
				return resp.Value
			},
		)
	} else {
		pager = newAPIPager(
			c.virtualMachinesLister.NewListPager(resourceGroup, &armcompute.VirtualMachinesClientListOptions{}),
			func(resp armcompute.VirtualMachinesClientListResponse) []*armcompute.VirtualMachine {
				return resp.Value
			},
		)
	}

	for pager.more() {
		res, err := pager.nextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		for _, rawVM := range res {
			vm, err := virtualMachineFromARMComputeVirtualMachine(rawVM)
			if err != nil {
				c.logger.DebugContext(ctx, "skipping Azure VM", "resource_id", StringVal(rawVM.ID), "error", err)
				continue
			}
			allVMs = append(allVMs, vm)
		}
	}

	return allVMs, nil
}

func (c *vmClient) listVirtualMachinesFromUniformVMSS(ctx context.Context, resourceGroupFilter string) ([]*VirtualMachine, error) {
	var pager apiPager[armcompute.VirtualMachineScaleSet]
	if resourceGroupFilter == types.Wildcard {
		pager = newAPIPager(
			c.scaleSetsLister.NewListAllPager(&armcompute.VirtualMachineScaleSetsClientListAllOptions{}),
			func(resp armcompute.VirtualMachineScaleSetsClientListAllResponse) []*armcompute.VirtualMachineScaleSet {
				return resp.Value
			},
		)
	} else {
		pager = newAPIPager(
			c.scaleSetsLister.NewListPager(resourceGroupFilter, &armcompute.VirtualMachineScaleSetsClientListOptions{}),
			func(resp armcompute.VirtualMachineScaleSetsClientListResponse) []*armcompute.VirtualMachineScaleSet {
				return resp.Value
			},
		)
	}

	var virtualMachines []*VirtualMachine
	for pager.more() {
		scaleSetsPage, err := pager.nextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}

		for _, scaleSet := range scaleSetsPage {
			// Skip VM Scale Sets whose orchestration mode is Flexible, because those are returned in the regular List Virtual Machines API.
			if vmScaleSetIsFlexible(scaleSet.Properties) {
				continue
			}

			scaleSetName := StringVal(scaleSet.Name)

			scaleSetResourceID, err := arm.ParseResourceID(StringVal(scaleSet.ID))
			if err != nil {
				c.logger.DebugContext(ctx, "skipping entire Azure VM Scale Set", "scale_set_name", scaleSetName, "resource_id", StringVal(scaleSet.ID), "error", err)
				continue
			}
			resourceGroup := scaleSetResourceID.ResourceGroupName
			subscriptionID := scaleSetResourceID.SubscriptionID

			scaleSetVMsPager := newAPIPager(
				c.scaleSetVMsLister.NewListPager(resourceGroup, scaleSetName, &armcompute.VirtualMachineScaleSetVMsClientListOptions{}),
				func(resp armcompute.VirtualMachineScaleSetVMsClientListResponse) []*armcompute.VirtualMachineScaleSetVM {
					return resp.Value
				},
			)
			pageCount := 0
			vmsCounter := 0
			for scaleSetVMsPager.more() {
				pageCount++
				scaleSetVMsPage, err := scaleSetVMsPager.nextPage(ctx)
				if err != nil {
					c.logger.DebugContext(ctx, "error when fetching Azure VM Scale Set VMs page", "scale_set_name", scaleSetName, "resource_id", StringVal(scaleSet.ID), "page", pageCount, "error", err)
					break
				}

				vmsCounter += len(scaleSetVMsPage)
				for _, vm := range scaleSetVMsPage {
					discoveredVM, err := virtualMachineFromARMComputeVirtualMachineScaleSetVM(vm, subscriptionID, resourceGroup, scaleSetName)
					if err != nil {
						c.logger.DebugContext(ctx, "skipping Azure VM Scale Set VM", "scale_set_name", scaleSetName, "resource_id", StringVal(vm.ID), "error", err)
						continue
					}
					virtualMachines = append(virtualMachines, discoveredVM)
				}
			}

			c.logger.DebugContext(ctx, "fetched all VMs from Azure VM Scale Set", "scale_set_name", scaleSetName, "resource_id", StringVal(scaleSet.ID), "pages_listed", pageCount, "vms_listed", vmsCounter)
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
	// UniformScaleSetName is the name of the Scale Set this VM belongs to.
	// Only used when the VM is part of a uniform VM Scale Set.
	// Regular VMs and VMs from flexible VM Scale Sets should leave this field empty.
	UniformScaleSetName string
	// UniformScaleSetVMInstanceID is the instance ID of this VM within the uniform Scale Set.
	// Only used when the VM is part of a uniform VM Scale Set.
	// Regular VMs and VMs from flexible VM Scale Sets should leave this field empty.
	UniformScaleSetVMInstanceID string
	// Script is the shell script to be executed in the virtual machine.
	Script string
}

func (r RunCommandRequest) isUniformVMSS() bool {
	return r.UniformScaleSetName != "" && r.UniformScaleSetVMInstanceID != ""
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

	var runCommandProperties *armcompute.VirtualMachineRunCommandProperties
	var err error

	if req.isUniformVMSS() {
		runCommandProperties, err = c.uniformScaleSetVirtualMachineRunCommand(ctx, req, runCommand)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		runCommandProperties, err = c.regularVirtualMachineRunCommand(ctx, req, runCommand)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	commandResult, err := commandResultFromInstanceView(runCommandProperties)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return commandResult, nil
}

func (c *runCommandClient) regularVirtualMachineRunCommand(ctx context.Context, req RunCommandRequest, runCommand armcompute.VirtualMachineRunCommand) (*armcompute.VirtualMachineRunCommandProperties, error) {
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

	return resp.Properties, nil
}

func (c *runCommandClient) uniformScaleSetVirtualMachineRunCommand(ctx context.Context, req RunCommandRequest, runCommand armcompute.VirtualMachineRunCommand) (*armcompute.VirtualMachineRunCommandProperties, error) {
	poller, err := c.scaleSetVMRunCommandsAPI.BeginCreateOrUpdate(ctx, req.ResourceGroup, req.UniformScaleSetName, req.UniformScaleSetVMInstanceID, runCommandName, runCommand, nil)
	if err != nil {
		return nil, trace.Wrap(ConvertResponseError(err))
	}

	_, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{Frequency: 10 * time.Second})
	if err != nil {
		return nil, trace.Wrap(ConvertResponseError(err))
	}

	// note: we are not guaranteed to receive the output of the command above if the req.Name is not unique.
	// in particular, two discovery services may race, causing the output to be empty: our attempt can be shadowed by a newer one.
	resp, err := c.scaleSetVMRunCommandsAPI.Get(ctx, req.ResourceGroup, req.UniformScaleSetName, req.UniformScaleSetVMInstanceID, runCommandName, &armcompute.VirtualMachineScaleSetVMRunCommandsClientGetOptions{
		Expand: to.Ptr("instanceView"),
	})
	if err != nil {
		return nil, trace.Wrap(ConvertResponseError(err))
	}

	return resp.Properties, nil
}

func commandResultFromInstanceView(properties *armcompute.VirtualMachineRunCommandProperties) (*RunCommandResult, error) {
	if properties == nil || properties.InstanceView == nil {
		return nil, trace.BadParameter("unable to query command execution state")
	}
	return &RunCommandResult{
		ExecutionState: string(fromPtr(properties.InstanceView.ExecutionState)),
		ExitCode:       fromPtr(properties.InstanceView.ExitCode),
		StdOut:         fromPtr(properties.InstanceView.Output),
		StdErr:         fromPtr(properties.InstanceView.Error),
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

func virtualMachineFromARMComputeVirtualMachine(vm *armcompute.VirtualMachine) (*VirtualMachine, error) {
	if vm == nil {
		return nil, trace.BadParameter("vm cannot be nil")
	}

	var vmid string
	if vm.Properties != nil {
		vmid = StringVal(vm.Properties.VMID)
	}

	// The ARM resource ID should never fail to parse, unless Azure changes the contract.
	resourceMetadata, err := arm.ParseResourceID(StringVal(vm.ID))
	if err != nil {
		return nil, trace.BadParameter("failed to parse Virtual Machine resource ID %q: %v", StringVal(vm.ID), err)
	}

	return &VirtualMachine{
		ID:            StringVal(vm.ID),
		VMID:          vmid,
		Name:          StringVal(vm.Name),
		Location:      StringVal(vm.Location),
		Subscription:  resourceMetadata.SubscriptionID,
		ResourceGroup: resourceMetadata.ResourceGroupName,
		Identities:    parseVMIdentities(vm.Identity),
		Tags:          ConvertTags(vm.Tags),
	}, nil
}

func virtualMachineFromARMComputeVirtualMachineScaleSetVM(vm *armcompute.VirtualMachineScaleSetVM, subscriptionID, resourceGroup, scaleSetName string) (*VirtualMachine, error) {
	if vm == nil {
		return nil, trace.BadParameter("vm cannot be nil")
	}

	var vmid string
	if vm.Properties != nil {
		vmid = StringVal(vm.Properties.VMID)
	}

	return &VirtualMachine{
		ID:                          StringVal(vm.ID),
		VMID:                        vmid,
		Name:                        StringVal(vm.Name),
		Location:                    StringVal(vm.Location),
		Subscription:                subscriptionID,
		ResourceGroup:               resourceGroup,
		Identities:                  parseVMIdentities(vm.Identity),
		Tags:                        ConvertTags(vm.Tags),
		UniformScaleSetName:         scaleSetName,
		UniformScaleSetVMInstanceID: StringVal(vm.InstanceID),
	}, nil
}

func parseVMIdentities(identity *armcompute.VirtualMachineIdentity) []Identity {
	if identity == nil {
		return nil
	}

	var identities []Identity
	if systemAssigned := StringVal(identity.PrincipalID); systemAssigned != "" {
		identities = append(identities, Identity{ResourceID: systemAssigned})
	}

	for identityID := range identity.UserAssignedIdentities {
		identities = append(identities, Identity{ResourceID: identityID})
	}

	return identities
}
