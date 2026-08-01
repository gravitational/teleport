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
	"iter"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
	authproto "github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/backendmetrics"
	"github.com/gravitational/teleport/lib/cache/internal"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/observability/tracing"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
	sortcache "github.com/gravitational/teleport/lib/utils/sortcache/v2"
)

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
	// Scope-aware kinds default (for unscoped callers) to matching only unscoped instances. Auth must have
	// access to all instances (scoped and unscoped) of the resources it manages, so it watches those kinds with an
	// explicit MODE_ALL filter.
	allScopes := types.ScopeFilterFromProto(scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ALL}.Build())
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: true},
		{Kind: types.KindCertAuthorityOverride},
		{Kind: types.KindPendingCSRRequest},
		{Kind: types.KindClusterName},
		{Kind: types.KindClusterAuditConfig},
		{Kind: types.KindClusterNetworkingConfig},
		{Kind: types.KindClusterAuthPreference},
		{Kind: types.KindSessionRecordingConfig},
		{Kind: types.KindUIConfig},
		{Kind: types.KindStaticTokens},
		{Kind: types.KindStaticScopedTokens},
		{Kind: types.KindToken},
		{Kind: types.KindUser},
		{Kind: types.KindRole},
		{Kind: scopedaccess.KindScopedRole},
		{Kind: scopedaccess.KindScopedRoleAssignment},
		{Kind: types.KindNode, ScopeFilter: allScopes},
		{Kind: types.KindProxy},
		{Kind: types.KindAuthServer},
		{Kind: types.KindReverseTunnel},
		{Kind: types.KindTunnelConnection},
		{Kind: types.KindAccessRequest},
		{Kind: types.KindAppServer, ScopeFilter: allScopes},
		{Kind: types.KindApp, ScopeFilter: allScopes},
		{Kind: types.KindBeam},
		{Kind: types.KindBeamsConfig},
		{Kind: types.KindWebSession, SubKind: types.KindSnowflakeSession, LoadSecrets: true},
		{Kind: types.KindWebSession, SubKind: types.KindAppSession, LoadSecrets: true},
		{Kind: types.KindWebSession, SubKind: types.KindWebSession, LoadSecrets: true},
		{Kind: types.KindWebToken},
		{Kind: types.KindRemoteCluster},
		{Kind: types.KindDatabaseServer, ScopeFilter: allScopes},
		{Kind: types.KindDatabaseService},
		{Kind: types.KindDatabase},
		{Kind: types.KindNetworkRestrictions},
		{Kind: types.KindLock},
		{Kind: types.KindWindowsDesktopService},
		{Kind: types.KindWindowsDesktop},
		{Kind: types.KindDynamicWindowsDesktop},
		{Kind: types.KindLinuxDesktop},
		{Kind: types.KindKubeServer, ScopeFilter: allScopes},
		{Kind: types.KindInstaller},
		{Kind: types.KindKubernetesCluster, ScopeFilter: allScopes},
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
		{Kind: types.KindAutoUpdateBotInstanceReport},
		{Kind: types.KindUserTask},
		{Kind: types.KindProvisioningPrincipalState},
		{Kind: types.KindIdentityCenterAccount},
		{Kind: types.KindIdentityCenterPrincipalAssignment},
		{Kind: types.KindIdentityCenterAccountAssignment},
		{Kind: types.KindPlugin, LoadSecrets: true},
		{Kind: types.KindPluginStaticCredentials},
		{Kind: types.KindGitServer},
		{Kind: types.KindWorkloadIdentity},
		{Kind: types.KindHealthCheckConfig},
		{Kind: types.KindRelayServer},
		{Kind: types.KindBotInstance},
		{Kind: types.KindRecordingEncryption},
		{Kind: types.KindAppAuthConfig},
		{Kind: types.KindInferenceModel},
		{Kind: types.KindInferencePolicy},
		{Kind: types.KindInferenceSecret},
		{Kind: types.KindClassifier},
		{Kind: types.KindRetrievalModel},
		{Kind: types.KindValidatedMFAChallenge},
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
	// Scope-aware kinds default (for unscoped callers) to matching only unscoped instances. The proxy is
	// unscoped but must see all instances (scoped and unscoped) of the resources it routes on, so it
	// watches those kinds with an explicit MODE_ALL filter.
	allScopes := types.ScopeFilterFromProto(scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ALL}.Build())
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
		{Kind: types.KindNode, ScopeFilter: allScopes},
		{Kind: types.KindProxy},
		{Kind: types.KindAuthServer},
		{Kind: types.KindReverseTunnel},
		{Kind: types.KindTunnelConnection},
		{Kind: types.KindAppServer, ScopeFilter: allScopes},
		{Kind: types.KindApp, ScopeFilter: allScopes},
		{Kind: types.KindWebSession, SubKind: types.KindSnowflakeSession, LoadSecrets: true},
		{Kind: types.KindWebSession, SubKind: types.KindAppSession, LoadSecrets: true},
		{Kind: types.KindWebSession, SubKind: types.KindWebSession, LoadSecrets: true},
		{Kind: types.KindWebToken},
		{Kind: types.KindRemoteCluster},
		{Kind: types.KindDatabaseServer, ScopeFilter: allScopes},
		{Kind: types.KindDatabaseService},
		{Kind: types.KindDatabase},
		{Kind: types.KindWindowsDesktopService},
		{Kind: types.KindWindowsDesktop},
		{Kind: types.KindDynamicWindowsDesktop},
		{Kind: types.KindLinuxDesktop},
		{Kind: types.KindKubeServer, ScopeFilter: allScopes},
		{Kind: types.KindKubernetesCluster, ScopeFilter: allScopes},
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
		{Kind: types.KindHealthCheckConfig},
		{Kind: types.KindAppAuthConfig},
	}
	cfg.QueueSize = defaults.ProxyQueueSize
	return cfg
}

// ForRelay sets up the given cache [Config] for use by the Relay cache.
func ForRelay(cfg Config) Config {
	cfg.target = "relay"
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, Filter: makeAllKnownCAsFilter().IntoMap()},
		{Kind: types.KindClusterAuthPreference},
		{Kind: types.KindClusterNetworkingConfig},
		{Kind: types.KindKubeServer},
		{Kind: types.KindNode},
		{Kind: types.KindRelayServer},
		{Kind: types.KindRole},
		{Kind: types.KindSessionRecordingConfig},
		{Kind: types.KindUser},
	}
	cfg.QueueSize = defaults.RelayQueueSize
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
		{Kind: types.KindLinuxDesktop},
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
		{Kind: types.KindHealthCheckConfig},
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
		{Kind: types.KindCertAuthorityOverride},
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

// ForLinuxDesktop sets up watch configuration for a Linux desktop service.
func ForLinuxDesktop(cfg Config) Config {
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
	cfg.target = "linux_desktop"
	cfg.Watches = []types.WatchKind{
		{Kind: types.KindCertAuthority, LoadSecrets: false, Filter: caFilter},
		{Kind: types.KindClusterName},
		{Kind: types.KindClusterAuditConfig},
		{Kind: types.KindClusterNetworkingConfig},
		{Kind: types.KindClusterAuthPreference},
		{Kind: types.KindSessionRecordingConfig},
		{Kind: types.KindUser},
		{Kind: types.KindRole},
		{Kind: types.KindLinuxDesktop},
	}
	cfg.QueueSize = defaults.LinuxDesktopQueueSize
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

	// engine drives the shared cache lifecycle: upstream watch, collection
	// seeding, event application, read-health publication, and event fanout.
	engine *internal.Engine

	// collections is a registry of resource collections.
	collections *collections

	// fnCache is used to perform short ttl-based caching of the results of
	// regularly called methods.
	fnCache *utils.FnCache
}

var _ authclient.Cache = (*Cache)(nil)

// acquireReadGuard provides a readGuard that may be used to determine how a
// cache read should operate. Acquiring a guard is wait-free: it takes no
// locks, and holding one does not block cache writes or resets. Reads that
// proceed against the cache operate on immutable store snapshots, so they
// remain consistent even if a reset replaces store contents mid-read.
func acquireReadGuard[T any, S collectionStore[T]](cache *Cache, c *storeCollection[T, S]) (readGuard[T, S], error) {
	if c == nil {
		// the cache was not configured to watch this resource kind; reads of
		// unwatched kinds are a misconfiguration, not a panic.
		return readGuard[T, S]{}, trace.NotImplemented("cache %q does not watch the requested resource kind", cache.Config.target)
	}
	if cache.engine.Closed() {
		return readGuard[T, S]{}, trace.Errorf("cache is closed")
	}

	kind := internal.ResourceKind{Kind: c.watch.Kind, SubKind: c.watch.SubKind}
	if cache.engine.KindConfirmed(kind) {
		return readGuard[T, S]{
			cacheRead: true,
			store:     c.store,
			snapshot:  c.store.snapshot(),
		}, nil
	}

	return readGuard[T, S]{
		cacheRead: false,
	}, nil
}

// readGuard indicates whether a read should be served from the cache or from
// the upstream backend, and carries the collection store for the cache case.
type readGuard[T any, S any] struct {
	cacheRead bool
	store     S
	// snapshot is the immutable view of the store captured when the guard
	// was acquired. All reads performed under one guard should use it so
	// that a single request observes a single consistent generation. Only
	// set when cacheRead is true.
	snapshot *sortcache.Snapshot[T]
}

// ReadCache checks if this readGuard holds a cache reference.
func (r *readGuard[T, S]) ReadCache() bool {
	return r.cacheRead
}

// Release is a no-op retained for call-site symmetry with the previous
// lock-holding guard implementation; snapshot-based reads hold no locks.
func (r *readGuard[T, S]) Release() {}

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
	// StaticScopedToken manages the cluster's static scoped tokens.
	StaticScopedToken services.StaticScopedTokenService
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
	Apps services.Applications
	// Beams is a beam reader service.
	Beams services.BeamReader
	// BeamsConfig is a beams config getter service.
	BeamsConfig services.BeamsConfigGetter
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
	AppSession services.AppSessionReader
	// WebSession holds regular web sessions.
	WebSession types.WebSessionInterface
	// WebToken holds web tokens.
	WebToken services.WebToken
	// WindowsDesktops is a Windows desktop service.
	WindowsDesktops services.WindowsDesktops
	// DynamicWindowsDesktops is a dynamic Windows desktop service.
	DynamicWindowsDesktops services.DynamicWindowsDesktops
	// LinuxDesktops is a Linux desktop service.
	LinuxDesktops services.LinuxDesktops
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
	// Registerer is used to register prometheus metrics.
	Registerer prometheus.Registerer
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
	// BotInstanceService is the upstream service that we're caching
	BotInstanceService services.BotInstance
	// RecordingEncryption manages state surrounding session recording encryption
	RecordingEncryption services.RecordingEncryption
	// Plugins is the plugin service used to retrieve plugin information.
	Plugin services.Plugins
	// AppAuthConfig is a app auth config service.
	AppAuthConfig services.AppAuthConfigReader
	// Summarizer is a summarizer service.
	Summarizer services.Summarizer
	// SubCAService reads CertAuthorityOverride resources.
	SubCAService services.SubCAServiceGetter
}

// CheckAndSetDefaults checks parameters and sets default values
func (c *Config) CheckAndSetDefaults() error {
	if c.Events == nil {
		return trace.BadParameter("missing Events parameter")
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
		if !internal.IsControlPlane(c.target) {
			c.MaxRetryPeriod = defaults.MaxLongWatcherBackoff
		}
	}
	if c.WatcherInitTimeout == 0 {
		c.WatcherInitTimeout = defaults.MaxWatcherBackoff

		// permit non-control-plane watchers to take a while to start up. slow receipt of
		// init events is a common symptom of the thundering herd effect caused by restarting
		// control plane elements.
		if !internal.IsControlPlane(c.target) {
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
	if c.Registerer == nil {
		c.Registerer = prometheus.DefaultRegisterer
	}
	if c.FanoutShards == 0 {
		c.FanoutShards = 1
	}
	return nil
}

// New creates a new instance of Cache
func New(config Config) (*Cache, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := backendmetrics.RegisterCollectors(config.Registerer); err != nil {
		return nil, trace.Wrap(err)
	}

	collections, err := setupCollections(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cs := &Cache{
		Config:      config,
		collections: collections,
		Logger: slog.With(
			teleport.ComponentKey, config.Component,
			"target", config.target,
		),
	}

	engine, err := internal.NewEngine(internal.Config{
		Target:                      config.target,
		Logger:                      cs.Logger,
		Watches:                     config.Watches,
		Handlers:                    collections.byKind,
		Events:                      config.Events,
		FanoutShards:                config.FanoutShards,
		MaxRetryPeriod:              config.MaxRetryPeriod,
		WatcherInitTimeout:          config.WatcherInitTimeout,
		CacheInitTimeout:            config.CacheInitTimeout,
		EnableRelativeExpiry:        config.EnableRelativeExpiry,
		RelativeExpiryCheckInterval: config.RelativeExpiryCheckInterval,
		RelativeExpiry:              cs.performRelativeNodeExpiry,
		EventsC:                     config.EventsC,
		Clock:                       config.Clock,
		Component:                   config.Component,
		MetricComponent:             config.MetricComponent,
		QueueSize:                   config.QueueSize,
		NeverOK:                     config.neverOK,
		Tracer:                      config.Tracer,
		Registerer:                  config.Registerer,
		DisablePartialHealth:        config.DisablePartialHealth,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cs.engine = engine

	fnCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:     time.Second,
		Clock:   config.Clock,
		Context: engine.ExitContext(),
	})
	if err != nil {
		engine.Close()
		return nil, trace.Wrap(err)
	}
	cs.fnCache = fnCache

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
	return trace.Wrap(c.engine.Start())
}

// Close closes all outstanding and active cache operations.
func (c *Cache) Close() error {
	return trace.Wrap(c.engine.Close())
}

// FirstInit returns a channel that is closed when the cache successfully initializes for the first time.
func (c *Cache) FirstInit() <-chan struct{} {
	return c.engine.FirstInit()
}

// NewStream is equivalent to NewWatcher except that it represents the event
// stream as a stream.Stream rather than a channel. Watcher style event handling
// is generally more common, but this API may be preferable for usecases where
// *many* event streams need to be allocated as it is slightly more resource-efficient.
func (c *Cache) NewStream(ctx context.Context, watch types.Watch) (stream.Stream[types.Event], error) {
	return c.engine.NewStream(ctx, watch)
}

// NewWatcher returns a new event watcher. In case of a cache
// this watcher will return events as seen by the cache,
// not the backend. This feature allows auth server
// to handle subscribers connected to the in-memory caches
// instead of reading from the backend.
func (c *Cache) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	return c.engine.NewWatcher(ctx, watch)
}

// readOK reports whether the cache is currently in a valid state for reads.
func (c *Cache) readOK() bool {
	return c.engine.ReadOK()
}

// setReadOK flips the health bit while preserving the confirmed kinds of the
// current generation. It is a test helper for simulating an unhealthy cache.
func (c *Cache) setReadOK(ok bool) {
	c.engine.SetReadOK(ok)
}

// processEvent applies a single synthesized event to the cache and emits it
// via the fanouts; used by relative expiry.
func (c *Cache) processEvent(ctx context.Context, event types.Event) error {
	return trace.Wrap(c.engine.ProcessEvent(ctx, event))
}

// Event is event used in tests.
type Event = internal.Event

const (
	// EventProcessed is emitted whenever event is processed
	EventProcessed = internal.EventProcessed
	// WatcherStarted is emitted when a new event watcher is started
	WatcherStarted = internal.WatcherStarted
	// WatcherFailed is emitted when event watcher has failed
	WatcherFailed = internal.WatcherFailed
	// Reloading is emitted when an error occurred watching events
	// and the cache is waiting to create a new watcher
	Reloading = internal.Reloading
	// RelativeExpiry notifies that relative expiry operations have
	// been run.
	RelativeExpiry = internal.RelativeExpiry
)

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

// ListResources is a part of auth.Cache implementation
func (c *Cache) ListResources(ctx context.Context, req authproto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListResources")
	defer span.End()

	if c.engine.Closed() {
		return nil, trace.Errorf("cache is closed")
	}
	kind := types.WatchKind{Kind: req.ResourceType}
	if !c.engine.KindConfirmed(internal.ResourceKind{Kind: kind.Kind, SubKind: kind.SubKind}) {
		// read from the upstream.
		resp, err := c.listResourcesFallback(ctx, req)
		return resp, trace.Wrap(err)
	}

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
	//nolint:ineffassign,staticcheck // ctx is shadowed so future downstream calls inherit the span.
	ctx, span := c.Tracer.Start(ctx, "cache/listResources")
	defer span.End()

	filter, err := services.MatchResourceFilterFromListResourceRequest(&req)
	if err != nil {
		return nil, trace.Wrap(err)
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
				resp.NextKey = services.GetCursorForResource(r)
				return &resp, nil
			}

			resp.Resources = append(resp.Resources, cloneFn(r))
		}
	}

	return &resp, nil
}

// GetUnifiedResourcesAndBotsCount returns the combined total number of nodes, app servers, database servers, kube servers, desktops, and bot instances.
func (c *Cache) GetUnifiedResourcesAndBotsCount() int {
	if !c.readOK() {
		return -1
	}

	count := 0
	if c.collections.nodes != nil {
		count += c.collections.nodes.store.len()
	}
	if c.collections.appServers != nil {
		count += c.collections.appServers.store.len()
	}
	if c.collections.dbServers != nil {
		count += c.collections.dbServers.store.len()
	}
	if c.collections.kubeServers != nil {
		count += c.collections.kubeServers.store.len()
	}
	if c.collections.windowsDesktops != nil {
		count += c.collections.windowsDesktops.store.len()
	}
	if c.collections.botInstances != nil {
		count += c.collections.botInstances.store.len()
	}

	return count
}
