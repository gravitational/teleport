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
	UpdatePlugin(ctx context.Context, plugin types.Plugin) (types.Plugin, error)
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
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch plugin := plugin.(type) {
	case *types.PluginV1:
		if err := plugin.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		var buf bytes.Buffer
		err := (&jsonpb.Marshaler{}).Marshal(&buf, maybeResetProtoRevision(cfg.PreserveRevision, plugin))
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
		m := jsonpb.Unmarshaler{AllowUnknownFields: true}
		if err := m.Unmarshal(bytes.NewReader(data), &plugin); err != nil {
			return nil, trace.BadParameter("%s", err)
		}
		if err := plugin.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.Revision != "" {
			plugin.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			plugin.SetExpiry(cfg.Expires)
		}
		return &plugin, nil
	}
	return nil, trace.BadParameter("unsupported plugin resource version %q", h.Version)
}
