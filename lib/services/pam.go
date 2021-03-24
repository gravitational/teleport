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

package services

import (
	"encoding/json"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// PAMConfigSchema is the JSON schema that the serialized form of PAMConfig is validated against.
const PAMConfigSchema = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
		"enabled": {
			"type": "boolean"
		},
		"service_name": {
			"type": "string"
		},
		"use_pam_auth": {
			"type": "boolean"
		},
		"environment": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"key": {"type": "string"},
					"value": {"type": "string"}
				}
			}
		},
	}
}`

// UnmarshalPAMConfig unmarshals JSON into a PAMConfig resource.
func UnmarshalPAMConfig(bytes []byte, opts ...MarshalOption) (types.PAMConfig, error) {
	var config types.PAMConfigV3

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.SkipValidation {
		if err := utils.FastUnmarshal(bytes, &config); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	} else {
		err := utils.UnmarshalWithSchema(PAMConfigSchema, &config, bytes)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	}

	if cfg.ID != 0 {
		config.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		config.SetExpiry(cfg.Expires)
	}

	return &config, nil
}

// MarshalPAMConfig marshals the PAMConfig resource to JSON.
func MarshalPAMConfig(c types.PAMConfig, opts ...MarshalOption) ([]byte, error) {
	return json.Marshal(c)
}
