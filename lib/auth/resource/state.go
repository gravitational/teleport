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

import "fmt"

// IdentitySpecV2Schema is a schema for identity spec.
const IdentitySpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["key", "ssh_cert", "tls_cert", "tls_ca_certs"],
  "properties": {
    "key": {"type": "string"},
    "ssh_cert": {"type": "string"},
    "tls_cert": {"type": "string"},
    "tls_ca_certs": {
      "type": "array",
      "items": {"type": "string"}
    },
    "ssh_ca_certs": {
      "type": "array",
      "items": {"type": "string"}
    }
  }
}`

// GetIdentitySchema returns JSON Schema for cert authorities.
func GetIdentitySchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, IdentitySpecV2Schema, DefaultDefinitions)
}

// StateSpecV2Schema is a schema for local server state.
const StateSpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["rotation"],
  "properties": {
    "rotation": %v
  }
}`

// GetStateSchema returns JSON Schema for cert authorities.
func GetStateSchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, fmt.Sprintf(StateSpecV2Schema, RotationSchema), DefaultDefinitions)
}
