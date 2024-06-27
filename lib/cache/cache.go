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

package cache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	apitracing "github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/simple"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

var (
	cacheEventsReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricCacheEventsReceived,
			Help:      "Number of events received by a Teleport service cache. Teleport's Auth Service, Proxy Service, and other services cache incoming events related to their service.",
		},
		[]string{teleport.TagCacheComponent},
	)
	cacheStaleEventsReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricStaleCacheEventsReceived,
			Help:      "Number of stale events received by a Teleport service cache. A high percentage of stale events can indicate a degraded backend.",
		},
		[]string{teleport.TagCacheComponent},
	)

	cacheCollectors = []prometheus.Collector{cacheEventsReceived, cacheStaleEventsReceived}
)

// highVolumeResources is the set of cached resources that tend to produce high
// event volumes (e.g. heartbeat resources). high volume events, and the watchers that
// care about them, are separated into a dedicated event fanout system in order to
// reduce the amount of load on watchers that only care about cluster state resources.
// peripheral agents that scale linearly with cluster resources (e.g. nodes) should never
// watch events of this kind.
var highVolumeResources = map[string]struct{}{
	types.KindNode:                  {},
	types.KindAppServer:             {},
	types.KindDatabaseServer:        {},
	types.KindDatabaseService:       {},
	types.KindWindowsDesktopService: {},
	types.KindKubeServer:            {},
	types.KindDatabaseObject:        {},
}

func isHighVolumeResource(kind string) bool {
	_, ok := highVolumeResources[kind]
	return ok
}

// makeAllKnownCAsFilter makes a filter that matches all known CA types.
// This should be installed by default on every CA watcher, unless a filter is
// otherwise specified, to avoid complicated server-side hacks if/when we add
// a new CA type.
// This is different from a nil/empty filter in that all the CA types that the
// client knows about will be returned rather than all the CA types that the
// server knows about.
func makeAllKnownCAsFilter() types.CertAuthorityFilter {
	filter := make(types.CertAuthorityFilter, len(types.CertAuthTypes))
	for _, t := range types.CertAuthTypes {
		filter[t] = types.Wildcard
	}
	return filter
}

// ForAuth sets up watch configuration for the auth server
func ForAuth(cfg Config) Config {
	cfg.target = "auth"
	cfg.EnableRelativeExpiry = true
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: true},
		{Kind: types.KindClusterName},
		{Kind: types.KindClusterAuditConfig},
		{Kind: types.KindClusterNetworkingConfig},
		{Kind: types.KindClusterAuthPreference},
		{Kind: types.KindSessionRecordingConfig},
		{Kind: types.KindUIConfig},
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
		{Kind: types.KindWebSession, SubKind: types.KindSAMLIdPSession, LoadSecrets: true},
		{Kind: types.KindWebSession, SubKind: types.KindSnowflakeSession, LoadSecrets: true},
		{Kind: types.KindWebSession, SubKind: types.KindAppSession, LoadSecrets: true},
		{Kind: types.KindWebSession, SubKind: types.KindWebSession, LoadSecrets: true},
		{Kind: types.KindWebToken},
		{Kind: types.KindRemoteCluster},
		{Kind: types.KindDatabaseServer},
		{Kind: types.KindDatabaseService},
		{Kind: types.KindDatabase},
		{Kind: types.KindNetworkRestrictions},
		{Kind: types.KindLock},
		{Kind: types.KindWindowsDesktopService},
		{Kind: types.KindWindowsDesktop},
		{Kind: types.KindKubeServer},
		{Kind: types.KindInstaller},
		{Kind: types.KindKubernetesCluster},
		{Kind: types.KindCrownJewel},
		{Kind: types.KindSAMLIdPServiceProvider},
		{Kind: types.KindUserGroup},
		{Kind: types.KindOktaImportRule},
		{Kind: types.KindOktaAssignment},
		{Kind: types.KindIntegration},
		{Kind: types.KindHeadlessAuthentication},
		{Kind: types.KindUserLoginState},
		{Kind: types.KindDiscoveryConfig},
		{Kind: types.KindAuditQuery},
		{Kind: types.KindSecurityReport},
		{Kind: types.KindSecurityReportState},
		{Kind: types.KindAccessList},
		{Kind: types.KindAccessListMember},
		{Kind: types.KindAccessListReview},
		{Kind: types.KindKubeWaitingContainer},
		{Kind: types.KindNotification},
		{Kind: types.KindGlobalNotification},
		{Kind: types.KindAccessMonitoringRule},
		{Kind: types.KindDatabaseObject},
	}
	cfg.QueueSize = defaults.AuthQueueSize
	// We don't want to enable partial health for auth cache because auth uses an event stream
	// from the local backend which must support all resource kinds. We want to catch it early if it doesn't.
	cfg.DisablePartialHealth = true
	// auth server shards its event fanout system in order to reduce lock contention in very large clusters.
	cfg.FanoutShards = 64
	return cfg
}

// ForProxy sets up watch configuration for proxy
func ForProxy(cfg Config) Config {
	cfg.target = "proxy"
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: false, Filter: makeAllKnownCAsFilter().IntoMap()},
		{Kind: types.KindClusterName},
		{Kind: types.KindClusterAuditConfig},
		{Kind: types.KindClusterNetworkingConfig},
		{Kind: types.KindClusterAuthPreference},
		{Kind: types.KindSessionRecordingConfig},
		{Kind: types.KindUIConfig},
		{Kind: types.KindUser},
		{Kind: types.KindRole},
		{Kind: types.KindNamespace},
		{Kind: types.KindNode},
		{Kind: types.KindProxy},
		{Kind: types.KindAuthServer},
		{Kind: types.KindReverseTunnel},
		{Kind: types.KindTunnelConnection},
		{Kind: types.KindAppServer},
		{Kind: types.KindApp},
		{Kind: types.KindWebSession, SubKind: types.KindSAMLIdPSession, LoadSecrets: true},
		{Kind: types.KindWebSession, SubKind: types.KindSnowflakeSession, LoadSecrets: true},
		{Kind: types.KindWebSession, SubKind: types.KindAppSession, LoadSecrets: true},
		{Kind: types.KindWebSession, SubKind: types.KindWebSession, LoadSecrets: true},
		{Kind: types.KindWebToken},
		{Kind: types.KindRemoteCluster},
		{Kind: types.KindDatabaseServer},
		{Kind: types.KindDatabaseService},
		{Kind: types.KindDatabase},
		{Kind: types.KindWindowsDesktopService},
		{Kind: types.KindWindowsDesktop},
		{Kind: types.KindKubeServer},
		{Kind: types.KindInstaller},
		{Kind: types.KindKubernetesCluster},
		{Kind: types.KindSAMLIdPServiceProvider},
		{Kind: types.KindUserGroup},
		{Kind: types.KindIntegration},
		{Kind: types.KindAuditQuery},
		{Kind: types.KindSecurityReport},
		{Kind: types.KindSecurityReportState},
		{Kind: types.KindKubeWaitingContainer},
	}
	cfg.QueueSize = defaults.ProxyQueueSize
	return cfg
}

// ForRemoteProxy sets up watch configuration for remote proxies.
func ForRemoteProxy(cfg Config) Config {
	cfg.target = "remote-proxy"
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: false, Filter: makeAllKnownCAsFilter().IntoMap()},
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
		{Kind: types.KindRemoteCluster},
		{Kind: types.KindDatabaseServer},
		{Kind: types.KindDatabaseService},
		{Kind: types.KindKubeServer},
	}
	cfg.QueueSize = defaults.ProxyQueueSize
	return cfg
}

// ForNode sets up watch configuration for node
func ForNode(cfg Config) Config {
	var caFilter map[string]string
	if cfg.ClusterConfig != nil {
		clusterName, err := cfg.ClusterConfig.GetClusterName()
		if err == nil {
			caFilter = types.CertAuthorityFilter{
				types.HostCA: clusterName.GetClusterName(),
				types.UserCA: types.Wildcard,
			}.IntoMap()
		}
	}
	cfg.target = "node"
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, Filter: caFilter},
		{Kind: types.KindClusterName},
		{Kind: types.KindClusterAuditConfig},
		{Kind: types.KindClusterNetworkingConfig},
		{Kind: types.KindClusterAuthPreference},
		{Kind: types.KindSessionRecordingConfig},
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
		{Kind: types.KindCertAuthority, LoadSecrets: false, Filter: makeAllKnownCAsFilter().IntoMap()},
		{Kind: types.KindClusterName},
		{Kind: types.KindClusterAuditConfig},
		{Kind: types.KindClusterNetworkingConfig},
		{Kind: types.KindClusterAuthPreference},
		{Kind: types.KindSessionRecordingConfig},
		{Kind: types.KindUser},
		{Kind: types.KindRole},
		{Kind: types.KindNamespace, Name: apidefaults.Namespace},
		{Kind: types.KindKubeServer},
		{Kind: types.KindKubernetesCluster},
		{Kind: types.KindKubeWaitingContainer},
	}
	cfg.QueueSize = defaults.KubernetesQueueSize
	return cfg
}

// ForApps sets up watch configuration for apps.
func ForApps(cfg Config) Config {
	cfg.target = "apps"
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: false, Filter: makeAllKnownCAsFilter().IntoMap()},
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
	cfg.target = "db"
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: false, Filter: makeAllKnownCAsFilter().IntoMap()},
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
		{Kind: types.KindCertAuthority, LoadSecrets: false, Filter: makeAllKnownCAsFilter().IntoMap()},
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

// ForDiscovery sets up watch configuration for discovery servers.
func ForDiscovery(cfg Config) Config {
	cfg.target = "discovery"
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: false, Filter: makeAllKnownCAsFilter().IntoMap()},
		{Kind: types.KindClusterName},
		{Kind: types.KindNamespace, Name: apidefaults.Namespace},
		{Kind: types.KindNode},
		{Kind: types.KindKubernetesCluster},
		{Kind: types.KindKubeServer},
		{Kind: types.KindDatabase},
		{Kind: types.KindApp},
		{Kind: types.KindDiscoveryConfig},
		{Kind: types.KindIntegration},
		{Kind: types.KindProxy},
	}
	cfg.QueueSize = defaults.DiscoveryQueueSize
	return cfg
}

// ForOkta sets up watch configuration for Okta servers.
func ForOkta(cfg Config) Config {
	cfg.target = "okta"
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindClusterName},
		{Kind: types.KindCertAuthority, LoadSecrets: false, Filter: makeAllKnownCAsFilter().IntoMap()},
		{Kind: types.KindUser},
		{Kind: types.KindAppServer},
		{Kind: types.KindClusterNetworkingConfig},
		{Kind: types.KindUserGroup},
		{Kind: types.KindOktaImportRule},
		{Kind: types.KindOktaAssignment},
		{Kind: types.KindProxy},
		{Kind: types.KindRole},
		{Kind: types.KindClusterAuthPreference},
		{Kind: types.KindAccessList},
		{Kind: types.KindAccessListMember},
	}
	cfg.QueueSize = defaults.DiscoveryQueueSize
	return cfg
}

// SetupConfigFn is a function that sets up configuration
// for cache
type SetupConfigFn func(c Config) Config

// Cache implements auth.Cache interface and remembers
// the previously returned upstream value for each API call.
//
// This which can be used if the upstream AccessPoint goes offline
type Cache struct {
	Config

	// Entry is a logging entry
	Logger *log.Entry

	// rw is used to prevent reads of invalid cache states.  From a
	// memory-safety perspective, this RWMutex is used to protect
	// the `ok` and `confirmedKinds` fields. *However*, cache reads
	// must hold the read lock for the duration of the read, not just
	// when checking the `ok` or `confirmedKinds` fields. Since the write
	// lock must be held in order to modify the `ok` field, this serves
	// to ensure that all in-progress reads complete *before* a reset can begin.
	rw sync.RWMutex
	// ok indicates whether the cache is in a valid state for reads.
	// If `ok` is `false`, reads are forwarded directly to the backend.
	ok bool

	// generation is a counter that is incremented each time a healthy
	// state is established.  A generation of zero means that a healthy
	// state was never established.  Note that a generation of zero does
	// not preclude `ok` being true in the case that we have loaded a
	// previously healthy state from the backend.
	generation atomic.Uint64

	// initOnce protects initC and initErr.
	initOnce sync.Once
	// initC is closed on the first attempt to initialize the
	// cache, whether or not it is successful.  Once initC
	// has returned, initErr is safe to read.
	initC chan struct{}
	// initErr is set if the first attempt to initialize the cache
	// fails.
	initErr error

	// ctx is a cache exit context
	ctx context.Context
	// cancel triggers exit context closure
	cancel context.CancelFunc

	// collections is a registry of resource collections
	collections *cacheCollections

	// confirmedKinds is a map of kinds confirmed by the server to be included in the current generation
	// by resource Kind/SubKind
	confirmedKinds map[resourceKind]types.WatchKind

	// fnCache is used to perform short ttl-based caching of the results of
	// regularly called methods.
	fnCache *utils.FnCache

	trustCache                   services.Trust
	clusterConfigCache           services.ClusterConfiguration
	provisionerCache             services.Provisioner
	usersCache                   services.UsersService
	accessCache                  services.Access
	dynamicAccessCache           services.DynamicAccessExt
	presenceCache                services.Presence
	restrictionsCache            services.Restrictions
	appsCache                    services.Apps
	kubernetesCache              services.Kubernetes
	crownJewelsCache             services.CrownJewels
	databaseServicesCache        services.DatabaseServices
	databasesCache               services.Databases
	databaseObjectsCache         *local.DatabaseObjectService
	appSessionCache              services.AppSession
	snowflakeSessionCache        services.SnowflakeSession
	samlIdPSessionCache          services.SAMLIdPSession //nolint:revive // Because we want this to be IdP.
	webSessionCache              types.WebSessionInterface
	webTokenCache                types.WebTokenInterface
	windowsDesktopsCache         services.WindowsDesktops
	samlIdPServiceProvidersCache services.SAMLIdPServiceProviders //nolint:revive // Because we want this to be IdP.
	userGroupsCache              services.UserGroups
	oktaCache                    services.Okta
	integrationsCache            services.Integrations
	discoveryConfigsCache        services.DiscoveryConfigs
	headlessAuthenticationsCache services.HeadlessAuthenticationService
	secReportsCache              services.SecReports
	userLoginStateCache          services.UserLoginStates
	accessListCache              *simple.AccessListService
	eventsFanout                 *services.FanoutV2
	lowVolumeEventsFanout        *utils.RoundRobin[*services.FanoutV2]
	kubeWaitingContsCache        *local.KubeWaitingContainerService
	notificationsCache           services.Notifications
	accessMontoringRuleCache     services.AccessMonitoringRules

	// closed indicates that the cache has been closed
	closed atomic.Bool
}

func (c *Cache) setInitError(err error) {
	c.initOnce.Do(func() {
		c.initErr = err
		close(c.initC)
	})
}

// setReadStatus updates Cache.ok, which determines whether the
// cache is overall accessible for reads, and confirmedKinds
// which stores resource kinds accessible in current generation.
func (c *Cache) setReadStatus(ok bool, confirmedKinds map[resourceKind]types.WatchKind) {
	if c.neverOK {
		// we are running inside of a test where the cache
		// needs to pretend that it never becomes healthy.
		return
	}
	c.rw.Lock()
	defer c.rw.Unlock()
	c.ok = ok
	c.confirmedKinds = confirmedKinds
}

// readCollectionCache acquires the cache read lock and uses getReader() to select the appropriate target for read
// operations on resources of the specified collection. The returned guard *must* be released to prevent deadlocks.
func readCollectionCache[R any](cache *Cache, collection collectionReader[R]) (rg readGuard[R], err error) {
	if collection == nil {
		return rg, trace.BadParameter("cannot read from an uninitialized cache collection")
	}
	return readCache(cache, collection.watchKind(), collection.getReader)
}

// readListResourcesCache acquires the cache read lock and uses getReader() to select the appropriate target
// for listing resources of the specified resourceType. The returned guard *must* be released to prevent deadlocks.
func readListResourcesCache(cache *Cache, resourceType string) (readGuard[resourceGetter], error) {
	getResourceReader := func(cacheOK bool) resourceGetter {
		if cacheOK {
			return cache.presenceCache
		}
		return cache.Config.Presence
	}

	return readCache(cache, types.WatchKind{Kind: resourceType}, getResourceReader)
}

// readCache acquires the cache read lock and uses getReader() to select the appropriate target for read operations
// on resources of the specified kind. The returned guard *must* be released to prevent deadlocks.
func readCache[R any](cache *Cache, kind types.WatchKind, getReader func(cacheOK bool) R) (readGuard[R], error) {
	if cache.closed.Load() {
		return readGuard[R]{}, trace.Errorf("cache is closed")
	}
	cache.rw.RLock()

	if cache.ok {
		if _, kindOK := cache.confirmedKinds[resourceKind{kind: kind.Kind, subkind: kind.SubKind}]; kindOK {
			return readGuard[R]{
				reader:  getReader(true),
				release: cache.rw.RUnlock,
			}, nil
		}
	}

	cache.rw.RUnlock()
	return readGuard[R]{
		reader:  getReader(false),
		release: nil,
	}, nil
}

// readGuard holds a reference to a read-only "backend" R. If the referenced backed is the cache, then readGuard
// also holds the release function for the read lock, and ensures that it is not double-called.
type readGuard[R any] struct {
	reader   R
	release  func()
	released bool
}

// Release releases the read lock if it is held.  This method
// can be called multiple times, but is not thread-safe.
func (r *readGuard[R]) Release() {
	if r.release != nil && !r.released {
		r.release()
		r.released = true
	}
}

// IsCacheRead checks if this readGuard holds a cache reference.
func (r *readGuard[R]) IsCacheRead() bool {
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
	// FanoutShards is the number of event fanout shards to allocate
	FanoutShards int
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
	// Kubernetes is an kubernetes service.
	Kubernetes services.Kubernetes
	// CrownJewels is a CrownJewels service.
	CrownJewels services.CrownJewels
	// DatabaseServices is a DatabaseService service.
	DatabaseServices services.DatabaseServices
	// Databases is a databases service.
	Databases services.Databases
	// DatabaseObjects is a database object service.
	DatabaseObjects services.DatabaseObjects
	// SAMLIdPSession holds SAML IdP sessions.
	SAMLIdPSession services.SAMLIdPSession
	// SnowflakeSession holds Snowflake sessions.
	SnowflakeSession services.SnowflakeSession
	// AppSession holds application sessions.
	AppSession services.AppSession
	// WebSession holds regular web sessions.
	WebSession types.WebSessionInterface
	// WebToken holds web tokens.
	WebToken types.WebTokenInterface
	// WindowsDesktops is a windows desktop service.
	WindowsDesktops services.WindowsDesktops
	// SAMLIdPServiceProviders is a SAML IdP service providers service.
	SAMLIdPServiceProviders services.SAMLIdPServiceProviders
	// UserGroups is a user groups service.
	UserGroups services.UserGroups
	// Okta is an Okta service.
	Okta services.Okta
	// Integrations is an Integrations service.
	Integrations services.Integrations
	// DiscoveryConfigs is a DiscoveryConfigs service.
	DiscoveryConfigs services.DiscoveryConfigs
	// UserLoginStates is the user login state service.
	UserLoginStates services.UserLoginStates
	// SecEvents is the security report service.
	SecReports services.SecReports
	// AccessLists is the access lists service.
	AccessLists services.AccessLists
	// KubeWaitingContainers is the Kubernetes waiting container service.
	KubeWaitingContainers services.KubeWaitingContainer
	// Notifications is the notifications service
	Notifications services.Notifications
	// AccessMonitoringRules is the access monitoring rules service.
	AccessMonitoringRules services.AccessMonitoringRules
	// Backend is a backend for local cache
	Backend backend.Backend
	// MaxRetryPeriod is the maximum period between cache retries on failures
	MaxRetryPeriod time.Duration
	// WatcherInitTimeout is the maximum acceptable delay for an
	// OpInit after a watcher has been started (default=1m).
	WatcherInitTimeout time.Duration
	// CacheInitTimeout is the maximum amount of time that cache.New
	// will block, waiting for initialization (default=20s).
	CacheInitTimeout time.Duration
	// RelativeExpiryCheckInterval determines how often the cache performs special
	// "relative expiration" checks which are used to compensate for real backends
	// that have suffer from overly lazy ttl'ing of resources.
	RelativeExpiryCheckInterval time.Duration
	// RelativeExpiryLimit determines the maximum number of nodes that may be
	// removed during relative expiration.
	RelativeExpiryLimit int
	// EventsC is a channel for event notifications,
	// used in tests
	EventsC chan Event
	// Clock can be set to control time,
	// uses runtime clock by default
	Clock clockwork.Clock
	// Component is a component used in logs
	Component string
	// MetricComponent is a component used in metrics
	MetricComponent string
	// QueueSize is a desired queue Size
	QueueSize int
	// neverOK is used in tests to create a cache that appears to never
	// becomes healthy, meaning that it will always end up hitting the
	// real backend and the ttl cache.
	neverOK bool
	// Tracer is used to create spans
	Tracer oteltrace.Tracer
	// Unstarted indicates that the cache should not be started during New. The
	// cache is usable before it's started, but it will always hit the backend.
	Unstarted bool
	// DisablePartialHealth disables the default mode in which cache can become
	// healthy even if some of the requested resource kinds aren't
	// supported by the event source.
	DisablePartialHealth bool
	// EnableRelativeExpiry turns on purging expired items from the cache even
	// if delete events have not been received from the backend.
	EnableRelativeExpiry bool
}

// CheckAndSetDefaults checks parameters and sets default values
func (c *Config) CheckAndSetDefaults() error {
	if c.Events == nil {
		return trace.BadParameter("missing Events parameter")
	}
	if c.Backend == nil {
		return trace.BadParameter("missing Backend parameter")
	}
	if c.Context == nil {
		c.Context = context.Background()
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.MaxRetryPeriod == 0 {
		c.MaxRetryPeriod = defaults.MaxWatcherBackoff

		// non-control-plane caches should use a longer backoff in order to limit
		// thundering herd effects upon restart of control-plane elements.
		if !isControlPlane(c.target) {
			c.MaxRetryPeriod = defaults.MaxLongWatcherBackoff
		}
	}
	if c.WatcherInitTimeout == 0 {
		c.WatcherInitTimeout = defaults.MaxWatcherBackoff

		// permit non-control-plane watchers to take a while to start up. slow receipt of
		// init events is a common symptom of the thundering herd effect caused by restarting
		// control plane elements.
		if !isControlPlane(c.target) {
			c.WatcherInitTimeout = defaults.MaxLongWatcherBackoff
		}
	}
	if c.CacheInitTimeout == 0 {
		c.CacheInitTimeout = time.Second * 20
	}
	if c.RelativeExpiryCheckInterval == 0 {
		c.RelativeExpiryCheckInterval = apidefaults.ServerKeepAliveTTL() + 5*time.Second
	}
	if c.RelativeExpiryLimit == 0 {
		c.RelativeExpiryLimit = 2000
	}
	if c.Component == "" {
		c.Component = teleport.ComponentCache
	}
	if c.Tracer == nil {
		c.Tracer = tracing.NoopTracer(c.Component)
	}
	if c.FanoutShards == 0 {
		c.FanoutShards = 1
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
	// Reloading is emitted when an error occurred watching events
	// and the cache is waiting to create a new watcher
	Reloading = "reloading_cache"
	// RelativeExpiry notifies that relative expiry operations have
	// been run.
	RelativeExpiry = "relative_expiry"
)

// New creates a new instance of Cache
func New(config Config) (*Cache, error) {
	if err := metrics.RegisterPrometheusCollectors(cacheCollectors...); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	clusterConfigCache, err := local.NewClusterConfigurationService(config.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(config.Context)
	fnCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:     time.Second,
		Clock:   config.Clock,
		Context: ctx,
	})
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	//nolint:revive // Because we want this to be IdP.
	samlIdPServiceProvidersCache, err := local.NewSAMLIdPServiceProviderService(config.Backend)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	userGroupsCache, err := local.NewUserGroupService(config.Backend)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	oktaCache, err := local.NewOktaService(config.Backend, config.Clock)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	integrationsCache, err := local.NewIntegrationsService(config.Backend, local.WithIntegrationsServiceCacheMode(true))
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	discoveryConfigsCache, err := local.NewDiscoveryConfigService(config.Backend)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	secReportsCache, err := local.NewSecReportsService(config.Backend, config.Clock)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	userLoginStatesCache, err := local.NewUserLoginStateService(config.Backend)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	accessListCache, err := simple.NewAccessListService(config.Backend)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	databaseObjectsCache, err := local.NewDatabaseObjectService(config.Backend)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	notificationsCache, err := local.NewNotificationsService(config.Backend, config.Clock)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	accessMonitoringRuleCache, err := local.NewAccessMonitoringRulesService(config.Backend)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	fanout := services.NewFanoutV2(services.FanoutV2Config{})
	lowVolumeFanouts := make([]*services.FanoutV2, 0, config.FanoutShards)
	for i := 0; i < config.FanoutShards; i++ {
		lowVolumeFanouts = append(lowVolumeFanouts, services.NewFanoutV2(services.FanoutV2Config{}))
	}

	kubeWaitingContsCache, err := local.NewKubeWaitingContainerService(config.Backend)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	crownJewelCache, err := local.NewCrownJewelsService(config.Backend)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	cs := &Cache{
		ctx:                          ctx,
		cancel:                       cancel,
		Config:                       config,
		initC:                        make(chan struct{}),
		fnCache:                      fnCache,
		trustCache:                   local.NewCAService(config.Backend),
		clusterConfigCache:           clusterConfigCache,
		provisionerCache:             local.NewProvisioningService(config.Backend),
		usersCache:                   local.NewIdentityService(config.Backend),
		accessCache:                  local.NewAccessService(config.Backend),
		dynamicAccessCache:           local.NewDynamicAccessService(config.Backend),
		presenceCache:                local.NewPresenceService(config.Backend),
		restrictionsCache:            local.NewRestrictionsService(config.Backend),
		appsCache:                    local.NewAppService(config.Backend),
		kubernetesCache:              local.NewKubernetesService(config.Backend),
		crownJewelsCache:             crownJewelCache,
		databaseServicesCache:        local.NewDatabaseServicesService(config.Backend),
		databasesCache:               local.NewDatabasesService(config.Backend),
		appSessionCache:              local.NewIdentityService(config.Backend),
		snowflakeSessionCache:        local.NewIdentityService(config.Backend),
		samlIdPSessionCache:          local.NewIdentityService(config.Backend),
		webSessionCache:              local.NewIdentityService(config.Backend).WebSessions(),
		webTokenCache:                local.NewIdentityService(config.Backend).WebTokens(),
		windowsDesktopsCache:         local.NewWindowsDesktopService(config.Backend),
		accessMontoringRuleCache:     accessMonitoringRuleCache,
		samlIdPServiceProvidersCache: samlIdPServiceProvidersCache,
		userGroupsCache:              userGroupsCache,
		oktaCache:                    oktaCache,
		integrationsCache:            integrationsCache,
		discoveryConfigsCache:        discoveryConfigsCache,
		headlessAuthenticationsCache: local.NewIdentityService(config.Backend),
		secReportsCache:              secReportsCache,
		userLoginStateCache:          userLoginStatesCache,
		accessListCache:              accessListCache,
		databaseObjectsCache:         databaseObjectsCache,
		notificationsCache:           notificationsCache,
		eventsFanout:                 fanout,
		lowVolumeEventsFanout:        utils.NewRoundRobin(lowVolumeFanouts),
		kubeWaitingContsCache:        kubeWaitingContsCache,
		Logger: log.WithFields(log.Fields{
			teleport.ComponentKey: config.Component,
		}),
	}
	collections, err := setupCollections(cs, config.Watches)
	if err != nil {
		cs.Close()
		return nil, trace.Wrap(err)
	}
	cs.collections = collections

	if config.Unstarted {
		return cs, nil
	}

	if err := cs.Start(); err != nil {
		cs.Close()
		return nil, trace.Wrap(err)
	}

	return cs, nil
}

// Start the cache. Should only be called once.
func (c *Cache) Start() error {
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:  utils.FullJitter(c.MaxRetryPeriod / 16),
		Driver: retryutils.NewExponentialDriver(c.MaxRetryPeriod / 16),
		Max:    c.MaxRetryPeriod,
		Jitter: retryutils.NewHalfJitter(),
		Clock:  c.Clock,
	})
	if err != nil {
		c.Close()
		return trace.Wrap(err)
	}

	go c.update(c.ctx, retry)

	select {
	case <-c.initC:
		if c.initErr == nil {
			c.Logger.Infof("Cache %q first init succeeded.", c.Config.target)
		} else {
			c.Logger.WithError(c.initErr).Warnf("Cache %q first init failed, continuing re-init attempts in background.", c.Config.target)
		}
	case <-c.ctx.Done():
		c.Close()
		return trace.Wrap(c.ctx.Err(), "context closed during cache init")
	case <-time.After(c.Config.CacheInitTimeout):
		c.Logger.Warn("Cache init is taking too long, will continue in background.")
	}
	return nil
}

// NewStream is equivalent to NewWatcher except that it represents the event
// stream as a stream.Stream rather than a channel. Watcher style event handling
// is generally more common, but this API may be preferable for usecases where
// *many* event streams need to be allocated as it is slightly more resource-efficient.
func (c *Cache) NewStream(ctx context.Context, watch types.Watch) (stream.Stream[types.Event], error) {
	ctx, span := c.Tracer.Start(ctx, "cache/NewStream")
	defer span.End()

	validKinds, highVolume, err := c.validateWatchRequest(watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	watch.Kinds = validKinds
	if highVolume {
		// watch request includes high volume resources, register with the
		// full fanout instance.
		return c.eventsFanout.NewStream(ctx, watch), nil
	}
	// watch request does not contain high volume resources, register with
	// the low volume fanout instance (improves performance at scale).
	return c.lowVolumeEventsFanout.Next().NewStream(ctx, watch), nil
}

// NewWatcher returns a new event watcher. In case of a cache
// this watcher will return events as seen by the cache,
// not the backend. This feature allows auth server
// to handle subscribers connected to the in-memory caches
// instead of reading from the backend.
func (c *Cache) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/NewWatcher")
	defer span.End()

	validKinds, highVolume, err := c.validateWatchRequest(watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	watch.Kinds = validKinds
	if highVolume {
		// watch request includes high volume resources, register with the
		// full fanout instance.
		return c.eventsFanout.NewWatcher(ctx, watch)
	}
	// watch request does not contain high volume resources, register with
	// the low volume fanout instance (improves performance at scale).
	return c.lowVolumeEventsFanout.Next().NewWatcher(ctx, watch)
}

func (c *Cache) validateWatchRequest(watch types.Watch) (kinds []types.WatchKind, highVolume bool, err error) {
	c.rw.RLock()
	cacheOK := c.ok
	confirmedKinds := c.confirmedKinds
	c.rw.RUnlock()

	validKinds := make([]types.WatchKind, 0, len(watch.Kinds))
	var containsHighVolumeResource bool
Outer:
	for _, requested := range watch.Kinds {
		if isHighVolumeResource(requested.Kind) {
			containsHighVolumeResource = true
		}
		if cacheOK {
			// if cache has been initialized, we already know which kinds are confirmed by the event source
			// and can validate the kinds requested for fanout against that.
			key := resourceKind{kind: requested.Kind, subkind: requested.SubKind}
			if confirmed, ok := confirmedKinds[key]; !ok || !confirmed.Contains(requested) {
				if watch.AllowPartialSuccess {
					continue
				}
				return nil, false, trace.BadParameter("cache %q does not support watching resource %q", c.Config.target, requested.Kind)
			}
			validKinds = append(validKinds, requested)
		} else {
			// otherwise, we can only perform preliminary validation against the kinds that cache has been configured for,
			// and the returned fanout watcher might fail later when cache receives and propagates its OpInit event.
			for _, configured := range c.Config.Watches {
				if requested.Kind == configured.Kind && requested.SubKind == configured.SubKind && configured.Contains(requested) {
					validKinds = append(validKinds, requested)
					continue Outer
				}
			}
			if watch.AllowPartialSuccess {
				continue
			}
			return nil, false, trace.BadParameter("cache %q does not support watching resource %q", c.Config.target, requested.Kind)
		}
	}

	if len(validKinds) == 0 {
		return nil, false, trace.BadParameter("cache %q does not support any of the requested resources", c.Config.target)
	}

	return validKinds, containsHighVolumeResource, nil
}

func (c *Cache) update(ctx context.Context, retry retryutils.Retry) {
	defer func() {
		c.Logger.Debug("Cache is closing, returning from update loop.")
		// ensure that close operations have been run
		c.Close()
	}()
	timer := time.NewTimer(c.Config.WatcherInitTimeout)
	for {
		err := c.fetchAndWatch(ctx, retry, timer)
		c.setInitError(err)
		if c.isClosing() {
			return
		}
		if err != nil {
			c.Logger.Warnf("Re-init the cache on error: %v", err)
		}

		// events cache should be closed as well
		c.Logger.Debug("Reloading cache.")

		c.notify(ctx, Event{Type: Reloading, Event: types.Event{
			Resource: &types.ResourceHeader{
				Kind: retry.Duration().String(),
			},
		}})

		startedWaiting := c.Clock.Now()
		select {
		case t := <-retry.After():
			c.Logger.Debugf("Initiating new watch after waiting %v.", t.Sub(startedWaiting))
			retry.Inc()
		case <-c.ctx.Done():
			return
		}
	}
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
// own unique iterator over the event stream. If client loses connection
// or fails to keep up with the stream, the server will terminate
// the channel and client will have to re-initialize.
//
// 2. Replays of stale events. Etcd provides a strong
// mechanism to track the versions of the storage - revisions
// of every operation that are uniquely numbered and monotonically
// and consistently ordered thanks to Raft. Unfortunately, DynamoDB
// does not provide such a mechanism for its event system, so
// some tradeoffs have to be made:
//
//	a. We assume that events are ordered in regards to the
//	individual key operations which is the guarantees both Etcd and DynamoDB
//	provide.
//	b. Thanks to the init event sent by the server on a successful connect,
//	and guarantees 1 and 2a, client assumes that once it connects and receives an event,
//	it will not miss any events, however it can receive stale events.
//	Event could be stale, if it relates to a change that happened before
//	the version read by client from the database, for example,
//	given the event stream: 1. Update a=1 2. Delete a 3. Put a = 2
//	Client could have subscribed before event 1 happened,
//	read the value a=2 and then received events 1 and 2 and 3.
//	The cache will replay all events 1, 2 and 3 and end up in the correct
//	state 3. If we had a consistent revision number, we could
//	have skipped 1 and 2, but in the absence of such mechanism in Dynamo
//	we assume that this cache will eventually end up in a correct state
//	potentially lagging behind the state of the database.
func (c *Cache) fetchAndWatch(ctx context.Context, retry retryutils.Retry, timer *time.Timer) error {
	requestKinds := c.watchKinds()
	watcher, err := c.Events.NewWatcher(c.ctx, types.Watch{
		Name:                c.Component,
		Kinds:               requestKinds,
		QueueSize:           c.QueueSize,
		MetricComponent:     c.MetricComponent,
		AllowPartialSuccess: !c.DisablePartialHealth,
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

	var confirmedKinds []types.WatchKind

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
	// wrapping up without of sync replica.
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
		if watchStatus, ok := event.Resource.(types.WatchStatus); ok {
			confirmedKinds = watchStatus.GetKinds()
		} else {
			// this event was generated by an old Auth service that doesn't support partial success mode,
			// which means that we can assume all requested kinds to be confirmed.
			confirmedKinds = requestKinds
		}
	case <-timer.C:
		return trace.ConnectionProblem(nil, "timeout waiting for watcher init")
	}

	confirmedKindsMap := make(map[resourceKind]types.WatchKind, len(confirmedKinds))
	for _, kind := range confirmedKinds {
		confirmedKindsMap[resourceKind{kind: kind.Kind, subkind: kind.SubKind}] = kind
	}
	if len(confirmedKinds) < len(requestKinds) {
		rejectedKinds := make([]string, 0, len(requestKinds)-len(confirmedKinds))
		for _, kind := range requestKinds {
			key := resourceKind{kind: kind.Kind, subkind: kind.SubKind}
			if _, ok := confirmedKindsMap[key]; !ok {
				rejectedKinds = append(rejectedKinds, key.String())
			}
		}
		c.Logger.WithField("rejected", rejectedKinds).Warn("Some resource kinds unsupported by the server cannot be cached")
	}

	apply, err := c.fetch(ctx, confirmedKindsMap)
	if err != nil {
		return trace.Wrap(err)
	}

	// apply will mutate cache, and possibly leave it in an invalid state
	// if an error occurs, so ensure that cache is not read.
	c.setReadStatus(false, nil)
	err = apply(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// apply was successful; cache is now readable.
	c.generation.Add(1)
	c.setReadStatus(true, confirmedKindsMap)
	c.setInitError(nil)

	// watchers have been queuing up since the last time
	// the cache was in a healthy state; broadcast OpInit.
	// It is very important that OpInit is not broadcast until
	// after we've placed the cache into a readable state.  This ensures
	// that any derivative caches do not perform their fetch operations
	// until this cache has finished its apply operations.
	c.eventsFanout.SetInit(confirmedKinds)
	c.lowVolumeEventsFanout.ForEach(func(f *services.FanoutV2) {
		f.SetInit(confirmedKinds)
	})
	defer c.eventsFanout.Reset()
	defer c.lowVolumeEventsFanout.ForEach(func(f *services.FanoutV2) {
		f.Reset()
	})

	retry.Reset()

	// Only enable relative node expiry for the auth cache.
	relativeExpiryInterval := interval.NewNoop()
	if c.EnableRelativeExpiry {
		relativeExpiryInterval = interval.New(interval.Config{
			Duration:      c.Config.RelativeExpiryCheckInterval,
			FirstDuration: utils.HalfJitter(c.Config.RelativeExpiryCheckInterval),
			Jitter:        retryutils.NewSeventhJitter(),
		})
	}
	defer relativeExpiryInterval.Stop()

	c.notify(c.ctx, Event{Type: WatcherStarted})

	var lastStalenessWarning time.Time
	var staleEventCount int
	for {
		select {
		case <-watcher.Done():
			return trace.ConnectionProblem(watcher.Error(), "watcher is closed: %v", watcher.Error())
		case <-c.ctx.Done():
			return trace.ConnectionProblem(c.ctx.Err(), "context is closing")
		case <-relativeExpiryInterval.Next():
			if err := c.performRelativeNodeExpiry(ctx); err != nil {
				return trace.Wrap(err)
			}
			c.notify(ctx, Event{Type: RelativeExpiry})
		case event := <-watcher.Events():
			// check for expired resources in OpPut events and log them periodically. stale OpPut events
			// may be an indicator of poor performance, and can lead to confusing and inconsistent state
			// as the cache may prune items that aught to exist.
			//
			// NOTE: The inconsistent state mentioned above is a symptom of a deeper issue with the cache
			// design.  The cache should not expire individual items.  It should instead rely on OpDelete events
			// from backend expires. As soon as the cache has expired at least one item, it is no longer
			// a faithful representation of a real backend state, since it is 'anticipating' a change in
			// backend state that may or may not have actually happened.  Instead, it aught to serve the
			// most recent internally-consistent "view" of the backend, and individual consumers should
			// determine if the resources they are handling are sufficiently fresh.  Resource-level expiry
			// is a convenience/cleanup feature and aught not be relied upon for meaningful logic anyhow.
			// If we need to protect against a stale cache, we aught to invalidate the cache in its entirety, rather
			// than pruning the resources that we think *might* have been removed from the real backend.
			// TODO(fspmarshall): ^^^
			//
			cacheEventsReceived.WithLabelValues(c.target).Inc()
			if event.Type == types.OpPut && !event.Resource.Expiry().IsZero() {
				if now := c.Clock.Now(); now.After(event.Resource.Expiry()) {
					cacheStaleEventsReceived.WithLabelValues(c.target).Inc()
					staleEventCount++
					if now.After(lastStalenessWarning.Add(time.Minute)) {
						kind := event.Resource.GetKind()
						if sk := event.Resource.GetSubKind(); sk != "" {
							kind = fmt.Sprintf("%s/%s", kind, sk)
						}
						c.Logger.WithField("last_kind", kind).Warnf("Encountered %d stale event(s), may indicate degraded backend or event system performance.", staleEventCount)
						lastStalenessWarning = now
						staleEventCount = 0
					}
				}
			}

			if err := c.processEvent(ctx, event); err != nil {
				return trace.Wrap(err)
			}
			c.notify(c.ctx, Event{Event: event, Type: EventProcessed})
		}
	}
}

// performRelativeNodeExpiry performs a special kind of active expiration where we remove nodes
// which are clearly stale relative to their more recently heartbeated counterparts as well as
// the current time. This strategy lets us side-step issues of clock drift or general cache
// staleness by only removing items which are stale from within the cache's own "frame of
// reference".
//
// to better understand why we use this expiry strategy, it's important to understand the two
// distinct scenarios that we're trying to accommodate:
//
// 1. Expiry events are being emitted very lazily by the real backend (*hours* after the time
// at which the resource was supposed to expire).
//
// 2. The event stream itself is stale (i.e. all events show up late, not just expiry events).
//
// In the first scenario, removing items from the cache after they have passed some designated
// threshold of staleness seems reasonable.  In the second scenario, your best option is to
// faithfully serve the delayed, but internally consistent, view created by the event stream and
// not expire any items.
//
// Relative expiry is the compromise between the two above scenarios. We calculate a staleness
// threshold after which items would be removed, but we calculate it relative to the most recent
// expiry *or* the current time, depending on which is earlier. The result is that nodes are
// removed only if they are both stale from the perspective of the current clock, *and* stale
// relative to our current view of the world.
//
// *note*: this function is only sane to call when the cache and event stream are healthy, and
// cannot run concurrently with event processing.
func (c *Cache) performRelativeNodeExpiry(ctx context.Context) error {
	// TODO(fspmarshall): Start using dynamic value once it is implemented.
	gracePeriod := apidefaults.ServerAnnounceTTL

	// latestExp will be the value that we choose to consider the most recent "expired"
	// timestamp.  This will either end up being the most recently seen node expiry, or
	// the current time (whichever is earlier).
	var latestExp time.Time

	nodes, err := c.GetNodes(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err)
	}

	// iterate nodes and determine the most recent expiration value.
	for _, node := range nodes {
		if node.Expiry().IsZero() {
			continue
		}

		if node.Expiry().After(latestExp) || latestExp.IsZero() {
			// this node's expiry is more recent than the previously
			// recorded value.
			latestExp = node.Expiry()
		}
	}

	if latestExp.IsZero() {
		return nil
	}

	// if the most recent expiration value is still in the future, we use the current time
	// as the most recent expiration value instead. Unless the event stream is unhealthy, or
	// all nodes were recently removed, this should always be true.
	if now := c.Clock.Now(); latestExp.After(now) {
		latestExp = now
	}

	// we subtract gracePeriod from our most recent expiry value to get the retention
	// threshold. nodes which expired earlier than the retention threshold will be
	// removed, as we expect well-behaved backends to have emitted an expiry event
	// within the grace period.
	retentionThreshold := latestExp.Add(-gracePeriod)

	var removed int
	for _, node := range nodes {
		if node.Expiry().IsZero() || node.Expiry().After(retentionThreshold) {
			continue
		}

		// remove the node locally without emitting an event, other caches will
		// eventually remove the same node when they run their expiry logic.
		if err := c.processEvent(ctx, types.Event{
			Type: types.OpDelete,
			Resource: &types.ResourceHeader{
				Kind:     types.KindNode,
				Metadata: node.GetMetadata(),
			},
		}); err != nil {
			return trace.Wrap(err)
		}

		// high churn rates can cause purging a very large number of nodes
		// per interval, limit to a sane number such that we don't overwhelm
		// things or get too far out of sync with other caches.
		if removed++; removed >= c.Config.RelativeExpiryLimit {
			break
		}
	}

	if removed > 0 {
		c.Logger.Debugf("Removed %d nodes via relative expiry (retentionThreshold=%s).", removed, retentionThreshold)
	}

	return nil
}

func (c *Cache) watchKinds() []types.WatchKind {
	out := make([]types.WatchKind, 0, len(c.collections.byKind))
	for _, collection := range c.collections.byKind {
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
	c.lowVolumeEventsFanout.ForEach(func(f *services.FanoutV2) {
		f.Close()
	})
	return nil
}

// applyFn applies the fetched resources for a
// particular collection
type applyFn func(ctx context.Context) error

// tracedApplyFn wraps an apply function with a span that is
// a child of the provided parent span. Since the context provided
// to the applyFn won't be from fetch, we need to manually link
// the spans.
func tracedApplyFn(parent oteltrace.Span, tracer oteltrace.Tracer, kind resourceKind, f applyFn) applyFn {
	return func(ctx context.Context) (err error) {
		ctx, span := tracer.Start(
			oteltrace.ContextWithSpan(ctx, parent),
			fmt.Sprintf("cache/apply/%s", kind.String()),
		)
		defer func() { apitracing.EndSpan(span, err) }()

		return f(ctx)
	}
}

// fetchLimit determines the parallelism of the
// fetch operations based on the target. Both the
// auth and proxy caches are permitted to run parallel
// fetches for resources, while all other targets are
// throttled to limit load spiking during a mass
// restart of nodes
func fetchLimit(target string) int {
	if isControlPlane(target) {
		return 5
	}

	return 1
}

// isControlPlane checks if the cache target is a control-plane element.
func isControlPlane(target string) bool {
	switch target {
	case "auth", "proxy":
		return true
	}

	return false
}

func (c *Cache) fetch(ctx context.Context, confirmedKinds map[resourceKind]types.WatchKind) (fn applyFn, err error) {
	ctx, fetchSpan := c.Tracer.Start(ctx, "cache/fetch", oteltrace.WithAttributes(attribute.String("target", c.target)))
	defer func() { apitracing.EndSpan(fetchSpan, err) }()

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(fetchLimit(c.target))
	applyfns := make([]applyFn, len(c.collections.byKind))
	i := 0
	for kind, collection := range c.collections.byKind {
		kind, collection := kind, collection
		ii := i
		i++

		g.Go(func() (err error) {
			ctx, span := c.Tracer.Start(
				ctx,
				fmt.Sprintf("cache/fetch/%s", kind.String()),
				oteltrace.WithAttributes(
					attribute.String("target", c.target),
				),
			)
			defer func() { apitracing.EndSpan(span, err) }()

			_, cacheOK := confirmedKinds[resourceKind{kind: kind.kind, subkind: kind.subkind}]
			applyfn, err := collection.fetch(ctx, cacheOK)
			if err != nil {
				return trace.Wrap(err, "failed to fetch resource: %q", kind)
			}

			applyfns[ii] = tracedApplyFn(fetchSpan, c.Tracer, kind, applyfn)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, trace.Wrap(err)
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

// processEvent hands the event off to the appropriate collection for processing. Any
// resources which were not registered are ignored. If processing completed successfully,
// the event will be emitted via the fanout.
func (c *Cache) processEvent(ctx context.Context, event types.Event) error {
	resourceKind := resourceKindFromResource(event.Resource)
	collection, ok := c.collections.byKind[resourceKind]
	if !ok {
		c.Logger.Warnf("Skipping unsupported event %v/%v", event.Resource.GetKind(), event.Resource.GetSubKind())
		return nil
	}
	if err := collection.processEvent(ctx, event); err != nil {
		return trace.Wrap(err)
	}

	c.eventsFanout.Emit(event)
	if !isHighVolumeResource(resourceKind.kind) {
		c.lowVolumeEventsFanout.ForEach(func(f *services.FanoutV2) {
			f.Emit(event)
		})
	}

	return nil
}

type getCertAuthorityCacheKey struct {
	id types.CertAuthID
}

var _ map[getCertAuthorityCacheKey]struct{} // compile-time hashability check

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (c *Cache) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadSigningKeys bool) (types.CertAuthority, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetCertAuthority")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.certAuthorities)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.IsCacheRead() && !loadSigningKeys {
		cachedCA, err := utils.FnCacheGet(ctx, c.fnCache, getCertAuthorityCacheKey{id}, func(ctx context.Context) (types.CertAuthority, error) {
			ca, err := rg.reader.GetCertAuthority(ctx, id, loadSigningKeys)
			return ca, err
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return cachedCA.Clone(), nil
	}

	ca, err := rg.reader.GetCertAuthority(ctx, id, loadSigningKeys)
	if trace.IsNotFound(err) && rg.IsCacheRead() {
		// release read lock early
		rg.Release()
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if ca, err := c.Config.Trust.GetCertAuthority(ctx, id, loadSigningKeys); err == nil {
			return ca, nil
		}
	}
	return ca, trace.Wrap(err)
}

type getCertAuthoritiesCacheKey struct {
	caType types.CertAuthType
}

var _ map[getCertAuthoritiesCacheKey]struct{} // compile-time hashability check

// GetCertAuthorities returns a list of authorities of a given type
// loadSigningKeys controls whether signing keys should be loaded or not
func (c *Cache) GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadSigningKeys bool) ([]types.CertAuthority, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetCertAuthorities")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.certAuthorities)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	if !rg.IsCacheRead() && !loadSigningKeys {
		cachedCAs, err := utils.FnCacheGet(ctx, c.fnCache, getCertAuthoritiesCacheKey{caType}, func(ctx context.Context) ([]types.CertAuthority, error) {
			cas, err := rg.reader.GetCertAuthorities(ctx, caType, loadSigningKeys)
			return cas, trace.Wrap(err)
		})
		if err != nil || cachedCAs == nil {
			return nil, trace.Wrap(err)
		}
		cas := make([]types.CertAuthority, 0, len(cachedCAs))
		for _, ca := range cachedCAs {
			cas = append(cas, ca.Clone())
		}
		return cas, nil
	}
	return rg.reader.GetCertAuthorities(ctx, caType, loadSigningKeys)
}

// GetStaticTokens gets the list of static tokens used to provision nodes.
func (c *Cache) GetStaticTokens() (types.StaticTokens, error) {
	_, span := c.Tracer.Start(context.TODO(), "cache/GetStaticTokens")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.staticTokens)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetStaticTokens()
}

// GetTokens returns all active (non-expired) provisioning tokens
func (c *Cache) GetTokens(ctx context.Context) ([]types.ProvisionToken, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetTokens")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.tokens)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetTokens(ctx)
}

// GetToken finds and returns token by ID
func (c *Cache) GetToken(ctx context.Context, name string) (types.ProvisionToken, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetToken")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.tokens)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	token, err := rg.reader.GetToken(ctx, name)
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

type clusterConfigCacheKey struct {
	kind string
}

var _ map[clusterConfigCacheKey]struct{} // compile-time hashability check

// GetClusterAuditConfig gets ClusterAuditConfig from the backend.
func (c *Cache) GetClusterAuditConfig(ctx context.Context) (types.ClusterAuditConfig, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetClusterAuditConfig")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.clusterAuditConfigs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	if !rg.IsCacheRead() {
		cachedCfg, err := utils.FnCacheGet(ctx, c.fnCache, clusterConfigCacheKey{"audit"}, func(ctx context.Context) (types.ClusterAuditConfig, error) {
			cfg, err := rg.reader.GetClusterAuditConfig(ctx)
			return cfg, err
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return cachedCfg.Clone(), nil
	}
	return rg.reader.GetClusterAuditConfig(ctx)
}

// GetClusterNetworkingConfig gets ClusterNetworkingConfig from the backend.
func (c *Cache) GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetClusterNetworkingConfig")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.clusterNetworkingConfigs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	if !rg.IsCacheRead() {
		cachedCfg, err := utils.FnCacheGet(ctx, c.fnCache, clusterConfigCacheKey{"networking"}, func(ctx context.Context) (types.ClusterNetworkingConfig, error) {
			cfg, err := rg.reader.GetClusterNetworkingConfig(ctx)
			return cfg, err
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return cachedCfg.Clone(), nil
	}
	return rg.reader.GetClusterNetworkingConfig(ctx)
}

// GetClusterName gets the name of the cluster from the backend.
func (c *Cache) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
	ctx, span := c.Tracer.Start(context.TODO(), "cache/GetClusterName")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.clusterNames)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	if !rg.IsCacheRead() {
		cachedName, err := utils.FnCacheGet(ctx, c.fnCache, clusterConfigCacheKey{"name"}, func(ctx context.Context) (types.ClusterName, error) {
			cfg, err := rg.reader.GetClusterName(opts...)
			return cfg, err
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return cachedName.Clone(), nil
	}
	return rg.reader.GetClusterName(opts...)
}

func (c *Cache) GetUIConfig(ctx context.Context) (types.UIConfig, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetUIConfig")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.uiConfigs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	uiconfig, err := rg.reader.GetUIConfig(ctx)
	return uiconfig, trace.Wrap(err)
}

// GetInstaller gets the installer script resource for the cluster
func (c *Cache) GetInstaller(ctx context.Context, name string) (types.Installer, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetInstaller")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.installers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	inst, err := rg.reader.GetInstaller(ctx, name)
	return inst, trace.Wrap(err)
}

// GetInstallers gets all the installer script resources for the cluster
func (c *Cache) GetInstallers(ctx context.Context) ([]types.Installer, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetInstallers")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.installers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	inst, err := rg.reader.GetInstallers(ctx)
	return inst, trace.Wrap(err)
}

// GetRoles is a part of auth.Cache implementation
func (c *Cache) GetRoles(ctx context.Context) ([]types.Role, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetRoles")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.roles)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetRoles(ctx)
}

// ListRoles is a paginated role getter.
func (c *Cache) ListRoles(ctx context.Context, req *proto.ListRolesRequest) (*proto.ListRolesResponse, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListRoles")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.roles)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListRoles(ctx, req)
}

// GetRole is a part of auth.Cache implementation
func (c *Cache) GetRole(ctx context.Context, name string) (types.Role, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetRole")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.roles)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	role, err := rg.reader.GetRole(ctx, name)
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
	_, span := c.Tracer.Start(context.TODO(), "cache/GetNamespace")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.namespaces)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetNamespace(name)
}

// GetNamespaces is a part of auth.Cache implementation
func (c *Cache) GetNamespaces() ([]types.Namespace, error) {
	_, span := c.Tracer.Start(context.TODO(), "cache/GetNamespaces")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.namespaces)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetNamespaces()
}

// GetNode finds and returns a node by name and namespace.
func (c *Cache) GetNode(ctx context.Context, namespace, name string) (types.Server, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetNode")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.nodes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetNode(ctx, namespace, name)
}

type getNodesCacheKey struct {
	namespace string
}

var _ map[getNodesCacheKey]struct{} // compile-time hashability check

// GetNodes is a part of auth.Cache implementation
func (c *Cache) GetNodes(ctx context.Context, namespace string) ([]types.Server, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetNodes")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.nodes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.IsCacheRead() {
		nodes, err := c.getNodesWithTTLCache(ctx, rg.reader, namespace)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return nodes, nil
	}

	return rg.reader.GetNodes(ctx, namespace)
}

// getNodesWithTTLCache implements TTL-based caching for the GetNodes endpoint.  All nodes that will be returned from the caching layer
// must be cloned to avoid concurrent modification.
func (c *Cache) getNodesWithTTLCache(ctx context.Context, svc nodeGetter, namespace string, opts ...services.MarshalOption) ([]types.Server, error) {
	cachedNodes, err := utils.FnCacheGet(ctx, c.fnCache, getNodesCacheKey{namespace}, func(ctx context.Context) ([]types.Server, error) {
		nodes, err := svc.GetNodes(ctx, namespace)
		return nodes, err
	})

	// Nodes returned from the TTL caching layer
	// must be cloned to avoid concurrent modification.
	clonedNodes := make([]types.Server, 0, len(cachedNodes))
	for _, node := range cachedNodes {
		clonedNodes = append(clonedNodes, node.DeepCopy())
	}
	return clonedNodes, trace.Wrap(err)
}

// GetAuthServers returns a list of registered servers
func (c *Cache) GetAuthServers() ([]types.Server, error) {
	_, span := c.Tracer.Start(context.TODO(), "cache/GetAuthServers")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.authServers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetAuthServers()
}

// GetReverseTunnels is a part of auth.Cache implementation
func (c *Cache) GetReverseTunnels(ctx context.Context, opts ...services.MarshalOption) ([]types.ReverseTunnel, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetReverseTunnels")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.reverseTunnels)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetReverseTunnels(ctx, opts...)
}

// GetProxies is a part of auth.Cache implementation
func (c *Cache) GetProxies() ([]types.Server, error) {
	_, span := c.Tracer.Start(context.TODO(), "cache/GetProxies")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.proxies)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetProxies()
}

type remoteClustersCacheKey struct {
	name string
}

var _ map[remoteClustersCacheKey]struct{} // compile-time hashability check

// GetRemoteClusters returns a list of remote clusters
func (c *Cache) GetRemoteClusters(ctx context.Context) ([]types.RemoteCluster, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetRemoteClusters")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.remoteClusters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	if !rg.IsCacheRead() {
		cachedRemotes, err := utils.FnCacheGet(ctx, c.fnCache, remoteClustersCacheKey{}, func(ctx context.Context) ([]types.RemoteCluster, error) {
			remotes, err := rg.reader.GetRemoteClusters(ctx)
			return remotes, err
		})
		if err != nil || cachedRemotes == nil {
			return nil, trace.Wrap(err)
		}

		remotes := make([]types.RemoteCluster, 0, len(cachedRemotes))
		for _, remote := range cachedRemotes {
			remotes = append(remotes, remote.Clone())
		}
		return remotes, nil
	}
	return rg.reader.GetRemoteClusters(ctx)
}

// GetRemoteCluster returns a remote cluster by name
func (c *Cache) GetRemoteCluster(ctx context.Context, clusterName string) (types.RemoteCluster, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetRemoteCluster")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.remoteClusters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	if !rg.IsCacheRead() {
		cachedRemote, err := utils.FnCacheGet(ctx, c.fnCache, remoteClustersCacheKey{clusterName}, func(ctx context.Context) (types.RemoteCluster, error) {
			remote, err := rg.reader.GetRemoteCluster(ctx, clusterName)
			return remote, err
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return cachedRemote.Clone(), nil
	}
	rc, err := rg.reader.GetRemoteCluster(ctx, clusterName)
	if trace.IsNotFound(err) && rg.IsCacheRead() {
		// release read lock early
		rg.Release()
		// fallback is sane because this method is never used
		// in construction of derivative caches.
		if rc, err := c.Config.Presence.GetRemoteCluster(ctx, clusterName); err == nil {
			return rc, nil
		}
	}
	return rc, trace.Wrap(err)
}

// ListRemoteClusters returns a page of remote clusters.
func (c *Cache) ListRemoteClusters(ctx context.Context, pageSize int, nextToken string) ([]types.RemoteCluster, string, error) {
	_, span := c.Tracer.Start(ctx, "cache/ListRemoteClusters")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.remoteClusters)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	remoteClusters, token, err := rg.reader.ListRemoteClusters(ctx, pageSize, nextToken)
	return remoteClusters, token, trace.Wrap(err)
}

// GetUser is a part of auth.Cache implementation.
func (c *Cache) GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error) {
	_, span := c.Tracer.Start(ctx, "cache/GetUser")
	defer span.End()

	if withSecrets { // cache never tracks user secrets
		return c.Config.Users.GetUser(ctx, name, withSecrets)
	}
	rg, err := readCollectionCache(c, c.collections.users)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	user, err := rg.reader.GetUser(ctx, name, withSecrets)
	if trace.IsNotFound(err) && rg.IsCacheRead() {
		// release read lock early
		rg.Release()
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if user, err := c.Config.Users.GetUser(ctx, name, withSecrets); err == nil {
			return user, nil
		}
	}
	return user, trace.Wrap(err)
}

// GetUsers is a part of auth.Cache implementation
func (c *Cache) GetUsers(ctx context.Context, withSecrets bool) ([]types.User, error) {
	_, span := c.Tracer.Start(ctx, "cache/GetUsers")
	defer span.End()

	if withSecrets { // cache never tracks user secrets
		return c.Users.GetUsers(ctx, withSecrets)
	}
	rg, err := readCollectionCache(c, c.collections.users)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetUsers(ctx, withSecrets)
}

// ListUsers returns a page of users.
func (c *Cache) ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error) {
	_, span := c.Tracer.Start(ctx, "cache/ListUsers")
	defer span.End()

	if req.WithSecrets { // cache never tracks user secrets
		rsp, err := c.Users.ListUsers(ctx, req)
		return rsp, trace.Wrap(err)
	}
	rg, err := readCollectionCache(c, c.collections.users)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	rsp, err := rg.reader.ListUsers(ctx, req)
	return rsp, trace.Wrap(err)
}

// GetTunnelConnections is a part of auth.Cache implementation
func (c *Cache) GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]types.TunnelConnection, error) {
	_, span := c.Tracer.Start(context.TODO(), "cache/GetTunnelConnections")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.tunnelConnections)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetTunnelConnections(clusterName, opts...)
}

// GetAllTunnelConnections is a part of auth.Cache implementation
func (c *Cache) GetAllTunnelConnections(opts ...services.MarshalOption) (conns []types.TunnelConnection, err error) {
	_, span := c.Tracer.Start(context.TODO(), "cache/GetAllTunnelConnections")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.tunnelConnections)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetAllTunnelConnections(opts...)
}

// GetKubernetesServers is a part of auth.Cache implementation
func (c *Cache) GetKubernetesServers(ctx context.Context) ([]types.KubeServer, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetKubernetesServers")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.kubeServers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetKubernetesServers(ctx)
}

// ListKubernetesWaitingContainers lists Kubernetes ephemeral
// containers that are waiting to be created until moderated
// session conditions are met.
func (c *Cache) ListKubernetesWaitingContainers(ctx context.Context, pageSize int, pageToken string) ([]*kubewaitingcontainerpb.KubernetesWaitingContainer, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListKubernetesWaitingContainers")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.kubeWaitingContainers)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListKubernetesWaitingContainers(ctx, pageSize, pageToken)
}

// GetKubernetesWaitingContainer returns a Kubernetes ephemeral
// container that are waiting to be created until moderated
// session conditions are met.
func (c *Cache) GetKubernetesWaitingContainer(ctx context.Context, req *kubewaitingcontainerpb.GetKubernetesWaitingContainerRequest) (*kubewaitingcontainerpb.KubernetesWaitingContainer, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetKubernetesWaitingContainer")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.kubeWaitingContainers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetKubernetesWaitingContainer(ctx, req)
}

// GetApplicationServers returns all registered application servers.
func (c *Cache) GetApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetApplicationServers")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.appServers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetApplicationServers(ctx, namespace)
}

// GetKubernetesClusters returns all kubernetes cluster resources.
func (c *Cache) GetKubernetesClusters(ctx context.Context) ([]types.KubeCluster, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetKubernetesClusters")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.kubeClusters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetKubernetesClusters(ctx)
}

// GetKubernetesCluster returns the specified kubernetes cluster resource.
func (c *Cache) GetKubernetesCluster(ctx context.Context, name string) (types.KubeCluster, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetKubernetesCluster")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.kubeClusters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetKubernetesCluster(ctx, name)
}

// GetApps returns all application resources.
func (c *Cache) GetApps(ctx context.Context) ([]types.Application, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetApps")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.apps)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetApps(ctx)
}

// GetApp returns the specified application resource.
func (c *Cache) GetApp(ctx context.Context, name string) (types.Application, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetApp")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.apps)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetApp(ctx, name)
}

// GetAppSession gets an application web session.
func (c *Cache) GetAppSession(ctx context.Context, req types.GetAppSessionRequest) (types.WebSession, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAppSession")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.appSessions)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	sess, err := rg.reader.GetAppSession(ctx, req)
	if trace.IsNotFound(err) && rg.IsCacheRead() {
		// release read lock early
		rg.Release()
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if sess, err := c.Config.AppSession.GetAppSession(ctx, req); err == nil {
			c.Logger.Debugf("Cache was forced to load session %v/%v from upstream.", sess.GetSubKind(), sess.GetName())
			return sess, nil
		}
	}

	return sess, trace.Wrap(err)
}

// ListAppSessions returns a page of application web sessions.
func (c *Cache) ListAppSessions(ctx context.Context, pageSize int, pageToken, user string) ([]types.WebSession, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAppSessions")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.appSessions)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListAppSessions(ctx, pageSize, pageToken, user)
}

// GetSnowflakeSession gets Snowflake web session.
func (c *Cache) GetSnowflakeSession(ctx context.Context, req types.GetSnowflakeSessionRequest) (types.WebSession, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSnowflakeSession")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.snowflakeSessions)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	sess, err := rg.reader.GetSnowflakeSession(ctx, req)
	if trace.IsNotFound(err) && rg.IsCacheRead() {
		// release read lock early
		rg.Release()
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if sess, err := c.Config.SnowflakeSession.GetSnowflakeSession(ctx, req); err == nil {
			c.Logger.Debugf("Cache was forced to load session %v/%v from upstream.", sess.GetSubKind(), sess.GetName())
			return sess, nil
		}
	}

	return sess, trace.Wrap(err)
}

// GetSAMLIdPSession gets a SAML IdP session.
func (c *Cache) GetSAMLIdPSession(ctx context.Context, req types.GetSAMLIdPSessionRequest) (types.WebSession, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSAMLIdPSession")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.samlIdPSessions)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	sess, err := rg.reader.GetSAMLIdPSession(ctx, req)
	if trace.IsNotFound(err) && rg.IsCacheRead() {
		// release read lock early
		rg.Release()
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if sess, err := c.Config.SAMLIdPSession.GetSAMLIdPSession(ctx, req); err == nil {
			c.Logger.Debugf("Cache was forced to load session %v/%v from upstream.", sess.GetSubKind(), sess.GetName())
			return sess, nil
		}
	}

	return sess, trace.Wrap(err)
}

// GetDatabaseServers returns all registered database proxy servers.
func (c *Cache) GetDatabaseServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.DatabaseServer, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetDatabaseServers")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.databaseServers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetDatabaseServers(ctx, namespace, opts...)
}

// GetDatabases returns all database resources.
func (c *Cache) GetDatabases(ctx context.Context) ([]types.Database, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetDatabases")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.databases)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetDatabases(ctx)
}

func (c *Cache) GetDatabaseObject(ctx context.Context, name string) (*dbobjectv1.DatabaseObject, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetDatabaseObject")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.databaseObjects)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetDatabaseObject(ctx, name)
}

func (c *Cache) ListDatabaseObjects(ctx context.Context, size int, pageToken string) ([]*dbobjectv1.DatabaseObject, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListWindowsDesktopServices")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.databaseObjects)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListDatabaseObjects(ctx, size, pageToken)
}

// GetDatabase returns the specified database resource.
func (c *Cache) GetDatabase(ctx context.Context, name string) (types.Database, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetDatabase")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.databases)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetDatabase(ctx, name)
}

// GetWebSession gets a regular web session.
func (c *Cache) GetWebSession(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetWebSession")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.webSessions)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	sess, err := rg.reader.Get(ctx, req)

	if trace.IsNotFound(err) && rg.IsCacheRead() {
		// release read lock early
		rg.Release()
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if sess, err := c.Config.WebSession.Get(ctx, req); err == nil {
			c.Logger.Debugf("Cache was forced to load session %v/%v from upstream.", sess.GetSubKind(), sess.GetName())
			return sess, nil
		}
	}
	return sess, trace.Wrap(err)
}

// GetWebToken gets a web token.
func (c *Cache) GetWebToken(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetWebToken")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.webTokens)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.Get(ctx, req)
}

// GetAuthPreference gets the cluster authentication config.
func (c *Cache) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAuthPreference")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.authPreferences)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetAuthPreference(ctx)
}

// GetSessionRecordingConfig gets session recording configuration.
func (c *Cache) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSessionRecordingConfig")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.sessionRecordingConfigs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetSessionRecordingConfig(ctx)
}

// GetNetworkRestrictions gets the network restrictions.
func (c *Cache) GetNetworkRestrictions(ctx context.Context) (types.NetworkRestrictions, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetNetworkRestrictions")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.networkRestrictions)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	return rg.reader.GetNetworkRestrictions(ctx)
}

// GetLock gets a lock by name.
func (c *Cache) GetLock(ctx context.Context, name string) (types.Lock, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetLock")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.locks)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	lock, err := rg.reader.GetLock(ctx, name)
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
	ctx, span := c.Tracer.Start(ctx, "cache/GetLocks")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.locks)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetLocks(ctx, inForceOnly, targets...)
}

// GetWindowsDesktopServices returns all registered Windows desktop services.
func (c *Cache) GetWindowsDesktopServices(ctx context.Context) ([]types.WindowsDesktopService, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetWindowsDesktopServices")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.windowsDesktopServices)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetWindowsDesktopServices(ctx)
}

// GetWindowsDesktopService returns a registered Windows desktop service by name.
func (c *Cache) GetWindowsDesktopService(ctx context.Context, name string) (types.WindowsDesktopService, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetWindowsDesktopService")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.windowsDesktopServices)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetWindowsDesktopService(ctx, name)
}

// GetWindowsDesktops returns all registered Windows desktop hosts.
func (c *Cache) GetWindowsDesktops(ctx context.Context, filter types.WindowsDesktopFilter) ([]types.WindowsDesktop, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetWindowsDesktops")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.windowsDesktops)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetWindowsDesktops(ctx, filter)
}

// ListWindowsDesktops returns all registered Windows desktop hosts.
func (c *Cache) ListWindowsDesktops(ctx context.Context, req types.ListWindowsDesktopsRequest) (*types.ListWindowsDesktopsResponse, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListWindowsDesktops")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.windowsDesktops)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListWindowsDesktops(ctx, req)
}

// ListWindowsDesktopServices returns all registered Windows desktop hosts.
func (c *Cache) ListWindowsDesktopServices(ctx context.Context, req types.ListWindowsDesktopServicesRequest) (*types.ListWindowsDesktopServicesResponse, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListWindowsDesktopServices")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.windowsDesktopServices)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListWindowsDesktopServices(ctx, req)
}

// ListSAMLIdPServiceProviders returns a paginated list of SAML IdP service provider resources.
func (c *Cache) ListSAMLIdPServiceProviders(ctx context.Context, pageSize int, nextKey string) ([]types.SAMLIdPServiceProvider, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListSAMLIdPServiceProviders")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.samlIdPServiceProviders)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListSAMLIdPServiceProviders(ctx, pageSize, nextKey)
}

// GetSAMLIdPServiceProvider returns the specified SAML IdP service provider resources.
func (c *Cache) GetSAMLIdPServiceProvider(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSAMLIdPServiceProvider")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.samlIdPServiceProviders)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetSAMLIdPServiceProvider(ctx, name)
}

// ListUserGroups returns a paginated list of user group resources.
func (c *Cache) ListUserGroups(ctx context.Context, pageSize int, nextKey string) ([]types.UserGroup, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListUserGroups")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.userGroups)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListUserGroups(ctx, pageSize, nextKey)
}

// GetUserGroup returns the specified user group resources.
func (c *Cache) GetUserGroup(ctx context.Context, name string) (types.UserGroup, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetUserGroup")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.userGroups)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetUserGroup(ctx, name)
}

// ListOktaImportRules returns a paginated list of all Okta import rule resources.
func (c *Cache) ListOktaImportRules(ctx context.Context, pageSize int, nextKey string) ([]types.OktaImportRule, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListOktaImportRules")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.oktaImportRules)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListOktaImportRules(ctx, pageSize, nextKey)
}

// GetOktaImportRule returns the specified Okta import rule resources.
func (c *Cache) GetOktaImportRule(ctx context.Context, name string) (types.OktaImportRule, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetOktaImportRule")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.oktaImportRules)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetOktaImportRule(ctx, name)
}

// ListOktaAssignments returns a paginated list of all Okta assignment resources.
func (c *Cache) ListOktaAssignments(ctx context.Context, pageSize int, nextKey string) ([]types.OktaAssignment, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListOktaAssignments")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.oktaAssignments)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListOktaAssignments(ctx, pageSize, nextKey)
}

// GetOktaAssignment returns the specified Okta assignment resources.
func (c *Cache) GetOktaAssignment(ctx context.Context, name string) (types.OktaAssignment, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetOktaAssignment")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.oktaAssignments)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetOktaAssignment(ctx, name)
}

// ListIntegrations returns a paginated list of all Integrations resources.
func (c *Cache) ListIntegrations(ctx context.Context, pageSize int, nextKey string) ([]types.Integration, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListIntegrations")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.integrations)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListIntegrations(ctx, pageSize, nextKey)
}

// GetIntegration returns the specified Integration resources.
func (c *Cache) GetIntegration(ctx context.Context, name string) (types.Integration, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetIntegration")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.integrations)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetIntegration(ctx, name)
}

// ListDiscoveryConfigs returns a paginated list of all DiscoveryConfig resources.
func (c *Cache) ListDiscoveryConfigs(ctx context.Context, pageSize int, nextKey string) ([]*discoveryconfig.DiscoveryConfig, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListDiscoveryConfigs")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.discoveryConfigs)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListDiscoveryConfigs(ctx, pageSize, nextKey)
}

// GetDiscoveryConfig returns the specified DiscoveryConfig resource.
func (c *Cache) GetDiscoveryConfig(ctx context.Context, name string) (*discoveryconfig.DiscoveryConfig, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetDiscoveryConfig")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.discoveryConfigs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetDiscoveryConfig(ctx, name)
}

// ListCrownJewels returns a list of CrownJewel resources.
func (c *Cache) ListCrownJewels(ctx context.Context, pageSize int64, nextKey string) ([]*crownjewelv1.CrownJewel, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListCrownJewels")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.crownJewels)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListCrownJewels(ctx, pageSize, nextKey)
}

// GetCrownJewel returns the specified CrownJewel resource.
func (c *Cache) GetCrownJewel(ctx context.Context, name string) (*crownjewelv1.CrownJewel, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetCrownJewel")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.crownJewels)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetCrownJewel(ctx, name)
}

// GetSecurityAuditQuery returns the specified audit query resource.
func (c *Cache) GetSecurityAuditQuery(ctx context.Context, name string) (*secreports.AuditQuery, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSecurityAuditQuery")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.auditQueries)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetSecurityAuditQuery(ctx, name)
}

// GetSecurityAuditQueries returns a list of all audit query resources.
func (c *Cache) GetSecurityAuditQueries(ctx context.Context) ([]*secreports.AuditQuery, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSecurityAuditQueries")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.auditQueries)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetSecurityAuditQueries(ctx)
}

// ListSecurityAuditQueries returns a paginated list of all audit query resources.
func (c *Cache) ListSecurityAuditQueries(ctx context.Context, pageSize int, nextKey string) ([]*secreports.AuditQuery, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListSecurityAuditQueries")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.auditQueries)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListSecurityAuditQueries(ctx, pageSize, nextKey)
}

// GetSecurityReport returns the specified security report resource.
func (c *Cache) GetSecurityReport(ctx context.Context, name string) (*secreports.Report, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSecurityReport")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.secReports)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetSecurityReport(ctx, name)
}

// GetSecurityReports returns a list of all security report resources.
func (c *Cache) GetSecurityReports(ctx context.Context) ([]*secreports.Report, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSecurityReports")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.secReports)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetSecurityReports(ctx)
}

// ListSecurityReports returns a paginated list of all security report resources.
func (c *Cache) ListSecurityReports(ctx context.Context, pageSize int, nextKey string) ([]*secreports.Report, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListSecurityReports")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.secReports)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListSecurityReports(ctx, pageSize, nextKey)
}

// GetSecurityReportState returns the specified security report state resource.
func (c *Cache) GetSecurityReportState(ctx context.Context, name string) (*secreports.ReportState, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSecurityReportState")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.secReportsStates)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetSecurityReportState(ctx, name)
}

// GetSecurityReportsStates returns a list of all security report resources.
func (c *Cache) GetSecurityReportsStates(ctx context.Context) ([]*secreports.ReportState, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSecurityReportsStates")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.secReportsStates)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetSecurityReportsStates(ctx)
}

// ListSecurityReportsStates returns a paginated list of all security report resources.
func (c *Cache) ListSecurityReportsStates(ctx context.Context, pageSize int, nextKey string) ([]*secreports.ReportState, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListSecurityReportsStates")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.secReportsStates)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListSecurityReportsStates(ctx, pageSize, nextKey)
}

// GetUserLoginStates returns the all user login state resources.
func (c *Cache) GetUserLoginStates(ctx context.Context) ([]*userloginstate.UserLoginState, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetUserLoginStates")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.userLoginStates)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetUserLoginStates(ctx)
}

// GetUserLoginState returns the specified user login state resource.
func (c *Cache) GetUserLoginState(ctx context.Context, name string) (*userloginstate.UserLoginState, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetUserLoginState")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.userLoginStates)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uls, err := rg.reader.GetUserLoginState(ctx, name)
	if trace.IsNotFound(err) && rg.IsCacheRead() {
		// release read lock early
		rg.Release()
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if uls, err := c.Config.UserLoginStates.GetUserLoginState(ctx, name); err == nil {
			return uls, nil
		}
	}
	defer rg.Release()
	return uls, trace.Wrap(err)
}

// GetAccessLists returns a list of all access lists.
func (c *Cache) GetAccessLists(ctx context.Context) ([]*accesslist.AccessList, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAccessLists")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.accessLists)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetAccessLists(ctx)
}

// ListAccessLists returns a paginated list of access lists.
func (c *Cache) ListAccessLists(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessList, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAccessLists")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.accessLists)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListAccessLists(ctx, pageSize, nextToken)
}

// GetAccessList returns the specified access list resource.
func (c *Cache) GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAccessList")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.accessLists)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	item, err := rg.reader.GetAccessList(ctx, name)
	if trace.IsNotFound(err) && rg.IsCacheRead() {
		// release read lock early
		rg.Release()
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if item, err := c.Config.AccessLists.GetAccessList(ctx, name); err == nil {
			return item, nil
		}
	}
	return item, trace.Wrap(err)
}

// CountAccessListMembers will count all access list members.
func (c *Cache) CountAccessListMembers(ctx context.Context, accessListName string) (uint32, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/CountAccessListMembers")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.accessListMembers)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.CountAccessListMembers(ctx, accessListName)
}

// ListAccessListMembers returns a paginated list of all access list members.
// May return a DynamicAccessListError if the requested access list has an
// implicit member list and the underlying implementation does not have
// enough information to compute the dynamic member list.
func (c *Cache) ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAccessListMembers")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.accessListMembers)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListAccessListMembers(ctx, accessListName, pageSize, pageToken)
}

// ListAllAccessListMembers returns a paginated list of all access list members for all access lists.
func (c *Cache) ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAllAccessListMembers")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.accessListMembers)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListAllAccessListMembers(ctx, pageSize, pageToken)
}

// GetAccessListMember returns the specified access list member resource.
// May return a DynamicAccessListError if the requested access list has an
// implicit member list and the underlying implementation does not have
// enough information to compute the dynamic member record.
func (c *Cache) GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAccessListMember")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.accessListMembers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetAccessListMember(ctx, accessList, memberName)
}

// ListAccessListReviews will list access list reviews for a particular access list.
func (c *Cache) ListAccessListReviews(ctx context.Context, accessList string, pageSize int, pageToken string) (reviews []*accesslist.Review, nextToken string, err error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAccessListReviews")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.accessListReviews)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListAccessListReviews(ctx, accessList, pageSize, pageToken)
}

// ListUserNotifications returns a paginated list of user-specific notifications for all users.
func (c *Cache) ListUserNotifications(ctx context.Context, pageSize int, startKey string) ([]*notificationsv1.Notification, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListUserNotifications")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.userNotifications)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	defer rg.Release()

	out, nextKey, err := rg.reader.ListUserNotifications(ctx, pageSize, startKey)
	return out, nextKey, trace.Wrap(err)
}

// ListGlobalNotifications returns a paginated list of global notifications.
func (c *Cache) ListGlobalNotifications(ctx context.Context, pageSize int, startKey string) ([]*notificationsv1.GlobalNotification, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListGlobalNotifications")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.globalNotifications)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	out, nextKey, err := rg.reader.ListGlobalNotifications(ctx, pageSize, startKey)
	return out, nextKey, trace.Wrap(err)
}

// ListAccessMonitoringRules returns a paginated list of access monitoring rules.
func (c *Cache) ListAccessMonitoringRules(ctx context.Context, pageSize int, nextToken string) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAccessMonitoringRules")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.accessMonitoringRules)

	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	out, nextKey, err := rg.reader.ListAccessMonitoringRules(ctx, pageSize, nextToken)
	return out, nextKey, trace.Wrap(err)
}

// ListAccessMonitoringRulesWithFilter returns a paginated list of access monitoring rules.
func (c *Cache) ListAccessMonitoringRulesWithFilter(ctx context.Context, pageSize int, nextToken string, subjects []string, notificationName string) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAccessMonitoringRules")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.accessMonitoringRules)

	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	out, nextKey, err := rg.reader.ListAccessMonitoringRulesWithFilter(ctx, pageSize, nextToken, subjects, notificationName)
	return out, nextKey, trace.Wrap(err)
}

// GetAccessMonitoringRule returns the specified AccessMonitoringRule resources.
func (c *Cache) GetAccessMonitoringRule(ctx context.Context, name string) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAccessMonitoringRule")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.accessMonitoringRules)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetAccessMonitoringRule(ctx, name)
}

// ListResources is a part of auth.Cache implementation
func (c *Cache) ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListResources")
	defer span.End()

	rg, err := readListResourcesCache(c, req.ResourceType)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	// Cache is not healthy, but right now, only `Node` kind has an
	// implementation that falls back to TTL cache.
	if !rg.IsCacheRead() {
		switch req.ResourceType {
		case types.KindNode:
			cachedNodes, err := c.getNodesWithTTLCache(ctx, c.Config.Presence, req.Namespace)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			servers := types.Servers(cachedNodes)
			// Since TTLCaching falls back to retrieving all resources upfront, we also support
			// sorting.
			if err := servers.SortByCustom(req.SortBy); err != nil {
				return nil, trace.Wrap(err)
			}

			params := local.FakePaginateParams{
				ResourceType:   req.ResourceType,
				Limit:          req.Limit,
				Labels:         req.Labels,
				SearchKeywords: req.SearchKeywords,
				StartKey:       req.StartKey,
			}

			if req.PredicateExpression != "" {
				expression, err := services.NewResourceExpression(req.PredicateExpression)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				params.PredicateExpression = expression
			}

			return local.FakePaginate(servers.AsResources(), params)
		}
	}

	return rg.reader.ListResources(ctx, req)
}
