/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package app

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"slices"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// checkCacheTTL is how long the app servers read by checks that run on every request stay cached. The checks only
// depend on how the app is defined, which rarely changes.
const checkCacheTTL = 10 * time.Second

// appServerResolver resolves the app servers that serve one session's route, in the order the session dials them.
// Every dial resolves them again, so app servers that register after the session was created can serve it.
type appServerResolver struct {
	clusterGetter reversetunnelclient.ClusterGetter
	// route is the app whose servers are resolved.
	route routeKey
	// sessionID keys the session's rendezvous order.
	sessionID string
	// list returns the app servers registered for the route in the given cluster. It is a field rather than a method
	// so tests can control membership without running an app server watcher.
	list func(ctx context.Context, cluster reversetunnelclient.Cluster) ([]types.AppServer, error)
	// cache holds the route's app servers for checks that run on every request, shared by all of the route's sessions.
	// It must reload failed lookups, or an error would be served until the TTL passes.
	cache *utils.FnCache
}

// routeKey identifies the app servers of a route. Its own type keys their cached lookup apart from the sessions the
// cache also holds.
type routeKey struct {
	clusterName string
	name        string
	publicAddr  string
}

// newAppServerResolver returns the resolver for a session's route, which lists the route's app servers from the
// cluster's app server watcher.
func newAppServerResolver(clusterGetter reversetunnelclient.ClusterGetter, route tlsca.RouteToApp, sessionID string,
	cache *utils.FnCache) *appServerResolver {
	return &appServerResolver{
		clusterGetter: clusterGetter,
		route:         routeKey{clusterName: route.ClusterName, name: route.Name, publicAddr: route.PublicAddr},
		sessionID:     sessionID,
		list: func(ctx context.Context, cluster reversetunnelclient.Cluster) ([]types.AppServer, error) {
			servers, err := MatchUnshuffled(ctx, cluster, MatchAppServerForRoute(route.Name, route.PublicAddr))
			return servers, trace.Wrap(err)
		},
		cache: cache,
	}
}

// resolve looks up the route's cluster and returns it with the app servers currently registered for the route, in the
// order to dial them.
func (r *appServerResolver) resolve(ctx context.Context) (reversetunnelclient.Cluster, []types.AppServer, error) {
	cluster, err := r.clusterGetter.Cluster(ctx, r.route.clusterName)
	if err != nil {
		// A cluster this Proxy cannot reach, such as a leaf cluster whose tunnels are gone, leaves no app server to
		// dial, so callers answer it the way they answer unreachable app servers. The cause's message is kept.
		if !trace.IsConnectionProblem(err) {
			err = trace.ConnectionProblem(err, "")
		}
		return nil, nil, trace.Wrap(err)
	}

	servers, err := r.list(ctx, cluster)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return cluster, orderCandidates(servers, r.sessionID), nil
}

// forChecks returns the route's app servers for checks of the app itself, such as whether to redirect to it. Those
// checks run on every request, so they read a lookup cached for checkCacheTTL instead of listing the cluster's app
// servers each time. Dials must use resolve instead, because the cached set can be up to checkCacheTTL old and is
// ordered for whichever session loaded it.
func (r *appServerResolver) forChecks(ctx context.Context) ([]types.AppServer, error) {
	servers, err := utils.FnCacheGetWithTTL(ctx, r.cache, r.route, checkCacheTTL,
		func(ctx context.Context) ([]types.AppServer, error) {
			_, servers, err := r.resolve(ctx)
			return servers, trace.Wrap(err)
		})
	return servers, trace.Wrap(err)
}

// orderCandidates returns the order in which to dial app servers for a session, which is the session's rendezvous
// order. It depends only on the session and the candidates, so every connection and every Proxy dials the same app
// server first. The session stays on it while it answers, falls back in the same order while it does not, and returns
// once it recovers. The sessions of a failed app server spread evenly over the rest, and an app server that registers
// takes over only the sessions it now ranks first for.
func orderCandidates(servers []types.AppServer, sessionID string) []types.AppServer {
	ranks := make(map[string]uint64, len(servers))
	for _, server := range servers {
		ranks[server.GetHostID()] = rendezvousRank(sessionID, server.GetHostID())
	}

	ordered := slices.Clone(servers)
	slices.SortFunc(ordered, func(a, b types.AppServer) int {
		return cmp.Or(
			cmp.Compare(ranks[b.GetHostID()], ranks[a.GetHostID()]),
			cmp.Compare(a.GetHostID(), b.GetHostID()),
		)
	})
	return ordered
}

// rendezvousRank scores a host for a session. Higher scores are dialed first. The hash must be unseeded so that every
// Proxy computes the same order for a session, which a per-process seeded hash such as hash/maphash would not.
func rendezvousRank(sessionID, hostID string) uint64 {
	sum := sha256.Sum256([]byte(sessionID + "\x00" + hostID))
	return binary.BigEndian.Uint64(sum[:8])
}
