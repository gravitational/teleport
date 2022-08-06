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

package azure

import (
	"context"
	"fmt"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewAzureFetcher returns a Azure DB server fetcher for the provided subscription, group, regions, and tags.
func NewAzureFetcher(client azure.AzureClient, group string, regions []string, tags types.Labels) (*azureFetcher, error) {
	config := azureFetcherConfig{
		Client:        client,
		ResourceGroup: group,
		Labels:        tags,
		Regions:       utils.StringsSet(regions),
	}
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	fetcher := &azureFetcher{
		cfg: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: fmt.Sprintf("watch:azure:%v", client.Kind()),
			"labels":        config.Labels,
			"regions":       utils.StringsSliceFromSet(config.Regions),
			"group":         config.ResourceGroup,
			"subscription":  client.Subscription(),
		}),
	}
	fetcher.log.Errorf("HERE: testing logger")
	return fetcher, nil
}

// azureFetcherConfig is the Azure MySQL databases fetcher configuration.
type azureFetcherConfig struct {
	// Client is the Azure API client.
	Client azure.AzureClient
	// ResourceGroup is a selector to match cloud resource group.
	ResourceGroup string
	// Labels is a selector to match cloud databases.
	Labels types.Labels
	// regions is the Azure regions to filter databases.
	Regions map[string]struct{}
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *azureFetcherConfig) CheckAndSetDefaults() error {
	if len(c.ResourceGroup) == 0 {
		return trace.BadParameter("missing parameter ResourceGroup")
	}
	if len(c.Labels) == 0 {
		return trace.BadParameter("missing parameter Labels")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	if len(c.Regions) == 0 {
		return trace.BadParameter("missing parameter Regions")
	}
	return nil
}

// azureFetcher retrieves Azure DB single-server databases.
type azureFetcher struct {
	cfg azureFetcherConfig
	log logrus.FieldLogger
}

// Get returns Azure DB servers matching the watcher's selectors.
func (f *azureFetcher) Get(ctx context.Context) (types.Databases, error) {
	databases, err := f.getDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return common.FilterDatabasesByLabels(databases, f.cfg.Labels, f.log), nil
}

// getDatabases returns a list of database resources representing Azure database servers.
func (f *azureFetcher) getDatabases(ctx context.Context) (types.Databases, error) {
	servers, err := f.cfg.Client.ListServers(ctx, f.cfg.ResourceGroup, common.MaxPages)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	databases := make(types.Databases, 0, len(servers))
	for _, server := range servers {
		if server == nil {
			continue
		}
		// azure sdk provides no way to query by region, so we have to filter results
		region := server.GetRegion()
		if _, ok := f.cfg.Regions[region]; !ok {
			continue
		}

		if !server.IsVersionSupported() {
			f.log.Debugf("Azure server %q (version %v) does not support AAD authentication. Skipping.",
				server.GetName(),
				server.GetVersion())
			continue
		}

		if !server.IsAvailable() {
			f.log.Debugf("The current status of Azure server %q is %q. Skipping.",
				server.GetName(),
				server.GetVersion())
			continue
		}

		database, err := services.NewDatabaseFromAzureDBServer(server)
		if err != nil {
			f.log.Warnf("Could not convert Azure server %q to database resource: %v.",
				server.GetName(),
				err)
		} else {
			databases = append(databases, database)
		}
	}
	return databases, nil
}

// String returns the fetcher's string description.
func (f *azureFetcher) String() string {
	return fmt.Sprintf("azureFetcher(Kind=%v, Subscription=%v, ResourceGroup=%v, Region=%v, Labels=%v)",
		f.cfg.Client.Kind(), f.cfg.Client.Subscription(), f.cfg.ResourceGroup, f.cfg.Regions, f.cfg.Labels)
}
