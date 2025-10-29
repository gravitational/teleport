/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package mongodb

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"
)

var (
	// depCache is a singleton connection cache.
	depCache deploymentCache
)

// deploymentCache is used to cache topology connection pools and share them
// for client connections from the same client session.
// The reason this was added is that MongoDB clients establish multiple
// connections to Teleport and the agent's MongoDB driver also establishes
// multiple connections to the upstream database server, which amplifies the
// number of connections established, so we avoid connection amplification by
// sharing connections.
type deploymentCache struct {
	cache utils.SyncMap[sessionKey, *sharedDeployment]
}

// sessionKey is the key used to determine if two connections belong to
// the same session, which we share to reduce the number of connections to the
// database server.
type sessionKey struct {
	username            string
	dbUser              string
	dbName              string
	dbServerName        string
	teleportDBURI       string
	teleportClusterName string
}

func newSessionKey(sctx *common.Session) sessionKey {
	return sessionKey{
		username:            sctx.Identity.Username,
		dbUser:              sctx.DatabaseUser,
		dbName:              sctx.DatabaseName,
		dbServerName:        sctx.Database.GetName(),
		teleportDBURI:       sctx.Database.GetURI(),
		teleportClusterName: sctx.ClusterName,
	}
}

// sharedDeployment is a reference counted [deployment].
type sharedDeployment struct {
	deployment *deployment
	// count is the shared session count. After the last client closes their
	// session, count will be negative and the session will be deleted from
	// the cache. It is not safe to read or write to this field without holding
	// the cache lock.
	count int
}

// get looks for a [deployment] in the cache and if one does not exist it calls
// the given connect func to establish and cache a new deployment.
func (c *deploymentCache) get(
	ctx context.Context,
	sessionCtx *common.Session,
	connectFn func(ctx context.Context, sessionCtx *common.Session) (*deployment, error),
) (*deployment, error) {
	key := newSessionKey(sessionCtx)
	var (
		dep        *deployment
		connectErr error
	)
	c.cache.Write(func(deployments map[sessionKey]*sharedDeployment) {
		if item, ok := deployments[key]; ok {
			item.count++
			dep = item.deployment
			return
		}
		dep, connectErr = connectFn(ctx, sessionCtx)
		if connectErr != nil {
			return
		}
		deployments[key] = &sharedDeployment{deployment: dep}
	})
	return dep, trace.Wrap(connectErr)
}

// put returns a shared deployment to the cache and returns true if it was
// evicted from the cache as a result.
func (c *deploymentCache) put(sessionCtx *common.Session) bool {
	key := newSessionKey(sessionCtx)
	var evict bool
	c.cache.Write(func(deployments map[sessionKey]*sharedDeployment) {
		item, ok := deployments[key]
		if !ok {
			evict = true
			return
		}
		item.count--
		evict = item.count < 0
		if evict {
			delete(deployments, key)
		}
	})
	return evict
}
