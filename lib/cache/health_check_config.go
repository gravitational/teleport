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
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/cache/internal"
	"github.com/gravitational/teleport/lib/itertools/stream"
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
			types.KindHealthCheckConfig,
			proto.CloneOf[*healthcheckconfigv1.HealthCheckConfig],
			map[healthCheckConfigIndex]func(*healthcheckconfigv1.HealthCheckConfig) string{
				healthCheckConfigNameIndex: func(r *healthcheckconfigv1.HealthCheckConfig) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*healthcheckconfigv1.HealthCheckConfig, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListHealthCheckConfigs))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) *healthcheckconfigv1.HealthCheckConfig {
			return healthcheckconfigv1.HealthCheckConfig_builder{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: headerv1.Metadata_builder{
					Name: hdr.Metadata.Name,
				}.Build(),
			}.Build()
		},
		watch: w,
	}, nil
}

// healthCheckConfigCollection provides read access to the cached health
// check configs. Its exported methods are promoted onto every topology cache
// that embeds it; the reads are implemented exactly once here. It is a
// stateless value assembled inline by each of its consumers so that no
// shared scaffolding couples their lifetimes.
type healthCheckConfigCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.HealthCheckConfigReader
	col      *collection[*healthcheckconfigv1.HealthCheckConfig, healthCheckConfigIndex]
}

// ListHealthCheckConfigs lists health check configs with pagination.
func (c healthCheckConfigCollection) ListHealthCheckConfigs(ctx context.Context, pageSize int, nextToken string) ([]*healthcheckconfigv1.HealthCheckConfig, string, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListHealthCheckConfigs")
	defer span.End()

	lister := genericLister[*healthcheckconfigv1.HealthCheckConfig, healthCheckConfigIndex]{
		engine:          c.engine,
		collection:      c.col,
		index:           healthCheckConfigNameIndex,
		defaultPageSize: defaults.DefaultChunkSize,
		upstreamList:    c.upstream.ListHealthCheckConfigs,
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
func (c healthCheckConfigCollection) GetHealthCheckConfig(ctx context.Context, name string) (*healthcheckconfigv1.HealthCheckConfig, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetHealthCheckConfig")
	defer span.End()

	getter := genericGetter[*healthcheckconfigv1.HealthCheckConfig, healthCheckConfigIndex]{
		engine:      c.engine,
		collection:  c.col,
		index:       healthCheckConfigNameIndex,
		upstreamGet: c.upstream.GetHealthCheckConfig,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

// ListHealthCheckConfigs lists health check configs with pagination.
func (c *Cache) ListHealthCheckConfigs(ctx context.Context, pageSize int, nextToken string) ([]*healthcheckconfigv1.HealthCheckConfig, string, error) {
	return healthCheckConfigCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.HealthCheckConfig,
		col:      c.collections.healthCheckConfig,
	}.ListHealthCheckConfigs(ctx, pageSize, nextToken)
}

// GetHealthCheckConfig fetches a health check config by name.
func (c *Cache) GetHealthCheckConfig(ctx context.Context, name string) (*healthcheckconfigv1.HealthCheckConfig, error) {
	return healthCheckConfigCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.HealthCheckConfig,
		col:      c.collections.healthCheckConfig,
	}.GetHealthCheckConfig(ctx, name)
}
