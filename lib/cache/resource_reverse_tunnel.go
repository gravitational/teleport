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
)

// GetReverseTunnels is a part of auth.Cache implementation
// Deprecated: use ListReverseTunnels
func (c *Cache) GetReverseTunnels(ctx context.Context) ([]types.ReverseTunnel, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetReverseTunnels")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.reverseTunnels)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetReverseTunnels(ctx)
}

// ListReverseTunnels is a part of auth.Cache implementation
func (c *Cache) ListReverseTunnels(ctx context.Context, pageSize int, pageToken string) ([]types.ReverseTunnel, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListReverseTunnels")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.reverseTunnels)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListReverseTunnels(ctx, pageSize, pageToken)
}

type reverseTunnelGetter interface {
	GetReverseTunnels(ctx context.Context) ([]types.ReverseTunnel, error)
	ListReverseTunnels(ctx context.Context, pageSize int, pageToken string) ([]types.ReverseTunnel, string, error)
}

var _ executor[types.ReverseTunnel, reverseTunnelGetter] = reverseTunnelExecutor{}

type reverseTunnelExecutor struct{}

func (reverseTunnelExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.ReverseTunnel, error) {
	return cache.Presence.GetReverseTunnels(ctx)
}

func (reverseTunnelExecutor) upsert(ctx context.Context, cache *Cache, resource types.ReverseTunnel) error {
	return cache.presenceCache.UpsertReverseTunnel(ctx, resource)
}

func (reverseTunnelExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.presenceCache.DeleteAllReverseTunnels(ctx)
}

func (reverseTunnelExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.presenceCache.DeleteReverseTunnel(ctx, resource.GetName())
}

func (reverseTunnelExecutor) isSingleton() bool { return false }

func (reverseTunnelExecutor) getReader(cache *Cache, cacheOK bool) reverseTunnelGetter {
	if cacheOK {
		return cache.presenceCache
	}
	return cache.Config.Presence
}
