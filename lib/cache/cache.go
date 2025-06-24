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
	"iter"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
	authproto "github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	apitracing "github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/observability/tracing"
	scopedrole "github.com/gravitational/teleport/lib/scopes/roles"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
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

	cacheHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "cache",
			Name:      "health",
			Help:      "Whether the cache for a particular Teleport service is healthy.",
		},
		[]string{teleport.TagCacheComponent},
	)

	cacheLastReset = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "cache",
			Name:      "last_reset_seconds",
			Help:      "The unix time in seconds that the last cache reset was performed.",
		},
		[]string{teleport.TagCacheComponent},
	)
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
	types.KindGitServer:             {},
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
		{Kind: scopedrole.KindScopedRole},
		{Kind: scopedrole.KindScopedRoleAssignment},
		{Kind: types.KindNode},
		{Kind: types.KindProxy},
		{Kind: types.KindAuthServer},
		{Kind: types.KindReverseTunnel},
		{Kind: types.KindTunnelConnection},
		{Kind: types.KindAccessRequest},
		{Kind: types.KindAppServer},
		{Kind: types.KindApp},
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
		{Kind: types.KindDynamicWindowsDesktop},
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
		{Kind: types.KindAccessGraphSettings},
		{Kind: types.KindSPIFFEFederation},
		{Kind: types.KindStaticHostUser},
		{Kind: types.KindAutoUpdateVersion},
		{Kind: types.KindAutoUpdateConfig},
		{Kind: types.KindAutoUpdateAgentRollout},
		{Kind: types.KindAutoUpdateAgentReport},
		{Kind: types.KindUserTask},
		{Kind: types.KindProvisioningPrincipalState},
		{Kind: types.KindIdentityCenterAccount},
		{Kind: types.KindIdentityCenterPrincipalAssignment},
		{Kind: types.KindIdentityCenterAccountAssignment},
		{Kind: types.KindPluginStaticCredentials},
		{Kind: types.KindGitServer},
		{Kind: types.KindWorkloadIdentity},
		{Kind: types.KindHealthCheckConfig},
		{Kind: types.KindRelayServer},
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
		{Kind: types.KindNode},
		{Kind: types.KindProxy},
		{Kind: types.KindAuthServer},
		{Kind: types.KindReverseTunnel},
		{Kind: types.KindTunnelConnection},
		{Kind: types.KindAppServer},
		{Kind: types.KindApp},
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
		{Kind: types.KindDynamicWindowsDesktop},
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
		{Kind: types.KindAutoUpdateConfig},
		{Kind: types.KindAutoUpdateVersion},
		{Kind: types.KindAutoUpdateAgentRollout},
		{Kind: types.KindUserTask},
		{Kind: types.KindGitServer},
		{Kind: types.KindRelayServer},
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
		{Kind: types.KindNode},
		{Kind: types.KindWindowsDesktop},
		{Kind: types.KindWindowsDesktopService},
		{Kind: types.KindProxy},
		{Kind: types.KindAuthServer},
		{Kind: types.KindReverseTunnel},
		{Kind: types.KindTunnelConnection},
		{Kind: types.KindAppServer},
		{Kind: types.KindRemoteCluster},
		{Kind: types.KindDatabaseServer},
		{Kind: types.KindDatabaseService},
		{Kind: types.KindKubeServer},
		{Kind: types.KindGitServer},
	}
	cfg.QueueSize = defaults.ProxyQueueSize
	return cfg
}

// ForNode sets up watch configuration for node
func ForNode(cfg Config) Config {
	var caFilter map[string]string
	if cfg.ClusterConfig != nil {
		clusterName, err := cfg.ClusterConfig.GetClusterName(context.TODO())
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
		{Kind: types.KindNetworkRestrictions},
		{Kind: types.KindStaticHostUser},
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
		{Kind: types.KindDatabase},
		{Kind: types.KindHealthCheckConfig},
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
		{Kind: types.KindWindowsDesktopService},
		{Kind: types.KindWindowsDesktop},
		{Kind: types.KindDynamicWindowsDesktop},
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
		{Kind: types.KindNode},
		{Kind: types.KindKubernetesCluster},
		{Kind: types.KindKubeServer},
		{Kind: types.KindDatabase},
		{Kind: types.KindApp},
		{Kind: types.KindDiscoveryConfig},
		{Kind: types.KindIntegration},
		{Kind: types.KindUserTask},
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

	// Logger emits log messages.
	Logger *slog.Logger

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

	// collections is a registry of resource collections.
	collections *collections

	// confirmedKinds is a map of kinds confirmed by the server to be included in the current generation
	// by resource Kind/SubKind
	confirmedKinds map[resourceKind]types.WatchKind

	// fnCache is used to perform short ttl-based caching of the results of
	// regularly called methods.
	fnCache *utils.FnCache

	eventsFanout          *services.FanoutV2
	lowVolumeEventsFanout *utils.RoundRobin[*services.FanoutV2]

	// closed indicates that the cache has been closed
	closed atomic.Bool
}

var _ authclient.Cache = (*Cache)(nil)

func (c *Cache) setInitError(err error) {
	c.initOnce.Do(func() {
		c.initErr = err
		close(c.initC)
	})

	if err == nil {
		cacheHealth.WithLabelValues(c.target).Set(1.0)
	} else {
		cacheHealth.WithLabelValues(c.target).Set(0.0)
	}
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

// acquireReadGuard provides a readGuard that may be used to determine how
// a cache read should operate. The returned guard *must* be released to prevent deadlocks.
func acquireReadGuard[T any, I comparable](cache *Cache, c *collection[T, I]) (readGuard[T, I], error) {
	if cache.closed.Load() {
		return readGuard[T, I]{}, trace.Errorf("cache is closed")
	}
	cache.rw.RLock()

	if cache.ok {
		if _, kindOK := cache.confirmedKinds[resourceKind{kind: c.watch.Kind, subkind: c.watch.SubKind}]; kindOK {
			return readGuard[T, I]{
				cacheRead: true,
				release:   cache.rw.RUnlock,
				store:     c.store,
			}, nil
		}
	}

	cache.rw.RUnlock()
	return readGuard[T, I]{
		cacheRead: false,
	}, nil
}

// readGuard holds a reference to a read-only "collection" T. If the referenced resource is in the cache,
// then readGuard also holds the release function for the read lock, and ensures that it is not double-called.
type readGuard[T any, I comparable] struct {
	cacheRead bool
	store     *store[T, I]
	once      sync.Once
	release   func()
}

// ReadCache checks if this readGuard holds a cache reference.
func (r *readGuard[T, I]) ReadCache() bool {
	return r.cacheRead
}

// Release releases the read lock if it is held.  This method
// can be called multiple times.
func (r *readGuard[T, I]) Release() {
	r.once.Do(func() {
		if r.release == nil {
			return
		}

		r.release()
	})
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
	// AutoUpdateService is an autoupdate service.
	AutoUpdateService services.AutoUpdateServiceGetter
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
	// DynamicWindowsDesktops is a dynamic Windows desktop service.
	DynamicWindowsDesktops services.DynamicWindowsDesktops
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
	// UserTasks is the user tasks service.
	UserTasks services.UserTasks
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
	// SPIFFEFederations is the SPIFFE federations service.
	SPIFFEFederations services.SPIFFEFederations
	// StaticHostUsers is the static host user service.
	StaticHostUsers services.StaticHostUser
	// WorkloadIdentity is the upstream Workload Identities service that we're
	// caching
	WorkloadIdentity services.WorkloadIdentities
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

	// ProvisioningStates is the upstream ProvisioningStates service that we're
	// caching
	ProvisioningStates services.ProvisioningStates

	// IdentityCenter is the upstream Identity Center service that we're caching
	IdentityCenter services.IdentityCenter
	// PluginStaticCredentials is the plugin static credentials services
	PluginStaticCredentials services.PluginStaticCredentials
	// GitServers is the Git server service.
	GitServers services.GitServerGetter
	// HealthCheckConfig is a health check config service.
	HealthCheckConfig services.HealthCheckConfigReader
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
	if err := metrics.RegisterPrometheusCollectors(
		cacheEventsReceived,
		cacheStaleEventsReceived,
		cacheHealth,
		cacheLastReset,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := config.CheckAndSetDefaults(); err != nil {
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

	fanout := services.NewFanoutV2(services.FanoutV2Config{})
	lowVolumeFanouts := make([]*services.FanoutV2, 0, config.FanoutShards)
	for range config.FanoutShards {
		lowVolumeFanouts = append(lowVolumeFanouts, services.NewFanoutV2(services.FanoutV2Config{}))
	}

	collections, err := setupCollections(config)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	cs := &Cache{
		ctx:                   ctx,
		cancel:                cancel,
		Config:                config,
		initC:                 make(chan struct{}),
		fnCache:               fnCache,
		eventsFanout:          fanout,
		collections:           collections,
		lowVolumeEventsFanout: utils.NewRoundRobin(lowVolumeFanouts),
		Logger: slog.With(
			teleport.ComponentKey, config.Component,
			"target", config.target,
		),
	}

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
		First:  retryutils.FullJitter(c.MaxRetryPeriod / 16),
		Driver: retryutils.NewExponentialDriver(c.MaxRetryPeriod / 16),
		Max:    c.MaxRetryPeriod,
		Jitter: retryutils.HalfJitter,
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
			c.Logger.InfoContext(c.ctx, "Cache first init succeeded")
		} else {
			c.Logger.WarnContext(c.ctx, "Cache first init failed, continuing re-init attempts in background", "error", c.initErr)
		}
	case <-c.ctx.Done():
		c.Close()
		return trace.Wrap(c.ctx.Err(), "context closed during cache init")
	case <-time.After(c.Config.CacheInitTimeout):
		c.Logger.WarnContext(c.ctx, "Cache init is taking too long, will continue in background")
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
		c.Logger.DebugContext(ctx, "Cache is closing, returning from update loop")
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
			c.Logger.WarnContext(ctx, "Re-init the cache on error", "error", err)
		}

		// events cache should be closed as well
		c.Logger.DebugContext(ctx, "Reloading cache")

		c.notify(ctx, Event{Type: Reloading, Event: types.Event{
			Resource: &types.ResourceHeader{
				Kind: retry.Duration().String(),
			},
		}})

		startedWaiting := c.Clock.Now()
		select {
		case t := <-retry.After():
			c.Logger.DebugContext(ctx, "Initiating new watch after backoff", "backoff_time", t.Sub(startedWaiting))
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
	cacheLastReset.WithLabelValues(c.target).SetToCurrentTime()
	requestKinds := c.Config.Watches
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

	fetchAndApplyStart := time.Now()

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
		c.Logger.WarnContext(ctx, "Some resource kinds unsupported by the server cannot be cached",
			"rejected", rejectedKinds,
		)
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
			FirstDuration: retryutils.HalfJitter(c.Config.RelativeExpiryCheckInterval),
			Jitter:        retryutils.SeventhJitter,
		})
	}
	defer relativeExpiryInterval.Stop()

	c.notify(c.ctx, Event{Type: WatcherStarted})

	fetchAndApplyDuration := time.Since(fetchAndApplyStart)
	if fetchAndApplyDuration > time.Second*20 {
		c.Logger.WarnContext(ctx, "slow fetch and apply",
			"cache_target", c.Config.target,
			"duration", fetchAndApplyDuration.String(),
		)
	} else {
		c.Logger.DebugContext(ctx, "fetch and apply",
			"cache_target", c.Config.target,
			"duration", fetchAndApplyDuration.String(),
		)
	}

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
						c.Logger.WarnContext(ctx, "Encountered stale event(s), may indicate degraded backend or event system performance",
							"stale_event_count", staleEventCount,
							"last_kind", kind,
						)
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

	// because event streams are not necessarily ordered across keys expiring on the
	// server announce TTL may sometimes generate false positives. Using the watcher
	// creation grace period as our safety buffer is mostly an arbitrary choice, but
	// since it approximates our expected worst-case staleness of the event stream its
	// a fairly reasonable one.
	gracePeriod := apidefaults.ServerAnnounceTTL + backend.DefaultCreationGracePeriod

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
		c.Logger.DebugContext(ctx, "Removed nodes via relative expiry",
			"removed_node_count", removed,
			"retention_threshold", retentionThreshold,
		)
	}

	return nil
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

	for kind, handler := range c.collections.byKind {
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
			applyfn, err := handler.fetch(ctx, cacheOK)
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

	handler, handlerFound := c.collections.byKind[resourceKind]

	switch {
	case handlerFound:
		switch event.Type {
		case types.OpDelete:
			if err := handler.onDelete(event.Resource); err != nil {
				if !trace.IsNotFound(err) {
					c.Logger.WarnContext(ctx, "Failed to delete resource", "error", err)
					return trace.Wrap(err)
				}
			}
		case types.OpPut:
			if err := handler.onPut(event.Resource); err != nil {
				return trace.Wrap(err)
			}
		default:
			c.Logger.WarnContext(ctx, "Skipping unsupported event type", "event", event.Type)
		}
	}

	c.eventsFanout.Emit(event)
	if !isHighVolumeResource(resourceKind.kind) {
		c.lowVolumeEventsFanout.ForEach(func(f *services.FanoutV2) {
			f.Emit(event)
		})
	}

	return nil
}

// ListResources is a part of auth.Cache implementation
func (c *Cache) ListResources(ctx context.Context, req authproto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListResources")
	defer span.End()

	if c.closed.Load() {
		return nil, trace.Errorf("cache is closed")
	}
	c.rw.RLock()

	kind := types.WatchKind{Kind: req.ResourceType}
	_, kindOK := c.confirmedKinds[resourceKind{kind: kind.Kind, subkind: kind.SubKind}]
	if !c.ok || !kindOK {
		// release the lock early and read from the upstream.
		c.rw.RUnlock()
		resp, err := c.listResourcesFallback(ctx, req)
		return resp, trace.Wrap(err)

	}

	defer c.rw.RUnlock()

	resp, err := c.listResources(ctx, req)
	return resp, trace.Wrap(err)
}

func (c *Cache) listResourcesFallback(ctx context.Context, req authproto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/listResourcesFallback")
	defer span.End()

	if req.ResourceType != types.KindNode {
		out, err := c.Config.Presence.ListResources(ctx, req)
		return out, trace.Wrap(err)
	}

	cachedNodes, err := c.getNodesWithTTLCache(ctx)
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
		ResourceType:   types.KindNode,
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

	resp, err := local.FakePaginate(servers.AsResources(), params)
	return resp, trace.Wrap(err)
}

func (c *Cache) listResources(ctx context.Context, req authproto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	_, span := c.Tracer.Start(ctx, "cache/listResources")
	defer span.End()

	filter := services.MatchResourceFilter{
		ResourceKind:   req.ResourceType,
		Labels:         req.Labels,
		SearchKeywords: req.SearchKeywords,
	}

	if req.PredicateExpression != "" {
		expression, err := services.NewResourceExpression(req.PredicateExpression)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		filter.PredicateExpression = expression
	}

	// Adjust page size, so it can't be empty.
	limit := int(req.Limit)
	if limit <= 0 {
		limit = apidefaults.DefaultChunkSize
	}

	switch req.ResourceType {
	case types.KindDatabaseServer:
		resp, err := buildListResourcesResponse(
			c.collections.dbServers.store.resources(databaseServerNameIndex, req.StartKey, ""),
			limit,
			filter,
			types.DatabaseServer.CloneResource,
		)
		return resp, trace.Wrap(err)
	case types.KindDatabaseService:
		resp, err := buildListResourcesResponse(
			c.collections.dbServices.store.resources(databaseServiceNameIndex, req.StartKey, ""),
			limit,
			filter,
			func(d types.DatabaseService) types.ResourceWithLabels {
				return d.Clone()
			},
		)
		return resp, trace.Wrap(err)
	case types.KindAppServer:
		resp, err := buildListResourcesResponse(
			c.collections.appServers.store.resources(appServerNameIndex, req.StartKey, ""),
			limit,
			filter,
			types.AppServer.CloneResource,
		)
		return resp, trace.Wrap(err)
	case types.KindNode:
		resp, err := buildListResourcesResponse(
			c.collections.nodes.store.resources(nodeNameIndex, req.StartKey, ""),
			limit,
			filter,
			types.Server.CloneResource,
		)
		return resp, trace.Wrap(err)
	case types.KindWindowsDesktopService:
		resp, err := buildListResourcesResponse(
			c.collections.windowsDesktopServices.store.resources(windowsDesktopServiceNameIndex, req.StartKey, ""),
			limit,
			filter,
			func(d types.WindowsDesktopService) types.ResourceWithLabels {
				return d.Clone()
			},
		)
		return resp, trace.Wrap(err)
	case types.KindKubeServer:
		resp, err := buildListResourcesResponse(
			c.collections.kubeServers.store.resources(kubeServerNameIndex, req.StartKey, ""),
			limit,
			filter,
			types.KubeServer.CloneResource,
		)
		return resp, trace.Wrap(err)
	case types.KindUserGroup:
		resp, err := buildListResourcesResponse(
			c.collections.userGroups.store.resources(userGroupNameIndex, req.StartKey, ""),
			limit,
			filter,
			func(g types.UserGroup) types.ResourceWithLabels {
				return g.Clone()
			},
		)
		return resp, trace.Wrap(err)
	case types.KindIdentityCenterAccount:
		resp, err := buildListResourcesResponse(
			func(yield func(types.ResourceWithLabels) bool) {
				for account := range c.collections.identityCenterAccounts.store.resources(identityCenterAccountNameIndex, req.StartKey, "") {
					if !yield(types.Resource153ToResourceWithLabels(account)) {
						return
					}
				}
			},
			limit,
			filter,
			func(r types.ResourceWithLabels) types.ResourceWithLabels {
				unwrapper := r.(types.Resource153UnwrapperT[*identitycenterv1.Account])
				return types.Resource153ToResourceWithLabels(services.IdentityCenterAccount{
					Account: proto.CloneOf(unwrapper.UnwrapT()),
				})
			},
		)
		return resp, trace.Wrap(err)
	case types.KindIdentityCenterAccountAssignment:
		resp, err := buildListResourcesResponse(
			func(yield func(types.ResourceWithLabels) bool) {
				for assignment := range c.collections.identityCenterAccountAssignments.store.resources(identityCenterAccountAssignmentNameIndex, req.StartKey, "") {
					if !yield(types.Resource153ToResourceWithLabels(assignment)) {
						return
					}
				}
			},
			limit,
			filter,
			func(r types.ResourceWithLabels) types.ResourceWithLabels {
				unwrapper := r.(types.Resource153UnwrapperT[*identitycenterv1.AccountAssignment])
				return types.Resource153ToResourceWithLabels(services.IdentityCenterAccountAssignment{
					AccountAssignment: proto.CloneOf(unwrapper.UnwrapT()),
				})
			},
		)
		return resp, trace.Wrap(err)
	case types.KindSAMLIdPServiceProvider:
		resp, err := buildListResourcesResponse(
			c.collections.samlIdPServiceProviders.store.resources(samlIdPServiceProviderNameIndex, req.StartKey, ""),
			limit,
			filter,
			types.SAMLIdPServiceProvider.CloneResource,
		)
		return resp, trace.Wrap(err)
	default:
		return nil, trace.NotImplemented("%s not implemented at ListResources", req.ResourceType)
	}
}

func buildListResourcesResponse[T types.ResourceWithLabels](resources iter.Seq[T], limit int, filter services.MatchResourceFilter, cloneFn func(T) types.ResourceWithLabels) (*types.ListResourcesResponse, error) {
	var resp types.ListResourcesResponse
	for r := range resources {
		switch match, err := services.MatchResourceByFilters(r, filter, nil /* ignore dup matches */); {
		case err != nil:
			return nil, trace.Wrap(err)
		case match:
			if len(resp.Resources) == limit {
				resp.NextKey = backend.GetPaginationKey(r)
				return &resp, nil
			}

			resp.Resources = append(resp.Resources, cloneFn(r))
		}
	}

	return &resp, nil
}
