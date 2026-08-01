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

type authServerIndex string

const authServerNameIndex authServerIndex = "name"

func newAuthServerCollection(p services.Presence, w types.WatchKind) (*collection[types.Server, authServerIndex], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Presence")
	}

	return &collection[types.Server, authServerIndex]{
		store: newStore(
			types.KindAuthServer,
			types.Server.DeepCopy,
			map[authServerIndex]func(types.Server) string{
				authServerNameIndex: types.Server.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Server, error) {
			out, err := clientutils.CollectWithFallback(ctx, p.ListAuthServers, func(context.Context) ([]types.Server, error) {
				//nolint:staticcheck // TODO(kiosion) DELETE IN 21.0.0
				return p.GetAuthServers()
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

// authServerCollection provides read access to cached auth servers. Its
// exported methods are promoted onto every topology cache that embeds it;
// the reads are implemented exactly once here. It is a stateless value
// assembled inline by each of its consumers so that no shared scaffolding
// couples their lifetimes.
type authServerCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.Presence
	col      *collection[types.Server, authServerIndex]
}

// GetAuthServers returns a list of registered servers
//
// Deprecated: Prefer paginated gRPC variant [ListAuthServers].
//
// TODO(kiosion): DELETE IN 21.0.0
func (c authServerCollection) GetAuthServers() ([]types.Server, error) {
	_, span := c.tracer.Start(context.TODO(), "cache/GetAuthServers")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		//nolint:staticcheck // TODO(kiosion) DELETE IN 21.0.0
		servers, err := c.upstream.GetAuthServers()
		return servers, trace.Wrap(err)
	}

	servers := make([]types.Server, 0, rg.store.len())
	for s := range rg.store.resources(authServerNameIndex, "", "") {
		servers = append(servers, s.DeepCopy())
	}

	return servers, nil
}

// GetAuthServers returns a list of registered servers
//
// Deprecated: Prefer paginated gRPC variant [ListAuthServers].
//
// TODO(kiosion): DELETE IN 21.0.0
func (c *Cache) GetAuthServers() ([]types.Server, error) {
	return authServerCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Presence,
		col:      c.collections.authServers,
	}.GetAuthServers()
}

// ListAuthServers returns a paginated list of registered auth servers.
func (c authServerCollection) ListAuthServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListAuthServers")
	defer span.End()

	lister := genericLister[types.Server, authServerIndex]{
		engine:       c.engine,
		collection:   c.col,
		index:        authServerNameIndex,
		upstreamList: c.upstream.ListAuthServers,
		nextToken:    types.Server.GetName,
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// ListAuthServers returns a paginated list of registered auth servers.
func (c *Cache) ListAuthServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
	return authServerCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Presence,
		col:      c.collections.authServers,
	}.ListAuthServers(ctx, pageSize, pageToken)
}
