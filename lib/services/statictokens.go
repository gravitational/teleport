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
	"fmt"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// DefaultStaticTokens is used to get the default static tokens (empty list)
// when nothing is specified in file configuration.
func DefaultStaticTokens() StaticTokens {
	return &StaticTokensV2{
		Kind:    KindStaticTokens,
		Version: V2,
		Metadata: Metadata{
			Name:      MetaNameStaticTokens,
			Namespace: defaults.Namespace,
		},
		Spec: StaticTokensSpecV2{
			StaticTokens: []ProvisionTokenV1{},
		},
	}
}

// StaticTokensSpecSchemaTemplate is a template for StaticTokens schema.
const StaticTokensSpecSchemaTemplate = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
		"static_tokens": {
			"type": "array",
			"items": {
				"type": "object",
				"additionalProperties": false,
				"properties": {
					"expires": {
						"type": "string"
					},
					"roles": {
						"type": "array",
						"items": {
							"type": "string"
						}
					},
					"token": {
						"type": "string"
					}
				}
			}
		}%v
  	}
}`

// GetStaticTokensSchema returns the schema with optionally injected
// schema for extensions.
func GetStaticTokensSchema(extensionSchema string) string {
	var staticTokensSchema string
	if staticTokensSchema == "" {
		staticTokensSchema = fmt.Sprintf(StaticTokensSpecSchemaTemplate, "")
	} else {
		staticTokensSchema = fmt.Sprintf(StaticTokensSpecSchemaTemplate, ","+extensionSchema)
	}
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, staticTokensSchema, DefaultDefinitions)
}

// UnmarshalStaticTokens unmarshals the StaticTokens resource.
func UnmarshalStaticTokens(bytes []byte, opts ...MarshalOption) (StaticTokens, error) {
	var staticTokens StaticTokensV2

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.SkipValidation {
		if err := utils.FastUnmarshal(bytes, &staticTokens); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	} else {
		err = utils.UnmarshalWithSchema(GetStaticTokensSchema(""), &staticTokens, bytes)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	}

	err = staticTokens.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		staticTokens.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		staticTokens.SetExpiry(cfg.Expires)
	}
	return &staticTokens, nil
}

// MarshalStaticTokens marshals the StaticTokens resource.
func MarshalStaticTokens(c StaticTokens, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch resource := c.(type) {
	case *StaticTokensV2:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *resource
			copy.SetResourceID(0)
			resource = &copy
		}
		return utils.FastMarshal(resource)
	default:
		return nil, trace.BadParameter("unrecognized resource version %T", c)
	}
}
