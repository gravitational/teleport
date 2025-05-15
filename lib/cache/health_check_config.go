/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

	"github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/services"
)

type healthCheckConfigIndex string

const healthCheckConfigNameIndex healthCheckConfigIndex = "name"

func newHealthCheckConfigCollection(upstream services.HealthCheckConfigReader, w types.WatchKind) (*collection[*healthcheckconfigv1.HealthCheckConfig, healthCheckConfigIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter HealthCheckConfigReader")
	}

	return &collection[*healthcheckconfigv1.HealthCheckConfig, healthCheckConfigIndex]{
		store: newStore(
			proto.CloneOf[*healthcheckconfigv1.HealthCheckConfig],
			map[healthCheckConfigIndex]func(*healthcheckconfigv1.HealthCheckConfig) string{
				healthCheckConfigNameIndex: func(r *healthcheckconfigv1.HealthCheckConfig) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*healthcheckconfigv1.HealthCheckConfig, error) {
			var out []*healthcheckconfigv1.HealthCheckConfig
			clientutils.IterateResources(ctx,
				upstream.ListHealthCheckConfigs,
				func(hcc *healthcheckconfigv1.HealthCheckConfig) error {
					out = append(out, hcc)
					return nil
				},
			)
			return out, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *healthcheckconfigv1.HealthCheckConfig {
			return &healthcheckconfigv1.HealthCheckConfig{
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

// ListHealthCheckConfigs lists health check configs with pagination.
func (c *Cache) ListHealthCheckConfigs(ctx context.Context, pageSize int, nextToken string) ([]*healthcheckconfigv1.HealthCheckConfig, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListHealthCheckConfigs")
	defer span.End()

	lister := genericLister[*healthcheckconfigv1.HealthCheckConfig, healthCheckConfigIndex]{
		cache:           c,
		collection:      c.collections.healthCheckConfig,
		index:           healthCheckConfigNameIndex,
		defaultPageSize: defaults.DefaultChunkSize,
		upstreamList:    c.Config.HealthCheckConfig.ListHealthCheckConfigs,
		nextToken: func(t *healthcheckconfigv1.HealthCheckConfig) string {
			return t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx,
		pageSize,
		nextToken,
	)
	return out, next, trace.Wrap(err)
}

// GetHealthCheckConfig fetches a health check config by name.
func (c *Cache) GetHealthCheckConfig(ctx context.Context, name string) (*healthcheckconfigv1.HealthCheckConfig, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetHealthCheckConfig")
	defer span.End()

	getter := genericGetter[*healthcheckconfigv1.HealthCheckConfig, healthCheckConfigIndex]{
		cache:       c,
		collection:  c.collections.healthCheckConfig,
		index:       healthCheckConfigNameIndex,
		upstreamGet: c.Config.HealthCheckConfig.GetHealthCheckConfig,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}
