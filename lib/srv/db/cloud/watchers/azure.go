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

package watchers

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	azureutils "github.com/gravitational/teleport/api/utils/azure"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/services"
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
	NewDatabaseFromServer(server DBType, log logrus.FieldLogger) types.Database
}

// newAzureFetcher returns a Azure DB server fetcher for the provided subscription, group, regions, and tags.
func newAzureFetcher[DBType comparable, ListClient azureListClient[DBType]](config azureFetcherConfig, plugin azureFetcherPlugin[DBType, ListClient]) (Fetcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	fetcher := &azureFetcher[DBType, ListClient]{
		cfg: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "watch:azure",
			"labels":        config.Labels,
			"regions":       config.Regions,
			"group":         config.ResourceGroup,
			"subscription":  config.Subscription,
			"type":          config.Type,
		}),
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
}

// regionMatches returns whether a given region matches the configured Regions selector
func (f *azureFetcher[DBType, ListClient]) regionMatches(region string) bool {
	if _, ok := f.cfg.regionSet[types.Wildcard]; ok {
		// wildcard matches all regions
		return true
	}
	_, ok := f.cfg.regionSet[azureutils.NormailizeLocation(region)]
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

	cfg azureFetcherConfig
	log logrus.FieldLogger
}

// Get returns Azure DB servers matching the watcher's selectors.
func (f *azureFetcher[DBType, ListClient]) Get(ctx context.Context) (types.Databases, error) {
	databases, err := f.getDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return filterDatabasesByLabels(databases, f.cfg.Labels, f.log), nil
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
				f.log.WithError(err).Debugf("Skipping subscription %q", subID)
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

		if database := f.NewDatabaseFromServer(server, f.log); database != nil {
			databases = append(databases, database)
		}
	}
	return databases, nil
}

// String returns the fetcher's string description.
func (f *azureFetcher[DBType, ListClient]) String() string {
	return fmt.Sprintf("azureFetcher(Type=%v, Subscription=%v, ResourceGroup=%v, Regions=%v, Labels=%v)",
		f.cfg.Type, f.cfg.Subscription, f.cfg.ResourceGroup, f.cfg.Regions, f.cfg.Labels)
}

// simplifyMatchers returns simplified Azure Matchers.
// Selectors are deduplicated, wildcard in a selector reduces the selector
// to just the wildcard, and defaults are applied.
func simplifyMatchers(matchers []services.AzureMatcher) []services.AzureMatcher {
	result := make([]services.AzureMatcher, 0, len(matchers))
	for _, m := range matchers {
		subs := apiutils.Deduplicate(m.Subscriptions)
		groups := apiutils.Deduplicate(m.ResourceGroups)
		regions := apiutils.Deduplicate(m.Regions)
		ts := apiutils.Deduplicate(m.Types)
		if len(subs) == 0 || slices.Contains(subs, types.Wildcard) {
			subs = []string{types.Wildcard}
		}
		if len(groups) == 0 || slices.Contains(groups, types.Wildcard) {
			groups = []string{types.Wildcard}
		}
		if len(regions) == 0 || slices.Contains(regions, types.Wildcard) {
			regions = []string{types.Wildcard}
		} else {
			for i, region := range regions {
				regions[i] = azureutils.NormailizeLocation(region)
			}
		}
		result = append(result, services.AzureMatcher{
			Subscriptions:  subs,
			ResourceGroups: groups,
			Regions:        regions,
			Types:          ts,
			ResourceTags:   m.ResourceTags,
		})
	}
	return result
}
