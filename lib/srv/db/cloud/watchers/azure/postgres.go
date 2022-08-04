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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewAzurePostgresFetcher returns a Azure Postgres server fetcher for the provided subscription, group, regions, and tags.
func NewAzurePostgresFetcher(cs common.CloudClients, cred azcore.TokenCredential, sub, group string, regions []string, tags types.Labels) (*PostgresFetcher, error) {
	client, err := cs.GetAzurePostgresClient(sub, cred)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := postgresFetcherConfig{
		Client:        client,
		ResourceGroup: group,
		Labels:        tags,
		Regions:       utils.StringsSet(regions),
	}
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	fetcher := &PostgresFetcher{
		cfg: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "watch:azurepostgres",
			"labels":        config.Labels,
			"regions":       config.Regions,
		}),
	}
	return fetcher, nil
}

// postgresFetcherConfig is the Azure Postgres databases fetcher configuration.
type postgresFetcherConfig struct {
	// Client is the Azure API client.
	Client common.AzurePostgresClient
	// ResourceGroup is a selector to match cloud resource group.
	ResourceGroup string
	// Labels is a selector to match cloud databases.
	Labels types.Labels
	// regions is the Azure regions to filter databases.
	Regions map[string]struct{}
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *postgresFetcherConfig) CheckAndSetDefaults() error {
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

// PostgresFetcher retrieves Azure Postgres single-server databases.
type PostgresFetcher struct {
	cfg postgresFetcherConfig
	log logrus.FieldLogger
}

// Get returns Azure Postgres servers matching the watcher's selectors.
func (f *PostgresFetcher) Get(ctx context.Context) (types.Databases, error) {
	databases, err := f.getDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return common.FilterDatabasesByLabels(databases, f.cfg.Labels, f.log), nil
}

// getDatabases returns a list of database resources representing Azure database servers.
func (f *PostgresFetcher) getDatabases(ctx context.Context) (types.Databases, error) {
	servers, err := f.cfg.Client.ListServers(ctx, f.cfg.ResourceGroup)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	databases := make(types.Databases, 0, len(servers))
	for _, server := range servers {
		if server == nil {
			continue
		}

		// azure sdk provides no way to query by region, so we have to filter results
		location := stringVal(server.Location)
		if _, ok := f.cfg.Regions[location]; !ok {
			continue
		}

		name := stringVal(server.Name)
		var version armpostgresql.ServerVersion
		var state armpostgresql.ServerState
		if server.Properties != nil {
			if server.Properties.Version != nil {
				version = *server.Properties.Version
			}
			if server.Properties.UserVisibleState != nil {
				state = *server.Properties.UserVisibleState
			}
		}
		if !services.IsAzurePostgresVersionSupported(version) {
			f.log.Debugf("Azure server %q (version %v) does not support AAD authentication. Skipping.",
				name,
				version)
			continue
		}

		if !services.IsAzurePostgresServerAvailable(state) {
			f.log.Debugf("The current status of Azure server %q is %q. Skipping.",
				name,
				state)
			continue
		}

		database, err := services.NewDatabaseFromAzurePostgresServer(server)
		if err != nil {
			f.log.Warnf("Could not convert Azure server %q to database resource: %v.",
				name,
				err)
		} else {
			databases = append(databases, database)
		}
	}
	return databases, nil
}

// String returns the fetcher's string description.
func (f *PostgresFetcher) String() string {
	return fmt.Sprintf("azurePostgresFetcher(Region=%v, Labels=%v)",
		f.cfg.Regions, f.cfg.Labels)
}
