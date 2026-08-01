// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// ProxyCache is the typed read surface of the proxy's cache: the complete
// composition of the collection read types backing the proxy topology.
// Method promotion assembles the read API with no duplicated implementations,
// and the embedded set is the compile-time capability guard — a read of a
// kind the proxy cache does not serve does not exist on this type.
//
// ProxyCache structurally satisfies [authclient.ReadProxyAccessPoint], so the
// legacy access-point wrappers are wired directly over it while consumers
// migrate to the concrete type; the interface assertion below is the
// completeness check for the composition.
//
// The watch set currently remains [ForProxy] (exact parity with the legacy
// proxy cache, including kinds that back only fanout watchers rather than
// reads); deriving the watch set from this composition is deferred until
// those remaining kinds have collection reads of their own.
type ProxyCache struct {
	certAuthorityCollection
	clusterNameCollection
	clusterAuditConfigCollection
	clusterNetworkingConfigCollection
	authPreferenceCollection
	sessionRecordingConfigCollection
	webUIConfigCollection
	userCollection
	roleCollection
	nodeCollection
	authServerCollection
	proxyServerCollection
	reverseTunnelCollection
	tunnelConnectionCollection
	remoteClusterCollection
	appCollection
	appServerCollection
	webSessionCollection
	appSessionCollection
	snowflakeSessionCollection
	webTokenCollection
	databaseCollection
	databaseServerCollection
	windowsDesktopCollection
	windowsDesktopServiceCollection
	dynamicWindowsDesktopCollection
	linuxDesktopCollection
	kubeClusterCollection
	kubeServerCollection
	kubeWaitingContainerCollection
	samlIdPServiceProviderCollection
	userGroupCollection
	integrationCollection
	autoUpdateConfigCollection
	autoUpdateVersionCollection
	autoUpdateRolloutCollection
	gitServerCollection
	relayServerCollection
	healthCheckConfigCollection

	// cache is the underlying replication cache whose lifecycle this view
	// owns. It is deliberately a named field, not an embedded one, so that
	// none of the remaining legacy-only surface leaks onto the topology type.
	cache *Cache
}

var _ authclient.ReadProxyAccessPoint = (*ProxyCache)(nil)

// NewProxyCache creates the proxy topology cache. The proxy watch set is
// applied internally — callers supply only the upstream service dependencies
// via cfg and never interact with the underlying replication cache.
func NewProxyCache(cfg Config) (*ProxyCache, error) {
	c, err := New(ForProxy(cfg))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pc, err := composeProxyCache(c)
	if err != nil {
		c.Close()
		return nil, trace.Wrap(err)
	}

	return pc, nil
}

// NewWatcher returns a new event watcher backed by the proxy cache's fanout;
// events are emitted as seen by the cache.
func (p *ProxyCache) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	return p.cache.NewWatcher(ctx, watch)
}

// NewStream is equivalent to NewWatcher except that it represents the event
// stream as a stream.Stream.
func (p *ProxyCache) NewStream(ctx context.Context, watch types.Watch) (stream.Stream[types.Event], error) {
	return p.cache.NewStream(ctx, watch)
}

// Close releases the underlying replication cache and all associated
// resources.
func (p *ProxyCache) Close() error {
	return trace.Wrap(p.cache.Close())
}

// composeProxyCache assembles the proxy view over an existing cache,
// verifying that the cache's configured watch set covers every collection the
// view serves. The verification guards against drift between [ForProxy] and
// this composition.
func composeProxyCache(c *Cache) (*ProxyCache, error) {
	p := &ProxyCache{
		certAuthorityCollection: certAuthorityCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			fnCache:  c.fnCache,
			upstream: c.Config.Trust,
			col:      c.collections.certAuthorities,
		},
		clusterNameCollection: clusterNameCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			fnCache:  c.fnCache,
			upstream: c.Config.ClusterConfig,
			col:      c.collections.clusterName,
		},
		clusterAuditConfigCollection: clusterAuditConfigCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			fnCache:  c.fnCache,
			upstream: c.Config.ClusterConfig,
			col:      c.collections.auditConfig,
		},
		clusterNetworkingConfigCollection: clusterNetworkingConfigCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			fnCache:  c.fnCache,
			upstream: c.Config.ClusterConfig,
			col:      c.collections.networkingConfig,
		},
		authPreferenceCollection: authPreferenceCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.ClusterConfig,
			col:      c.collections.authPreference,
		},
		sessionRecordingConfigCollection: sessionRecordingConfigCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.ClusterConfig,
			col:      c.collections.sessionRecordingConfig,
		},
		webUIConfigCollection: webUIConfigCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.ClusterConfig,
			col:      c.collections.uiConfigs,
		},
		userCollection: userCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.Users,
			col:      c.collections.users,
		},
		roleCollection: roleCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.Access,
			col:      c.collections.roles,
		},
		nodeCollection: nodeCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			fnCache:  c.fnCache,
			upstream: c.Config.Presence,
			col:      c.collections.nodes,
		},
		authServerCollection: authServerCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.Presence,
			col:      c.collections.authServers,
		},
		proxyServerCollection: proxyServerCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.Presence,
			col:      c.collections.proxyServers,
		},
		reverseTunnelCollection: reverseTunnelCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.Presence,
			col:      c.collections.reverseTunnels,
		},
		tunnelConnectionCollection: tunnelConnectionCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.Trust,
			col:      c.collections.tunnelConnections,
		},
		remoteClusterCollection: remoteClusterCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			fnCache:  c.fnCache,
			upstream: c.Config.Trust,
			col:      c.collections.remoteClusters,
		},
		appCollection: appCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.Apps,
			col:      c.collections.apps,
		},
		appServerCollection: appServerCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			logger:   c.Logger,
			upstream: c.Config.Presence,
			col:      c.collections.appServers,
		},
		webSessionCollection: webSessionCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			logger:   c.Logger,
			upstream: c.Config.WebSession,
			col:      c.collections.webSessions,
		},
		appSessionCollection: appSessionCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			logger:   c.Logger,
			upstream: c.Config.AppSession,
			col:      c.collections.appSessions,
		},
		snowflakeSessionCollection: snowflakeSessionCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			logger:   c.Logger,
			upstream: c.Config.SnowflakeSession,
			col:      c.collections.snowflakeSessions,
		},
		webTokenCollection: webTokenCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.WebToken,
			col:      c.collections.webTokens,
		},
		databaseCollection: databaseCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.Databases,
			col:      c.collections.dbs,
		},
		databaseServerCollection: databaseServerCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			logger:   c.Logger,
			upstream: c.Config.Presence,
			col:      c.collections.dbServers,
		},
		windowsDesktopCollection: windowsDesktopCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.WindowsDesktops,
			col:      c.collections.windowsDesktops,
		},
		windowsDesktopServiceCollection: windowsDesktopServiceCollection{
			engine:           c.engine,
			tracer:           c.Tracer,
			upstream:         c.Config.Presence,
			upstreamDesktops: c.Config.WindowsDesktops,
			col:              c.collections.windowsDesktopServices,
		},
		dynamicWindowsDesktopCollection: dynamicWindowsDesktopCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.DynamicWindowsDesktops,
			col:      c.collections.dynamicWindowsDesktops,
		},
		linuxDesktopCollection: linuxDesktopCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.LinuxDesktops,
			col:      c.collections.linuxDesktops,
		},
		kubeClusterCollection: kubeClusterCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.Kubernetes,
			col:      c.collections.kubeClusters,
		},
		kubeServerCollection: kubeServerCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			logger:   c.Logger,
			upstream: c.Config.Presence,
			col:      c.collections.kubeServers,
		},
		kubeWaitingContainerCollection: kubeWaitingContainerCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.KubeWaitingContainers,
			col:      c.collections.kubeWaitingContainers,
		},
		samlIdPServiceProviderCollection: samlIdPServiceProviderCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.SAMLIdPServiceProviders,
			col:      c.collections.samlIdPServiceProviders,
		},
		userGroupCollection: userGroupCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.UserGroups,
			col:      c.collections.userGroups,
		},
		integrationCollection: integrationCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.Integrations,
			col:      c.collections.integrations,
		},
		autoUpdateConfigCollection: autoUpdateConfigCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			fnCache:  c.fnCache,
			upstream: c.Config.AutoUpdateService,
			col:      c.collections.autoUpdateConfig,
		},
		autoUpdateVersionCollection: autoUpdateVersionCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			fnCache:  c.fnCache,
			upstream: c.Config.AutoUpdateService,
			col:      c.collections.autoUpdateVersion,
		},
		autoUpdateRolloutCollection: autoUpdateRolloutCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			fnCache:  c.fnCache,
			upstream: c.Config.AutoUpdateService,
			col:      c.collections.autoUpdateRollout,
		},
		gitServerCollection: gitServerCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.GitServers,
			col:      c.collections.gitServers,
		},
		relayServerCollection: relayServerCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.Presence,
			col:      c.collections.relayServers,
		},
		healthCheckConfigCollection: healthCheckConfigCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.HealthCheckConfig,
			col:      c.collections.healthCheckConfig,
		},
		cache: c,
	}

	for name, configured := range map[string]bool{
		types.KindCertAuthority:           p.certAuthorityCollection.col != nil,
		types.KindClusterName:             p.clusterNameCollection.col != nil,
		types.KindClusterAuditConfig:      p.clusterAuditConfigCollection.col != nil,
		types.KindClusterNetworkingConfig: p.clusterNetworkingConfigCollection.col != nil,
		types.KindClusterAuthPreference:   p.authPreferenceCollection.col != nil,
		types.KindSessionRecordingConfig:  p.sessionRecordingConfigCollection.col != nil,
		types.KindUIConfig:                p.webUIConfigCollection.col != nil,
		types.KindUser:                    p.userCollection.col != nil,
		types.KindRole:                    p.roleCollection.col != nil,
		types.KindNode:                    p.nodeCollection.col != nil,
		types.KindAuthServer:              p.authServerCollection.col != nil,
		types.KindProxy:                   p.proxyServerCollection.col != nil,
		types.KindReverseTunnel:           p.reverseTunnelCollection.col != nil,
		types.KindTunnelConnection:        p.tunnelConnectionCollection.col != nil,
		types.KindRemoteCluster:           p.remoteClusterCollection.col != nil,
		types.KindApp:                     p.appCollection.col != nil,
		types.KindAppServer:               p.appServerCollection.col != nil,
		types.KindWebSession:              p.webSessionCollection.col != nil,
		types.KindAppSession:              p.appSessionCollection.col != nil,
		types.KindSnowflakeSession:        p.snowflakeSessionCollection.col != nil,
		types.KindWebToken:                p.webTokenCollection.col != nil,
		types.KindDatabase:                p.databaseCollection.col != nil,
		types.KindDatabaseServer:          p.databaseServerCollection.col != nil,
		types.KindWindowsDesktop:          p.windowsDesktopCollection.col != nil,
		types.KindWindowsDesktopService:   p.windowsDesktopServiceCollection.col != nil,
		types.KindDynamicWindowsDesktop:   p.dynamicWindowsDesktopCollection.col != nil,
		types.KindLinuxDesktop:            p.linuxDesktopCollection.col != nil,
		types.KindKubernetesCluster:       p.kubeClusterCollection.col != nil,
		types.KindKubeServer:              p.kubeServerCollection.col != nil,
		types.KindKubeWaitingContainer:    p.kubeWaitingContainerCollection.col != nil,
		types.KindSAMLIdPServiceProvider:  p.samlIdPServiceProviderCollection.col != nil,
		types.KindUserGroup:               p.userGroupCollection.col != nil,
		types.KindIntegration:             p.integrationCollection.col != nil,
		types.KindAutoUpdateConfig:        p.autoUpdateConfigCollection.col != nil,
		types.KindAutoUpdateVersion:       p.autoUpdateVersionCollection.col != nil,
		types.KindAutoUpdateAgentRollout:  p.autoUpdateRolloutCollection.col != nil,
		types.KindGitServer:               p.gitServerCollection.col != nil,
		types.KindRelayServer:             p.relayServerCollection.col != nil,
		types.KindHealthCheckConfig:       p.healthCheckConfigCollection.col != nil,
	} {
		if !configured {
			return nil, trace.BadParameter("cache %q does not watch %q, which is required by ProxyCache", c.Config.target, name)
		}
	}

	return p, nil
}
