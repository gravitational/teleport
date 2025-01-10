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
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/protoadapt"

	"github.com/gravitational/teleport/api/types"
)

// PluginStaticCredentials is the plugin static credentials service
type PluginStaticCredentials interface {
	// CreatePluginStaticCredentials will create a new plugin static credentials resource.
	CreatePluginStaticCredentials(ctx context.Context, pluginStaticCredentials types.PluginStaticCredentials) error

	// GetPluginStaticCredentials will get a plugin static credentials resource by name.
	GetPluginStaticCredentials(ctx context.Context, name string) (types.PluginStaticCredentials, error)

	// GetPluginStaticCredentialsByLabels will get a list of plugin static credentials resource by matching labels.
	GetPluginStaticCredentialsByLabels(ctx context.Context, labels map[string]string) ([]types.PluginStaticCredentials, error)

	// UpdatePluginStaticCredentials will update a plugin static credentials' resource.
	UpdatePluginStaticCredentials(ctx context.Context, pluginStaticCredentials types.PluginStaticCredentials) (types.PluginStaticCredentials, error)

	// DeletePluginStaticCredentials will delete a plugin static credentials resource.
	DeletePluginStaticCredentials(ctx context.Context, name string) error

	// GetAllPluginStaticCredentials will get all plugin static credentials.
	GetAllPluginStaticCredentials(ctx context.Context) ([]types.PluginStaticCredentials, error)
}

// MarshalPluginStaticCredentials marshals PluginStaticCredentials resource to JSON.
func MarshalPluginStaticCredentials(pluginStaticCredentials types.PluginStaticCredentials, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch pluginStaticCredentials := pluginStaticCredentials.(type) {
	case *types.PluginStaticCredentialsV1:
		if err := pluginStaticCredentials.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		data, err := protojson.Marshal(protoadapt.MessageV2Of(maybeResetProtoRevision(cfg.PreserveRevision, pluginStaticCredentials)))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return data, nil
	default:
		return nil, trace.BadParameter("unsupported plugin static credentials resource %T", pluginStaticCredentials)
	}
}

// UnmarshalPluginStaticCredentials unmarshals the plugin static credentials resource from JSON.
func UnmarshalPluginStaticCredentials(data []byte, opts ...MarshalOption) (types.PluginStaticCredentials, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing plugin static credentials resource data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	h, err := unmarshalHeaderWithProtoJSON(data)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	switch h.Version {
	case types.V1:
		var pluginStaticCredentials types.PluginStaticCredentialsV1
		if err := protojson.Unmarshal(data, protoadapt.MessageV2Of(&pluginStaticCredentials)); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := pluginStaticCredentials.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.Revision != "" {
			pluginStaticCredentials.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			pluginStaticCredentials.SetExpiry(cfg.Expires)
		}
		return &pluginStaticCredentials, nil
	}
	return nil, trace.BadParameter("unsupported plugin static credentials resource version %q", h.Version)
}
