// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/services"
)

type proxyServerIndex string

const proxyServerNameIndex = "name"

func newProxyServerCollection(p services.Presence, w types.WatchKind) (*collection[types.Server, proxyServerIndex], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Presence")
	}

	return &collection[types.Server, proxyServerIndex]{
		store: newStore(
			types.KindProxy,
			types.Server.DeepCopy,
			map[proxyServerIndex]func(types.Server) string{
				proxyServerNameIndex: types.Server.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Server, error) {
			out, err := clientutils.CollectWithFallback(ctx, p.ListProxyServers, func(context.Context) ([]types.Server, error) {
				//nolint:staticcheck // TODO(kiosion) DELETE IN 21.0.0
				return p.GetProxies()
			})
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.Server {
			return &types.ServerV2{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.GetName(),
				},
			}
		},
		watch: w,
	}, nil
}

// GetProxies is a part of auth.Cache implementation
//
// Deprecated: Prefer paginated gRPC variant [ListProxyServers].
//
// TODO(kiosion): DELETE IN 21.0.0
func (c *Cache) GetProxies() ([]types.Server, error) {
	_, span := c.Tracer.Start(context.TODO(), "cache/GetProxies")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.proxyServers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		//nolint:staticcheck // TODO(kiosion) DELETE IN 21.0.0
		servers, err := c.Config.Presence.GetAuthServers()
		return servers, trace.Wrap(err)
	}

	servers := make([]types.Server, 0, rg.store.len())
	for s := range rg.store.resources(proxyServerNameIndex, "", "") {
		servers = append(servers, s.DeepCopy())
	}

	return servers, nil
}

// ListProxyServers returns a paginated list of registered proxy servers.
func (c *Cache) ListProxyServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListProxyServers")
	defer span.End()

	lister := genericLister[types.Server, proxyServerIndex]{
		cache:        c,
		collection:   c.collections.proxyServers,
		index:        proxyServerNameIndex,
		upstreamList: c.Config.Presence.ListProxyServers,
		nextToken:    types.Server.GetName,
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}
