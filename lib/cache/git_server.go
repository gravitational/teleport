/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

	"github.com/gravitational/teleport/api/client/gitserver"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// GitServerReadOnlyClient returns the read-only client for Git servers.
//
// Note that Cache implements GitServerReadOnlyClient to satisfy
// auth.ProxyAccessPoint but also has the getter functions at top level to
// satisfy auth.Cache.
func (c *Cache) GitServerReadOnlyClient() gitserver.ReadOnlyClient {
	return c
}

func (c *Cache) GetGitServer(ctx context.Context, name string) (types.Server, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetGitServer")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.gitServers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetGitServer(ctx, name)
}

func (c *Cache) ListGitServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListGitServers")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.gitServers)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListGitServers(ctx, pageSize, pageToken)
}

type gitServerExecutor struct{}

func (gitServerExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) (all []types.Server, err error) {
	var page []types.Server
	var nextToken string
	for {
		page, nextToken, err = cache.GitServers.ListGitServers(ctx, apidefaults.DefaultChunkSize, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		all = append(all, page...)
		if nextToken == "" {
			break
		}
	}
	return all, nil
}

func (gitServerExecutor) upsert(ctx context.Context, cache *Cache, resource types.Server) error {
	_, err := cache.gitServersCache.UpsertGitServer(ctx, resource)
	return trace.Wrap(err)
}

func (gitServerExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.gitServersCache.DeleteAllGitServers(ctx)
}

func (gitServerExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.gitServersCache.DeleteGitServer(ctx, resource.GetName())
}

func (gitServerExecutor) isSingleton() bool { return false }

func (gitServerExecutor) getReader(cache *Cache, cacheOK bool) services.GitServerGetter {
	if cacheOK {
		return cache.gitServersCache
	}
	return cache.Config.GitServers
}

var _ executor[types.Server, services.GitServerGetter] = gitServerExecutor{}
