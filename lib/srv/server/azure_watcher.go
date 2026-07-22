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
	"maps"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/installers"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/azure/client"
	"github.com/gravitational/teleport/lib/cloud/azure/network"
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

// networkInterfacesClientGetter returns the Azure network interfaces client for a
// resolved azure.Clients.
type networkInterfacesClientGetter func(ctx context.Context, azureClients azure.Clients) (network.InterfacesClient, error)

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
	// NetworkInterfacesClientGetter returns the Azure network interfaces client used by
	// the Windows VM path to resolve private IPs. It exists so tests can inject a
	// fake. When nil, the fetcher builds the real InterfacesClient.
	NetworkInterfacesClientGetter networkInterfacesClientGetter
}

type azureInstanceFetcher struct {
	InstallerParams           *types.InstallerParams
	AzureClientGetter         azureClientGetter
	Regions                   []string
	Subscription              string
	ResourceGroup             string
	Labels                    types.Labels
	DiscoveryConfigName       string
	Integration               string
	Logger                    *slog.Logger
	MatcherType               string
	osMatches                 func(vm *azure.VirtualMachine) bool
	networkInterfacesClientFn networkInterfacesClientGetter
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
	fetcher.networkInterfacesClientFn = cfg.NetworkInterfacesClientGetter
	if fetcher.networkInterfacesClientFn == nil {
		fetcher.networkInterfacesClientFn = fetcher.newInterfacesClient
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
		nicsByVM, err := f.listNICsByVM(ctx, azureClients, instanceGroups)
		if err != nil {
			return nil, trace.Wrap(err, "listing network interfaces for Windows VM discovery")
		}
		for _, vms := range instanceGroups {
			for _, vm := range vms {
				vm.PrimaryPrivateIP = primaryPrivateIP(nicsByVM[strings.ToLower(vm.ID)])
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
			},
			Instances: vms,
		})
	}

	return instances, nil
}

// listNICsByVM lists the network interfaces in each resource group that
// contains a matched VM, grouped by the lower-cased resource ID of the VM each
// NIC is attached to. It is used by the Windows VM path to resolve each VM's
// private IP.
func (f *azureInstanceFetcher) listNICsByVM(
	ctx context.Context,
	azureClients azure.Clients,
	instanceGroups map[resourceGroupLocation][]*azure.VirtualMachine,
) (map[string][]*network.Interface, error) {
	nicClient, err := f.networkInterfacesClientFn(ctx, azureClients)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resourceGroups := make(map[string]struct{})
	for batchGroup := range instanceGroups {
		resourceGroups[strings.ToLower(batchGroup.resourceGroup)] = struct{}{}
	}

	var nicsByVM = make(map[string][]*network.Interface)
	for resourceGroup := range resourceGroups {
		nics, err := nicClient.List(ctx, f.Subscription, resourceGroup)
		if err != nil {
			return nil, trace.Wrap(err, "listing network interfaces for resource group %q", resourceGroup)
		}
		// We can safely copy the NICs into the map because VMs belong to a single
		// resource group, so there won't be any duplicates.
		maps.Copy(nicsByVM, nics)
	}
	return nicsByVM, nil
}

// newInterfacesClient builds the Azure network interfaces client from the
// shared token credential.
func (f *azureInstanceFetcher) newInterfacesClient(ctx context.Context, azureClients azure.Clients) (network.InterfacesClient, error) {
	cred, err := azureClients.GetCredential(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c, err := client.NewClient(cred,
		client.WithRetryOnRateLimitErrors(),
		client.WithLogger(f.Logger),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return network.NewInterfacesClient(c, network.WithLogger(f.Logger)), nil
}

// primaryPrivateIP returns the private IP to register for a VM given its
// network interfaces. It prefers the IP of the NIC that Azure flagged as
// primary. If no NIC is flagged primary (which would be an Azure error) it
// falls back to the first NIC with a usable private IP.
func primaryPrivateIP(nics []*network.Interface) string {
	var fallback string
	for _, nic := range nics {
		if nic.PrivateIP == "" {
			continue
		}
		if nic.Primary {
			return nic.PrivateIP
		}
		fallback = cmp.Or(fallback, nic.PrivateIP)
	}
	return fallback
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
