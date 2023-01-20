/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package local

import (
	"context"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

const (
	pluginsPrefix = "plugins"
)

// PluginsService manages plugin instances in the backend.
type PluginsService struct {
	backend backend.Backend
}

func NewPluginsService(backend backend.Backend) *PluginsService {
	return &PluginsService{backend: backend}
}

// CreatePlugin implements services.Plugins
func (s *PluginsService) CreatePlugin(ctx context.Context, req *proto.CreatePluginRequest) error {
	if req == nil {
		return trace.BadParameter("req is nil")
	}

	plugin := req.Plugin
	if plugin == nil {
		return trace.BadParameter("req.Plugin is nil")
	}

	// TODO: exchange initial credentials for live credentials

	if err := plugin.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalPlugin(plugin)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(pluginsPrefix, plugin.GetName()),
		Value:   value,
		Expires: plugin.Expiry(),
		ID:      plugin.GetResourceID(),
	}
	_, err = s.backend.Create(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DeletePlugin implements service.Plugins
func (s *PluginsService) DeletePlugin(ctx context.Context, name string) error {
	err := s.backend.Delete(ctx, backend.Key(pluginsPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("plugin %q doesn't exist", name)
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllPlugins implements service.Plugins
func (s *PluginsService) DeleteAllPlugins(ctx context.Context) error {
	startKey := backend.Key(pluginsPrefix)
	err := s.backend.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetPlugin implements services.Plugins
func (s *PluginsService) GetPlugin(ctx context.Context, name string, withSecrets bool) (types.Plugin, error) {
	item, err := s.backend.Get(ctx, backend.Key(pluginsPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("plugin %q doesn't exist", name)
		}
		return nil, trace.Wrap(err)
	}

	plugin, err := services.UnmarshalPlugin(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
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
	startKey := backend.Key(pluginsPrefix)
	result, err := s.backend.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plugins := make([]types.Plugin, len(result.Items))
	for _, item := range result.Items {
		plugin, err := services.UnmarshalPlugin(item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !withSecrets {
			plugin = plugin.WithoutSecrets().(types.Plugin)
		}
		plugins = append(plugins, plugin)
	}

	return plugins, nil
}

// SetPluginCredentials implements services.Plugins
func (s *PluginsService) SetPluginCredentials(ctx context.Context, name string, creds types.PluginCredentials) error {
	key := backend.Key(pluginsPrefix, name)
	item, err := s.backend.Get(ctx, key)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("plugin %q doesn't exist", name)
		}
		return trace.Wrap(err)
	}

	plugin, err := services.UnmarshalPlugin(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
	if err != nil {
		return trace.Wrap(err)
	}

	newPlugin := plugin.Clone()

	err = newPlugin.SetCredentials(creds)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := plugin.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalPlugin(newPlugin)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.backend.CompareAndSwap(ctx, *item, backend.Item{
		Key:     backend.Key(pluginsPrefix, plugin.GetName()),
		Value:   value,
		Expires: plugin.Expiry(),
		ID:      plugin.GetResourceID(),
	})

	return trace.Wrap(err)
}

// SetPluginStatus implements services.Plugins
func (s *PluginsService) SetPluginStatus(ctx context.Context, name string, status types.PluginStatus) error {
	key := backend.Key(pluginsPrefix, name)
	item, err := s.backend.Get(ctx, key)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("plugin %q doesn't exist", name)
		}
		return trace.Wrap(err)
	}

	plugin, err := services.UnmarshalPlugin(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
	if err != nil {
		return trace.Wrap(err)
	}

	newPlugin := plugin.Clone()

	err = newPlugin.SetStatus(status)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := plugin.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalPlugin(newPlugin)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.backend.CompareAndSwap(ctx, *item, backend.Item{
		Key:     backend.Key(pluginsPrefix, plugin.GetName()),
		Value:   value,
		Expires: plugin.Expiry(),
		ID:      plugin.GetResourceID(),
	})

	return trace.Wrap(err)
}
