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
	"iter"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type beamIndex string

const (
	beamNameIndex  beamIndex = "name"
	beamAliasIndex beamIndex = "alias"
)

func newBeamCollection(upstream services.BeamReader, w types.WatchKind) (*collection[*beamsv1.Beam, beamIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Beams")
	}

	return &collection[*beamsv1.Beam, beamIndex]{
		store: newStore(
			types.KindBeam,
			proto.CloneOf[*beamsv1.Beam],
			map[beamIndex]func(*beamsv1.Beam) string{
				beamNameIndex:  keyForBeamNameIndex,
				beamAliasIndex: keyForBeamAliasIndex,
			},
		),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*beamsv1.Beam, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListBeams))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) *beamsv1.Beam {
			return &beamsv1.Beam{
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

// GetBeam returns the specified beam resource.
func (c *Cache) GetBeam(ctx context.Context, name string) (*beamsv1.Beam, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetBeam")
	defer span.End()

	getter := genericGetter[*beamsv1.Beam, beamIndex]{
		cache:       c,
		collection:  c.collections.beams,
		index:       beamNameIndex,
		upstreamGet: c.Config.Beams.GetBeam,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

// GetBeamByAlias returns the specified beam resource by alias.
func (c *Cache) GetBeamByAlias(ctx context.Context, alias string) (*beamsv1.Beam, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetBeamByAlias")
	defer span.End()

	getter := genericGetter[*beamsv1.Beam, beamIndex]{
		cache:       c,
		collection:  c.collections.beams,
		index:       beamAliasIndex,
		upstreamGet: c.Config.Beams.GetBeamByAlias,
	}
	out, err := getter.get(ctx, alias)
	return out, trace.Wrap(err)
}

// ListBeams returns a page of beam resources ordered by metadata.name.
func (c *Cache) ListBeams(ctx context.Context, limit int, startKey string) ([]*beamsv1.Beam, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListBeams")
	defer span.End()

	lister := genericLister[*beamsv1.Beam, beamIndex]{
		cache:        c,
		collection:   c.collections.beams,
		index:        beamNameIndex,
		upstreamList: c.Config.Beams.ListBeams,
		nextToken:    keyForBeamNameIndex,
	}
	out, next, err := lister.list(ctx, limit, startKey)
	return out, next, trace.Wrap(err)
}

// IterateBeams returns a sequence of beams starting from the given pageToken.
func (c *Cache) IterateBeams(ctx context.Context, pageToken string) iter.Seq2[*beamsv1.Beam, error] {
	lister := genericLister[*beamsv1.Beam, beamIndex]{
		cache:        c,
		collection:   c.collections.beams,
		index:        beamNameIndex,
		upstreamList: c.Config.Beams.ListBeams,
		nextToken:    keyForBeamNameIndex,
	}
	return lister.Range(ctx, pageToken, "")
}

func keyForBeamNameIndex(beam *beamsv1.Beam) string {
	return beam.GetMetadata().GetName()
}

func keyForBeamAliasIndex(beam *beamsv1.Beam) string {
	return beam.GetStatus().GetAlias()
}
