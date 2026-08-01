/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

	"github.com/gravitational/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport/api/client/gitserver"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/cache/internal"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type gitServerIndex string

const gitServerNameIndex gitServerIndex = "name"

func newGitServerCollection(upstream services.GitServerGetter, w types.WatchKind) (*collection[types.Server, gitServerIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter GitServerGetter")
	}

	return &collection[types.Server, gitServerIndex]{
		store: newStore(
			types.KindGitServer,
			types.Server.DeepCopy,
			map[gitServerIndex]func(types.Server) string{
				gitServerNameIndex: types.Server.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Server, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListGitServers))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.Server {
			return &types.ServerV2{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

// GitServerReadOnlyClient returns the read-only client for Git servers.
//
// Note that Cache implements GitServerReadOnlyClient to satisfy
// auth.ProxyAccessPoint but also has the getter functions at top level to
// satisfy auth.Cache.
func (c *Cache) GitServerReadOnlyClient() gitserver.ReadOnlyClient {
	return c
}

// gitServerCollection provides read access to the cached Git servers. Its
// exported methods are promoted onto every topology cache that embeds it;
// the reads are implemented exactly once here. It is a stateless value
// assembled inline by each of its consumers so that no shared scaffolding
// couples their lifetimes.
type gitServerCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.GitServerGetter
	col      *collection[types.Server, gitServerIndex]
}

func (c gitServerCollection) GetGitServer(ctx context.Context, name string) (types.Server, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetGitServer")
	defer span.End()

	getter := genericGetter[types.Server, gitServerIndex]{
		engine:      c.engine,
		collection:  c.col,
		index:       gitServerNameIndex,
		upstreamGet: c.upstream.GetGitServer,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

func (c gitServerCollection) ListGitServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListGitServers")
	defer span.End()

	lister := genericLister[types.Server, gitServerIndex]{
		engine:       c.engine,
		collection:   c.col,
		index:        gitServerNameIndex,
		upstreamList: c.upstream.ListGitServers,
		nextToken: func(t types.Server) string {
			return t.GetMetadata().Name
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

func (c *Cache) GetGitServer(ctx context.Context, name string) (types.Server, error) {
	return gitServerCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.GitServers,
		col:      c.collections.gitServers,
	}.GetGitServer(ctx, name)
}

func (c *Cache) ListGitServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
	return gitServerCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.GitServers,
		col:      c.collections.gitServers,
	}.ListGitServers(ctx, pageSize, pageToken)
}
