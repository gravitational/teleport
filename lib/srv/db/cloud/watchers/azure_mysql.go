/*
Copyright 2021 Gravitational, Inc.

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

package watchers

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// azureMySQLFetcherConfig is the Azure MySQL databases fetcher configuration.
type azureMySQLFetcherConfig struct {
	// Labels is a selector to match cloud databases.
	Labels types.Labels
	// Client is the Azure resource manager API client.
	Client *armmysql.ServersClient
	// regions is the Azure regions to filter databases.
	Regions []string
	// regionSet is the Azure regions to filter databases, as a hashset for efficient lookup.
	regionSet map[string]struct{}
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *azureMySQLFetcherConfig) CheckAndSetDefaults() error {
	if len(c.Labels) == 0 {
		return trace.BadParameter("missing parameter Labels")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	if len(c.Regions) == 0 {
		return trace.BadParameter("missing parameter Regions")
	}
	if len(c.regionSet) == 0 {
		c.regionSet = utils.StringsSet(c.Regions)
	}
	return nil
}

// assert azureMySQLFetcher implements Fetcher
var _ Fetcher = (*azureMySQLFetcher)(nil)

// azureMySQLFetcher retrieves Azure MySQL single-server databases.
type azureMySQLFetcher struct {
	cfg azureMySQLFetcherConfig
	log logrus.FieldLogger
}

// newAzureMySQLFetcher returns a new Azure MySQL servers fetcher instance.
func newAzureMySQLFetcher(config azureMySQLFetcherConfig) (Fetcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &azureMySQLFetcher{
		cfg: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "watch:azuremysql",
			"labels":        config.Labels,
			"regions":       config.Regions,
		}),
	}, nil
}

// Get returns Azure MySQL servers matching the watcher's selectors.
func (f *azureMySQLFetcher) Get(ctx context.Context) (types.Databases, error) {
	databases, err := f.getDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return filterDatabasesByLabels(databases, f.cfg.Labels, f.log), nil
}

// getDatabases returns a list of database resources representing Azure database servers.
func (f *azureMySQLFetcher) getDatabases(ctx context.Context) (types.Databases, error) {
	var servers []*armmysql.Server
	options := &armmysql.ServersClientListOptions{}
	pager := f.cfg.Client.NewListPager(options)
	for pageNum := 0; pageNum <= maxPages && pager.More(); pageNum++ {
		res, err := pager.NextPage(ctx)
		if err != nil {
			// TODO(gavin): convert from azure error to trace error
			return nil, trace.Wrap(err)
		}
		servers = append(servers, res.Value...)
	}

	databases := make(types.Databases, 0, len(servers))
	for _, server := range servers {
		// azure sdk provides no way to query by region, so we have to filter results
		location := stringVal(server.Location)
		if _, ok := f.cfg.regionSet[location]; !ok {
			continue
		}

		name := stringVal(server.Name)
		var version armmysql.ServerVersion
		if server.Properties != nil && server.Properties.Version != nil {
			version = *server.Properties.Version
		}
		if !services.IsAzureMySQLVersionSupported(version) {
			f.log.Debugf("Azure server %q (version %v) doesn't support IAM authentication. Skipping.",
				name,
				version)
			continue
		}

		var state armmysql.ServerState
		if server.Properties != nil && server.Properties.UserVisibleState != nil {
			state = *server.Properties.UserVisibleState
		}
		if !services.IsAzureMySQLServerAvailable(state) {
			f.log.Debugf("The current status of Azure server %q is %q. Skipping.",
				name,
				state)
			continue
		}

		database, err := services.NewDatabaseFromAzureMySQLServer(server)
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
func (f *azureMySQLFetcher) String() string {
	return fmt.Sprintf("azureMySQLFetcher(Region=%v, Labels=%v)",
		f.cfg.Regions, f.cfg.Labels)
}

func stringVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
