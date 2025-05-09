// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

//nolint:unused // Because the executors generate a large amount of false positives.
package cache

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type reverseTunnelIndex string

const reverseTunnelNameIndex reverseTunnelIndex = "name"

func newReverseTunnelCollection(upstream services.Presence, w types.WatchKind) (*collection[types.ReverseTunnel, reverseTunnelIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Presence")
	}

	return &collection[types.ReverseTunnel, reverseTunnelIndex]{
		store: newStore(
			types.ReverseTunnel.Clone,
			map[reverseTunnelIndex]func(types.ReverseTunnel) string{
				reverseTunnelNameIndex: types.ReverseTunnel.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.ReverseTunnel, error) {
			var out []types.ReverseTunnel
			var nextToken string
			for {
				var page []types.ReverseTunnel
				var err error

				const defaultPageSize = 0
				page, nextToken, err = upstream.ListReverseTunnels(ctx, defaultPageSize, nextToken)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				out = append(out, page...)
				if nextToken == "" {
					break
				}
			}
			return out, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.ReverseTunnel {
			return &types.ReverseTunnelV2{
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

// ListReverseTunnels is a part of auth.Cache implementation
func (c *Cache) ListReverseTunnels(ctx context.Context, pageSize int, pageToken string) ([]types.ReverseTunnel, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListReverseTunnels")
	defer span.End()

	lister := genericLister[types.ReverseTunnel, reverseTunnelIndex]{
		cache:        c,
		collection:   c.collections.reverseTunnels,
		index:        reverseTunnelNameIndex,
		upstreamList: c.Config.Presence.ListReverseTunnels,
		nextToken: func(t types.ReverseTunnel) string {
			return t.GetMetadata().Name
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}
