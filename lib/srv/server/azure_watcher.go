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
	"slices"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/gravitational/trace"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/installers"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/services"
)

const azureEventPrefix = "azure/"

// AzureInstances contains information about discovered Azure virtual machines.
type AzureInstances struct {
	// Region is the Azure region where the instances are located.
	Region string
	// SubscriptionID is the subscription ID for the instances.
	SubscriptionID string
	// ResourceGroup is the resource group for the instances.
	ResourceGroup string
	// ScriptName is the name of the script to execute on the instances to
	// install Teleport.
	ScriptName string
	// PublicProxyAddr is the address of the proxy the discovered node should use
	// to connect to the cluster.
	PublicProxyAddr string
	// Parameters are the parameters passed to the installation script.
	Parameters []string
	// Instances is a list of discovered Azure virtual machines.
	Instances []*armcompute.VirtualMachine
	// ClientID is the client ID of the managed identity to use for installation.
	ClientID string
}

// MakeEvents generates MakeEvents for these instances.
func (instances *AzureInstances) MakeEvents() map[string]*usageeventsv1.ResourceCreateEvent {
	resourceType := types.DiscoveredResourceNode
	if instances.ScriptName == installers.InstallerScriptNameAgentless {
		resourceType = types.DiscoveredResourceAgentlessNode
	}
	events := make(map[string]*usageeventsv1.ResourceCreateEvent, len(instances.Instances))
	for _, inst := range instances.Instances {
		events[azureEventPrefix+azure.StringVal(inst.ID)] = &usageeventsv1.ResourceCreateEvent{
			ResourceType:   resourceType,
			ResourceOrigin: types.OriginCloud,
			CloudProvider:  types.CloudAzure,
		}
	}
	return events
}

type azureClientGetter interface {
	GetAzureVirtualMachinesClient(subscription string) (azure.VirtualMachinesClient, error)
}

// NewAzureWatcher creates a new Azure watcher instance.
func NewAzureWatcher(ctx context.Context, fetchersFn func() []Fetcher, opts ...Option) (*Watcher, error) {
	cancelCtx, cancelFn := context.WithCancel(ctx)
	watcher := Watcher{
		fetchersFn:    fetchersFn,
		ctx:           cancelCtx,
		cancel:        cancelFn,
		pollInterval:  time.Minute,
		triggerFetchC: make(<-chan struct{}),
		InstancesC:    make(chan Instances),
	}
	for _, opt := range opts {
		opt(&watcher)
	}
	return &watcher, nil
}

// MatchersToAzureInstanceFetchers converts a list of Azure VM Matchers into a list of Azure VM Fetchers.
func MatchersToAzureInstanceFetchers(matchers []types.AzureMatcher, clients azureClientGetter, discoveryConfigName string) []Fetcher {
	ret := make([]Fetcher, 0)
	for _, matcher := range matchers {
		for _, subscription := range matcher.Subscriptions {
			for _, resourceGroup := range matcher.ResourceGroups {
				fetcher := newAzureInstanceFetcher(azureFetcherConfig{
					Matcher:             matcher,
					Subscription:        subscription,
					ResourceGroup:       resourceGroup,
					AzureClientGetter:   clients,
					DiscoveryConfigName: discoveryConfigName,
				})
				ret = append(ret, fetcher)
			}
		}
	}
	return ret
}

type azureFetcherConfig struct {
	Matcher             types.AzureMatcher
	Subscription        string
	ResourceGroup       string
	AzureClientGetter   azureClientGetter
	DiscoveryConfigName string
	Integration         string
}

type azureInstanceFetcher struct {
	AzureClientGetter   azureClientGetter
	Regions             []string
	Subscription        string
	ResourceGroup       string
	Labels              types.Labels
	Parameters          map[string]string
	ClientID            string
	DiscoveryConfigName string
	Integration         string
}

func newAzureInstanceFetcher(cfg azureFetcherConfig) *azureInstanceFetcher {
	ret := &azureInstanceFetcher{
		AzureClientGetter:   cfg.AzureClientGetter,
		Regions:             cfg.Matcher.Regions,
		Subscription:        cfg.Subscription,
		ResourceGroup:       cfg.ResourceGroup,
		Labels:              cfg.Matcher.ResourceTags,
		DiscoveryConfigName: cfg.DiscoveryConfigName,
		Integration:         cfg.Integration,
	}

	if cfg.Matcher.Params != nil {
		ret.Parameters = map[string]string{
			"token":           cfg.Matcher.Params.JoinToken,
			"scriptName":      cfg.Matcher.Params.ScriptName,
			"publicProxyAddr": cfg.Matcher.Params.PublicProxyAddr,
		}
		ret.ClientID = cfg.Matcher.Params.Azure.ClientID
	}

	return ret
}

func (*azureInstanceFetcher) GetMatchingInstances(_ []types.Server, _ bool) ([]Instances, error) {
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

// GetInstances fetches all Azure virtual machines matching configured filters.
func (f *azureInstanceFetcher) GetInstances(ctx context.Context, _ bool) ([]Instances, error) {
	client, err := f.AzureClientGetter.GetAzureVirtualMachinesClient(f.Subscription)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	instancesByRegion := make(map[string][]*armcompute.VirtualMachine)
	allowAllRegions := slices.Contains(f.Regions, types.Wildcard)
	if !allowAllRegions {
		for _, region := range f.Regions {
			instancesByRegion[region] = []*armcompute.VirtualMachine{}
		}
	}

	vms, err := client.ListVirtualMachines(ctx, f.ResourceGroup)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, vm := range vms {
		location := azure.StringVal(vm.Location)
		if _, ok := instancesByRegion[location]; !ok && !allowAllRegions {
			continue
		}
		vmTags := make(map[string]string, len(vm.Tags))
		for key, value := range vm.Tags {
			vmTags[key] = azure.StringVal(value)
		}
		if match, _, _ := services.MatchLabels(f.Labels, vmTags); !match {
			continue
		}
		instancesByRegion[location] = append(instancesByRegion[location], vm)
	}

	var instances []Instances
	for region, vms := range instancesByRegion {
		if len(vms) > 0 {
			instances = append(instances, Instances{Azure: &AzureInstances{
				SubscriptionID:  f.Subscription,
				Region:          region,
				ResourceGroup:   f.ResourceGroup,
				Instances:       vms,
				ScriptName:      f.Parameters["scriptName"],
				PublicProxyAddr: f.Parameters["publicProxyAddr"],
				Parameters:      []string{f.Parameters["token"]},
				ClientID:        f.ClientID,
			}})
		}
	}

	return instances, nil
}
