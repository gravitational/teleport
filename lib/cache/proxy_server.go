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
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/cache/internal"
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

// proxyServerCollection provides read access to cached proxy servers. Its
// exported methods are promoted onto every topology cache that embeds it;
// the reads are implemented exactly once here. It is a stateless value
// assembled inline by each of its consumers so that no shared scaffolding
// couples their lifetimes.
type proxyServerCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.Presence
	col      *collection[types.Server, proxyServerIndex]
}

// GetProxies is a part of auth.Cache implementation
//
// Deprecated: Prefer paginated gRPC variant [ListProxyServers].
//
// TODO(kiosion): DELETE IN 21.0.0
func (c proxyServerCollection) GetProxies() ([]types.Server, error) {
	_, span := c.tracer.Start(context.TODO(), "cache/GetProxies")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		//nolint:staticcheck // TODO(kiosion) DELETE IN 21.0.0
		servers, err := c.upstream.GetProxies()
		return servers, trace.Wrap(err)
	}

	servers := make([]types.Server, 0, rg.store.len())
	for s := range rg.store.resources(proxyServerNameIndex, "", "") {
		servers = append(servers, s.DeepCopy())
	}

	return servers, nil
}

// GetProxies is a part of auth.Cache implementation
//
// Deprecated: Prefer paginated gRPC variant [ListProxyServers].
//
// TODO(kiosion): DELETE IN 21.0.0
func (c *Cache) GetProxies() ([]types.Server, error) {
	return proxyServerCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Presence,
		col:      c.collections.proxyServers,
	}.GetProxies()
}

// ListProxyServers returns a paginated list of registered proxy servers.
func (c proxyServerCollection) ListProxyServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListProxyServers")
	defer span.End()

	lister := genericLister[types.Server, proxyServerIndex]{
		engine:       c.engine,
		collection:   c.col,
		index:        proxyServerNameIndex,
		upstreamList: c.upstream.ListProxyServers,
		nextToken:    types.Server.GetName,
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// ListProxyServers returns a paginated list of registered proxy servers.
func (c *Cache) ListProxyServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
	return proxyServerCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Presence,
		col:      c.collections.proxyServers,
	}.ListProxyServers(ctx, pageSize, pageToken)
}
