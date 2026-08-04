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

	"github.com/gravitational/teleport/api/types"
)

// AppsCache is the typed read surface of the app service's cache: the
// complete composition of the collection read types backing the app topology.
// Method promotion assembles the read API with no duplicated implementations,
// and the embedded set is the compile-time capability guard — a read of a
// kind the app cache does not serve does not exist on this type.
//
// AppsCache structurally satisfies the app agent's access point interface;
// the completeness check for the composition is the interface assertion in
// lib/srv/app.
//
// The watch set currently remains [ForApps] (exact parity with the legacy app
// cache); deriving the watch set from this composition is deferred alongside
// the proxy's.
type AppsCache struct {
	certAuthorityCollection
	clusterNameCollection
	clusterAuditConfigCollection
	clusterNetworkingConfigCollection
	authPreferenceCollection
	sessionRecordingConfigCollection
	userCollection
	roleCollection
	proxyServerCollection
	appCollection

	// cache is the underlying replication cache whose lifecycle this view
	// owns. It is deliberately a named field, not an embedded one, so that
	// none of the remaining legacy-only surface leaks onto the topology type.
	cache *Cache
}

// NewAppsCache creates the app service topology cache. The app watch set is
// applied internally — callers supply only the upstream service dependencies
// via cfg and never interact with the underlying replication cache.
func NewAppsCache(cfg Config) (*AppsCache, error) {
	c, err := New(ForApps(cfg))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ac, err := composeAppsCache(c)
	if err != nil {
		c.Close()
		return nil, trace.Wrap(err)
	}

	return ac, nil
}

// NewWatcher returns a new event watcher backed by the app cache's fanout;
// events are emitted as seen by the cache.
func (a *AppsCache) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	return a.cache.NewWatcher(ctx, watch)
}

// Close releases the underlying replication cache and all associated
// resources.
func (a *AppsCache) Close() error {
	return trace.Wrap(a.cache.Close())
}

// composeAppsCache assembles the app view over an existing cache, verifying
// that the cache's configured watch set covers every collection the view
// serves. The verification guards against drift between [ForApps] and this
// composition.
func composeAppsCache(c *Cache) (*AppsCache, error) {
	a := &AppsCache{
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
		proxyServerCollection: proxyServerCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.Presence,
			col:      c.collections.proxyServers,
		},
		appCollection: appCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.Apps,
			col:      c.collections.apps,
		},
		cache: c,
	}

	for name, configured := range map[string]bool{
		types.KindCertAuthority:           a.certAuthorityCollection.col != nil,
		types.KindClusterName:             a.clusterNameCollection.col != nil,
		types.KindClusterAuditConfig:      a.clusterAuditConfigCollection.col != nil,
		types.KindClusterNetworkingConfig: a.clusterNetworkingConfigCollection.col != nil,
		types.KindClusterAuthPreference:   a.authPreferenceCollection.col != nil,
		types.KindSessionRecordingConfig:  a.sessionRecordingConfigCollection.col != nil,
		types.KindUser:                    a.userCollection.col != nil,
		types.KindRole:                    a.roleCollection.col != nil,
		types.KindProxy:                   a.proxyServerCollection.col != nil,
		types.KindApp:                     a.appCollection.col != nil,
	} {
		if !configured {
			return nil, trace.BadParameter("cache %q does not watch %q, which is required by AppsCache", c.Config.target, name)
		}
	}

	return a, nil
}
