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

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const pluginsPrefix = "plugins"

// PluginsService manages plugin instances in the backend.
type PluginsService struct {
	svc generic.Service[types.Plugin]
}

// NewPluginsService constructs a new PluginsService
func NewPluginsService(backend backend.Backend) (*PluginsService, error) {
	svc, err := generic.NewService(&generic.ServiceConfig[types.Plugin]{
		Backend:       backend,
		PageLimit:     apidefaults.DefaultChunkSize,
		ResourceKind:  types.KindPlugin,
		BackendPrefix: pluginsPrefix,
		MarshalFunc:   services.MarshalPlugin,
		UnmarshalFunc: services.UnmarshalPlugin,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &PluginsService{
		svc: *svc,
	}, nil
}

// CreatePlugin implements services.Plugins
func (s *PluginsService) CreatePlugin(ctx context.Context, plugin types.Plugin) error {
	return trace.Wrap(s.svc.CreateResource(ctx, plugin))
}

// DeletePlugin implements service.Plugins
func (s *PluginsService) DeletePlugin(ctx context.Context, name string) error {
	return trace.Wrap(s.svc.DeleteResource(ctx, name))
}

// DeleteAllPlugins implements service.Plugins
func (s *PluginsService) DeleteAllPlugins(ctx context.Context) error {
	return trace.Wrap(s.svc.DeleteAllResources(ctx))
}

// GetPlugin implements services.Plugins
func (s *PluginsService) GetPlugin(ctx context.Context, name string, withSecrets bool) (types.Plugin, error) {
	plugin, err := s.svc.GetResource(ctx, name)
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
	plugins, err := s.svc.GetResources(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !withSecrets {
		plugins = pluginsWithoutSecrets(plugins)
	}

	return plugins, nil
}

// ListPlugins returns a paginated list of plugin instances.
// StartKey is a resource name, which is the suffix of its key.
func (s *PluginsService) ListPlugins(ctx context.Context, limit int, startKey string, withSecrets bool) ([]types.Plugin, string, error) {
	plugins, nextKey, err := s.svc.ListResources(ctx, limit, startKey)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	if !withSecrets {
		plugins = pluginsWithoutSecrets(plugins)
	}

	return plugins, nextKey, nil
}

func pluginsWithoutSecrets(plugins []types.Plugin) []types.Plugin {
	pluginsNoSecrets := make([]types.Plugin, len(plugins))
	for i, plugin := range plugins {
		pluginsNoSecrets[i] = plugin.WithoutSecrets().(types.Plugin)
	}

	return pluginsNoSecrets
}

// SetPluginCredentials implements services.Plugins
func (s *PluginsService) SetPluginCredentials(ctx context.Context, name string, creds types.PluginCredentials) error {
	return s.svc.UpdateAndSwapResource(ctx, name, func(p types.Plugin) error {
		return trace.Wrap(p.SetCredentials(creds))
	})
}

// SetPluginStatus implements services.Plugins
func (s *PluginsService) SetPluginStatus(ctx context.Context, name string, status types.PluginStatus) error {
	return s.svc.UpdateAndSwapResource(ctx, name, func(p types.Plugin) error {
		return trace.Wrap(p.SetStatus(status))
	})
}

var _ services.Plugins = (*PluginsService)(nil)
