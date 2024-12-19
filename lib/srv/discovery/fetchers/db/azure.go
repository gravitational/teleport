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

package db

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	azureutils "github.com/gravitational/teleport/api/utils/azure"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/utils"
)

// azureListClient defines an interface for a common Azure client that can list
// Azure database resources.
type azureListClient[DBType comparable] interface {
	// ListAll returns all Azure DB servers within an Azure subscription.
	ListAll(ctx context.Context) ([]DBType, error)
	// ListWithinGroup returns all Azure DB servers within an Azure resource group.
	ListWithinGroup(ctx context.Context, group string) ([]DBType, error)
}

// azureFetcherPlugin defines an interface that provides DBType specific
// functions that can be used by the common Azure fetcher.
type azureFetcherPlugin[DBType comparable, ListClient azureListClient[DBType]] interface {
	// GetListClient returns the azureListClient for the provided subscription.
	GetListClient(cfg *azureFetcherConfig, subID string) (ListClient, error)
	// GetServerLocation returns the server location.
	GetServerLocation(server DBType) string
	// NewDatabaseFromServer creates a types.Database from provided server.
	NewDatabaseFromServer(ctx context.Context, server DBType, logger *slog.Logger) types.Database
}

// newAzureFetcher returns a Azure DB server fetcher for the provided subscription, group, regions, and tags.
func newAzureFetcher[DBType comparable, ListClient azureListClient[DBType]](config azureFetcherConfig, plugin azureFetcherPlugin[DBType, ListClient]) (common.Fetcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	fetcher := &azureFetcher[DBType, ListClient]{
		cfg: config,
		logger: slog.With(
			teleport.ComponentKey, "watch:azure",
			"labels", config.Labels,
			"regions", config.Regions,
			"group", config.ResourceGroup,
			"subscription", config.Subscription,
			"type", config.Type,
		),
		azureFetcherPlugin: plugin,
	}
	return fetcher, nil
}

// azureFetcherConfig is the Azure database servers fetcher configuration.
type azureFetcherConfig struct {
	// AzureClients are the Azure API clients.
	AzureClients cloud.AzureClients
	// Type is the type of DB matcher, such as "mysql" or "postgres"
	Type string
	// Subscription is the Azure subscription selector.
	// When the subscription is "*", this fetcher will query the Azure subscription API to list all subscriptions.
	Subscription string
	// ResourceGroup is a selector to match cloud resource group.
	ResourceGroup string
	// Labels is a selector to match cloud databases.
	Labels types.Labels
	// Regions is the Azure regions selectors to match cloud databases.
	Regions []string
	// regionSet is a set of regions, used for efficient region match lookup.
	regionSet map[string]struct{}
	// DiscoveryConfigName is the name of the discovery config which originated the resource.
	DiscoveryConfigName string
}

// regionMatches returns whether a given region matches the configured Regions selector
func (f *azureFetcher[DBType, ListClient]) regionMatches(region string) bool {
	if _, ok := f.cfg.regionSet[types.Wildcard]; ok {
		// wildcard matches all regions
		return true
	}
	_, ok := f.cfg.regionSet[azureutils.NormalizeLocation(region)]
	return ok
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *azureFetcherConfig) CheckAndSetDefaults() error {
	if c.AzureClients == nil {
		return trace.BadParameter("missing parameter AzureClients")
	}
	if len(c.Type) == 0 {
		return trace.BadParameter("missing parameter Type")
	}
	if len(c.Subscription) == 0 {
		return trace.BadParameter("missing parameter Subscription")
	}
	if len(c.ResourceGroup) == 0 {
		return trace.BadParameter("missing parameter ResourceGroup")
	}
	if len(c.Labels) == 0 {
		return trace.BadParameter("missing parameter Labels")
	}
	if len(c.Regions) == 0 {
		return trace.BadParameter("missing parameter Regions")
	}
	c.regionSet = utils.StringsSet(c.Regions)
	return nil
}

// azureFetcher retrieves Azure DB single-server databases.
type azureFetcher[DBType comparable, ListClient azureListClient[DBType]] struct {
	azureFetcherPlugin[DBType, ListClient]

	cfg    azureFetcherConfig
	logger *slog.Logger
}

// Cloud returns the cloud the fetcher is operating.
func (f *azureFetcher[DBType, ListClient]) Cloud() string {
	return types.CloudAzure
}

// ResourceType identifies the resource type the fetcher is returning.
func (f *azureFetcher[DBType, ListClient]) ResourceType() string {
	return types.KindDatabase
}

// FetcherType returns the type (`discovery_service.azure.[].types`) of the fetcher.
func (f *azureFetcher[DBType, ListClient]) FetcherType() string {
	return f.cfg.Type
}

// IntegrationName returns the integration name.
func (f *azureFetcher[DBType, ListClient]) IntegrationName() string {
	// There is currently no integration that supports Auto Discover for Azure resources.
	return ""
}

// GetDiscoveryConfigName is the name of the discovery config which originated the resource.
// It is used to report stats for a given discovery config.
// Might be empty when the fetcher is using static matchers:
// ie teleport.yaml/discovery_service.<cloud>.<matcher>
func (f *azureFetcher[DBType, ListClient]) GetDiscoveryConfigName() string {
	return f.cfg.DiscoveryConfigName
}

// Get returns Azure DB servers matching the watcher's selectors.
func (f *azureFetcher[DBType, ListClient]) Get(ctx context.Context) (types.ResourcesWithLabels, error) {
	databases, err := f.getDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	f.rewriteDatabases(databases)
	return databases.AsResources(), nil
}

// rewriteDatabases rewrites the discovered databases.
func (f *azureFetcher[DBType, ListClient]) rewriteDatabases(databases types.Databases) {
	for _, db := range databases {
		common.ApplyAzureDatabaseNameSuffix(db, f.cfg.Type)
	}
}

// getSubscriptions returns the subscriptions that this fetcher is configured to query.
// This will make an API call to list subscription IDs when the fetcher is configured to match "*" subscription,
// in order to discover and query new subscriptions.
// Otherwise, a list containing the fetcher's non-wildcard subscription is returned.
func (f *azureFetcher[DBType, ListClient]) getSubscriptions(ctx context.Context) ([]string, error) {
	if f.cfg.Subscription != types.Wildcard {
		return []string{f.cfg.Subscription}, nil
	}
	client, err := f.cfg.AzureClients.GetAzureSubscriptionClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	subIDs, err := client.ListSubscriptionIDs(ctx)
	return subIDs, trace.Wrap(err)
}

// getDBServersInSubscription fetches Azure DB servers within a given subscription.
func (f *azureFetcher[DBType, ListClient]) getDBServersInSubscription(ctx context.Context, subID string) ([]DBType, error) {
	client, err := f.GetListClient(&f.cfg, subID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if f.cfg.ResourceGroup == types.Wildcard {
		servers, err := client.ListAll(ctx)
		return servers, trace.Wrap(err)
	}
	servers, err := client.ListWithinGroup(ctx, f.cfg.ResourceGroup)
	return servers, trace.Wrap(err)
}

// getAllDBServers fetches Azure DB servers from all subscriptions that this fetcher is configured to query.
func (f *azureFetcher[DBType, ListClient]) getAllDBServers(ctx context.Context) ([]DBType, error) {
	var result []DBType
	subIDs, err := f.getSubscriptions(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, subID := range subIDs {
		servers, err := f.getDBServersInSubscription(ctx, subID)
		if err != nil {
			if trace.IsAccessDenied(err) || trace.IsNotFound(err) {
				f.logger.DebugContext(ctx, "Skipping subscription %q", "subscription", subID, "error", err)
				continue
			}
			return nil, trace.Wrap(err)
		}
		result = append(result, servers...)
	}
	return result, nil
}

// getDatabases returns a list of database resources representing Azure database servers.
func (f *azureFetcher[DBType, ListClient]) getDatabases(ctx context.Context) (types.Databases, error) {
	servers, err := f.getAllDBServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var nilServer DBType
	databases := make(types.Databases, 0, len(servers))
	for _, server := range servers {
		if server == nilServer {
			continue
		}
		// azure sdk provides no way to query by region, so we have to filter results
		if !f.regionMatches(f.GetServerLocation(server)) {
			continue
		}

		if database := f.NewDatabaseFromServer(ctx, server, f.logger); database != nil {
			databases = append(databases, database)
		}
	}
	return filterDatabasesByLabels(ctx, databases, f.cfg.Labels, f.logger), nil
}

// String returns the fetcher's string description.
func (f *azureFetcher[DBType, ListClient]) String() string {
	return fmt.Sprintf("azureFetcher(Type=%v, Subscription=%v, ResourceGroup=%v, Regions=%v, Labels=%v)",
		f.cfg.Type, f.cfg.Subscription, f.cfg.ResourceGroup, f.cfg.Regions, f.cfg.Labels)
}
