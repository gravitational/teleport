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

	// DeletePluginStaticCredentials will delete a plugin static credentials resource.
	DeletePluginStaticCredentials(ctx context.Context, name string) error
}

// MarshalPluginStaticCredentials marshals PluginStaticCredentials resource to JSON.
func MarshalPluginStaticCredentials(pluginStaticCredentials types.PluginStaticCredentials, opts ...MarshalOption) ([]byte, error) {
	if err := pluginStaticCredentials.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch pluginStaticCredentials := pluginStaticCredentials.(type) {
	case *types.PluginStaticCredentialsV1:
		if !cfg.PreserveResourceID {
			copy := *pluginStaticCredentials
			copy.SetResourceID(0)
			pluginStaticCredentials = &copy
		}
		data, err := protojson.Marshal(protoadapt.MessageV2Of(pluginStaticCredentials))
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
		if cfg.ID != 0 {
			pluginStaticCredentials.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			pluginStaticCredentials.SetExpiry(cfg.Expires)
		}
		return &pluginStaticCredentials, nil
	}
	return nil, trace.BadParameter("unsupported plugin static credentials resource version %q", h.Version)
}
