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
	"google.golang.org/protobuf/proto"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type spiffeFederationIndex string

const spiffeFederationNameIndex spiffeFederationIndex = "name"

func newSPIFFEFederationCollection(upstream services.SPIFFEFederations, w types.WatchKind) (*collection[*machineidv1.SPIFFEFederation, spiffeFederationIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter SPIFFEFederations")
	}

	return &collection[*machineidv1.SPIFFEFederation, spiffeFederationIndex]{
		store: newStore(
			proto.CloneOf[*machineidv1.SPIFFEFederation],
			map[spiffeFederationIndex]func(*machineidv1.SPIFFEFederation) string{
				spiffeFederationNameIndex: func(r *machineidv1.SPIFFEFederation) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*machineidv1.SPIFFEFederation, error) {
			var out []*machineidv1.SPIFFEFederation
			var nextToken string
			for {
				var page []*machineidv1.SPIFFEFederation
				var err error

				page, nextToken, err = upstream.ListSPIFFEFederations(ctx, 0 /* default page size */, nextToken)
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
		headerTransform: func(hdr *types.ResourceHeader) *machineidv1.SPIFFEFederation {
			return &machineidv1.SPIFFEFederation{
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

// ListSPIFFEFederations returns a paginated list of SPIFFE federations
func (c *Cache) ListSPIFFEFederations(ctx context.Context, pageSize int, nextToken string) ([]*machineidv1.SPIFFEFederation, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListSPIFFEFederations")
	defer span.End()

	lister := genericLister[*machineidv1.SPIFFEFederation, spiffeFederationIndex]{
		cache:        c,
		collection:   c.collections.spiffeFederations,
		index:        spiffeFederationNameIndex,
		upstreamList: c.Config.SPIFFEFederations.ListSPIFFEFederations,
		nextToken: func(t *machineidv1.SPIFFEFederation) string {
			return t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, pageSize, nextToken)
	return out, next, trace.Wrap(err)
}

// GetSPIFFEFederation returns a single SPIFFE federation by name
func (c *Cache) GetSPIFFEFederation(ctx context.Context, name string) (*machineidv1.SPIFFEFederation, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSPIFFEFederation")
	defer span.End()

	getter := genericGetter[*machineidv1.SPIFFEFederation, spiffeFederationIndex]{
		cache:       c,
		collection:  c.collections.spiffeFederations,
		index:       spiffeFederationNameIndex,
		upstreamGet: c.Config.SPIFFEFederations.GetSPIFFEFederation,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}
