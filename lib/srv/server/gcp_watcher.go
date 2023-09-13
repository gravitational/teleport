/*
Copyright 2023 Gravitational, Inc.

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
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/installers"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/services"
)

const gcpEventPrefix = "gcp/"

// GCPInstances contains information about discovered GCP virtual machines.
type GCPInstances struct {
	// Zone is the instances' zone.
	Zone string
	// ProjectID is the instances' project ID.
	ProjectID string
	// ScriptName is the name of the script to execute on the instances to
	// install Teleport.
	ScriptName string
	// PublicProxyAddr is the address of the proxy the discovered node should use
	// to connect to the cluster.
	PublicProxyAddr string
	// Parameters are the parameters passed to the installation script
	Parameters []string
	// Instances is a list of discovered GCP virtual machines.
	Instances []*gcp.Instance
}

// MakeEvents generates MakeEvents for these instances.
func (instances *GCPInstances) MakeEvents() map[string]*usageeventsv1.ResourceCreateEvent {
	resourceType := types.DiscoveredResourceNode
	if instances.ScriptName == installers.InstallerScriptNameAgentless {
		resourceType = types.DiscoveredResourceAgentlessNode
	}
	events := make(map[string]*usageeventsv1.ResourceCreateEvent, len(instances.Instances))
	for _, inst := range instances.Instances {
		events[fmt.Sprintf("%s%s/%s", gcpEventPrefix, inst.ProjectID, inst.Name)] = &usageeventsv1.ResourceCreateEvent{
			ResourceType:   resourceType,
			ResourceOrigin: types.OriginCloud,
			CloudProvider:  types.CloudGCP,
		}
	}
	return events
}

// NewGCPWatcher creates a new GCP watcher.
func NewGCPWatcher(ctx context.Context, matchers []types.GCPMatcher, clients cloud.Clients) (*Watcher, error) {
	cancelCtx, cancelFn := context.WithCancel(ctx)
	watcher := Watcher{
		fetchers:     []Fetcher{},
		ctx:          cancelCtx,
		cancel:       cancelFn,
		pollInterval: time.Minute,
		InstancesC:   make(chan Instances),
	}
	client, err := clients.GetGCPInstancesClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, matcher := range matchers {
		watcher.fetchers = append(watcher.fetchers, newGCPInstanceFetcher(gcpFetcherConfig{
			Matcher:   matcher,
			GCPClient: client,
		}))
	}
	return &watcher, nil
}

type gcpFetcherConfig struct {
	Matcher   types.GCPMatcher
	GCPClient gcp.InstancesClient
}

type gcpInstanceFetcher struct {
	GCP             gcp.InstancesClient
	ProjectIDs      []string
	Zones           []string
	ProjectID       string
	ServiceAccounts []string
	Labels          types.Labels
	Parameters      map[string]string
}

func newGCPInstanceFetcher(cfg gcpFetcherConfig) *gcpInstanceFetcher {
	fetcher := &gcpInstanceFetcher{
		GCP:             cfg.GCPClient,
		Zones:           cfg.Matcher.Locations,
		ProjectIDs:      cfg.Matcher.ProjectIDs,
		ServiceAccounts: cfg.Matcher.ServiceAccounts,
		Labels:          cfg.Matcher.Tags,
	}
	if cfg.Matcher.Params != nil {
		fetcher.Parameters = map[string]string{
			"token":           cfg.Matcher.Params.JoinToken,
			"scriptName":      cfg.Matcher.Params.ScriptName,
			"publicProxyAddr": cfg.Matcher.Params.PublicProxyAddr,
		}
	}
	return fetcher
}

func (*gcpInstanceFetcher) GetMatchingInstances(_ []types.Server, _ bool) ([]Instances, error) {
	return nil, trace.NotImplemented("not implemented for gcp fetchers")
}

// GetInstances fetches all GCP virtual machines matching configured filters.
func (f *gcpInstanceFetcher) GetInstances(ctx context.Context, _ bool) ([]Instances, error) {
	// Key by project ID, then by zone.
	instanceMap := make(map[string]map[string][]*gcp.Instance)
	for _, projectID := range f.ProjectIDs {
		instanceMap[projectID] = make(map[string][]*gcp.Instance)
		for _, zone := range f.Zones {
			instanceMap[projectID][zone] = make([]*gcp.Instance, 0)
			vms, err := f.GCP.ListInstances(ctx, projectID, zone)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			filteredVMs := make([]*gcp.Instance, 0, len(vms))
			for _, vm := range vms {
				if len(f.ServiceAccounts) > 0 && !slices.Contains(f.ServiceAccounts, vm.ServiceAccount) {
					continue
				}
				if match, _, _ := services.MatchLabels(f.Labels, vm.Labels); !match {
					continue
				}
				filteredVMs = append(filteredVMs, vm)
			}
			instanceMap[projectID][zone] = filteredVMs
		}
	}

	var instances []Instances
	for projectID, vmsByZone := range instanceMap {
		for zone, vms := range vmsByZone {
			if len(vms) > 0 {
				instances = append(instances, Instances{GCP: &GCPInstances{
					ProjectID:       projectID,
					Zone:            zone,
					Instances:       vms,
					ScriptName:      f.Parameters["scriptName"],
					PublicProxyAddr: f.Parameters["publicProxyAddr"],
					Parameters:      []string{f.Parameters["token"]},
				}})
			}
		}
	}

	return instances, nil
}
