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
	"encoding/json"
	"fmt"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// AuthPreferenceSpecSchemaTemplate is JSON schema for AuthPreferenceSpec
const AuthPreferenceSpecSchemaTemplate = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
		"type": {
			"type": "string"
		},
		"second_factor": {
			"type": "string"
		},
		"connector_name": {
			"type": "string"
		},
		"u2f": {
			"type": "object",
			"additionalProperties": false,
			"properties": {
				"app_id": {
					"type": "string"
				},
				"facets": {
					"type": "array",
					"items": {
						"type": "string"
					}
				},
				"device_attestation_cas": {
					"type": "array",
					"items": {
						"type": "string"
					}
				}
			}
		},
		"require_session_mfa": {
			"type": "boolean"
		}%v
	}
}`

// LocalAuthSecretsSchema is a JSON schema for LocalAuthSecrets
const LocalAuthSecretsSchema = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
		"password_hash": {"type": "string"},
		"totp_key": {"type": "string"},
		"u2f_registration": {
			"type": "object",
			"additionalProperties": false,
			"properties": {
				"raw": {"type": "string"},
				"key_handle": {"type": "string"},
				"pubkey": {"type": "string"}
			}
		},
		"u2f_counter": {"type": "number"},
		"mfa": {
			"type": "array",
			"items": {
				"type": "object",
				"additionalProperties": false,
				"properties": {
					"kind": {"type": "string"},
					"subKind": {"type": "string"},
					"version": {"type": "string"},
					"metadata": {
						"type": "object",
						"additionalProperties": false,
						"properties": {
							"Name": {"type": "string"},
							"Namespace": {"type": "string"}
						}
					},
					"id": {"type": "string"},
					"name": {"type": "string"},
					"addedAt": {"type": "string"},
					"lastUsed": {"type": "string"},
					"totp": {
						"type": "object",
						"additionalProperties": false,
						"properties": {
							"key": {"type": "string"}
						}
					},
					"u2f": {
						"type": "object",
						"additionalProperties": false,
						"properties": {
							"raw": {"type": "string"},
							"keyHandle": {"type": "string"},
							"pubKey": {"type": "string"},
							"counter": {"type": "number"}
						}
					}
				}
			}
		}
	}
}`

// GetAuthPreferenceSchema returns the schema with optionally injected
// schema for extensions.
func GetAuthPreferenceSchema(extensionSchema string) string {
	var authPreferenceSchema string
	if authPreferenceSchema == "" {
		authPreferenceSchema = fmt.Sprintf(AuthPreferenceSpecSchemaTemplate, "")
	} else {
		authPreferenceSchema = fmt.Sprintf(AuthPreferenceSpecSchemaTemplate, ","+extensionSchema)
	}
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, authPreferenceSchema, DefaultDefinitions)
}

// UnmarshalAuthPreference unmarshals the AuthPreference resource from JSON.
func UnmarshalAuthPreference(bytes []byte, opts ...auth.MarshalOption) (AuthPreference, error) {
	var authPreference AuthPreferenceV2

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.SkipValidation {
		if err := utils.FastUnmarshal(bytes, &authPreference); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	} else {
		err := utils.UnmarshalWithSchema(GetAuthPreferenceSchema(""), &authPreference, bytes)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	}
	if cfg.ID != 0 {
		authPreference.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		authPreference.SetExpiry(cfg.Expires)
	}
	return &authPreference, nil
}

// MarshalAuthPreference marshals the AuthPreference resource to JSON.
func MarshalAuthPreference(c AuthPreference, opts ...auth.MarshalOption) ([]byte, error) {
	return json.Marshal(c)
}
