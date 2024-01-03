package okta

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// defaultOktaConnectedCacheTTL is set to 30 seconds, will allow repeated commands to use the
// cached value without letting the data get too stale.
const defaultOktaConnectedCacheTTL time.Duration = 30 * time.Second

// ConnectedGetter is an interface used to retrieve the current inventory or the current plugins.
type ConnectedGetter interface {
	// GetInventoryConnectedServiceCount returns the counts of a particular connected service seen in the inventory.
	GetInventoryConnectedServiceCount(service types.SystemRole) uint64
}

// OktaConnectedConfig is the configuration for the OktaConnected utility.
type OktaConnectedConfig struct {
	// Log is the log to use for the OktaConnected utility.
	Log *logrus.Entry

	// Clock is the clock to use for the OktaConnected utility.
	Clock clockwork.Clock

	// DisableCache will disable the cache.
	DisableCache bool

	// CacheTTL is the amount of time a connected result should be cached for.
	CacheTTL time.Duration

	// ConnectedGetter is the service for getting service counts.
	ConnectedGetter ConnectedGetter

	// Plugins is the service for getting plugins. Is optional.
	Plugins services.Plugins
}

func (o *OktaConnectedConfig) CheckAndSetDefaults() error {
	if o.ConnectedGetter == nil {
		return trace.BadParameter("missing connected getter")
	}

	if o.Log == nil {
		o.Log = logrus.WithField(trace.Component, teleport.ComponentOktaConnected)
	}

	if o.Clock == nil {
		o.Clock = clockwork.NewRealClock()
	}

	if o.CacheTTL == 0 {
		o.CacheTTL = defaultOktaConnectedCacheTTL
	}

	return nil
}

// NewOktaConnected will create an Okta connected struct utility.
func NewOktaConnected(cfg OktaConnectedConfig) (*OktaConnected, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	var fnCache *utils.FnCache
	var err error
	if !cfg.DisableCache {
		fnCache, err = utils.NewFnCache(utils.FnCacheConfig{
			TTL:   cfg.CacheTTL,
			Clock: cfg.Clock,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	o := &OktaConnected{
		fnCache: fnCache,
		log:     cfg.Log,
		getter:  cfg.ConnectedGetter,
		plugins: cfg.Plugins,
	}

	if o.plugins == nil {
		o.log.Debug("This auth server does not support plugins, so the Okta access request reconciler will not check for Okta plugins.")
	} else {
		o.log.Debug("This auth server supports plugins, so the Okta access request reconciler will check for Okta plugins.")
	}

	return o, nil
}

// OktaConnected is a cache with a short TTL for repeated calls to IsConnected.
type OktaConnected struct {
	fnCache *utils.FnCache
	log     logrus.FieldLogger
	getter  ConnectedGetter
	plugins services.Plugins
}

// IsConnected will return true if an Okta service is seen in the inventory or in the plugins list.
func (o *OktaConnected) IsConnected(ctx context.Context) bool {
	var isConnected bool
	var err error
	if o.fnCache == nil {
		isConnected, err = o.load(ctx)
	} else {
		isConnected, err = utils.FnCacheGet(ctx, o.fnCache, "", o.load)
	}
	if err != nil {
		o.log.WithError(err).Error("Error trying to get plugins to test for Okta service connectivity")
	}

	return isConnected
}

// load is the loading function for determining if there's an Okta service connected.
func (o *OktaConnected) load(ctx context.Context) (bool, error) {
	// Check to see if the Okta service is in the inventory. This checks a single auth server's inventory.
	if o.getter.GetInventoryConnectedServiceCount(types.RoleOkta) > 0 {
		return true, nil
	}

	// If it's not in the inventory, check to see if there's an Okta plugin.
	if o.plugins != nil {
		hasPlugin, err := o.plugins.HasPluginType(ctx, types.PluginTypeOkta)
		if err != nil {
			return false, trace.Wrap(err)
		}

		return hasPlugin, nil
	}

	return false, nil
}
