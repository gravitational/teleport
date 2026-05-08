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

const (
	azureEventPrefix        = "azure/"
	azureScopeLogSampleSize = 10

	azureARGStageClientInit = "resource_graph_client_init"
	azureARGStageQuery      = "resource_graph_query"
)

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
	Instances []azure.DiscoveredVM
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
func (instances *AzureInstances) MakeUsageEvent(instance azure.DiscoveredVM) (string, *usageeventsv1.ResourceCreateEvent) {
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

	evt := &apievents.AzureRun{
		Metadata: apievents.Metadata{
			Type: libevents.AzureRunEvent,
			Code: eventCode,
		},
		AzureMetadata: apievents.AzureMetadata{
			SubscriptionID: instances.SubscriptionID,
			ResourceGroup:  instances.ResourceGroup,
			ResourceID:     result.Instance.ID,
			Region:         instances.Region,
		},
		AzureVMMetadata: apievents.AzureVMMetadata{
			VMID:   result.Instance.VMID,
			VMName: result.Instance.Name,
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

	instances.Instances = slices.DeleteFunc(instances.Instances, func(vm azure.DiscoveredVM) bool {
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
		if len(matcher.Subscriptions) == 0 {
			continue
		}
		fetcher := newAzureInstanceFetcher(azureFetcherConfig{
			Matcher:             matcher,
			Subscriptions:       matcher.Subscriptions,
			ResourceGroups:      matcher.ResourceGroups,
			AzureClientGetter:   getClient,
			DiscoveryConfigName: discoveryConfigName,
			Logger:              logger,
		})
		ret = append(ret, fetcher)
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
	Subscriptions       []string
	ResourceGroups      []string
	AzureClientGetter   azureClientGetter
	DiscoveryConfigName string
	Logger              *slog.Logger
}

// azureInstanceFetcher fetches Azure VMs matching a single Azure matcher.
type azureInstanceFetcher struct {
	InstallerParams     *types.InstallerParams
	AzureClientGetter   azureClientGetter
	Regions             []string
	Subscriptions       []string
	ResourceGroups      []string
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
		Subscriptions:       cfg.Subscriptions,
		ResourceGroups:      cfg.ResourceGroups,
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

// argBucketKey is the bucketing key for the ARG path. ARG returns rows
// flat across all queried subscriptions, so subscription is part of the key.
type argBucketKey struct {
	subscription  string
	resourceGroup string
	location      string
}

// armBucketKey is the bucketing key for the ARM fallback path. ARM
// buckets per-subscription, so subscription is not part of the key.
type armBucketKey struct {
	resourceGroup string
	location      string
}

// GetInstances fetches Azure virtual machines via Azure Resource Graph, applies label matching,
// and emits one AzureInstances bucket per (subscription, resource group, region) tuple.
//
// ARG filters out non-Linux and non-running VMs at query time. If ARG fails, GetInstances
// falls back to ARM VM listing, which applies region and label filters only.

func (f *azureInstanceFetcher) GetInstances(ctx context.Context, _ bool) ([]*AzureInstances, error) {
	azureClients, err := f.AzureClientGetter(ctx, f.IntegrationName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	argClient, err := azureClients.GetResourceGraphClient(ctx)
	if err != nil {
		return f.getInstancesARMFallback(ctx, azureClients,
			azureARGStageClientInit,
			"Azure Resource Graph client initialization failed",
			err)
	}

	instances, err := f.getInstancesARG(ctx, argClient)
	if err == nil {
		return instances, nil
	}

	return f.getInstancesARMFallback(ctx, azureClients,
		azureARGStageQuery,
		"Azure Resource Graph VM query failed",
		err)
}

func azureScopeLogFields(subscriptions, resourceGroups []string) []any {
	return []any{
		"subscription_count", len(subscriptions),
		"subscription_sample", azureScopeLogSample(subscriptions),
		"subscription_omitted", max(len(subscriptions)-azureScopeLogSampleSize, 0),
		"resource_group_count", len(resourceGroups),
		"resource_group_sample", azureScopeLogSample(resourceGroups),
		"resource_group_omitted", max(len(resourceGroups)-azureScopeLogSampleSize, 0),
	}
}

func azureScopeLogSample(values []string) []string {
	return slices.Clone(values[:min(len(values), azureScopeLogSampleSize)])
}

func (f *azureInstanceFetcher) getInstancesARMFallback(ctx context.Context, azureClients azure.Clients, argStage, argFailure string, argErr error) ([]*AzureInstances, error) {
	if err := ctx.Err(); err != nil {
		return nil, trace.Wrap(err)
	}

	attrs := azureScopeLogFields(f.Subscriptions, f.ResourceGroups)
	attrs = append(attrs,
		"stage", argStage,
		"integration", f.Integration,
		"fallback", "arm_vm_listing",
		"error", argErr,
	)

	switch argStage {
	case azureARGStageClientInit:
		f.Logger.WarnContext(ctx, "Azure Resource Graph client initialization failed; falling back to ARM VM listing", attrs...)
	case azureARGStageQuery:
		f.Logger.WarnContext(ctx, "Azure Resource Graph VM query failed; falling back to ARM VM listing", attrs...)
	default:
		f.Logger.WarnContext(ctx, "Azure Resource Graph VM discovery failed; falling back to ARM VM listing", attrs...)
	}

	instances, fallbackErr := f.getInstancesARM(ctx, azureClients.GetVirtualMachinesClient)
	if fallbackErr != nil {
		return nil, trace.NewAggregate(
			trace.Wrap(argErr, "%s: %s", argStage, argFailure),
			trace.Wrap(fallbackErr, "ARM VM listing fallback failed"),
		)
	}

	return instances, nil
}

func (f *azureInstanceFetcher) getInstancesARG(ctx context.Context, client azure.ResourceGraphClient) ([]*AzureInstances, error) {
	vms, err := client.QueryVMs(ctx, azure.QueryVMsParams{
		SubscriptionIDs: f.Subscriptions,
		Regions:         f.Regions,
		ResourceGroups:  f.ResourceGroups,
	})
	if err != nil {
		return nil, trace.Wrap(err, "querying Azure Resource Graph for VMs")
	}

	queryMatched := len(vms)
	vms = f.filterByLabels(ctx, vms)
	if queryMatched == 0 || len(vms) == 0 {
		attrs := azureScopeLogFields(f.Subscriptions, f.ResourceGroups)
		attrs = append(attrs,
			"regions", f.Regions,
			"integration", f.Integration,
			"arg_matched", queryMatched,
			"label_matched", len(vms),
		)

		switch {
		case queryMatched == 0 && azureLabelsMatchAll(f.Labels):
			f.Logger.InfoContext(ctx, "Azure Resource Graph VM discovery returned no VMs for match-all Azure VM matcher", attrs...)
		case queryMatched == 0:
			f.Logger.DebugContext(ctx, "Azure Resource Graph VM discovery returned no VMs before label filtering", attrs...)
		case len(vms) == 0:
			f.Logger.DebugContext(ctx, "Azure Resource Graph VM discovery returned no VMs after label filtering", attrs...)
		}
	}

	return f.bucket(vms), nil
}

func azureLabelsMatchAll(labels types.Labels) bool {
	if len(labels) == 0 {
		return true
	}
	values, ok := labels[types.Wildcard]
	return ok && len(labels) == 1 && len(values) == 1 && values[0] == types.Wildcard
}

// getInstancesARM is the ARM fallback path used when ARG VM discovery fails. It best-effort
// iterates over the configured (subscription, resource group) pairs, applies region and label
// filters, and emits one AzureInstances bucket per (subscription, resource group, region) tuple.
// Failures are isolated to the failing ARM scope.
func (f *azureInstanceFetcher) getInstancesARM(
	ctx context.Context,
	getVMClient func(ctx context.Context, subscription string) (azure.VirtualMachinesClient, error),
) ([]*AzureInstances, error) {
	resourceGroups := f.ResourceGroups
	if len(resourceGroups) == 0 {
		resourceGroups = []string{types.Wildcard}
	}

	regions := f.Regions
	if len(regions) == 0 {
		regions = []string{types.Wildcard}
	}

	allowAllLocations := slices.Contains(regions, types.Wildcard)

	var out []*AzureInstances
	var errs []error
	var clientInitErrs []struct {
		subscription string
		err          error
	}
	successfulScopes := 0
	for _, subscription := range f.Subscriptions {
		client, err := getVMClient(ctx, subscription)
		if err != nil {
			clientInitErrs = append(clientInitErrs, struct {
				subscription string
				err          error
			}{subscription: subscription, err: err})
			errs = append(errs, trace.Wrap(err, "getting Azure VM client for subscription %q", subscription))
			continue
		}

		for _, configuredRG := range resourceGroups {
			allowAllResourceGroups := configuredRG == types.Wildcard

			vms, err := client.ListVirtualMachines(ctx, configuredRG)
			if err != nil {
				errs = append(errs, trace.Wrap(err, "listing Azure VMs in subscription %q resource group %q", subscription, configuredRG))
				continue
			}
			successfulScopes++

			byGroup := map[armBucketKey][]azure.DiscoveredVM{}
			for _, vm := range vms {
				location := azure.StringVal(vm.Location)
				if !slices.Contains(regions, location) && !allowAllLocations {
					continue
				}

				vmTags := make(map[string]string, len(vm.Tags))
				for key, value := range vm.Tags {
					vmTags[key] = azure.StringVal(value)
				}
				match, _, err := services.MatchLabels(f.Labels, vmTags)
				if err != nil {
					f.Logger.DebugContext(ctx, "Skipping Azure VM due to malformed labels matcher",
						"vm_name", azure.StringVal(vm.Name),
						"resource_id", azure.StringVal(vm.ID),
						"error", err,
					)
					continue
				}
				if !match {
					continue
				}

				resourceGroup := configuredRG
				if allowAllResourceGroups {
					resourceMetadata, err := arm.ParseResourceID(azure.StringVal(vm.ID))
					if err != nil {
						f.Logger.WarnContext(ctx, "Skipping Teleport installation on Azure VM - failed to infer resource group from vm id",
							"subscription_id", subscription,
							"vm_id", azure.VMID(vm),
							"resource_id", azure.StringVal(vm.ID),
							"error", err,
						)
						continue
					}
					resourceGroup = resourceMetadata.ResourceGroupName
				}

				key := armBucketKey{
					resourceGroup: resourceGroup,
					location:      location,
				}
				byGroup[key] = append(byGroup[key], azure.DiscoveredVM{
					ID:             azure.StringVal(vm.ID),
					SubscriptionID: subscription,
					Name:           azure.StringVal(vm.Name),
					VMID:           azure.VMID(vm),
					Location:       location,
					ResourceGroup:  resourceGroup,
					Tags:           vmTags,
				})
			}

			for key, bucket := range byGroup {
				out = append(out, &AzureInstances{
					SubscriptionID:      subscription,
					Region:              key.location,
					ResourceGroup:       key.resourceGroup,
					Instances:           bucket,
					Integration:         f.Integration,
					InstallerParams:     f.InstallerParams,
					DiscoveryConfigName: f.DiscoveryConfigName,
				})
			}
		}
	}

	if len(errs) > 0 {
		if successfulScopes == 0 && len(clientInitErrs) > 1 && len(clientInitErrs) == len(f.Subscriptions) {
			firstErr := clientInitErrs[0].err
			allSame := true
			for _, clientErr := range clientInitErrs[1:] {
				if clientErr.err.Error() != firstErr.Error() {
					allSame = false
					break
				}
			}
			if allSame {
				return nil, trace.Wrap(firstErr,
					"getting Azure VM clients failed for all %d subscriptions (first subscription %q)",
					len(clientInitErrs), clientInitErrs[0].subscription)
			}
		}

		aggErr := trace.NewAggregate(errs...)
		if successfulScopes == 0 {
			return nil, trace.Wrap(aggErr)
		}
		attrs := azureScopeLogFields(f.Subscriptions, f.ResourceGroups)
		attrs = append(attrs,
			"integration", f.Integration,
			"successful_scopes", successfulScopes,
			"failed_scopes", len(errs),
			"error", aggErr,
		)
		f.Logger.WarnContext(ctx, "Azure ARM VM listing fallback skipped some subscription/resource group scopes", attrs...)
	}

	return out, nil
}

// filterByLabels keeps only VMs whose tags satisfy the configured label matcher.
func (f *azureInstanceFetcher) filterByLabels(ctx context.Context, vms []azure.DiscoveredVM) []azure.DiscoveredVM {
	if len(f.Labels) == 0 {
		return vms
	}

	kept := make([]azure.DiscoveredVM, 0, len(vms))
	skipped := 0
	for _, vm := range vms {
		match, _, err := services.MatchLabels(f.Labels, vm.Tags)
		if err != nil {
			f.Logger.DebugContext(ctx, "Skipping Azure VM due to malformed labels matcher",
				"vm_name", vm.Name,
				"resource_id", vm.ID,
				"error", err,
			)
			skipped++
			continue
		}
		if !match {
			skipped++
			continue
		}
		kept = append(kept, vm)
	}

	if skipped > 0 {
		attrs := azureScopeLogFields(f.Subscriptions, f.ResourceGroups)
		attrs = append(attrs,
			"integration", f.Integration,
			"matched", len(kept),
			"skipped", skipped,
		)
		f.Logger.DebugContext(ctx, "Filtered Azure VMs by label matcher", attrs...)
	}

	return kept
}

// bucket groups VMs by subscription, resource group, and location.
func (f *azureInstanceFetcher) bucket(vms []azure.DiscoveredVM) []*AzureInstances {
	byGroup := map[argBucketKey][]azure.DiscoveredVM{}
	for _, vm := range vms {
		key := argBucketKey{
			subscription:  vm.SubscriptionID,
			resourceGroup: vm.ResourceGroup,
			location:      vm.Location,
		}
		byGroup[key] = append(byGroup[key], vm)
	}

	out := make([]*AzureInstances, 0, len(byGroup))
	for key, bucket := range byGroup {
		out = append(out, &AzureInstances{
			SubscriptionID:      key.subscription,
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

// LogValue implements [slog.LogValuer].
func (f *azureInstanceFetcher) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Any("labels", f.Labels),
		slog.Any("regions", f.Regions),
		slog.String("discovery_config", f.GetDiscoveryConfigName()),
		slog.String("integration", f.IntegrationName()),
		slog.Any("resource_groups", f.ResourceGroups),
		slog.Any("subscriptions", f.Subscriptions),
	)
}
