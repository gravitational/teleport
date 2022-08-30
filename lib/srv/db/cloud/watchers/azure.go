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

	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// newAzureFetcher returns a Azure DB server fetcher for the provided subscription, group, regions, and tags.
func newAzureFetcher(config azureFetcherConfig) (*azureFetcher, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	fetcher := &azureFetcher{
		cfg: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "watch:azure",
			"labels":        config.Labels,
			"regions":       config.Regions,
			"group":         config.ResourceGroup,
			"subscription":  config.Subscription,
			"type":          config.Type,
		}),
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
func (f *azureFetcher) regionMatches(region string) bool {
	if _, ok := f.cfg.regionSet[types.Wildcard]; ok {
		// wildcard matches all regions
		return true
	}
	_, ok := f.cfg.regionSet[region]
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
	switch c.Type {
	case services.AzureMatcherMySQL, services.AzureMatcherPostgres:
	default:
		return trace.BadParameter("unknown matcher type %q", c.Type)
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

	return filterDatabasesByLabels(databases, f.cfg.Labels, f.log), nil
}

// getDBServersClient returns the appropriate Azure DBServersClient for this fetcher's configured Type.
func (f *azureFetcher) getDBServersClient(subID string) (azure.DBServersClient, error) {
	switch f.cfg.Type {
	case services.AzureMatcherMySQL:
		client, err := f.cfg.AzureClients.GetAzureMySQLClient(subID)
		return client, trace.Wrap(err)
	case services.AzureMatcherPostgres:
		client, err := f.cfg.AzureClients.GetAzurePostgresClient(subID)
		return client, trace.Wrap(err)
	default:
		return nil, trace.BadParameter("unknown matcher type %q", f.cfg.Type)
	}
}

// getSubscriptions returns the subscriptions that this fetcher is configured to query.
// This will make an API call to list subscription IDs when the fetcher is configured to match "*" subscription,
// in order to discover and query new subscriptions.
// Otherwise, a list containing the fetcher's non-wildcard subscription is returned.
func (f *azureFetcher) getSubscriptions(ctx context.Context) ([]string, error) {
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
func (f *azureFetcher) getDBServersInSubscription(ctx context.Context, subID string) ([]*azure.DBServer, error) {
	client, err := f.getDBServersClient(subID)
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
func (f *azureFetcher) getAllDBServers(ctx context.Context) ([]*azure.DBServer, error) {
	var result []*azure.DBServer
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
func (f *azureFetcher) getDatabases(ctx context.Context) (types.Databases, error) {
	servers, err := f.getAllDBServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	databases := make(types.Databases, 0, len(servers))
	for _, server := range servers {
		if server == nil {
			continue
		}
		// azure sdk provides no way to query by region, so we have to filter results
		if !f.regionMatches(server.Location) {
			continue
		}

		if !server.IsSupported() {
			f.log.Debugf("Azure server %q (version %v) does not support AAD authentication. Skipping.",
				server.Name,
				server.Properties.Version)
			continue
		}

		if !server.IsAvailable() {
			f.log.Debugf("The current status of Azure server %q is %q. Skipping.",
				server.Name,
				server.Properties.UserVisibleState)
			continue
		}

		database, err := services.NewDatabaseFromAzureServer(server)
		if err != nil {
			f.log.Warnf("Could not convert Azure server %q to database resource: %v.",
				server.Name,
				err)
			continue
		}
		databases = append(databases, database)
	}
	return databases, nil
}

// String returns the fetcher's string description.
func (f *azureFetcher) String() string {
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
		if len(subs) == 0 || apiutils.SliceContainsStr(subs, types.Wildcard) {
			subs = []string{types.Wildcard}
		}
		if len(groups) == 0 || apiutils.SliceContainsStr(groups, types.Wildcard) {
			groups = []string{types.Wildcard}
		}
		if len(regions) == 0 || apiutils.SliceContainsStr(regions, types.Wildcard) {
			regions = []string{types.Wildcard}
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
