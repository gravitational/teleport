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

package resource

import (
	"fmt"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// PluginDataSpecSchema is JSON schema for PluginData
const PluginDataSpecSchema = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
		"entries": { "type":"object" }
	}
}`

// GetPluginDataSchema returns the full PluginDataSchema string
func GetPluginDataSchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, PluginDataSpecSchema, DefaultDefinitions)
}

//MarshalPluginData marshals the PluginData resource to JSON.
func MarshalPluginData(pluginData PluginData, opts ...auth.MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch pluginData := pluginData.(type) {
	case *PluginDataV3:
		if version := pluginData.GetVersion(); version != V3 {
			return nil, trace.BadParameter("mismatched plugin data version %v and type %T", version, pluginData)
		}
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			cp := *pluginData
			cp.SetResourceID(0)
			pluginData = &cp
		}
		return utils.FastMarshal(pluginData)
	default:
		return nil, trace.BadParameter("unrecognized plugin data type: %T", pluginData)
	}
}

// UnmarshalPluginData unmarshals the PluginData resource from JSON.
func UnmarshalPluginData(raw []byte, opts ...auth.MarshalOption) (PluginData, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var data PluginDataV3
	if cfg.SkipValidation {
		if err := utils.FastUnmarshal(raw, &data); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if err := utils.UnmarshalWithSchema(GetPluginDataSchema(), &data, raw); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if err := data.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		data.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		data.SetExpiry(cfg.Expires)
	}
	return &data, nil
}
