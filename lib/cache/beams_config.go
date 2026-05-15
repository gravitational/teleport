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

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type beamsConfigIndex string

const beamsConfigNameIndex beamsConfigIndex = "name"

func newBeamsConfigCollection(upstream services.BeamsConfigGetter, w types.WatchKind) (*collection[*beamsv1.BeamsConfig, beamsConfigIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter BeamsConfig")
	}

	return &collection[*beamsv1.BeamsConfig, beamsConfigIndex]{
		store: newStore(
			types.KindBeamsConfig,
			proto.CloneOf[*beamsv1.BeamsConfig],
			map[beamsConfigIndex]func(*beamsv1.BeamsConfig) string{
				beamsConfigNameIndex: func(r *beamsv1.BeamsConfig) string {
					return r.GetMetadata().GetName()
				},
			},
		),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*beamsv1.BeamsConfig, error) {
			config, err := upstream.GetBeamsConfig(ctx)
			if err != nil {
				// BeamsConfig is feature-gated. Return empty results to avoid
				// blocking cache initialization.
				if trace.IsNotFound(err) || trace.IsNotImplemented(err) {
					return []*beamsv1.BeamsConfig{}, nil
				}
				return nil, trace.Wrap(err)
			}
			return []*beamsv1.BeamsConfig{config}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *beamsv1.BeamsConfig {
			return beamsv1.BeamsConfig_builder{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: &headerv1.Metadata{
					Name: hdr.Metadata.Name,
				},
			}.Build()
		},
		watch: w,
	}, nil
}

// GetBeamsConfig returns the singleton BeamsConfig resource.
func (c *Cache) GetBeamsConfig(ctx context.Context) (*beamsv1.BeamsConfig, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetBeamsConfig")
	defer span.End()

	getter := genericGetter[*beamsv1.BeamsConfig, beamsConfigIndex]{
		cache:      c,
		collection: c.collections.beamsConfig,
		index:      beamsConfigNameIndex,
		upstreamGet: func(ctx context.Context, name string) (*beamsv1.BeamsConfig, error) {
			return c.Config.BeamsConfig.GetBeamsConfig(ctx)
		},
	}
	out, err := getter.get(ctx, types.MetaNameBeamsConfig)
	return out, trace.Wrap(err)
}
