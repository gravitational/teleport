package watchers

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

// newAzureFetcher returns a Azure DB server fetcher for the provided subscription, group, regions, and tags.
func newAzureFetcher(client azure.DBServersClient, sub, group string, regions []string, tags types.Labels) (*azureFetcher, error) {
	config := azureFetcherConfig{
		Client:        client,
		Subscription:  sub,
		ResourceGroup: group,
		Labels:        tags,
		Regions:       regions,
	}
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
	// regionSet is a set of Azure regions to efficiently filter databases.
	regionSet map[string]struct{}
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
	if len(c.regionSet) == 0 {
		c.regionSet = utils.StringsSet(c.Regions)
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
		if _, ok := f.cfg.regionSet[server.Location]; !ok {
			continue
		}

		if !server.IsVersionSupported() {
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
	return fmt.Sprintf("azureFetcher(Subscription=%v, ResourceGroup=%v, Region=%v, Labels=%v)",
		f.cfg.Subscription, f.cfg.ResourceGroup, f.cfg.Regions, f.cfg.Labels)
}
