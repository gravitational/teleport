// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

type pluginIndex string

const pluginNameIndex pluginIndex = "name"

func newPluginsCollection(service services.Plugins, watch types.WatchKind) (*collection[types.Plugin, pluginIndex], error) {
	if service == nil {
		return nil, trace.BadParameter("missing parameter Plugin Service")
	}

	return &collection[types.Plugin, pluginIndex]{
		store: newStore(
			types.KindPlugin,
			types.Plugin.Clone,
			map[pluginIndex]func(types.Plugin) string{
				pluginNameIndex: types.Plugin.GetName,
			},
		),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Plugin, error) {
			var startKey string
			var allItems []types.Plugin
			for {
				items, nextKey, err := service.ListPlugins(ctx, apidefaults.DefaultChunkSize, startKey, loadSecrets)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				allItems = append(allItems, items...)
				if nextKey == "" {
					break
				}
				startKey = nextKey
			}
			return allItems, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.Plugin {
			return &types.PluginV1{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: watch,
	}, nil
}

// GetPlugin retrieves a plugin by name from the cache.
// If caching is disabled, it falls back to fetching the plugin from the backend API.
// The `withSecrets` flag controls whether secrets should be included in the result.
func (c *Cache) GetPlugin(ctx context.Context, name string, withSecrets bool) (types.Plugin, error) {
	_, span := c.Tracer.Start(ctx, "cache/GetPlugin")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.plugins)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		// Cache is disabled; fetch plugin directly from the backend.
		// Secrets will be included or excluded based on the withSecrets flag,
		// and filtering will be applied by the server.
		item, err := c.Config.Plugin.GetPlugin(ctx, name, withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return item, nil
	}

	// Cache is enabled; load plugins from cache store.
	// Depending on the watcher configuration, the cached plugin may or may not contain secrets.
	// The returned plugin will be stripped of secrets if requested.
	plugin, err := rg.store.get(pluginNameIndex, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return stripPluginSecrets(plugin, withSecrets), nil
}

// GetPlugins returns a full list of plugins from the cache.
// Falls back to backend API if caching is disabled.
// The `withSecrets` flag controls whether secrets are included in the response.
func (c *Cache) GetPlugins(ctx context.Context, withSecrets bool) ([]types.Plugin, error) {
	_, span := c.Tracer.Start(ctx, "cache/GetPlugins")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.plugins)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		// Cache is disabled; fetch all plugins from backend.
		// The backend will filter secrets based on withSecrets.
		plugins, err := c.Config.Plugin.GetPlugins(ctx, withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return plugins, nil
	}

	// Cache is enabled; load plugins from cache store.
	// Strip secrets if requested.
	plugins := make([]types.Plugin, 0, rg.store.len())
	for item := range rg.store.resources(pluginNameIndex, "", "") {
		plugins = append(plugins, stripPluginSecrets(item, withSecrets))
	}
	return plugins, nil
}

// ListPlugins returns a paginated list of plugins from the cache or backend.
// If caching is disabled, it uses the backend directly. Otherwise, it slices from in-memory cache.
// The `withSecrets` flag controls inclusion of secrets in the result.
func (c *Cache) ListPlugins(ctx context.Context, limit int, startKey string, withSecrets bool) ([]types.Plugin, string, error) {
	_, span := c.Tracer.Start(ctx, "cache/ListPlugins")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.plugins)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		// Cache is disabled; paginate from backend directly.
		// The backend applies secret filtering based on withSecrets.
		items, nextKey, err := c.Config.Plugin.ListPlugins(ctx, limit, startKey, withSecrets)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return items, nextKey, nil
	}

	// Cache is enabled; load plugins from cache store.
	// Limit the number of items returned to the page size.
	pageSize := limit
	if pageSize <= 0 || pageSize > apidefaults.DefaultChunkSize {
		pageSize = apidefaults.DefaultChunkSize
	}

	var plugins []types.Plugin
	for item := range rg.store.resources(pluginNameIndex, startKey, "") {
		if len(plugins) == pageSize {
			// Strip secrets from the full page if requested.
			if !withSecrets {
				for i := range plugins {
					plugins[i] = plugins[i].CloneWithoutSecrets()
				}
			}
			return plugins, backend.GetPaginationKey(item), nil
		}
		plugins = append(plugins, stripPluginSecrets(item, withSecrets).Clone())
	}

	return plugins, "", nil
}

// stripPluginSecrets returns a cloned plugin, optionally removing secrets.
// This allows conditional filtering based on the `withSecrets` flag.
func stripPluginSecrets(in types.Plugin, withSecrets bool) types.Plugin {
	if withSecrets {
		return in.Clone()
	}
	return in.CloneWithoutSecrets()
}
