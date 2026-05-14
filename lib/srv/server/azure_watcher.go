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
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
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
	// Instances is a list of discovered Azure virtual machines, populated by GetInstances
	// (the SDK-backed listing path). Nil when discovery ran via Resource Graph; see DiscoveredVMs.
	Instances []*armcompute.VirtualMachine
	// DiscoveredVMs is a list of VMs returned by GetInstancesARG (the Resource Graph path),
	// each carrying a resolved PrimaryPrivateIP when the matcher's flow requested it (Windows VM
	// discovery for dynamic desktop registration). Nil when discovery ran via the SDK path; see
	// Instances. Exactly one of Instances or DiscoveredVMs is populated per AzureInstances value.
	DiscoveredVMs []azure.DiscoveredVM
}

func (instances *AzureInstances) LogValue() slog.Value {
	if instances == nil {
		return slog.StringValue("<nil>")
	}
	// Exactly one of Instances or DiscoveredVMs is populated; sum so the count is correct
	// regardless of which discovery path produced this group.
	return slog.GroupValue(
		slog.Int("total_instances", len(instances.Instances)+len(instances.DiscoveredVMs)),
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

// MakeUsageEvent builds a usage event for a single installation result.
func (instances *AzureInstances) MakeUsageEvent(result AzureInstallResult) (string, *usageeventsv1.ResourceCreateEvent) {
	resourceID := installResultResourceID(result)
	return azureEventPrefix + resourceID, &usageeventsv1.ResourceCreateEvent{
		ResourceType:        instances.resourceType(),
		ResourceOrigin:      types.OriginCloud,
		CloudProvider:       types.CloudAzure,
		DiscoveryConfigName: instances.DiscoveryConfigName,
	}
}

// installResultResourceID returns the ARM resource ID for a result, sourcing from whichever of
// Instance / DiscoveredVM the result carries.
func installResultResourceID(result AzureInstallResult) string {
	if result.DiscoveredVM != nil {
		return result.DiscoveredVM.ID
	}
	if result.Instance != nil {
		return azure.StringVal(result.Instance.ID)
	}
	return ""
}

// MakeRunEvent builds run event for a single command run.
func (instances *AzureInstances) MakeRunEvent(result AzureInstallResult) *apievents.AzureRun {
	eventCode := libevents.AzureRunSuccessCode

	if result.Failure() {
		eventCode = libevents.AzureRunFailCode
	}

	var vmID, vmName, resourceID string
	switch {
	case result.DiscoveredVM != nil:
		vmName = result.DiscoveredVM.Name
		resourceID = result.DiscoveredVM.ID
		vmID = result.DiscoveredVM.VMID
	case result.Instance != nil:
		vmName = azure.StringVal(result.Instance.Name)
		resourceID = azure.StringVal(result.Instance.ID)
		if result.Instance.Properties != nil {
			vmID = azure.StringVal(result.Instance.Properties.VMID)
		}
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

	instances.Instances = slices.DeleteFunc(instances.Instances, func(instance *armcompute.VirtualMachine) bool {
		var vmID string
		if instance.Properties != nil && instance.Properties.VMID != nil {
			vmID = *instance.Properties.VMID
		}
		_, found := vmIDs[vmID]
		return found
	})
	instances.DiscoveredVMs = slices.DeleteFunc(instances.DiscoveredVMs, func(vm azure.DiscoveredVM) bool {
		_, found := vmIDs[vm.VMID]
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
					MatcherType:         matcher.Types[0],
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
	MatcherType         string
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
	MatcherType         string
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
		MatcherType:         cfg.MatcherType,
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

type resourceGroupLocation struct {
	resourceGroup string
	location      string
}

// GetInstances fetches all Azure virtual machines matching configured filters. The Windows VM
// matcher path is served by Resource Graph (GetInstancesARG) because the Windows desktop
// registration flow needs each VM's primary private IP, which ARG can fetch alongside the VM
// listing. All other matchers continue to use the SDK ListVirtualMachines path below.
func (f *azureInstanceFetcher) GetInstances(ctx context.Context, rotation bool) ([]*AzureInstances, error) {
	if f.MatcherType == types.AzureMatcherWindowsVM {
		return f.GetInstancesARG(ctx, rotation)
	}

	azureClients, err := f.AzureClientGetter(ctx, f.IntegrationName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := azureClients.GetVirtualMachinesClient(ctx, f.Subscription)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	vms, err := client.ListVirtualMachines(ctx, f.ResourceGroup)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	instancesByRegionAndResourceGroup := make(map[resourceGroupLocation][]*armcompute.VirtualMachine)

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

		batchGroup := resourceGroupLocation{
			resourceGroup: resourceGroup,
			location:      location,
		}

		if _, ok := instancesByRegionAndResourceGroup[batchGroup]; !ok {
			instancesByRegionAndResourceGroup[batchGroup] = make([]*armcompute.VirtualMachine, 0)
		}

		instancesByRegionAndResourceGroup[batchGroup] = append(instancesByRegionAndResourceGroup[batchGroup], vm)
	}

	var instances []*AzureInstances
	for batchGroup, vms := range instancesByRegionAndResourceGroup {
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

// GetInstancesARG fetches Azure virtual machines via Resource Graph. Compared to GetInstances
// (which paginates the SDK ListVirtualMachines API and filters in Go), this path pushes the
// region, resource-group, and OS filters server-side, and — for the Windows matcher — fetches
// each VM's primary private IP via the follow-up NIC query inside the ARG client. Label matching
// against the matcher's ResourceTags still runs in Go: ARG could express tag predicates in KQL,
// but the existing services.MatchLabels supports operator semantics (regex, "in", etc.) that the
// callers configure, so we keep that matching local.
func (f *azureInstanceFetcher) GetInstancesARG(ctx context.Context, _ bool) ([]*AzureInstances, error) {
	azureClients, err := f.AzureClientGetter(ctx, f.IntegrationName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := azureClients.GetResourceGraphClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	osTypes, err := osTypesForMatcher(f.MatcherType)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	vms, err := client.QueryVMs(ctx, azure.QueryVMsParams{
		SubscriptionIDs:         []string{f.Subscription},
		Regions:                 f.Regions,
		ResourceGroups:          []string{f.ResourceGroup},
		OSTypes:                 osTypes,
		IncludePrimaryPrivateIP: f.MatcherType == types.AzureMatcherWindowsVM,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Group by (region, resourceGroup) so the downstream installer batches calls correctly.
	// Region and resource group already came from server-side filtering, but for wildcard
	// matchers the actual VM's location/RG (from ARG's projection) is what defines the batch.
	instancesByRegionAndResourceGroup := make(map[resourceGroupLocation][]azure.DiscoveredVM)
	for _, vm := range vms {
		if match, _, _ := services.MatchLabels(f.Labels, vm.Tags); !match {
			continue
		}
		batchGroup := resourceGroupLocation{
			resourceGroup: vm.ResourceGroup,
			location:      vm.Location,
		}
		instancesByRegionAndResourceGroup[batchGroup] = append(instancesByRegionAndResourceGroup[batchGroup], vm)
	}

	var instances []*AzureInstances
	for batchGroup, vms := range instancesByRegionAndResourceGroup {
		instances = append(instances, &AzureInstances{
			SubscriptionID:      f.Subscription,
			Region:              batchGroup.location,
			ResourceGroup:       batchGroup.resourceGroup,
			DiscoveredVMs:       vms,
			Integration:         f.Integration,
			InstallerParams:     f.InstallerParams,
			DiscoveryConfigName: f.DiscoveryConfigName,
		})
	}

	return instances, nil
}

// osTypesForMatcher maps an Azure matcher type to the OS filter passed to QueryVMs. The Windows
// matcher restricts ARG to Windows VMs; the generic VM matcher restricts to Linux (preserving the
// SDK path's implicit Linux-only behavior — Teleport's run-command installer is bash). Unknown
// matcher types are rejected rather than silently widening the query.
func osTypesForMatcher(matcherType string) ([]string, error) {
	switch matcherType {
	case types.AzureMatcherVM:
		return []string{azure.OSTypeLinux}, nil
	case types.AzureMatcherWindowsVM:
		return []string{azure.OSTypeWindows}, nil
	default:
		return nil, trace.BadParameter("matcher type %q is not supported for Resource Graph VM discovery", matcherType)
	}
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
