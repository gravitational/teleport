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

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type crownJewelIndex string

const crownJewelNameIndex crownJewelIndex = "name"

func newCrownJewelCollection(upstream services.CrownJewels, w types.WatchKind) (*collection[*crownjewelv1.CrownJewel, crownJewelIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter CrownJewels")
	}

	return &collection[*crownjewelv1.CrownJewel, crownJewelIndex]{
		store: newStore(
			proto.CloneOf[*crownjewelv1.CrownJewel],
			map[crownJewelIndex]func(*crownjewelv1.CrownJewel) string{
				crownJewelNameIndex: func(r *crownjewelv1.CrownJewel) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*crownjewelv1.CrownJewel, error) {
			var out []*crownjewelv1.CrownJewel
			var nextToken string
			for {
				var page []*crownjewelv1.CrownJewel
				var err error

				const defaultPageSize = 0
				page, nextToken, err = upstream.ListCrownJewels(ctx, defaultPageSize, nextToken)
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
		headerTransform: func(hdr *types.ResourceHeader) *crownjewelv1.CrownJewel {
			return &crownjewelv1.CrownJewel{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: &headerv1.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

// ListCrownJewels returns a list of CrownJewel resources.
func (c *Cache) ListCrownJewels(ctx context.Context, pageSize int64, pageToken string) ([]*crownjewelv1.CrownJewel, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListCrownJewels")
	defer span.End()

	lister := genericLister[*crownjewelv1.CrownJewel, crownJewelIndex]{
		cache:      c,
		collection: c.collections.crownJewels,
		index:      crownJewelNameIndex,
		upstreamList: func(ctx context.Context, pageSize int, pageToken string) ([]*crownjewelv1.CrownJewel, string, error) {
			out, next, err := c.Config.CrownJewels.ListCrownJewels(ctx, int64(pageSize), pageToken)
			return out, next, trace.Wrap(err)
		},
		nextToken: func(t *crownjewelv1.CrownJewel) string {
			return t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, int(pageSize), pageToken)
	return out, next, trace.Wrap(err)
}

// GetCrownJewel returns the specified CrownJewel resource.
func (c *Cache) GetCrownJewel(ctx context.Context, name string) (*crownjewelv1.CrownJewel, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetCrownJewel")
	defer span.End()

	getter := genericGetter[*crownjewelv1.CrownJewel, crownJewelIndex]{
		cache:       c,
		collection:  c.collections.crownJewels,
		index:       crownJewelNameIndex,
		upstreamGet: c.Config.CrownJewels.GetCrownJewel,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}
