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

package server

import (
	"context"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// AzureInstances contains information about discovered Azure virtual machines.
type AzureInstances struct {
	// Region is the Azure region where the instances are located.
	Region string
	// SubscriptionID is the subscription ID for the instances.
	SubscriptionID string
	// ResourceGroup is the resource group for the instances.
	ResourceGroup string
	// Instances is a list of discovered Azure virtual machines.
	Instances []*armcompute.VirtualMachine
}

// AzureWatcher allows callers to discover Azure virtual machines matching specified filters.
type AzureWatcher struct {
	// InstancesC can be used to consume newly discoverd Azure virtual machines.
	InstancesC chan AzureInstances

	fetchers      []*azureInstanceFetcher
	fetchInterval time.Duration
	ctx           context.Context
	cancel        context.CancelFunc
}

// Run starts the watcher's main watch loop.
func (w *AzureWatcher) Run() {
	ticker := time.NewTicker(w.fetchInterval)
	defer ticker.Stop()
	for {
		for _, fetcher := range w.fetchers {
			instancesColl, err := fetcher.GetAzureVMs(w.ctx)
			if err != nil {
				if trace.IsNotFound(err) {
					continue
				}
				log.WithError(err).Error("Failed to fetch Azure VMs")
				continue
			}
			for _, inst := range instancesColl {
				select {
				case w.InstancesC <- inst:
				case <-w.ctx.Done():
				}
			}
		}
		select {
		case <-ticker.C:
			continue
		case <-w.ctx.Done():
			return
		}
	}
}

// Stop stops the watcher.
func (w *AzureWatcher) Stop() {
	w.cancel()
}

// NewAzureWatcher creates a new Azure watcher instance.
func NewAzureWatcher(ctx context.Context, matchers []services.AzureMatcher, clients cloud.Clients) (*AzureWatcher, error) {
	cancelCtx, cancelFn := context.WithCancel(ctx)
	watcher := AzureWatcher{
		fetchers:      []*azureInstanceFetcher{},
		ctx:           cancelCtx,
		cancel:        cancelFn,
		fetchInterval: time.Minute,
		InstancesC:    make(chan AzureInstances, 2),
	}
	for _, matcher := range matchers {
		for _, subscription := range matcher.Subscriptions {
			for _, resourceGroup := range matcher.ResourceGroups {
				cl, err := clients.GetAzureVirtualMachinesClient(subscription)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				fetcher := newAzureInstanceFetcher(azureFetcherConfig{
					Matcher:       matcher,
					Subscription:  subscription,
					ResourceGroup: resourceGroup,
					AzureClient:   cl,
				})
				watcher.fetchers = append(watcher.fetchers, fetcher)
			}
		}
	}
	return &watcher, nil
}

type azureFetcherConfig struct {
	Matcher       services.AzureMatcher
	Subscription  string
	ResourceGroup string
	AzureClient   azure.VirtualMachinesClient
}

type azureInstanceFetcher struct {
	Azure         azure.VirtualMachinesClient
	Regions       []string
	Subscription  string
	ResourceGroup string
	Labels        types.Labels
}

func newAzureInstanceFetcher(cfg azureFetcherConfig) *azureInstanceFetcher {
	return &azureInstanceFetcher{
		Azure:         cfg.AzureClient,
		Regions:       cfg.Matcher.Regions,
		Subscription:  cfg.Subscription,
		ResourceGroup: cfg.ResourceGroup,
		Labels:        cfg.Matcher.ResourceTags,
	}
}

// GetAzureVMs fetches all Azure virtual machines matching configured filters.
func (f *azureInstanceFetcher) GetAzureVMs(ctx context.Context) ([]AzureInstances, error) {
	instancesByRegion := make(map[string][]*armcompute.VirtualMachine)
	for _, region := range f.Regions {
		instancesByRegion[region] = []*armcompute.VirtualMachine{}
	}

	vms, err := f.Azure.ListVirtualMachines(ctx, f.ResourceGroup)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, vm := range vms {
		location := aws.StringValue(vm.Location)
		if _, ok := instancesByRegion[location]; !ok {
			continue
		}
		vmTags := make(map[string]string, len(vm.Tags))
		for key, value := range vm.Tags {
			vmTags[key] = aws.StringValue(value)
		}
		if match, _, _ := services.MatchLabels(f.Labels, vmTags); !match {
			continue
		}
		instancesByRegion[location] = append(instancesByRegion[location], vm)
	}

	var instances []AzureInstances
	for region, vms := range instancesByRegion {
		if len(vms) > 0 {
			instances = append(instances, AzureInstances{
				SubscriptionID: f.Subscription,
				Region:         region,
				ResourceGroup:  f.ResourceGroup,
				Instances:      vms,
			})
		}
	}

	return instances, nil
}
