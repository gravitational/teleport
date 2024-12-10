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

package cache

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

func (c *Cache) GetPluginStaticCredentials(ctx context.Context, name string) (types.PluginStaticCredentials, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetPluginStaticCredentials")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.pluginStaticCredentials)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetPluginStaticCredentials(ctx, name)
}

func (c *Cache) GetPluginStaticCredentialsByLabels(ctx context.Context, labels map[string]string) ([]types.PluginStaticCredentials, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetPluginStaticCredentialsByLabels")
	defer span.End()

	rg, err := readCollectionCache(c, c.collections.pluginStaticCredentials)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetPluginStaticCredentialsByLabels(ctx, labels)
}

type pluginStaticCredentialsGetter interface {
	GetPluginStaticCredentials(ctx context.Context, name string) (types.PluginStaticCredentials, error)
	GetPluginStaticCredentialsByLabels(ctx context.Context, labels map[string]string) ([]types.PluginStaticCredentials, error)
}

var _ executor[types.PluginStaticCredentials, pluginStaticCredentialsGetter] = pluginStaticCredentialsExecutor{}

type pluginStaticCredentialsExecutor struct{}

func (pluginStaticCredentialsExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.PluginStaticCredentials, error) {
	return cache.PluginStaticCredentials.GetAllPluginStaticCredentials(ctx)
}

func (pluginStaticCredentialsExecutor) upsert(ctx context.Context, cache *Cache, resource types.PluginStaticCredentials) error {
	_, err := cache.pluginStaticCredentialsCache.UpsertPluginStaticCredentials(ctx, resource)
	return trace.Wrap(err)
}

func (pluginStaticCredentialsExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.pluginStaticCredentialsCache.DeleteAllPluginStaticCredentials(ctx)
}

func (pluginStaticCredentialsExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.pluginStaticCredentialsCache.DeletePluginStaticCredentials(ctx, resource.GetName())
}

func (pluginStaticCredentialsExecutor) isSingleton() bool { return false }

func (pluginStaticCredentialsExecutor) getReader(cache *Cache, cacheOK bool) pluginStaticCredentialsGetter {
	if cacheOK {
		return cache.pluginStaticCredentialsCache
	}
	return cache.Config.PluginStaticCredentials
}
