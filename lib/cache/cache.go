/*
Copyright 2018-2019 Gravitational, Inc.

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

package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"go.uber.org/atomic"
)

func tombstoneKey() []byte {
	return backend.Key("cache", teleport.Version, "tombstone", "ok")
}

// ForAuth sets up watch configuration for the auth server
func ForAuth(cfg Config) Config {
	cfg.target = "auth"
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: true},
		{Kind: types.KindClusterName},
		{Kind: types.KindClusterAuditConfig},
		{Kind: types.KindClusterNetworkingConfig},
		{Kind: types.KindClusterAuthPreference},
		{Kind: types.KindSessionRecordingConfig},
		{Kind: types.KindStaticTokens},
		{Kind: types.KindToken},
		{Kind: types.KindUser},
		{Kind: types.KindRole},
		{Kind: types.KindNamespace},
		{Kind: types.KindNode},
		{Kind: types.KindProxy},
		{Kind: types.KindAuthServer},
		{Kind: types.KindReverseTunnel},
		{Kind: types.KindTunnelConnection},
		{Kind: types.KindAccessRequest},
		{Kind: types.KindAppServer},
		{Kind: types.KindApp},
		{Kind: types.KindAppServer, Version: types.V2},
		{Kind: types.KindWebSession, SubKind: types.KindAppSession},
		{Kind: types.KindWebSession, SubKind: types.KindWebSession},
		{Kind: types.KindWebToken},
		{Kind: types.KindRemoteCluster},
		{Kind: types.KindKubeService},
		{Kind: types.KindDatabaseServer},
		{Kind: types.KindDatabase},
		{Kind: types.KindNetworkRestrictions},
		{Kind: types.KindLock},
		{Kind: types.KindWindowsDesktopService},
		{Kind: types.KindWindowsDesktop},
	}
	cfg.QueueSize = defaults.AuthQueueSize
	return cfg
}

// ForProxy sets up watch configuration for proxy
func ForProxy(cfg Config) Config {
	cfg.target = "proxy"
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: false},
		{Kind: types.KindClusterName},
		{Kind: types.KindClusterAuditConfig},
		{Kind: types.KindClusterNetworkingConfig},
		{Kind: types.KindClusterAuthPreference},
		{Kind: types.KindSessionRecordingConfig},
		{Kind: types.KindUser},
		{Kind: types.KindRole},
		{Kind: types.KindNamespace},
		{Kind: types.KindNode},
		{Kind: types.KindProxy},
		{Kind: types.KindAuthServer},
		{Kind: types.KindReverseTunnel},
		{Kind: types.KindTunnelConnection},
		{Kind: types.KindAppServer},
		{Kind: types.KindAppServer, Version: types.V2},
		{Kind: types.KindApp},
		{Kind: types.KindWebSession, SubKind: types.KindAppSession},
		{Kind: types.KindWebSession, SubKind: types.KindWebSession},
		{Kind: types.KindWebToken},
		{Kind: types.KindRemoteCluster},
		{Kind: types.KindKubeService},
		{Kind: types.KindDatabaseServer},
		{Kind: types.KindDatabase},
		{Kind: types.KindWindowsDesktopService},
		{Kind: types.KindWindowsDesktop},
	}
	cfg.QueueSize = defaults.ProxyQueueSize
	return cfg
}

// ForRemoteProxy sets up watch configuration for remote proxies.
func ForRemoteProxy(cfg Config) Config {
	cfg.target = "remote-proxy"
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: false},
		{Kind: types.KindClusterName},
		{Kind: types.KindClusterAuditConfig},
		{Kind: types.KindClusterNetworkingConfig},
		{Kind: types.KindClusterAuthPreference},
		{Kind: types.KindSessionRecordingConfig},
		{Kind: types.KindUser},
		{Kind: types.KindRole},
		{Kind: types.KindNamespace},
		{Kind: types.KindNode},
		{Kind: types.KindProxy},
		{Kind: types.KindAuthServer},
		{Kind: types.KindReverseTunnel},
		{Kind: types.KindTunnelConnection},
		{Kind: types.KindAppServer},
		{Kind: types.KindAppServer, Version: types.V2},
		{Kind: types.KindApp},
		{Kind: types.KindRemoteCluster},
		{Kind: types.KindKubeService},
		{Kind: types.KindDatabaseServer},
		{Kind: types.KindDatabase},
	}
	cfg.QueueSize = defaults.ProxyQueueSize
	return cfg
}

// ForOldRemoteProxy sets up watch configuration for older remote proxies.
func ForOldRemoteProxy(cfg Config) Config {
	cfg.target = "remote-proxy-old"
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: false},
		{Kind: types.KindClusterName},
		{Kind: types.KindClusterAuditConfig},
		{Kind: types.KindClusterNetworkingConfig},
		{Kind: types.KindClusterAuthPreference},
		{Kind: types.KindSessionRecordingConfig},
		{Kind: types.KindUser},
		{Kind: types.KindRole},
		{Kind: types.KindNamespace},
		{Kind: types.KindNode},
		{Kind: types.KindProxy},
		{Kind: types.KindAuthServer},
		{Kind: types.KindReverseTunnel},
		{Kind: types.KindTunnelConnection},
		{Kind: types.KindAppServer, Version: types.V2},
		{Kind: types.KindRemoteCluster},
		{Kind: types.KindKubeService},
		{Kind: types.KindDatabaseServer},
	}
	cfg.QueueSize = defaults.ProxyQueueSize
	return cfg
}

// ForNode sets up watch configuration for node
func ForNode(cfg Config) Config {
	cfg.target = "node"
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: false},
		{Kind: types.KindClusterName},
		{Kind: types.KindClusterAuditConfig},
		{Kind: types.KindClusterNetworkingConfig},
		{Kind: types.KindClusterAuthPreference},
		{Kind: types.KindSessionRecordingConfig},
		{Kind: types.KindUser},
		{Kind: types.KindRole},
		// Node only needs to "know" about default
		// namespace events to avoid matching too much
		// data about other namespaces or node events
		{Kind: types.KindNamespace, Name: apidefaults.Namespace},
		{Kind: types.KindNetworkRestrictions},
	}
	cfg.QueueSize = defaults.NodeQueueSize
	return cfg
}

// ForKubernetes sets up watch configuration for a kubernetes service.
func ForKubernetes(cfg Config) Config {
	cfg.target = "kube"
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: false},
		{Kind: types.KindClusterName},
		{Kind: types.KindClusterAuditConfig},
		{Kind: types.KindClusterNetworkingConfig},
		{Kind: types.KindClusterAuthPreference},
		{Kind: types.KindSessionRecordingConfig},
		{Kind: types.KindUser},
		{Kind: types.KindRole},
		{Kind: types.KindNamespace, Name: apidefaults.Namespace},
		{Kind: types.KindKubeService},
	}
	cfg.QueueSize = defaults.KubernetesQueueSize
	return cfg
}

// ForApps sets up watch configuration for apps.
func ForApps(cfg Config) Config {
	cfg.target = "apps"
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: false},
		{Kind: types.KindClusterName},
		{Kind: types.KindClusterAuditConfig},
		{Kind: types.KindClusterNetworkingConfig},
		{Kind: types.KindClusterAuthPreference},
		{Kind: types.KindSessionRecordingConfig},
		{Kind: types.KindUser},
		{Kind: types.KindRole},
		{Kind: types.KindProxy},
		// Applications only need to "know" about default namespace events to avoid
		// matching too much data about other namespaces or events.
		{Kind: types.KindNamespace, Name: apidefaults.Namespace},
		{Kind: types.KindApp},
	}
	cfg.QueueSize = defaults.AppsQueueSize
	return cfg
}

// ForDatabases sets up watch configuration for database proxy servers.
func ForDatabases(cfg Config) Config {
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: false},
		{Kind: types.KindClusterName},
		{Kind: types.KindClusterAuditConfig},
		{Kind: types.KindClusterNetworkingConfig},
		{Kind: types.KindClusterAuthPreference},
		{Kind: types.KindSessionRecordingConfig},
		{Kind: types.KindUser},
		{Kind: types.KindRole},
		{Kind: types.KindProxy},
		// Databases only need to "know" about default namespace events to
		// avoid matching too much data about other namespaces or events.
		{Kind: types.KindNamespace, Name: apidefaults.Namespace},
		{Kind: types.KindDatabase},
	}
	cfg.QueueSize = defaults.DatabasesQueueSize
	return cfg
}

// ForWindowsDesktop sets up watch configuration for a Windows desktop service.
func ForWindowsDesktop(cfg Config) Config {
	cfg.target = "windows_desktop"
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: false},
		{Kind: types.KindClusterName},
		{Kind: types.KindClusterAuditConfig},
		{Kind: types.KindClusterNetworkingConfig},
		{Kind: types.KindClusterAuthPreference},
		{Kind: types.KindSessionRecordingConfig},
		{Kind: types.KindUser},
		{Kind: types.KindRole},
		{Kind: types.KindNamespace, Name: apidefaults.Namespace},
		{Kind: types.KindWindowsDesktopService},
		{Kind: types.KindWindowsDesktop},
	}
	cfg.QueueSize = defaults.WindowsDesktopQueueSize
	return cfg
}

// SetupConfigFn is a function that sets up configuration
// for cache
type SetupConfigFn func(c Config) Config

// Cache implements auth.AccessPoint interface and remembers
// the previously returned upstream value for each API call.
//
// This which can be used if the upstream AccessPoint goes offline
type Cache struct {
	Config

	// Entry is a logging entry
	*log.Entry

	// rw is used to prevent reads of invalid cache states.  From a
	// memory-safety perspective, this RWMutex is just used to protect
	// the `ok` field.  *However*, cache reads must hold the read lock
	// for the duration of the read, not just when checking the `ok`
	// field.  Since the write lock must be held in order to modify
	// the `ok` field, this serves to ensure that all in-progress reads
	// complete *before* a reset can begin.
	rw sync.RWMutex
	// ok indicates whether the cache is in a valid state for reads.
	// If `ok` is `false`, reads are forwarded directly to the backend.
	ok bool

	// generation is a counter that is incremented each time a healthy
	// state is established.  A generation of zero means that a healthy
	// state was never established.  Note that a generation of zero does
	// not preclude `ok` being true in the case that we have loaded a
	// previously healthy state from the backend.
	generation *atomic.Uint64

	// initOnce protects initC and initErr.
	initOnce sync.Once
	// initC is closed on the first attempt to initialize the
	// cache, whether or not it is successful.  Once initC
	// has returned, initErr is safe to read.
	initC chan struct{}
	// initErr is set if the first attempt to initialize the cache
	// fails.
	initErr error

	// wrapper is a wrapper around cache backend that
	// allows to set backend into failure mode,
	// intercepting all calls and returning errors instead
	wrapper *backend.Wrapper
	// ctx is a cache exit context
	ctx context.Context
	// cancel triggers exit context closure
	cancel context.CancelFunc

	// collections is a map of registered collections by resource Kind/SubKind
	collections map[resourceKind]collection

	trustCache           services.Trust
	clusterConfigCache   services.ClusterConfiguration
	provisionerCache     services.Provisioner
	usersCache           services.UsersService
	accessCache          services.Access
	dynamicAccessCache   services.DynamicAccessExt
	presenceCache        services.Presence
	restrictionsCache    services.Restrictions
	appsCache            services.Apps
	databasesCache       services.Databases
	appSessionCache      services.AppSession
	webSessionCache      types.WebSessionInterface
	webTokenCache        types.WebTokenInterface
	windowsDesktopsCache services.WindowsDesktops
	eventsFanout         *services.Fanout

	// closed indicates that the cache has been closed
	closed *atomic.Bool
}

func (c *Cache) setInitError(err error) {
	c.initOnce.Do(func() {
		c.initErr = err
		close(c.initC)
	})
}

// setReadOK updates Cache.ok, which determines whether the
// cache is accessible for reads.
func (c *Cache) setReadOK(ok bool) {
	if ok == c.getReadOK() {
		return
	}
	c.rw.Lock()
	defer c.rw.Unlock()
	c.ok = ok
}

func (c *Cache) getReadOK() (ok bool) {
	c.rw.RLock()
	defer c.rw.RUnlock()
	return c.ok
}

// read acquires the cache read lock and selects the appropriate
// target for read operations.  The returned guard *must* be
// released to prevent deadlocks.
func (c *Cache) read() (readGuard, error) {
	if c.closed.Load() {
		return readGuard{}, trace.Errorf("cache is closed")
	}
	c.rw.RLock()
	if c.ok {
		return readGuard{
			trust:           c.trustCache,
			clusterConfig:   c.clusterConfigCache,
			provisioner:     c.provisionerCache,
			users:           c.usersCache,
			access:          c.accessCache,
			dynamicAccess:   c.dynamicAccessCache,
			presence:        c.presenceCache,
			restrictions:    c.restrictionsCache,
			apps:            c.appsCache,
			databases:       c.databasesCache,
			appSession:      c.appSessionCache,
			webSession:      c.webSessionCache,
			webToken:        c.webTokenCache,
			release:         c.rw.RUnlock,
			windowsDesktops: c.windowsDesktopsCache,
		}, nil
	}
	c.rw.RUnlock()
	return readGuard{
		trust:           c.Config.Trust,
		clusterConfig:   c.Config.ClusterConfig,
		provisioner:     c.Config.Provisioner,
		users:           c.Config.Users,
		access:          c.Config.Access,
		dynamicAccess:   c.Config.DynamicAccess,
		presence:        c.Config.Presence,
		restrictions:    c.Config.Restrictions,
		apps:            c.Config.Apps,
		databases:       c.Config.Databases,
		appSession:      c.Config.AppSession,
		webSession:      c.Config.WebSession,
		webToken:        c.Config.WebToken,
		windowsDesktops: c.Config.WindowsDesktops,
		release:         nil,
	}, nil
}

// readGuard holds references to a "backend".  if the referenced
// backed is the cache, then readGuard also holds the release
// function for the read lock, and ensures that it is not
// double-called.
type readGuard struct {
	trust           services.Trust
	clusterConfig   services.ClusterConfiguration
	provisioner     services.Provisioner
	users           services.UsersService
	access          services.Access
	dynamicAccess   services.DynamicAccessCore
	presence        services.Presence
	appSession      services.AppSession
	restrictions    services.Restrictions
	apps            services.Apps
	databases       services.Databases
	webSession      types.WebSessionInterface
	webToken        types.WebTokenInterface
	windowsDesktops services.WindowsDesktops
	release         func()
	released        bool
}

// Release releases the read lock if it is held.  This method
// can be called multiple times, but is not thread-safe.
func (r *readGuard) Release() {
	if r.release != nil && !r.released {
		r.release()
		r.released = true
	}
}

// IsCacheRead checks if this readGuard holds a cache reference.
func (r *readGuard) IsCacheRead() bool {
	return r.release != nil
}

// Config defines cache configuration parameters
type Config struct {
	// target is an identifying string that allows errors to
	// indicate the target presets used (e.g. "auth").
	target string
	// Context is context for parent operations
	Context context.Context
	// Watches provides a list of resources
	// for the cache to watch
	Watches []types.WatchKind
	// Events provides events watchers
	Events types.Events
	// Trust is a service providing information about certificate
	// authorities
	Trust services.Trust
	// ClusterConfig is a cluster configuration service
	ClusterConfig services.ClusterConfiguration
	// Provisioner is a provisioning service
	Provisioner services.Provisioner
	// Users is a users service
	Users services.UsersService
	// Access is an access service
	Access services.Access
	// DynamicAccess is a dynamic access service
	DynamicAccess services.DynamicAccessCore
	// Presence is a presence service
	Presence services.Presence
	// Restrictions is a restrictions service
	Restrictions services.Restrictions
	// Apps is an apps service.
	Apps services.Apps
	// Databases is a databases service.
	Databases services.Databases
	// AppSession holds application sessions.
	AppSession services.AppSession
	// WebSession holds regular web sessions.
	WebSession types.WebSessionInterface
	// WebToken holds web tokens.
	WebToken types.WebTokenInterface
	// WindowsDesktops is a windows desktop service.
	WindowsDesktops services.WindowsDesktops
	// Backend is a backend for local cache
	Backend backend.Backend
	// RetryPeriod is a period between cache retries on failures
	RetryPeriod time.Duration
	// WatcherInitTimeout is the maximum acceptable delay for an
	// OpInit after a watcher has been started (default=1m).
	WatcherInitTimeout time.Duration
	// CacheInitTimeout is the maximum amount of time that cache.New
	// will block, waiting for initialization (default=20s).
	CacheInitTimeout time.Duration
	// EventsC is a channel for event notifications,
	// used in tests
	EventsC chan Event
	// OnlyRecent configures cache behavior that always uses
	// recent values, see OnlyRecent for details
	OnlyRecent OnlyRecent
	// PreferRecent configures cache behavior that prefer recent values
	// when available, but falls back to stale data, see PreferRecent
	// for details
	PreferRecent PreferRecent
	// Clock can be set to control time,
	// uses runtime clock by default
	Clock clockwork.Clock
	// Component is a component used in logs
	Component string
	// MetricComponent is a component used in metrics
	MetricComponent string
	// QueueSize is a desired queue Size
	QueueSize int
}

// OnlyRecent defines cache behavior always
// using recent data and failing otherwise.
// Used by auth servers and other systems
// having direct access to the backend.
type OnlyRecent struct {
	// Enabled enables cache behavior
	Enabled bool
}

// PreferRecent defined cache behavior
// that always prefers recent data, but will
// serve stale data in case if disconnect is detected
type PreferRecent struct {
	// Enabled enables cache behavior
	Enabled bool
	// MaxTTL sets maximum TTL the cache keeps the value
	// in case if there is no connection to auth servers
	MaxTTL time.Duration
	// NeverExpires if set, never expires stale cache values
	NeverExpires bool
}

// CheckAndSetDefaults checks parameters and sets default values
func (p *PreferRecent) CheckAndSetDefaults() error {
	if p.MaxTTL == 0 {
		p.MaxTTL = defaults.CacheTTL
	}
	return nil
}

// CheckAndSetDefaults checks parameters and sets default values
func (c *Config) CheckAndSetDefaults() error {
	if c.Events == nil {
		return trace.BadParameter("missing Events parameter")
	}
	if c.Backend == nil {
		return trace.BadParameter("missing Backend parameter")
	}
	if c.OnlyRecent.Enabled && c.PreferRecent.Enabled {
		return trace.BadParameter("either one of OnlyRecent or PreferRecent should be enabled at a time")
	}
	if !c.OnlyRecent.Enabled && !c.PreferRecent.Enabled {
		c.OnlyRecent.Enabled = true
	}
	if err := c.PreferRecent.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if c.Context == nil {
		c.Context = context.Background()
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.RetryPeriod == 0 {
		c.RetryPeriod = defaults.HighResPollingPeriod
	}
	if c.WatcherInitTimeout == 0 {
		c.WatcherInitTimeout = time.Minute
	}
	if c.CacheInitTimeout == 0 {
		c.CacheInitTimeout = time.Second * 20
	}
	if c.Component == "" {
		c.Component = teleport.ComponentCache
	}
	return nil
}

// Event is event used in tests
type Event struct {
	// Type is event type
	Type string
	// Event is event processed
	// by the event cycle
	Event types.Event
}

const (
	// EventProcessed is emitted whenever event is processed
	EventProcessed = "event_processed"
	// WatcherStarted is emitted when a new event watcher is started
	WatcherStarted = "watcher_started"
	// WatcherFailed is emitted when event watcher has failed
	WatcherFailed = "watcher_failed"
	// TombstoneWritten is emitted if cache is closed in a healthy
	// state and successfully writes its tombstone.
	TombstoneWritten = "tombstone_written"
)

// New creates a new instance of Cache
func New(config Config) (*Cache, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	wrapper := backend.NewWrapper(config.Backend)

	clusterConfigCache, err := local.NewClusterConfigurationService(wrapper)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(config.Context)
	cs := &Cache{
		wrapper:              wrapper,
		ctx:                  ctx,
		cancel:               cancel,
		Config:               config,
		generation:           atomic.NewUint64(0),
		initC:                make(chan struct{}),
		trustCache:           local.NewCAService(wrapper),
		clusterConfigCache:   clusterConfigCache,
		provisionerCache:     local.NewProvisioningService(wrapper),
		usersCache:           local.NewIdentityService(wrapper),
		accessCache:          local.NewAccessService(wrapper),
		dynamicAccessCache:   local.NewDynamicAccessService(wrapper),
		presenceCache:        local.NewPresenceService(wrapper),
		restrictionsCache:    local.NewRestrictionsService(wrapper),
		appsCache:            local.NewAppService(wrapper),
		databasesCache:       local.NewDatabasesService(wrapper),
		appSessionCache:      local.NewIdentityService(wrapper),
		webSessionCache:      local.NewIdentityService(wrapper).WebSessions(),
		webTokenCache:        local.NewIdentityService(wrapper).WebTokens(),
		windowsDesktopsCache: local.NewWindowsDesktopService(wrapper),
		eventsFanout:         services.NewFanout(),
		Entry: log.WithFields(log.Fields{
			trace.Component: config.Component,
		}),
		closed: atomic.NewBool(false),
	}
	collections, err := setupCollections(cs, config.Watches)
	if err != nil {
		cs.Close()
		return nil, trace.Wrap(err)
	}
	cs.collections = collections

	// if the ok tombstone is present, set the initial read state of the cache
	// to ok. this tombstone's presence indicates that we are dealing with an
	// on-disk cache produced by the same teleport version which gracefully shutdown
	// while in an ok state.  We delete the tombstone rather than check for its
	// presence to ensure self-healing in the event that the tombstone wasn't actually
	// valid.  Note that setting the cache's read state to ok does not cause us to skip
	// our normal init logic, it just means that reads against the local cache will
	// be allowed in the event the init step fails before it starts applying.
	// Note also that we aren't setting our event fanout system to an initialized state
	// or incrementing the generation counter; this cache isn't so much "healthy" as it is
	// "slightly preferable to an unreachable auth server".
	err = cs.wrapper.Delete(ctx, tombstoneKey())
	switch {
	case err == nil:
		if !cs.OnlyRecent.Enabled {
			cs.setReadOK(true)
		}
	case trace.IsNotFound(err):
		// do nothing
	default:
		cs.Close()
		return nil, trace.Wrap(err)
	}

	retry, err := utils.NewLinear(utils.LinearConfig{
		Step: cs.Config.RetryPeriod / 10,
		Max:  cs.Config.RetryPeriod,
	})
	if err != nil {
		cs.Close()
		return nil, trace.Wrap(err)
	}

	go cs.update(ctx, retry)

	select {
	case <-cs.initC:
		if cs.initErr != nil && cs.OnlyRecent.Enabled {
			cs.Close()
			return nil, trace.Wrap(cs.initErr)
		}
		if cs.initErr == nil {
			cs.Infof("Cache %q first init succeeded.", cs.Config.target)
		} else {
			cs.WithError(cs.initErr).Warnf("Cache %q first init failed, continuing re-init attempts in background.", cs.Config.target)
		}
	case <-ctx.Done():
		cs.Close()
		return nil, trace.Wrap(ctx.Err(), "context closed during cache init")
	case <-time.After(cs.Config.CacheInitTimeout):
		if cs.OnlyRecent.Enabled {
			cs.Close()
			return nil, trace.ConnectionProblem(nil, "timeout waiting for cache init")
		}
		cs.Warningf("Cache init is taking too long, will continue in background.")
	}
	return cs, nil
}

// NewWatcher returns a new event watcher. In case of a cache
// this watcher will return events as seen by the cache,
// not the backend. This feature allows auth server
// to handle subscribers connected to the in-memory caches
// instead of reading from the backend.
func (c *Cache) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
Outer:
	for _, requested := range watch.Kinds {
		for _, configured := range c.Config.Watches {
			if requested.Kind == configured.Kind {
				continue Outer
			}
		}
		return nil, trace.BadParameter("cache %q does not support watching resource %q", c.Config.target, requested.Kind)
	}
	return c.eventsFanout.NewWatcher(ctx, watch)
}

func (c *Cache) update(ctx context.Context, retry utils.Retry) {
	defer func() {
		c.Debugf("Cache is closing, returning from update loop.")
		// ensure that close operations have been run
		c.Close()
		// run tombstone operations in an orphaned context
		tombCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		c.writeTombstone(tombCtx)
	}()
	timer := time.NewTimer(c.Config.WatcherInitTimeout)
	for {
		err := c.fetchAndWatch(ctx, retry, timer)
		c.setInitError(err)
		if c.isClosing() {
			return
		}
		if err != nil {
			c.Warningf("Re-init the cache on error: %v.", err)
			if c.OnlyRecent.Enabled {
				c.setReadOK(false)
			}
		}
		// events cache should be closed as well
		c.Debugf("Reloading %v.", retry)
		select {
		case <-retry.After():
			retry.Inc()
		case <-c.ctx.Done():
			return
		}
	}
}

// writeTombstone writes the cache tombstone.
func (c *Cache) writeTombstone(ctx context.Context) {
	if !c.getReadOK() || c.generation.Load() == 0 {
		// state is unhealthy or was loaded from a previously
		// entombed state; do nothing.
		return
	}
	item := backend.Item{
		Key:   tombstoneKey(),
		Value: []byte("{}"),
	}
	if _, err := c.wrapper.Create(ctx, item); err != nil {
		c.Warningf("Failed to set tombstone: %v", err)
	} else {
		c.notify(ctx, Event{Type: TombstoneWritten})
	}
}

// setTTL overrides TTL supplied by the resource
// based on the cache behavior:
// - for "only recent", does nothing
// - for "prefer recent", honors TTL set on the resource, otherwise
//   sets TTL to max TTL
func (c *Cache) setTTL(r types.Resource) {
	if c.OnlyRecent.Enabled || (c.PreferRecent.Enabled && c.PreferRecent.NeverExpires) {
		return
	}
	// honor expiry set in the resource
	if !r.Expiry().IsZero() {
		return
	}
	// set max TTL on the resources
	r.SetExpiry(c.Clock.Now().UTC().Add(c.PreferRecent.MaxTTL))
}

func (c *Cache) notify(ctx context.Context, event Event) {
	if c.EventsC == nil {
		return
	}
	select {
	case c.EventsC <- event:
		return
	case <-ctx.Done():
		return
	}
}

// fetchAndWatch keeps cache up to date by replaying
// events and syncing local cache storage.
//
// Here are some thoughts on consistency in face of errors:
//
// 1. Every client is connected to the database fan-out
// system. This system creates a buffered channel for every
// client and tracks the channel overflow. Thanks to channels every client gets its
// own unique iterator over the event stream. If client looses connection
// or fails to keep up with the stream, the server will terminate
// the channel and client will have to re-initialize.
//
// 2. Replays of stale events. Etcd provides a strong
// mechanism to track the versions of the storage - revisions
// of every operation that are uniquely numbered and monothonically
// and consistently ordered thanks to Raft. Unfortunately, DynamoDB
// does not provide such a mechanism for its event system, so
// some tradeofs have to be made:
//   a. We assume that events are ordered in regards to the
//   individual key operations which is the guarantees both Etcd and DynamodDB
//   provide.
//   b. Thanks to the init event sent by the server on a successful connect,
//   and guarantees 1 and 2a, client assumes that once it connects and receives an event,
//   it will not miss any events, however it can receive stale events.
//   Event could be stale, if it relates to a change that happened before
//   the version read by client from the database, for example,
//   given the event stream: 1. Update a=1 2. Delete a 3. Put a = 2
//   Client could have subscribed before event 1 happened,
//   read the value a=2 and then received events 1 and 2 and 3.
//   The cache will replay all events 1, 2 and 3 and end up in the correct
//   state 3. If we had a consistent revision number, we could
//   have skipped 1 and 2, but in the absence of such mechanism in Dynamo
//   we assume that this cache will eventually end up in a correct state
//   potentially lagging behind the state of the database.
//
func (c *Cache) fetchAndWatch(ctx context.Context, retry utils.Retry, timer *time.Timer) error {
	watcher, err := c.Events.NewWatcher(c.ctx, types.Watch{
		QueueSize:       c.QueueSize,
		Name:            c.Component,
		Kinds:           c.watchKinds(),
		MetricComponent: c.MetricComponent,
	})
	if err != nil {
		c.notify(c.ctx, Event{Type: WatcherFailed})
		return trace.Wrap(err)
	}
	defer watcher.Close()

	// ensure that the timer is stopped and drained
	timer.Stop()
	select {
	case <-timer.C:
	default:
	}
	// set timer to watcher init timeout
	timer.Reset(c.Config.WatcherInitTimeout)

	// before fetch, make sure watcher is synced by receiving init event,
	// to avoid the scenario:
	// 1. Cache process:   w = NewWatcher()
	// 2. Cache process:   c.fetch()
	// 3. Backend process: addItem()
	// 4. Cache process:   <- w.Events()
	//
	// If there is a way that NewWatcher() on line 1 could
	// return without subscription established first,
	// Code line 3 could execute and line 4 could miss event,
	// wrapping up with out of sync replica.
	// To avoid this, before doing fetch,
	// cache process makes sure the connection is established
	// by receiving init event first.
	select {
	case <-watcher.Done():
		return trace.ConnectionProblem(watcher.Error(), "watcher is closed: %v", watcher.Error())
	case <-c.ctx.Done():
		return trace.ConnectionProblem(c.ctx.Err(), "context is closing")
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	case <-timer.C:
		return trace.ConnectionProblem(nil, "timeout waiting for watcher init")
	}
	apply, err := c.fetch(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// apply will mutate cache, and possibly leave it in an invalid state
	// if an error occurs, so ensure that cache is not read.
	c.setReadOK(false)
	err = apply(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// apply was successful; cache is now readable.
	c.generation.Inc()
	c.setReadOK(true)
	c.setInitError(nil)

	// watchers have been queuing up since the last time
	// the cache was in a healthy state; broadcast OpInit.
	// It is very important that OpInit is not broadcast until
	// after we've placed the cache into a readable state.  This ensures
	// that any derivative caches do not peform their fetch operations
	// until this cache has finished its apply operations.
	c.eventsFanout.SetInit()
	defer c.eventsFanout.Reset()

	retry.Reset()

	c.notify(c.ctx, Event{Type: WatcherStarted})

	var lastStalenessWarning time.Time
	var staleEventCount int
	for {
		select {
		case <-watcher.Done():
			return trace.ConnectionProblem(watcher.Error(), "watcher is closed: %v", watcher.Error())
		case <-c.ctx.Done():
			return trace.ConnectionProblem(c.ctx.Err(), "context is closing")
		case event := <-watcher.Events():
			// check for expired resources in OpPut events and log them periodically. stale OpPut events
			// may be an indicator of poor performance, and can lead to confusing and inconsistent state
			// as the cache may prune items that aught to exist.
			//
			// NOTE: The inconsistent state mentioned above is a symptom of a deeper issue with the cache
			// design.  The cache should not expire individual items.  It should instead rely on OpDelete events
			// from backend expiries.  As soon as the cache has expired at least one item, it is no longer
			// a faithful representation of a real backend state, since it is 'anticipating' a change in
			// backend state that may or may not have actually happened.  Instead, it aught to serve the
			// most recent internally-consistent "view" of the backend, and individual consumers should
			// determine if the resources they are handling are sufficiently fresh.  Resource-level expiry
			// is a convenience/cleanup feature and aught not be relied upon for meaningful logic anyhow.
			// If we need to protect against a stale cache, we aught to invalidate the cache in its entirity, rather
			// than pruning the resources that we think *might* have been removed from the real backend.
			// TODO(fspmarshall): ^^^
			//
			if event.Type == types.OpPut && !event.Resource.Expiry().IsZero() {
				if now := c.Clock.Now(); now.After(event.Resource.Expiry()) {
					staleEventCount++
					if now.After(lastStalenessWarning.Add(time.Minute)) {
						kind := event.Resource.GetKind()
						if sk := event.Resource.GetSubKind(); sk != "" {
							kind = fmt.Sprintf("%s/%s", kind, sk)
						}
						c.Warningf("Encountered %d stale event(s), may indicate degraded backend or event system performance. last_kind=%q", staleEventCount, kind)
						lastStalenessWarning = now
						staleEventCount = 0
					}
				}
			}

			err = c.processEvent(ctx, event)
			if err != nil {
				return trace.Wrap(err)
			}
			c.notify(c.ctx, Event{Event: event, Type: EventProcessed})
		}
	}
}

func (c *Cache) watchKinds() []types.WatchKind {
	out := make([]types.WatchKind, 0, len(c.collections))
	for _, collection := range c.collections {
		out = append(out, collection.watchKind())
	}
	return out
}

// isClosing checks if the cache has begun closing.
func (c *Cache) isClosing() bool {
	if c.closed.Load() {
		// closing due to Close being called
		return true
	}

	select {
	case <-c.ctx.Done():
		// closing due to context cancellation
		return true
	default:
		// not closing
		return false
	}
}

// Close closes all outstanding and active cache operations
func (c *Cache) Close() error {
	c.closed.Store(true)
	c.cancel()
	c.eventsFanout.Close()
	return nil
}

func (c *Cache) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	applyfns := make([]func(ctx context.Context) error, 0, len(c.collections))
	for _, collection := range c.collections {
		applyfn, err := collection.fetch(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		applyfns = append(applyfns, applyfn)
	}
	return func(ctx context.Context) error {
		for _, applyfn := range applyfns {
			if err := applyfn(ctx); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *Cache) processEvent(ctx context.Context, event types.Event) error {
	resourceKind := resourceKindFromResource(event.Resource)
	collection, ok := c.collections[resourceKind]
	if !ok {
		c.Warningf("Skipping unsupported event %v/%v",
			event.Resource.GetKind(), event.Resource.GetSubKind())
		return nil
	}
	if err := collection.processEvent(ctx, event); err != nil {
		return trace.Wrap(err)
	}
	c.eventsFanout.Emit(event)
	return nil
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (c *Cache) GetCertAuthority(id types.CertAuthID, loadSigningKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	ca, err := rg.trust.GetCertAuthority(id, loadSigningKeys, opts...)
	if trace.IsNotFound(err) && rg.IsCacheRead() {
		// release read lock early
		rg.Release()
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if ca, err := c.Config.Trust.GetCertAuthority(id, loadSigningKeys, opts...); err == nil {
			return ca, nil
		}
	}
	return ca, trace.Wrap(err)
}

// GetCertAuthorities returns a list of authorities of a given type
// loadSigningKeys controls whether signing keys should be loaded or not
func (c *Cache) GetCertAuthorities(caType types.CertAuthType, loadSigningKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.trust.GetCertAuthorities(caType, loadSigningKeys, opts...)
}

// GetStaticTokens gets the list of static tokens used to provision nodes.
func (c *Cache) GetStaticTokens() (types.StaticTokens, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.clusterConfig.GetStaticTokens()
}

// GetTokens returns all active (non-expired) provisioning tokens
func (c *Cache) GetTokens(ctx context.Context, opts ...services.MarshalOption) ([]types.ProvisionToken, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.provisioner.GetTokens(ctx, opts...)
}

// GetToken finds and returns token by ID
func (c *Cache) GetToken(ctx context.Context, name string) (types.ProvisionToken, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	token, err := rg.provisioner.GetToken(ctx, name)
	if trace.IsNotFound(err) && rg.IsCacheRead() {
		// release read lock early
		rg.Release()
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if token, err := c.Config.Provisioner.GetToken(ctx, name); err == nil {
			return token, nil
		}
	}
	return token, trace.Wrap(err)
}

// GetClusterAuditConfig gets ClusterAuditConfig from the backend.
func (c *Cache) GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.clusterConfig.GetClusterAuditConfig(ctx, opts...)
}

// GetClusterNetworkingConfig gets ClusterNetworkingConfig from the backend.
func (c *Cache) GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.clusterConfig.GetClusterNetworkingConfig(ctx, opts...)
}

// GetClusterName gets the name of the cluster from the backend.
func (c *Cache) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.clusterConfig.GetClusterName(opts...)
}

// GetRoles is a part of auth.AccessPoint implementation
func (c *Cache) GetRoles(ctx context.Context) ([]types.Role, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.access.GetRoles(ctx)
}

// GetRole is a part of auth.AccessPoint implementation
func (c *Cache) GetRole(ctx context.Context, name string) (types.Role, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	role, err := rg.access.GetRole(ctx, name)
	if trace.IsNotFound(err) && rg.IsCacheRead() {
		// release read lock early
		rg.Release()
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if role, err := c.Config.Access.GetRole(ctx, name); err == nil {
			return role, nil
		}
	}
	return role, err
}

// GetNamespace returns namespace
func (c *Cache) GetNamespace(name string) (*types.Namespace, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.presence.GetNamespace(name)
}

// GetNamespaces is a part of auth.AccessPoint implementation
func (c *Cache) GetNamespaces() ([]types.Namespace, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.presence.GetNamespaces()
}

// GetNode finds and returns a node by name and namespace.
func (c *Cache) GetNode(ctx context.Context, namespace, name string) (types.Server, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.presence.GetNode(ctx, namespace, name)
}

// GetNodes is a part of auth.AccessPoint implementation
func (c *Cache) GetNodes(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.presence.GetNodes(ctx, namespace, opts...)
}

// ListNodes is a part of auth.AccessPoint implementation
func (c *Cache) ListNodes(ctx context.Context, namespace string, limit int, startKey string) ([]types.Server, string, error) {
	rg, err := c.read()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.presence.ListNodes(ctx, namespace, limit, startKey)
}

// GetAuthServers returns a list of registered servers
func (c *Cache) GetAuthServers() ([]types.Server, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.presence.GetAuthServers()
}

// GetReverseTunnels is a part of auth.AccessPoint implementation
func (c *Cache) GetReverseTunnels(opts ...services.MarshalOption) ([]types.ReverseTunnel, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.presence.GetReverseTunnels(opts...)
}

// GetProxies is a part of auth.AccessPoint implementation
func (c *Cache) GetProxies() ([]types.Server, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.presence.GetProxies()
}

// GetRemoteClusters returns a list of remote clusters
func (c *Cache) GetRemoteClusters(opts ...services.MarshalOption) ([]types.RemoteCluster, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.presence.GetRemoteClusters(opts...)
}

// GetRemoteCluster returns a remote cluster by name
func (c *Cache) GetRemoteCluster(clusterName string) (types.RemoteCluster, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.presence.GetRemoteCluster(clusterName)
}

// GetUser is a part of auth.AccessPoint implementation.
func (c *Cache) GetUser(name string, withSecrets bool) (user types.User, err error) {
	if withSecrets { // cache never tracks user secrets
		return c.Config.Users.GetUser(name, withSecrets)
	}
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	user, err = rg.users.GetUser(name, withSecrets)
	if trace.IsNotFound(err) && rg.IsCacheRead() {
		// release read lock early
		rg.Release()
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if user, err := c.Config.Users.GetUser(name, withSecrets); err == nil {
			return user, nil
		}
	}
	return user, trace.Wrap(err)
}

// GetUsers is a part of auth.AccessPoint implementation
func (c *Cache) GetUsers(withSecrets bool) (users []types.User, err error) {
	if withSecrets { // cache never tracks user secrets
		return c.Users.GetUsers(withSecrets)
	}
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.users.GetUsers(withSecrets)
}

// GetTunnelConnections is a part of auth.AccessPoint implementation
// GetTunnelConnections are not using recent cache as they are designed
// to be called periodically and always return fresh data
func (c *Cache) GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]types.TunnelConnection, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.presence.GetTunnelConnections(clusterName, opts...)
}

// GetAllTunnelConnections is a part of auth.AccessPoint implementation
// GetAllTunnelConnections are not using recent cache, as they are designed
// to be called periodically and always return fresh data
func (c *Cache) GetAllTunnelConnections(opts ...services.MarshalOption) (conns []types.TunnelConnection, err error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.presence.GetAllTunnelConnections(opts...)
}

// GetKubeServices is a part of auth.AccessPoint implementation
func (c *Cache) GetKubeServices(ctx context.Context) ([]types.Server, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.presence.GetKubeServices(ctx)
}

// GetApplicationServers returns all registered application servers.
func (c *Cache) GetApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.presence.GetApplicationServers(ctx, namespace)
}

// GetApps returns all application resources.
func (c *Cache) GetApps(ctx context.Context) ([]types.Application, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.apps.GetApps(ctx)
}

// GetApp returns the specified application resource.
func (c *Cache) GetApp(ctx context.Context, name string) (types.Application, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.apps.GetApp(ctx, name)
}

// GetAppServers gets all application servers.
//
// DELETE IN 9.0. Deprecated, use GetApplicationServers.
func (c *Cache) GetAppServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.presence.GetAppServers(ctx, namespace, opts...)
}

// GetAppSession gets an application web session.
func (c *Cache) GetAppSession(ctx context.Context, req types.GetAppSessionRequest) (types.WebSession, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.appSession.GetAppSession(ctx, req)
}

// GetDatabaseServers returns all registered database proxy servers.
func (c *Cache) GetDatabaseServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.DatabaseServer, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.presence.GetDatabaseServers(ctx, namespace, opts...)
}

// GetDatabases returns all database resources.
func (c *Cache) GetDatabases(ctx context.Context) ([]types.Database, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.databases.GetDatabases(ctx)
}

// GetDatabase returns the specified database resource.
func (c *Cache) GetDatabase(ctx context.Context, name string) (types.Database, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.databases.GetDatabase(ctx, name)
}

// GetWebSession gets a regular web session.
func (c *Cache) GetWebSession(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.webSession.Get(ctx, req)
}

// GetWebToken gets a web token.
func (c *Cache) GetWebToken(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.webToken.Get(ctx, req)
}

// GetAuthPreference gets the cluster authentication config.
func (c *Cache) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.clusterConfig.GetAuthPreference(ctx)
}

// GetSessionRecordingConfig gets session recording configuration.
func (c *Cache) GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.clusterConfig.GetSessionRecordingConfig(ctx, opts...)
}

// GetNetworkRestrictions gets the network restrictions.
func (c *Cache) GetNetworkRestrictions(ctx context.Context) (types.NetworkRestrictions, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	return rg.restrictions.GetNetworkRestrictions(ctx)
}

// GetLock gets a lock by name.
func (c *Cache) GetLock(ctx context.Context, name string) (types.Lock, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	lock, err := rg.access.GetLock(ctx, name)
	if trace.IsNotFound(err) && rg.IsCacheRead() {
		// release read lock early
		rg.Release()
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if lock, err := c.Config.Access.GetLock(ctx, name); err == nil {
			return lock, nil
		}
	}
	return lock, trace.Wrap(err)
}

// GetLocks gets all/in-force locks that match at least one of the targets
// when specified.
func (c *Cache) GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.access.GetLocks(ctx, inForceOnly, targets...)
}

// GetWindowsDesktopServices returns all registered Windows desktop services.
func (c *Cache) GetWindowsDesktopServices(ctx context.Context) ([]types.WindowsDesktopService, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.presence.GetWindowsDesktopServices(ctx)
}

// GetWindowsDesktops returns all registered Windows desktop hosts.
func (c *Cache) GetWindowsDesktops(ctx context.Context) ([]types.WindowsDesktop, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.windowsDesktops.GetWindowsDesktops(ctx)
}

// GetWindowsDesktop returns a registered Windows desktop host.
func (c *Cache) GetWindowsDesktop(ctx context.Context, name string) (types.WindowsDesktop, error) {
	rg, err := c.read()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.windowsDesktops.GetWindowsDesktop(ctx, name)
}
