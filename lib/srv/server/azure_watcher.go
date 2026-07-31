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
	"cmp"
	"context"
	"log/slog"
	"net"
	"slices"
	"strings"

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

// AzureInstancesMetadata contains information about discovered Azure virtual machines.
type AzureInstancesMetadata struct {
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

	// MatcherType is the type of matcher that discovered these instances.
	MatcherType string
}

func (md AzureInstancesMetadata) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("discovery_config", md.DiscoveryConfigName),
		slog.String("integration", md.Integration),
		slog.String("region", md.Region),
		slog.String("resource_group", md.ResourceGroup),
		slog.String("subscription_id", md.SubscriptionID),
	)
}

func (md *AzureInstancesMetadata) resourceType() string {
	if md.InstallerParams != nil && md.InstallerParams.ScriptName == installers.InstallerScriptNameAgentless {
		return types.DiscoveredResourceAgentlessNode
	}
	return types.DiscoveredResourceNode
}

// MakeUsageEvent builds usage event for a single installation result.
func (md *AzureInstancesMetadata) MakeUsageEvent(instance *azure.VirtualMachine) (string, *usageeventsv1.ResourceCreateEvent) {
	return azureEventPrefix + instance.ID, &usageeventsv1.ResourceCreateEvent{
		ResourceType:        md.resourceType(),
		ResourceOrigin:      types.OriginCloud,
		CloudProvider:       types.CloudAzure,
		DiscoveryConfigName: md.DiscoveryConfigName,
	}
}

// MakeRunEvent builds run event for a single command run.
func (md *AzureInstancesMetadata) MakeRunEvent(result AzureInstallResult) *apievents.AzureRun {
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
			SubscriptionID: md.SubscriptionID,
			ResourceGroup:  md.ResourceGroup,
			ResourceID:     resourceID,
			Region:         md.Region,
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

// AzureInstances contains a list of discovered Azure virtual machines and
// metadata.
type AzureInstances struct {
	Metadata AzureInstancesMetadata

	// Instances is a list of discovered Azure virtual machines.
	Instances []*azure.VirtualMachine
}

// LogValue implements [slog.LogValuer].
func (instances *AzureInstances) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("count", len(instances.Instances)),
		slog.Any("metadata", instances.Metadata),
	)
}

// FilterExistingNodes removes instances matching existing nodes in place.
func (instances *AzureInstances) FilterExistingNodes(existingNodes []types.Server) {
	vmIDs := make(map[string]struct{})
	for _, node := range existingNodes {
		if subID := types.GetAzureSubscriptionID(node); subID != instances.Metadata.SubscriptionID {
			continue
		}
		if vmID := types.GetAzureVMID(node); vmID != "" {
			vmIDs[vmID] = struct{}{}
		}
	}

	instances.Instances = slices.DeleteFunc(instances.Instances, func(instance *azure.VirtualMachine) bool {
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
		for _, matcherType := range matcher.Types {
			if matcherType != types.AzureMatcherVM && matcherType != types.AzureMatcherWindowsVM {
				logger.WarnContext(ctx, "Skipping unsupported matcher type", "matcher_type", matcherType)
				continue
			}
			for _, subscription := range matcher.Subscriptions {
				for _, resourceGroup := range matcher.ResourceGroups {
					fetcher := newAzureInstanceFetcher(azureFetcherConfig{
						Matcher:             matcher,
						MatcherType:         matcherType,
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
	osMatches           func(vm *azure.VirtualMachine) bool
}

func newAzureInstanceFetcher(cfg azureFetcherConfig) *azureInstanceFetcher {
	fetcher := &azureInstanceFetcher{
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
	fetcher.osMatches = (*azure.VirtualMachine).IsLinuxOrUnknown
	if cfg.MatcherType == types.AzureMatcherWindowsVM {
		fetcher.osMatches = (*azure.VirtualMachine).IsWindowsOrUnknown
	}
	return fetcher
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

// GetInstances fetches all Azure virtual machines matching configured filters.
func (f *azureInstanceFetcher) GetInstances(ctx context.Context, _ bool) ([]*AzureInstances, error) {
	azureClients, err := f.AzureClientGetter(ctx, f.IntegrationName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	vmClient, err := azureClients.GetVirtualMachinesClient(ctx, f.Subscription)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	vms, err := vmClient.ListVirtualMachines(ctx, f.ResourceGroup)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	instanceGroups := make(map[resourceGroupLocation][]*azure.VirtualMachine)

	allowAllLocations := slices.Contains(f.Regions, types.Wildcard)

	skippedVMIDs := make([]string, 0)
	for _, vm := range vms {
		// Skip VMs where the OS doesn't match this fetcher's matcher type. VMs with
		// an unknown OS are kept because the OS type is not always present in the
		// API response.
		if !f.osMatches(vm) {
			skippedVMIDs = append(skippedVMIDs, vm.ID)
			continue
		}

		if !slices.Contains(f.Regions, vm.Location) && !allowAllLocations {
			continue
		}
		if match, _, _ := services.MatchLabels(f.Labels, vm.Tags); !match {
			continue
		}

		batchGroup := resourceGroupLocation{
			resourceGroup: vm.ResourceGroup,
			location:      vm.Location,
		}

		instanceGroups[batchGroup] = append(instanceGroups[batchGroup], vm)
	}
	if len(skippedVMIDs) > 0 {
		// Show at most 10 skipped VM IDs in the log message to avoid spamming the logs.
		sampleSize := min(len(skippedVMIDs), 10)
		skippedVMIDsSample := make([]string, sampleSize)
		copy(skippedVMIDsSample, skippedVMIDs[:sampleSize])

		f.Logger.DebugContext(ctx, "Skipped VMs with non-matching OS in Azure Server Discovery",
			"fetcher", f,
			"matcher_type", f.MatcherType,
			"total_vms", len(vms),
			"skipped_vms", len(skippedVMIDs),
			"skipped_vms_sample", skippedVMIDsSample,
		)
	}

	// Windows VM discovery needs each VM's private IP to register a dynamic
	// Windows desktop, but the compute API doesn't return it. Resolve the IPs
	// from the VMs' network interfaces and join them to the VMs by resource ID.
	if f.MatcherType == types.AzureMatcherWindowsVM {
		privateIPByVM, err := f.primaryPrivateIPByVM(ctx, azureClients, instanceGroups)
		if err != nil {
			return nil, trace.Wrap(err, "listing network interfaces for Windows VM discovery")
		}
		for _, vms := range instanceGroups {
			for _, vm := range vms {
				vm.PrimaryPrivateIP = privateIPByVM[strings.ToLower(vm.ID)]
			}
		}
	}

	var instances []*AzureInstances
	for batchGroup, vms := range instanceGroups {
		instances = append(instances, &AzureInstances{
			Metadata: AzureInstancesMetadata{
				SubscriptionID:      f.Subscription,
				Region:              batchGroup.location,
				ResourceGroup:       batchGroup.resourceGroup,
				Integration:         f.Integration,
				InstallerParams:     f.InstallerParams,
				DiscoveryConfigName: f.DiscoveryConfigName,
				MatcherType:         f.MatcherType,
			},
			Instances: vms,
		})
	}

	return instances, nil
}

// primaryPrivateIPByVM gets the primary private IP address for each VM.
func (f *azureInstanceFetcher) primaryPrivateIPByVM(
	ctx context.Context,
	azureClients azure.Clients,
	instanceGroups map[resourceGroupLocation][]*azure.VirtualMachine,
) (map[string]string, error) {
	if len(instanceGroups) == 0 {
		return nil, nil
	}

	nicClient, err := azureClients.GetNetworkInterfacesClient(ctx, f.Subscription)
	if err != nil {
		return nil, trace.Wrap(err, "getting network interfaces client")
	}

	// Get a list of scaleSetIDs (this slice is deduped by the NIC client)
	scaleSetIDs := []string{}
	for _, vms := range instanceGroups {
		for _, vm := range vms {
			if vm.UniformScaleSetName != "" {
				id, err := arm.ParseResourceID(vm.ID)
				if err != nil {
					f.Logger.WarnContext(ctx, "Skipping uniform scale set with unparsable resource id", "resource_id", vm.ID)
					continue
				}
				if id.Parent == nil {
					f.Logger.WarnContext(ctx, "Skipping uniform scale set because parent of uniform scale set instance not found", "resource_id", vm.ID)
					continue
				}
				scaleSetIDs = append(scaleSetIDs, id.Parent.String())
			}
		}
	}

	// Gather all NICs for the matched resource groups
	var nicsByAttachedVM = make(map[string][]*azure.NetworkInterface)
	// We have to list NICs in the whole subscription because a NIC can be in a
	// different resource group than the VM it is attached to. There is currently
	// no way to filter this API call by any other means (e.g. location, tags, etc.)
	nics, err := nicClient.ListNetworkInterfaces(ctx, types.Wildcard, scaleSetIDs...)
	if err != nil {
		return nil, trace.Wrap(err, "listing network interfaces")
	}
	for _, nic := range nics {
		// Skip NICs that aren't attached to a VM
		if nic.AttachedVMID == "" {
			continue
		}
		nicsByAttachedVM[strings.ToLower(nic.AttachedVMID)] = append(nicsByAttachedVM[strings.ToLower(nic.AttachedVMID)], nic)
	}

	// For each VM, find the primary private IP
	var privateIPByVM = make(map[string]string)
	for vmId, nics := range nicsByAttachedVM {
		privateIPByVM[vmId] = primaryPrivateIP(nics)
	}
	return privateIPByVM, nil
}

// primaryPrivateIP returns the private IP to register for a VM given its
// network interfaces. It prefers the IP of the NIC and IP config that Azure
// flagged as primary. If no NIC/IP config is flagged primary (which would be an
// Azure error) it falls back to the first NIC with a usable private IP.
func primaryPrivateIP(nics []*azure.NetworkInterface) string {
	var fallback, primaryNICFallback, primaryConfigFallback string
	for _, nic := range nics {
		for _, ipConfig := range nic.IPConfigurations {
			// Some non-primary IP Configurations can have a CIDR address as their
			// private IP, so skip them.
			if net.ParseIP(ipConfig.PrivateIP) == nil {
				continue
			}
			// If the NIC is primary and the IP configuration is primary,
			// return that IP.
			if nic.Primary && ipConfig.Primary {
				return ipConfig.PrivateIP
			}
			// If the NIC is primary but the IP configuration is not, save it as a
			// fallback in case no primary IP configuration is found.
			if nic.Primary && primaryNICFallback == "" {
				primaryNICFallback = ipConfig.PrivateIP
			}
			// If the IP configuration is primary but the NIC is not, save it as a
			// fallback in case no primary NIC is found.
			if ipConfig.Primary && primaryConfigFallback == "" {
				primaryConfigFallback = ipConfig.PrivateIP
			}
			if fallback == "" {
				fallback = ipConfig.PrivateIP
			}
		}
	}
	return cmp.Or(primaryNICFallback, primaryConfigFallback, fallback)
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
