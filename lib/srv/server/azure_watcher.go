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

package server

import (
	"context"
	"log/slog"
	"slices"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/gravitational/trace"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/installers"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/cloud/azure"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/server/installstatus"
)

const azureEventPrefix = "azure/"

// AzureInstances contains information about discovered Azure virtual machines.
type AzureInstances struct {
	// DiscoveryConfigName is the name of discovery config.
	DiscoveryConfigName string
	// Integration is the optional name of the integration to use for auth.
	Integration string

	// Region is the Azure region where the instances are located.
	Region string
	// SubscriptionID is the subscription ID for the instances.
	SubscriptionID string
	// ResourceGroup is the resource group for the instances.
	ResourceGroup string

	// InstallerParams are the installer parameters used for installation.
	InstallerParams *types.InstallerParams
	// Instances is a list of discovered Azure virtual machines.
	Instances []*AzureVirtualMachine
}

func (instances *AzureInstances) LogValue() slog.Value {
	if instances == nil {
		return slog.StringValue("<nil>")
	}
	return slog.GroupValue(
		slog.Int("total_instances", len(instances.Instances)),
		slog.String("discovery_config", instances.DiscoveryConfigName),
		slog.String("integration", instances.Integration),
		slog.String("region", instances.Region),
		slog.String("resource_group", instances.ResourceGroup),
		slog.String("subscription_id", instances.SubscriptionID),
	)
}

func (instances *AzureInstances) resourceType() string {
	if instances.InstallerParams != nil && instances.InstallerParams.ScriptName == installers.InstallerScriptNameAgentless {
		return types.DiscoveredResourceAgentlessNode
	}
	return types.DiscoveredResourceNode
}

// MakeUsageEvent builds usage event for a single installation result.
func (instances *AzureInstances) MakeUsageEvent(instance *AzureVirtualMachine) (string, *usageeventsv1.ResourceCreateEvent) {
	return azureEventPrefix + instance.ID, &usageeventsv1.ResourceCreateEvent{
		ResourceType:        instances.resourceType(),
		ResourceOrigin:      types.OriginCloud,
		CloudProvider:       types.CloudAzure,
		DiscoveryConfigName: instances.DiscoveryConfigName,
	}
}

// MakeRunEvent builds run event for a single command run.
func (instances *AzureInstances) MakeRunEvent(result AzureInstallResult) *apievents.AzureRun {
	eventCode := libevents.AzureRunSuccessCode

	if result.Failure() {
		eventCode = libevents.AzureRunFailCode
	}

	var vmID, vmName, resourceID string
	if result.Instance != nil {
		vmName = result.Instance.Name
		resourceID = result.Instance.ID
		vmID = result.Instance.VMID
	}

	evt := &apievents.AzureRun{
		Metadata: apievents.Metadata{
			Type: libevents.AzureRunEvent,
			Code: eventCode,
		},
		AzureMetadata: apievents.AzureMetadata{
			SubscriptionID: instances.SubscriptionID,
			ResourceGroup:  instances.ResourceGroup,
			ResourceID:     resourceID,
			Region:         instances.Region,
		},
		AzureVMMetadata: apievents.AzureVMMetadata{
			VMID:   vmID,
			VMName: vmName,
		},
	}

	if result.APIError != nil {
		evt.APIError = result.APIError.Error()
		evt.Status = "API call failed"
	}

	if result.CommandResult != nil {
		evt.ExecutionState = result.CommandResult.ExecutionState
		evt.StandardError = result.CommandResult.StdErr
		evt.StandardOutput = result.CommandResult.StdOut
		evt.ExitCode = result.CommandResult.ExitCode
		if result.CommandResult.Failure() {
			evt.Status = installstatus.ExitCode(result.CommandResult.ExitCode).String()
		} else {
			// TODO(Tener): Consider extending installstatus.ExitCode to handle exit code 0,
			// so the success status message comes from the same place as failures.
			evt.Status = "Installation completed successfully."
		}
	}

	return evt
}

// FilterExistingNodes removes instances matching existing nodes in place.
func (instances *AzureInstances) FilterExistingNodes(existingNodes []types.Server) {
	vmIDs := make(map[string]struct{})
	for _, node := range existingNodes {
		labels := node.GetAllLabels()
		subscriptionID := labels[types.SubscriptionIDLabelInternal]
		if subscriptionID != instances.SubscriptionID {
			continue
		}
		vmID := labels[types.VMIDLabelInternal]
		if vmID != "" {
			vmIDs[vmID] = struct{}{}
		}
	}

	instances.Instances = slices.DeleteFunc(instances.Instances, func(instance *AzureVirtualMachine) bool {
		_, found := vmIDs[instance.VMID]
		return found
	})
}

type azureClientGetter func(ctx context.Context, integration string) (azure.Clients, error)

type listSubscriptionsFunc func(ctx context.Context, integration string) (subscriptions []string, err error)

// MatchersToAzureInstanceFetchers converts a list of Azure VM Matchers into a list of Azure VM Fetchers.
func MatchersToAzureInstanceFetchers(
	ctx context.Context,
	logger *slog.Logger,
	matchers []types.AzureMatcher,
	getClient azureClientGetter,
	discoveryConfigName string,
	listSubs listSubscriptionsFunc,
) []Fetcher[*AzureInstances] {
	ret := make([]Fetcher[*AzureInstances], 0)
	for _, matcher := range matchers {
		matcher.Subscriptions = expandAzureMatcherSubscriptions(ctx, logger, matcher.Subscriptions, matcher.Integration, listSubs)
		for _, subscription := range matcher.Subscriptions {
			for _, resourceGroup := range matcher.ResourceGroups {
				fetcher := newAzureInstanceFetcher(azureFetcherConfig{
					Matcher:             matcher,
					Subscription:        subscription,
					ResourceGroup:       resourceGroup,
					AzureClientGetter:   getClient,
					DiscoveryConfigName: discoveryConfigName,
					Logger:              logger,
				})
				ret = append(ret, fetcher)
			}
		}
	}
	return ret
}

// expandAzureMatcherSubscriptions fetches the subscriptions for any wildcard
// subscriptions and replaces the wildcard with the subscriptions list.
func expandAzureMatcherSubscriptions(
	ctx context.Context,
	logger *slog.Logger,
	subscriptions []string,
	integration string,
	listSubs listSubscriptionsFunc,
) []string {
	var out []string
	for _, sub := range subscriptions {
		if sub != types.Wildcard {
			out = append(out, sub)
			continue
		}
		subs, err := listSubs(ctx, integration)
		if err != nil {
			// TODO(gavin): make a user task
			logger.WarnContext(ctx, "Failed to fetch Azure subscription list for wildcard in discovery configuration",
				"integration", integration,
				"error", err,
			)
			continue
		}
		out = append(out, subs...)
	}
	return utils.Deduplicate(out)
}

type azureFetcherConfig struct {
	Matcher             types.AzureMatcher
	Subscription        string
	ResourceGroup       string
	AzureClientGetter   azureClientGetter
	DiscoveryConfigName string
	Logger              *slog.Logger
}

type azureInstanceFetcher struct {
	InstallerParams     *types.InstallerParams
	AzureClientGetter   azureClientGetter
	Regions             []string
	Subscription        string
	ResourceGroup       string
	Labels              types.Labels
	DiscoveryConfigName string
	Integration         string
	Logger              *slog.Logger
}

func newAzureInstanceFetcher(cfg azureFetcherConfig) *azureInstanceFetcher {
	return &azureInstanceFetcher{
		InstallerParams:     cfg.Matcher.Params,
		AzureClientGetter:   cfg.AzureClientGetter,
		Regions:             cfg.Matcher.Regions,
		Subscription:        cfg.Subscription,
		ResourceGroup:       cfg.ResourceGroup,
		Labels:              cfg.Matcher.ResourceTags,
		DiscoveryConfigName: cfg.DiscoveryConfigName,
		Integration:         cfg.Matcher.Integration,
		Logger:              cfg.Logger,
	}
}

func (*azureInstanceFetcher) GetMatchingInstances(_ context.Context, _ []types.Server, _ bool) ([]*AzureInstances, error) {
	return nil, trace.NotImplemented("not implemented for azure fetchers")
}

func (f *azureInstanceFetcher) GetDiscoveryConfigName() string {
	return f.DiscoveryConfigName
}

// IntegrationName identifies the integration name whose credentials were used to fetch the resources.
// Might be empty when the fetcher is using ambient credentials.
func (f *azureInstanceFetcher) IntegrationName() string {
	return f.Integration
}

type azureVMGroup struct {
	resourceGroup string
	location      string
	scaleSetName  string
}

// GetInstances fetches all Azure virtual machines matching configured filters.
func (f *azureInstanceFetcher) GetInstances(ctx context.Context, _ bool) ([]*AzureInstances, error) {
	azureClients, err := f.AzureClientGetter(ctx, f.IntegrationName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := azureClients.GetVirtualMachinesClient(ctx, f.Subscription)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var instances []*AzureInstances

	instanceGroups, err := f.fetchAndGroupRegularVMs(ctx, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for batchGroup, vms := range instanceGroups {
		instances = append(instances, &AzureInstances{
			SubscriptionID:      f.Subscription,
			Region:              batchGroup.location,
			ResourceGroup:       batchGroup.resourceGroup,
			Instances:           vms,
			Integration:         f.Integration,
			InstallerParams:     f.InstallerParams,
			DiscoveryConfigName: f.DiscoveryConfigName,
		})
	}

	scaleSetInstanceGroups, err := f.fetchAndGroupScaleSetVMs(ctx, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for batchGroup, vms := range scaleSetInstanceGroups {
		instances = append(instances, &AzureInstances{
			SubscriptionID:      f.Subscription,
			Region:              batchGroup.location,
			ResourceGroup:       batchGroup.resourceGroup,
			Instances:           vms,
			Integration:         f.Integration,
			InstallerParams:     f.InstallerParams,
			DiscoveryConfigName: f.DiscoveryConfigName,
		})
	}

	return instances, nil
}

func (f *azureInstanceFetcher) fetchAndGroupRegularVMs(ctx context.Context, client azure.VirtualMachinesClient) (map[azureVMGroup][]*AzureVirtualMachine, error) {
	vms, err := client.ListVirtualMachines(ctx, f.ResourceGroup)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	instanceGroups := make(map[azureVMGroup][]*AzureVirtualMachine)

	allowAllLocations := slices.Contains(f.Regions, types.Wildcard)
	allowAllResourceGroups := f.ResourceGroup == types.Wildcard

	for _, vm := range vms {
		location := azure.StringVal(vm.Location)
		if !slices.Contains(f.Regions, location) && !allowAllLocations {
			continue
		}

		vmTags := make(map[string]string, len(vm.Tags))
		for key, value := range vm.Tags {
			vmTags[key] = azure.StringVal(value)
		}
		if match, _, _ := services.MatchLabels(f.Labels, vmTags); !match {
			continue
		}

		resourceGroup := f.ResourceGroup
		if allowAllResourceGroups {
			resourceMetadata, err := arm.ParseResourceID(azure.StringVal(vm.ID))
			if err != nil {
				f.Logger.WarnContext(ctx, "Skipping Teleport installation on Azure VM - failed to infer resource group from vm id",
					"subscription_id", f.Subscription,
					"vm_id", azure.StringVal(vm.Properties.VMID),
					"resource_id", azure.StringVal(vm.ID),
					"error", err,
				)
				continue
			}
			resourceGroup = resourceMetadata.ResourceGroupName
		}

		batchGroup := azureVMGroup{
			resourceGroup: resourceGroup,
			location:      location,
		}

		instanceGroups[batchGroup] = append(instanceGroups[batchGroup], azureVirtualMachineFromAzureVM(vm))
	}

	return instanceGroups, nil
}

var azureVMSSServerDiscoveryRequiredPermissions = []string{
	"Microsoft.Compute/virtualMachineScaleSets/read",
	"Microsoft.Compute/virtualMachineScaleSets/virtualMachines/read",
	"Microsoft.Compute/virtualMachineScaleSets/virtualMachines/runCommand/action",
	"Microsoft.Compute/virtualMachineScaleSets/virtualMachines/runCommands/write",
	"Microsoft.Compute/virtualMachineScaleSets/virtualMachines/runCommands/read",
	"Microsoft.Compute/virtualMachineScaleSets/virtualMachines/runCommands/delete",
}

func (f *azureInstanceFetcher) fetchAndGroupScaleSetVMs(ctx context.Context, client azure.VirtualMachinesClient) (map[azureVMGroup][]*AzureVirtualMachine, error) {
	allowAllLocations := slices.Contains(f.Regions, types.Wildcard)
	allowAllResourceGroups := f.ResourceGroup == types.Wildcard

	vmsFromUniformScaleSets, err := client.ListVirtualMachinesFromUniformVMSS(ctx, f.ResourceGroup)
	if err != nil {
		if !trace.IsAccessDenied(err) {
			return nil, trace.Wrap(err)
		}

		// TODO(marco): create user task for missing permissions.
		f.Logger.WarnContext(ctx, "Failed to fetch VMs from uniform Scale Sets, continuing with other VMs. Adding the required permissions will allow you to enroll VMs from uniform Scale Sets.",
			"subscription_id", f.Subscription,
			"resource_group", f.ResourceGroup,
			"required_permissions", azureVMSSServerDiscoveryRequiredPermissions,
			"error", err,
		)
		return nil, nil
	}

	scaleSetInstanceGroups := make(map[azureVMGroup][]*AzureVirtualMachine)
	for _, vm := range vmsFromUniformScaleSets {
		location := azure.StringVal(vm.Location)
		if !slices.Contains(f.Regions, location) && !allowAllLocations {
			continue
		}

		vmTags := azure.ConvertTags(vm.Tags)
		if match, _, _ := services.MatchLabels(f.Labels, vmTags); !match {
			continue
		}

		resourceMetadata, err := arm.ParseResourceID(azure.StringVal(vm.ID))
		if err != nil {
			f.Logger.WarnContext(ctx, "Skipping Teleport installation on Azure VM: failed to infer resource group from vm id",
				"subscription_id", f.Subscription,
				"vm_id", azure.StringVal(vm.Properties.VMID),
				"resource_id", azure.StringVal(vm.ID),
				"error", err,
			)
			continue
		}
		vmScaleSetName, err := vmssNameFromResourceID(resourceMetadata)
		if err != nil {
			f.Logger.WarnContext(ctx, "Skipping Teleport installation on Azure VM: failed to identify Scale Set name from resource ID",
				"subscription_id", f.Subscription,
				"vm_id", azure.StringVal(vm.Properties.VMID),
				"resource_id", azure.StringVal(vm.ID),
				"error", err,
			)
			continue
		}

		resourceGroup := f.ResourceGroup
		if allowAllResourceGroups {
			resourceGroup = resourceMetadata.ResourceGroupName
		}

		batchGroup := azureVMGroup{
			resourceGroup: resourceGroup,
			location:      location,
			scaleSetName:  vmScaleSetName,
		}

		scaleSetInstanceGroups[batchGroup] = append(scaleSetInstanceGroups[batchGroup], azureVirtualMachineFromScaleSetVM(vm, vmScaleSetName))
	}

	return scaleSetInstanceGroups, nil
}

func vmssNameFromResourceID(resourceID *arm.ResourceID) (string, error) {
	if resourceID.Parent == nil || resourceID.Parent.ResourceType.String() != azure.VirtualMachineScaleSetsResourceType || resourceID.Parent.Name == "" {
		return "", trace.BadParameter("failed to identify Scale Set name from resource ID %q", resourceID.String())
	}
	return resourceID.Parent.Name, nil
}

// LogValue implements [slog.LogValuer].
func (f *azureInstanceFetcher) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Any("labels", f.Labels),
		slog.Any("regions", f.Regions),
		slog.String("discovery_config", f.GetDiscoveryConfigName()),
		slog.String("integration", f.IntegrationName()),
		slog.String("resource_group", f.ResourceGroup),
		slog.String("subscription_id", f.Subscription),
	)
}
