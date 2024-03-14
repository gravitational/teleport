package okta

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// the default cache TTL for the plugin config helper.
	defaultPluginConfigHelperCacheTTL = 10 * time.Minute
)

// PluginConfigHelperConfig is the configuration for the PluginConfigHelper.
type PluginConfigHelperConfig struct {
	// Clock is the clock to use.
	Clock clockwork.Clock

	// Log is the log to use.
	Log *logrus.Entry

	// CacheTTL is the Cache TTL of the application and group results coming from Okta.
	CacheTTL time.Duration

	// oktaClientCreator will create Okta clients. For use in tests.
	oktaClientCreator oktaClientFn
}

func (c *PluginConfigHelperConfig) CheckAndSetDefaults() {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.Log == nil {
		c.Log = logrus.WithField(teleport.ComponentKey, eteleport.ComponentOkta)
	}

	if c.CacheTTL == 0 {
		c.CacheTTL = defaultPluginConfigHelperCacheTTL
	}

	if c.oktaClientCreator == nil {
		c.oktaClientCreator = createNewOktaClient
	}
}

// PluginConfigHelper is a service to help with configuration of the more advanced
// features of the Okta plugin.
type PluginConfigHelper struct {
	log               *logrus.Entry
	cache             *utils.FnCache
	oktaClientCreator oktaClientFn
}

// NewPluginConfigHelper creates a new plugin config helper service.
func NewPluginConfigHelper(config PluginConfigHelperConfig) (*PluginConfigHelper, error) {
	config.CheckAndSetDefaults()

	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:   config.CacheTTL,
		Clock: config.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &PluginConfigHelper{
		log:               config.Log,
		cache:             cache,
		oktaClientCreator: config.oktaClientCreator,
	}, nil
}

// PluginConfigOktaGroup is a representation of an Okta group for display during the
// plugin configuration of the Okta plugin.
type PluginConfigOktaGroup struct {
	// Name is the name of the group.
	Name string `json:"name"`

	// Description is the description of the group.
	Description string `json:"description,omitempty"`
}

// GetOktaGroups will return all groups known to Okta.
func (p *PluginConfigHelper) GetOktaGroups(ctx context.Context, orgURL, apiToken string, queryFilters []string) ([]*PluginConfigOktaGroup, error) {
	client, filters, err := p.getClientAndFilters(ctx, orgURL, apiToken, queryFilters)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Cache these results so that users can repeatedly try different filters against these results
	// without hammering the Okta API.
	groups, err := utils.FnCacheGet(ctx, p.cache, fmt.Sprintf("%s-%s-groups", orgURL, apiToken),
		func(ctx context.Context) ([]*PluginConfigOktaGroup, error) {
			var groups []*PluginConfigOktaGroup
			err := client.iterateGroups(ctx, func(g *okta.Group) error {
				if g.Profile == nil {
					p.log.Debugf("Found a nil profile, skipping")
					return nil
				}

				groups = append(groups, &PluginConfigOktaGroup{
					Name:        g.Profile.Name,
					Description: g.Profile.Description,
				})

				return nil
			})

			return groups, trace.Wrap(err)
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(filters) == 0 {
		return groups, nil
	}

	return getMatches(groups, filters, func(a *PluginConfigOktaGroup) string { return a.Name }), nil
}

// PluginConfigOktaApp is a representation of an Okta app for display during the
// plugin configuration of the Okta plugin.
type PluginConfigOktaApp struct {
	Name string `json:"name"`
}

// GetOktaApps will return all applications known to Okta.
func (p *PluginConfigHelper) GetOktaApps(ctx context.Context, orgURL, apiToken string, queryFilters []string) ([]*PluginConfigOktaApp, error) {
	client, filters, err := p.getClientAndFilters(ctx, orgURL, apiToken, queryFilters)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Cache these results so that users can repeatedly try different filters against these results
	// without hammering the Okta API.
	apps, err := utils.FnCacheGet(ctx, p.cache, fmt.Sprintf("%s-%s-apps", orgURL, apiToken),
		func(ctx context.Context) ([]*PluginConfigOktaApp, error) {
			var apps []*PluginConfigOktaApp
			err := client.iterateApps(ctx, func(a okta.App) error {
				// This type assertion is necessary as okta.App, which is supplied by the Okta go SDK,
				// does not contain all of the information that we need to create a types.Application
				// object.
				var oktaApplication *okta.Application
				var ok bool
				if oktaApplication, ok = a.(*okta.Application); !ok {
					p.log.Debugf("Unable to process Okta application of unknown type %T", a)
					return nil
				}

				apps = append(apps, &PluginConfigOktaApp{
					Name: oktaApplication.Label,
				})

				return nil
			})

			return apps, trace.Wrap(err)
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(filters) == 0 {
		return apps, nil
	}

	return getMatches(apps, filters, func(a *PluginConfigOktaApp) string { return a.Name }), nil
}

func getMatches[T any](resources []T, filters []*regexp.Regexp, getNameFn func(T) string) []T {
	var filteredResources []T
	for _, resource := range resources {
		for _, filter := range filters {
			if filter.MatchString(getNameFn(resource)) {
				filteredResources = append(filteredResources, resource)
				break
			}
		}
	}

	return filteredResources
}

// getClientAndFilters will create the Okta client and compile the given filters.
func (p *PluginConfigHelper) getClientAndFilters(ctx context.Context, orgURL, token string, filters []string) (OktaClient, []*regexp.Regexp, error) {
	client, err := p.oktaClientCreator(ctx, ClientConfig{
		Endpoint: orgURL,
		Token:    token,
		Log:      p.log,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var compiledFilters []*regexp.Regexp
	for _, filter := range filters {
		compiledFilter, err := utils.CompileExpression(filter)
		if err != nil {
			return nil, nil, trace.Wrap(err, "error compiling filter: %s", filter)
		}

		compiledFilters = append(compiledFilters, compiledFilter)
	}

	return client, compiledFilters, nil
}
