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
	iterstream "github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type phaseIndex string

const phaseNameIndex phaseIndex = "name"

func phaseNameToken(phase *beamsv1.Phase) string {
	return phase.GetMetadata().GetName()
}

func newPhaseCollection(upstream services.PhaseReader, watch types.WatchKind) (*collection[*beamsv1.Phase, phaseIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Phases")
	}
	return &collection[*beamsv1.Phase, phaseIndex]{
		store: newStore(types.KindPhase, proto.CloneOf[*beamsv1.Phase], map[phaseIndex]func(*beamsv1.Phase) string{
			phaseNameIndex: func(phase *beamsv1.Phase) string {
				return phase.GetMetadata().GetName()
			},
		}),
		fetcher: func(ctx context.Context, _ bool) ([]*beamsv1.Phase, error) {
			return iterstream.Collect(upstream.RangePhases(ctx, ""))
		},
		headerTransform: func(header *types.ResourceHeader) *beamsv1.Phase {
			return beamsv1.Phase_builder{
				Kind:    header.Kind,
				Version: header.Version,
				Metadata: headerv1.Metadata_builder{
					Name: header.Metadata.Name,
				}.Build(),
			}.Build()
		},
		watch: watch,
	}, nil
}

// GetPhase returns a Phase by name.
func (c *Cache) GetPhase(ctx context.Context, name string) (*beamsv1.Phase, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetPhase")
	defer span.End()
	getter := genericGetter[*beamsv1.Phase, phaseIndex]{
		cache:       c,
		collection:  c.collections.phases,
		index:       phaseNameIndex,
		upstreamGet: c.Config.Phases.GetPhase,
	}
	phase, err := getter.get(ctx, name)
	return phase, trace.Wrap(err)
}

// ListPhases lists Phases with pagination and filtering.
func (c *Cache) ListPhases(ctx context.Context, pageSize int, pageToken string, opts *services.ListPhasesRequestOptions) ([]*beamsv1.Phase, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListPhases")
	defer span.End()
	lister := genericLister[*beamsv1.Phase, phaseIndex]{
		cache:      c,
		collection: c.collections.phases,
		index:      phaseNameIndex,
		upstreamList: func(ctx context.Context, limit int, startKey string) ([]*beamsv1.Phase, string, error) {
			return c.Config.Phases.ListPhases(ctx, limit, startKey, opts)
		},
		filter:    services.MakePhaseFilterFunc(opts),
		nextToken: phaseNameToken,
	}
	resources, next, err := lister.list(ctx, pageSize, pageToken)
	return resources, next, trace.Wrap(err)
}

// RangePhases returns Phases beginning at startToken.
func (c *Cache) RangePhases(ctx context.Context, startToken string) iter.Seq2[*beamsv1.Phase, error] {
	lister := genericLister[*beamsv1.Phase, phaseIndex]{
		cache:      c,
		collection: c.collections.phases,
		index:      phaseNameIndex,
		upstreamList: func(ctx context.Context, limit int, startKey string) ([]*beamsv1.Phase, string, error) {
			return c.Config.Phases.ListPhases(ctx, limit, startKey, nil)
		},
		nextToken: phaseNameToken,
	}
	return func(yield func(*beamsv1.Phase, error) bool) {
		ctx, span := c.Tracer.Start(ctx, "cache/RangePhases")
		defer span.End()
		for phase, err := range lister.Range(ctx, startToken, "") {
			if !yield(phase, err) {
				return
			}
			if err != nil {
				return
			}
		}
	}
}
