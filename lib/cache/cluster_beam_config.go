/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"google.golang.org/protobuf/proto"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type clusterBeamConfigIndex string

const clusterBeamConfigNameIndex clusterBeamConfigIndex = "name"

func newClusterBeamConfigCollection(upstream services.ClusterBeamConfigReader, w types.WatchKind) (*collection[*beamsv1.ClusterBeamConfig, clusterBeamConfigIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter ClusterBeamConfigReader")
	}

	return &collection[*beamsv1.ClusterBeamConfig, clusterBeamConfigIndex]{
		store: newStore(
			types.KindClusterBeamConfig,
			proto.CloneOf[*beamsv1.ClusterBeamConfig],
			map[clusterBeamConfigIndex]func(*beamsv1.ClusterBeamConfig) string{
				clusterBeamConfigNameIndex: func(r *beamsv1.ClusterBeamConfig) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*beamsv1.ClusterBeamConfig, error) {
			cfg, err := upstream.GetClusterBeamConfig(ctx)
			if err != nil {
				if trace.IsNotFound(err) {
					return nil, nil
				}
				return nil, trace.Wrap(err)
			}
			return []*beamsv1.ClusterBeamConfig{cfg}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *beamsv1.ClusterBeamConfig {
			return &beamsv1.ClusterBeamConfig{
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

// GetClusterBeamConfig returns the cluster-wide beam configuration.
func (c *Cache) GetClusterBeamConfig(ctx context.Context) (*beamsv1.ClusterBeamConfig, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetClusterBeamConfig")
	defer span.End()

	getter := genericGetter[*beamsv1.ClusterBeamConfig, clusterBeamConfigIndex]{
		cache:      c,
		collection: c.collections.clusterBeamConfig,
		index:      clusterBeamConfigNameIndex,
		upstreamGet: func(ctx context.Context, _ string) (*beamsv1.ClusterBeamConfig, error) {
			return c.Config.ClusterBeamConfig.GetClusterBeamConfig(ctx)
		},
	}
	out, err := getter.get(ctx, types.MetaNameClusterBeamConfig)
	return out, trace.Wrap(err)
}
