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
	"google.golang.org/protobuf/proto"

	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type discoveryServiceIndex struct{}

var discoveryServiceNameIndex = discoveryServiceIndex{}

func newDiscoveryServiceCollection(upstream services.DiscoveryServices, w types.WatchKind) (*collection[*discoveryservicev1.DiscoveryService, discoveryServiceIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter DiscoveryServices")
	}

	return &collection[*discoveryservicev1.DiscoveryService, discoveryServiceIndex]{
		store: newStore(
			types.KindDiscoveryService,
			proto.CloneOf[*discoveryservicev1.DiscoveryService],
			map[discoveryServiceIndex]func(*discoveryservicev1.DiscoveryService) string{
				discoveryServiceNameIndex: func(r *discoveryservicev1.DiscoveryService) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*discoveryservicev1.DiscoveryService, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListDiscoveryServices))
			return out, trace.Wrap(err)
		},
		watch: w,
	}, nil
}

// GetDiscoveryService implements [authclient.Cache].
func (c *Cache) GetDiscoveryService(ctx context.Context, name string) (*discoveryservicev1.DiscoveryService, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetDiscoveryService")
	defer span.End()

	getter := genericGetter[*discoveryservicev1.DiscoveryService, discoveryServiceIndex]{
		cache:       c,
		collection:  c.collections.discoveryServices,
		index:       discoveryServiceNameIndex,
		upstreamGet: c.Config.DiscoveryServices.GetDiscoveryService,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

// ListDiscoveryServices implements [authclient.Cache].
func (c *Cache) ListDiscoveryServices(ctx context.Context, pageSize int, pageToken string) ([]*discoveryservicev1.DiscoveryService, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListDiscoveryServices")
	defer span.End()

	lister := genericLister[*discoveryservicev1.DiscoveryService, discoveryServiceIndex]{
		cache:        c,
		collection:   c.collections.discoveryServices,
		index:        discoveryServiceNameIndex,
		upstreamList: c.Config.DiscoveryServices.ListDiscoveryServices,
		nextToken: func(t *discoveryservicev1.DiscoveryService) string {
			return t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}
