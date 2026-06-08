/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package local

import (
	"context"
	"iter"
	"log/slog"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/itertools/stream"
	libplugin "github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const pluginsPrefix = "plugins"

// PluginsService manages plugin instances in the backend.
type PluginsService struct {
	backend backend.Backend
}

// NewPluginsService constructs a new PluginsService
func NewPluginsService(backend backend.Backend) *PluginsService {
	return &PluginsService{backend: backend}
}

// CreatePlugin implements services.Plugins
func (s *PluginsService) CreatePlugin(ctx context.Context, plugin types.Plugin) error {
	if err := libplugin.Validate(plugin); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalPlugin(plugin)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.NewKey(pluginsPrefix, plugin.GetName()),
		Value:   value,
		Expires: plugin.Expiry(),
	}
	_, err = s.backend.Create(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeletePlugin implements service.Plugins
func (s *PluginsService) DeletePlugin(ctx context.Context, name string) error {
	err := s.backend.Delete(ctx, backend.NewKey(pluginsPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("plugin %q doesn't exist", name)
		}
		return trace.Wrap(err)
	}
	return nil
}

// UpdatePlugin updates a plugin resource.
func (s *PluginsService) UpdatePlugin(ctx context.Context, plugin types.Plugin) (types.Plugin, error) {
	if err := libplugin.Validate(plugin); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := services.CheckAndSetDefaults(plugin); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalPlugin(plugin)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(pluginsPrefix, plugin.GetName()),
		Value:    value,
		Expires:  plugin.Expiry(),
		Revision: plugin.GetRevision(),
	}
	lease, err := s.backend.ConditionalUpdate(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := types.SetRevision(plugin, lease.Revision); err != nil {
		return nil, trace.Wrap(err)
	}
	return plugin, nil
}

// DeleteAllPlugins implements service.Plugins
func (s *PluginsService) DeleteAllPlugins(ctx context.Context) error {
	startKey := backend.ExactKey(pluginsPrefix)
	err := s.backend.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetPlugin implements services.Plugins
func (s *PluginsService) GetPlugin(ctx context.Context, name string, withSecrets bool) (types.Plugin, error) {
	item, err := s.backend.Get(ctx, backend.NewKey(pluginsPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("plugin %q doesn't exist", name)
		}
		return nil, trace.Wrap(err)
	}

	plugin, err := services.UnmarshalPlugin(item.Value,
		services.WithExpires(item.Expires), services.WithRevision(item.Revision))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !withSecrets {
		plugin = plugin.WithoutSecrets().(types.Plugin)
	}
	return plugin, nil
}

// GetPlugins implements services.Plugins
func (s *PluginsService) GetPlugins(ctx context.Context, withSecrets bool) ([]types.Plugin, error) {
	const pageSize = apidefaults.DefaultChunkSize
	var results []types.Plugin

	var page []types.Plugin
	var startKey string
	var err error
	for {
		page, startKey, err = s.ListPlugins(ctx, pageSize, startKey, withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		results = append(results, page...)
		if startKey == "" {
			break
		}
	}

	return results, nil
}

// ListPlugins returns a paginated list of plugin instances.
// StartKey is a resource name, which is the suffix of its key.
func (s *PluginsService) ListPlugins(ctx context.Context, limit int, startKey string, withSecrets bool) ([]types.Plugin, string, error) {
	return generic.CollectPageAndCursor(s.rangePlugins(ctx, startKey, "", withSecrets), limit, func(p types.Plugin) string {
		return backend.GetPaginationKey(p)
	})
}

// rangePlugins returns plugin resources within the range [start, end).
func (s *PluginsService) rangePlugins(ctx context.Context, start, end string, withSecrets bool) iter.Seq2[types.Plugin, error] {
	mapFn := func(item backend.Item) (types.Plugin, bool) {
		plugin, err := services.UnmarshalPlugin(item.Value,
			services.WithExpires(item.Expires),
			services.WithRevision(item.Revision))

		if err != nil {
			slog.WarnContext(ctx, "Failed to unmarshal plugin",
				"key", logutils.StringerAttr(item.Key),
				"error", err,
			)
			return nil, false
		}

		if !withSecrets {
			plugin = plugin.WithoutSecrets().(types.Plugin)
		}

		return plugin, true
	}

	pluginKey := backend.NewKey(pluginsPrefix)
	startKey := pluginKey.AppendKey(backend.KeyFromString(start))
	endKey := backend.RangeEnd(pluginKey)
	if end != "" {
		endKey = pluginKey.AppendKey(backend.KeyFromString(end)).ExactKey()
	}

	return stream.TakeWhile(
		stream.FilterMap(
			s.backend.Items(ctx, backend.ItemsParams{
				StartKey: startKey,
				EndKey:   endKey,
			}),
			mapFn, // mapping function
		),
		func(plugin types.Plugin) bool {
			// The range is not inclusive of the end key, so return early
			// if the end has been reached.
			return end == "" || plugin.GetName() < end
		})
}

// HasPluginType will return true if a plugin of the given type is registered.
func (s *PluginsService) HasPluginType(ctx context.Context, pluginType types.PluginType) (bool, error) {
	plugins, err := s.GetPlugins(ctx, false)
	if err != nil {
		return false, trace.Wrap(err)
	}

	for _, plugin := range plugins {
		if plugin.GetType() == pluginType {
			return true, nil
		}
	}

	return false, nil
}

// SetPluginCredentials implements services.Plugins
func (s *PluginsService) SetPluginCredentials(ctx context.Context, name string, creds types.PluginCredentials) error {
	return s.updateAndSwap(ctx, name, func(p types.Plugin) error {
		return trace.Wrap(p.SetCredentials(creds))
	})
}

// SetPluginStatus implements services.Plugins
func (s *PluginsService) SetPluginStatus(ctx context.Context, name string, status types.PluginStatus) error {
	return s.updateAndSwap(ctx, name, func(p types.Plugin) error {
		return trace.Wrap(p.SetStatus(status))
	})
}

func (s *PluginsService) updateAndSwap(ctx context.Context, name string, modify func(types.Plugin) error) error {
	key := backend.NewKey(pluginsPrefix, name)
	item, err := s.backend.Get(ctx, key)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("plugin %q doesn't exist", name)
		}
		return trace.Wrap(err)
	}

	plugin, err := services.UnmarshalPlugin(item.Value,
		services.WithExpires(item.Expires), services.WithRevision(item.Revision))
	if err != nil {
		return trace.Wrap(err)
	}

	newPlugin := plugin.Clone()

	err = modify(newPlugin)
	if err != nil {
		return trace.Wrap(err)
	}

	rev := newPlugin.GetRevision()
	value, err := services.MarshalPlugin(newPlugin)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.backend.ConditionalUpdate(ctx, backend.Item{
		Key:      backend.NewKey(pluginsPrefix, plugin.GetName()),
		Value:    value,
		Expires:  plugin.Expiry(),
		Revision: rev,
	})

	return trace.Wrap(err)
}

var _ services.Plugins = (*PluginsService)(nil)
