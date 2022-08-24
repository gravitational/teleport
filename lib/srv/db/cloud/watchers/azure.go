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
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
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
		}),
	}
	return fetcher, nil
}

// azureFetcherConfig is the Azure database servers fetcher configuration.
type azureFetcherConfig struct {
	// Client is the Azure API client.
	Client azure.DBServersClient
	// Subscription is the Azure subscription being fetched from by the fetcher.
	Subscription string
	// ResourceGroup is a selector to match cloud resource group.
	ResourceGroup string
	// Labels is a selector to match cloud databases.
	Labels types.Labels
	// Regions is the Azure regions selectors to match cloud databases.
	Regions []string
	// regionMatches returns whether a given region matches the configured Regions selector
	regionMatches func(region string) bool
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *azureFetcherConfig) CheckAndSetDefaults() error {
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
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
	if apiutils.SliceContainsStr(c.Regions, types.Wildcard) {
		// wildcard matches all regions
		c.regionMatches = func(_ string) bool { return true }
	} else {
		regionSet := utils.StringsSet(c.Regions)
		c.regionMatches = func(region string) bool {
			_, ok := regionSet[region]
			return ok
		}
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

	return filterDatabasesByLabels(databases, f.cfg.Labels, f.log), nil
}

func (f *azureFetcher) getDBServers(ctx context.Context) ([]*azure.DBServer, error) {
	if f.cfg.ResourceGroup == types.Wildcard {
		return f.cfg.Client.ListAll(ctx, common.MaxPages)
	}
	return f.cfg.Client.ListWithinGroup(ctx, f.cfg.ResourceGroup, common.MaxPages)
}

// getDatabases returns a list of database resources representing Azure database servers.
func (f *azureFetcher) getDatabases(ctx context.Context) (types.Databases, error) {
	servers, err := f.getDBServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	databases := make(types.Databases, 0, len(servers))
	for _, server := range servers {
		if server == nil {
			continue
		}
		// azure sdk provides no way to query by region, so we have to filter results
		if !f.cfg.regionMatches(server.Location) {
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
	return fmt.Sprintf("azureFetcher(Subscription=%v, ResourceGroup=%v, Regions=%v, Labels=%v)",
		f.cfg.Subscription, f.cfg.ResourceGroup, f.cfg.Regions, f.cfg.Labels)
}

// reduceAzureMatcher simplifies a Azure Matcher.
// Selectors are deduplicated, wildcard in a selector reduces the selector
// to just the wildcard, and defaults are applied.
func reduceAzureMatcher(matcher *services.AzureMatcher) {
	matcher.Subscriptions = apiutils.Deduplicate(matcher.Subscriptions)
	matcher.ResourceGroups = apiutils.Deduplicate(matcher.ResourceGroups)
	matcher.Regions = apiutils.Deduplicate(matcher.Regions)
	matcher.Types = apiutils.Deduplicate(matcher.Types)
	if len(matcher.Subscriptions) == 0 || apiutils.SliceContainsStr(matcher.Subscriptions, types.Wildcard) {
		matcher.Subscriptions = []string{types.Wildcard}
	}
	if len(matcher.ResourceGroups) == 0 || apiutils.SliceContainsStr(matcher.ResourceGroups, types.Wildcard) {
		matcher.ResourceGroups = []string{types.Wildcard}
	}
	if len(matcher.Regions) == 0 || apiutils.SliceContainsStr(matcher.Regions, types.Wildcard) {
		matcher.Regions = []string{types.Wildcard}
	}
}
