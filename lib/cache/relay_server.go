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
	"google.golang.org/protobuf/proto"

	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type relayServerIndex struct{}

var relayServerNameIndex = relayServerIndex{}

func newRelayServerCollection(upstream services.Presence, w types.WatchKind) (*collection[*presencev1.RelayServer, relayServerIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Presence")
	}

	return &collection[*presencev1.RelayServer, relayServerIndex]{
		store: newStore(
			proto.CloneOf[*presencev1.RelayServer],
			map[relayServerIndex]func(*presencev1.RelayServer) string{
				relayServerNameIndex: func(r *presencev1.RelayServer) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*presencev1.RelayServer, error) {
			return fetchAll(ctx, upstream.ListRelayServers)
		},
		watch: w,
	}, nil
}

// GetRelayServer implements [authclient.Cache].
func (c *Cache) GetRelayServer(ctx context.Context, name string) (*presencev1.RelayServer, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetRelayServer")
	defer span.End()

	getter := genericGetter[*presencev1.RelayServer, relayServerIndex]{
		cache:       c,
		collection:  c.collections.relayServers,
		index:       relayServerNameIndex,
		upstreamGet: c.Config.Presence.GetRelayServer,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

// ListRelayServers implements [authclient.Cache].
func (c *Cache) ListRelayServers(ctx context.Context, pageSize int, pageToken string) ([]*presencev1.RelayServer, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListRelayServers")
	defer span.End()

	lister := genericLister[*presencev1.RelayServer, relayServerIndex]{
		cache:        c,
		collection:   c.collections.relayServers,
		index:        relayServerNameIndex,
		upstreamList: c.Config.Presence.ListRelayServers,
		nextToken: func(t *presencev1.RelayServer) string {
			return t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}
