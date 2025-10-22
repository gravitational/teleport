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
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
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
			fn := func(ctx context.Context, pageSize int, token string) ([]types.Plugin, string, error) {
				return service.ListPlugins(ctx, pageSize, token, loadSecrets)
			}
			out, err := stream.Collect(clientutils.Resources(ctx, fn))
			return out, trace.Wrap(err)
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
// If the cache is not healthy, it falls back to fetching the plugin from the upstream.
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
		// Secrets will be included or excluded based on the withSecrets flag.
		item, err := c.Config.Plugin.GetPlugin(ctx, name, withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return item, nil
	}

	// Depending on the watcher configuration, the cached plugin may or may not contain secrets.
	// The returned plugin will be stripped of secrets if requested.
	plugin, err := rg.store.get(pluginNameIndex, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return stripAndClonePluginSecrets(plugin, withSecrets), nil
}

// GetPlugins returns a full list of plugins from the cache.
// If the cache is not healthy, it falls back to fetching the plugin from the upstream.
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
		// Cache is currently not available; fetch all plugins from the upstream.
		// The backend will honor the withSecrets flag.
		plugins, err := c.Config.Plugin.GetPlugins(ctx, withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return plugins, nil
	}

	// Strip secrets if requested.
	plugins := make([]types.Plugin, 0, rg.store.len())
	for item := range rg.store.resources(pluginNameIndex, "", "") {
		plugins = append(plugins, stripAndClonePluginSecrets(item, withSecrets))
	}
	return plugins, nil
}

// ListPlugins returns a paginated list of plugins from the cache or backend.
// If the cache is not healthy, it fetches directly from the backend.
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
		// The backend honors withSecrets when populating the results.
		items, nextKey, err := c.Config.Plugin.ListPlugins(ctx, limit, startKey, withSecrets)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return items, nextKey, nil
	}

	// Limit the number of items returned to the page size.
	if limit <= 0 || limit > apidefaults.DefaultChunkSize {
		limit = apidefaults.DefaultChunkSize
	}

	var plugins []types.Plugin
	var nextKey string
	for item := range rg.store.resources(pluginNameIndex, startKey, "") {
		if len(plugins) == limit {
			return plugins, item.GetName(), nil
		}
		plugins = append(plugins, stripAndClonePluginSecrets(item, withSecrets))
	}
	return plugins, nextKey, nil
}

// HasPluginType will return true if a plugin of the given type is registered.
func (c *Cache) HasPluginType(ctx context.Context, pluginType types.PluginType) (bool, error) {
	_, span := c.Tracer.Start(ctx, "cache/HasPluginType")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.plugins)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		// Cache is currently not available; check for the plugin type existence upstream.
		ok, err := c.Config.Plugin.HasPluginType(ctx, pluginType)
		return ok, trace.Wrap(err)
	}

	for plugin := range rg.store.resources(pluginNameIndex, "", "") {
		if plugin.GetType() == pluginType {
			return true, nil
		}
	}
	return false, nil
}

// stripPluginSecrets returns a cloned plugin, optionally removing secrets.
// This allows conditional filtering based on the `withSecrets` flag.
func stripAndClonePluginSecrets(in types.Plugin, withSecrets bool) types.Plugin {
	if withSecrets {
		return in.Clone()
	}
	return in.CloneWithoutSecrets()
}
