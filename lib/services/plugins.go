/*
Copyright 2023 Gravitational, Inc.

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

package services

import (
	"bytes"
	"context"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// Plugins is the plugin service
type Plugins interface {
	CreatePlugin(ctx context.Context, plugin types.Plugin) error
	DeleteAllPlugins(ctx context.Context) error
	DeletePlugin(ctx context.Context, name string) error
	GetPlugin(ctx context.Context, name string, withSecrets bool) (types.Plugin, error)
	GetPlugins(ctx context.Context, withSecrets bool) ([]types.Plugin, error)
	ListPlugins(ctx context.Context, limit int, startKey string, withSecrets bool) ([]types.Plugin, string, error)
	HasPluginType(ctx context.Context, pluginType types.PluginType) (bool, error)
	SetPluginCredentials(ctx context.Context, name string, creds types.PluginCredentials) error
	SetPluginStatus(ctx context.Context, name string, creds types.PluginStatus) error
}

// MarshalPlugin marshals Plugin resource to JSON.
func MarshalPlugin(plugin types.Plugin, opts ...MarshalOption) ([]byte, error) {
	if err := plugin.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch plugin := plugin.(type) {
	case *types.PluginV1:
		if !cfg.PreserveResourceID {
			copy := *plugin
			copy.SetResourceID(0)
			plugin = &copy
		}
		var buf bytes.Buffer
		err := (&jsonpb.Marshaler{}).Marshal(&buf, plugin)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return buf.Bytes(), nil
	default:
		return nil, trace.BadParameter("unsupported plugin resource %T", plugin)
	}
}

// UnmarshalPlugin unmarshals the plugin resource from JSON.
func UnmarshalPlugin(data []byte, opts ...MarshalOption) (types.Plugin, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing plugin resource data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V1:
		var plugin types.PluginV1
		if err := jsonpb.Unmarshal(bytes.NewReader(data), &plugin); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := plugin.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			plugin.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			plugin.SetExpiry(cfg.Expires)
		}
		return &plugin, nil
	}
	return nil, trace.BadParameter("unsupported plugin resource version %q", h.Version)
}
