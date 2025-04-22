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

	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/services"
)

// ListHealthCheckConfigs lists health check configs with pagination.
func (c *Cache) ListHealthCheckConfigs(ctx context.Context, pageSize int, nextToken string) ([]*healthcheckconfigv1.HealthCheckConfig, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListHealthCheckConfigs")
	defer span.End()

	rg, err := readLegacyCollectionCache(c, c.legacyCacheCollections.healthCheckConfig)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	out, nextKey, err := rg.reader.ListHealthCheckConfigs(ctx, pageSize, nextToken)
	return out, nextKey, trace.Wrap(err)
}

// GetHealthCheckConfig fetches a health check config by name.
func (c *Cache) GetHealthCheckConfig(ctx context.Context, name string) (*healthcheckconfigv1.HealthCheckConfig, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetHealthCheckConfig")
	defer span.End()

	rg, err := readLegacyCollectionCache(c, c.legacyCacheCollections.healthCheckConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	out, err := rg.reader.GetHealthCheckConfig(ctx, name)
	return out, trace.Wrap(err)
}

type healthCheckConfigExecutor struct{}

var _ executor[*healthcheckconfigv1.HealthCheckConfig, services.HealthCheckConfigReader] = healthCheckConfigExecutor{}

func (healthCheckConfigExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*healthcheckconfigv1.HealthCheckConfig, error) {
	var out []*healthcheckconfigv1.HealthCheckConfig
	clientutils.IterateResources(ctx,
		cache.Config.HealthCheckConfig.ListHealthCheckConfigs,
		func(hcc *healthcheckconfigv1.HealthCheckConfig) error {
			out = append(out, hcc)
			return nil
		},
	)
	return out, nil
}

func (healthCheckConfigExecutor) upsert(ctx context.Context, cache *Cache, resource *healthcheckconfigv1.HealthCheckConfig) error {
	_, err := cache.healthCheckConfigCache.UpsertHealthCheckConfig(ctx, resource)
	return trace.Wrap(err)
}

func (healthCheckConfigExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.healthCheckConfigCache.DeleteAllHealthCheckConfigs(ctx)
}

func (healthCheckConfigExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.healthCheckConfigCache.DeleteHealthCheckConfig(ctx, resource.GetName())
}

func (healthCheckConfigExecutor) isSingleton() bool { return false }

func (healthCheckConfigExecutor) getReader(cache *Cache, cacheOK bool) services.HealthCheckConfigReader {
	if cacheOK {
		return cache.healthCheckConfigCache
	}
	return cache.Config.HealthCheckConfig
}
