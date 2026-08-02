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
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// DatabasesCache is the typed read surface of the database service's cache:
// the complete composition of the collection read types backing the database
// topology. Method promotion assembles the read API with no duplicated
// implementations, and the embedded set is the compile-time capability guard —
// a read of a kind the database cache does not serve does not exist on this
// type.
//
// DatabasesCache structurally satisfies [authclient.ReadDatabaseAccessPoint],
// so the legacy access-point wrappers are wired directly over it while
// consumers migrate to the concrete type; the interface assertion below is
// the completeness check for the composition.
//
// The watch set currently remains [ForDatabases] (exact parity with the
// legacy database cache); deriving the watch set from this composition is
// deferred alongside the proxy's.
type DatabasesCache struct {
	certAuthorityCollection
	clusterNameCollection
	clusterAuditConfigCollection
	clusterNetworkingConfigCollection
	authPreferenceCollection
	sessionRecordingConfigCollection
	userCollection
	roleCollection
	proxyServerCollection
	databaseCollection
	healthCheckConfigCollection

	// cache is the underlying replication cache whose lifecycle this view
	// owns. It is deliberately a named field, not an embedded one, so that
	// none of the remaining legacy-only surface leaks onto the topology type.
	cache *Cache
}

var _ authclient.ReadDatabaseAccessPoint = (*DatabasesCache)(nil)

// NewDatabasesCache creates the database service topology cache. The database
// watch set is applied internally — callers supply only the upstream service
// dependencies via cfg and never interact with the underlying replication
// cache.
func NewDatabasesCache(cfg Config) (*DatabasesCache, error) {
	c, err := New(ForDatabases(cfg))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dc, err := composeDatabasesCache(c)
	if err != nil {
		c.Close()
		return nil, trace.Wrap(err)
	}

	return dc, nil
}

// NewWatcher returns a new event watcher backed by the database cache's
// fanout; events are emitted as seen by the cache.
func (d *DatabasesCache) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	return d.cache.NewWatcher(ctx, watch)
}

// Close releases the underlying replication cache and all associated
// resources.
func (d *DatabasesCache) Close() error {
	return trace.Wrap(d.cache.Close())
}

// composeDatabasesCache assembles the database view over an existing cache,
// verifying that the cache's configured watch set covers every collection the
// view serves. The verification guards against drift between [ForDatabases]
// and this composition.
func composeDatabasesCache(c *Cache) (*DatabasesCache, error) {
	d := &DatabasesCache{
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
		databaseCollection: databaseCollection{
			engine:   c.engine,
			tracer:   c.Tracer,
			upstream: c.Config.Databases,
			col:      c.collections.dbs,
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
		types.KindCertAuthority:           d.certAuthorityCollection.col != nil,
		types.KindClusterName:             d.clusterNameCollection.col != nil,
		types.KindClusterAuditConfig:      d.clusterAuditConfigCollection.col != nil,
		types.KindClusterNetworkingConfig: d.clusterNetworkingConfigCollection.col != nil,
		types.KindClusterAuthPreference:   d.authPreferenceCollection.col != nil,
		types.KindSessionRecordingConfig:  d.sessionRecordingConfigCollection.col != nil,
		types.KindUser:                    d.userCollection.col != nil,
		types.KindRole:                    d.roleCollection.col != nil,
		types.KindProxy:                   d.proxyServerCollection.col != nil,
		types.KindDatabase:                d.databaseCollection.col != nil,
		types.KindHealthCheckConfig:       d.healthCheckConfigCollection.col != nil,
	} {
		if !configured {
			return nil, trace.BadParameter("cache %q does not watch %q, which is required by DatabasesCache", c.Config.target, name)
		}
	}

	return d, nil
}
