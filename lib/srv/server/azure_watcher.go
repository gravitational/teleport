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
	"strings"

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
	// Instances is a list of discovered Azure virtual machines.
	Instances []*armcompute.VirtualMachine
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
func (instances *AzureInstances) MakeUsageEvent(instance *armcompute.VirtualMachine) (string, *usageeventsv1.ResourceCreateEvent) {
	return azureEventPrefix + azure.StringVal(instance.ID), &usageeventsv1.ResourceCreateEvent{
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

type resourceGroupLocation struct {
	resourceGroup string
	location      string
}

// GetInstances fetches Azure virtual machines, applies configured discovery filters,
// drops known non-Linux VMs, and applies best-effort power-state filtering.
//
// OS filtering: VMs with a known non-Linux OS type (e.g. Windows) are excluded; VMs
// with unknown OS type pass through.
//
// Power-state filtering is best-effort: VMs pass through unfiltered when the bulk
// power-state fetch fails for non-cancellation reasons. Context cancellation
// propagates as an error. Only VMs positively identified as non-running are excluded.
func (f *azureInstanceFetcher) GetInstances(ctx context.Context, _ bool) ([]*AzureInstances, error) {
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

	vms = f.filterEligible(ctx, vms)
	vms = f.filterSupportedOS(ctx, vms)
	vms, err = f.filterSupportedPowerState(ctx, client, vms)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return f.emit(ctx, vms), nil
}

// filterEligible returns VMs that satisfy local discovery requirements: non-empty resource ID,
// configured region, matching labels, and for wildcard-RG fetchers, a parseable resource ID.
func (f *azureInstanceFetcher) filterEligible(
	ctx context.Context,
	vms []*armcompute.VirtualMachine,
) []*armcompute.VirtualMachine {
	allowAllRegions := slices.Contains(f.Regions, types.Wildcard)
	allowAllRGs := f.ResourceGroup == types.Wildcard

	kept := make([]*armcompute.VirtualMachine, 0, len(vms))
	for _, vm := range vms {
		resourceID := azure.StringVal(vm.ID)
		if resourceID == "" {
			f.Logger.WarnContext(ctx, "Skipping Azure VM with empty resource ID",
				"subscription_id", f.Subscription,
				"integration", f.Integration,
			)
			continue
		}

		location := azure.StringVal(vm.Location)
		if !allowAllRegions && !slices.Contains(f.Regions, location) {
			continue
		}

		vmTags := make(map[string]string, len(vm.Tags))
		for key, value := range vm.Tags {
			vmTags[key] = azure.StringVal(value)
		}
		if match, _, _ := services.MatchLabels(f.Labels, vmTags); !match {
			continue
		}

		if allowAllRGs {
			if _, err := arm.ParseResourceID(resourceID); err != nil {
				f.Logger.WarnContext(ctx, "Skipping Azure VM because resource group could not be inferred from resource ID",
					"subscription_id", f.Subscription,
					"vm_id", azure.VMID(vm),
					"resource_id", resourceID,
					"error", err,
				)
				continue
			}
		}

		kept = append(kept, vm)
	}

	return kept
}

// filterSupportedOS removes VMs with a known unsupported OS. VMs with unknown
// OS type are kept to avoid dropping Linux VMs due to incomplete metadata.
func (f *azureInstanceFetcher) filterSupportedOS(
	ctx context.Context,
	vms []*armcompute.VirtualMachine,
) []*armcompute.VirtualMachine {
	kept, skipped := azure.FilterLinuxVMs(vms)
	if len(skipped) == 0 {
		return kept
	}

	f.Logger.InfoContext(ctx,
		"Skipping Azure VMs with non-Linux OS type",
		"subscription_id", f.Subscription,
		"resource_group", f.ResourceGroup,
		"integration", f.Integration,
		"matched_vms", len(skipped)+len(kept),
		"non_linux_vms", len(skipped),
	)

	for _, vm := range skipped {
		f.Logger.DebugContext(ctx,
			"Skipping Azure VM with non-Linux OS type",
			"vm_name", azure.StringVal(vm.Name),
			"resource_id", azure.StringVal(vm.ID),
			"os_type", azure.VMOSType(vm),
		)
	}

	return kept
}

// filterSupportedPowerState removes VMs positively identified as non-running.
// On context cancellation it returns the cancellation error; any other lookup
// failure fails open (all VMs kept).
func (f *azureInstanceFetcher) filterSupportedPowerState(
	ctx context.Context,
	client azure.VirtualMachinesClient,
	vms []*armcompute.VirtualMachine,
) ([]*armcompute.VirtualMachine, error) {
	if len(vms) == 0 {
		return vms, nil
	}

	// ARM applies RBAC to the subscription-wide response. Scoped identities only
	// see VMs within their scope; VMs omitted from the response are treated as
	// running or indeterminate by the fail-open logic below.
	rawNonRunning, err := client.ListNonRunningVirtualMachineStates(ctx)
	if err != nil {
		// Do not fail open on context cancellation; callers are shutting down or timed out.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, trace.Wrap(ctxErr)
		}
		f.logPowerFetchFailure(ctx, err)

		return vms, nil
	}

	// Normalize keys for case-insensitive matching. ARM treats resource path segments as
	// case-insensitive; implementations of VirtualMachinesClient may return any casing.
	nonRunning := make(map[string]azure.PowerState, len(rawNonRunning))
	for k, v := range rawNonRunning {
		nonRunning[strings.ToLower(k)] = v
	}

	kept := make([]*armcompute.VirtualMachine, 0, len(vms))
	for _, vm := range vms {
		// VMs not present in the non-running map pass through as running or
		// indeterminate. The method's contract returns only non-running entries.
		state, isNonRunning := nonRunning[strings.ToLower(azure.StringVal(vm.ID))]
		if !isNonRunning {
			kept = append(kept, vm)
			continue
		}

		f.Logger.DebugContext(ctx, "Skipping Azure VM that is not running",
			"vm_name", azure.StringVal(vm.Name),
			"resource_id", azure.StringVal(vm.ID),
			"power_state", string(state),
		)
	}

	f.logPowerFilterSummary(ctx, len(vms), len(nonRunning), len(vms)-len(kept))
	return kept, nil
}

// emit buckets the VM list by (resourceGroup, region) and produces one *AzureInstances
// per non-empty bucket. For wildcard-RG fetchers, the resource group is parsed from each
// VM's resource ID; VMs with unparseable IDs are skipped with a warn log.
func (f *azureInstanceFetcher) emit(ctx context.Context, vms []*armcompute.VirtualMachine) []*AzureInstances {
	if len(vms) == 0 {
		return nil
	}

	byGroup := map[resourceGroupLocation][]*armcompute.VirtualMachine{}
	for _, vm := range vms {
		rg := f.ResourceGroup
		if rg == types.Wildcard {
			resourceID := azure.StringVal(vm.ID)
			meta, err := arm.ParseResourceID(resourceID)
			if err != nil {
				f.Logger.WarnContext(ctx,
					"Skipping Azure VM because resource group could not be inferred during emit",
					"subscription_id", f.Subscription,
					"vm_id", azure.VMID(vm),
					"resource_id", resourceID,
					"error", err,
				)
				continue
			}
			rg = meta.ResourceGroupName
		}

		key := resourceGroupLocation{
			resourceGroup: rg,
			location:      azure.StringVal(vm.Location),
		}

		byGroup[key] = append(byGroup[key], vm)
	}

	out := make([]*AzureInstances, 0, len(byGroup))
	for key, bucket := range byGroup {
		out = append(out, &AzureInstances{
			SubscriptionID:      f.Subscription,
			Region:              key.location,
			ResourceGroup:       key.resourceGroup,
			Instances:           bucket,
			Integration:         f.Integration,
			InstallerParams:     f.InstallerParams,
			DiscoveryConfigName: f.DiscoveryConfigName,
		})
	}

	return out
}

// logPowerFetchFailure emits a warn-level log signaling that power-state filtering was
// skipped because the bulk ARM call failed. AccessDenied errors get an actionable remediation message.
func (f *azureInstanceFetcher) logPowerFetchFailure(ctx context.Context, err error) {
	msg := "Failed to fetch VM power states; skipping power-state filtering"
	if trace.IsAccessDenied(err) {
		msg = "Identity lacks permission to fetch VM power states; skipping power-state filtering. " +
			"Grant Microsoft.Compute/virtualMachines/read at the subscription scope to enable filtering"
	}

	f.Logger.WarnContext(ctx, msg,
		"subscription_id", f.Subscription,
		"resource_group", f.ResourceGroup,
		"integration", f.Integration,
		"error", err,
	)
}

// logPowerFilterSummary emits one per-iteration summary log after power-state filtering.
// The level is info when VMs were skipped, debug otherwise.
func (f *azureInstanceFetcher) logPowerFilterSummary(ctx context.Context, linuxEligible, powerStateLookupEntries, filtered int) {
	level := slog.LevelDebug
	msg := "Azure VM power-state filtering summary"
	if filtered > 0 {
		level = slog.LevelInfo
		msg = "Skipped Azure VMs that are not running"
	}
	f.Logger.LogAttrs(ctx, level, msg,
		slog.String("subscription_id", f.Subscription),
		slog.String("resource_group", f.ResourceGroup),
		slog.String("integration", f.Integration),
		slog.Int("linux_eligible_vms", linuxEligible),
		slog.Int("power_state_lookup_entries", powerStateLookupEntries),
		slog.Int("non_running_vms", filtered),
	)
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
