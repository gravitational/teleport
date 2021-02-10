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

// TrustedClusterSpecSchemaTemplate is a template for trusted cluster schema
const TrustedClusterSpecSchemaTemplate = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
	  "enabled": {"type": "boolean"},
	  "roles": {
		"type": "array",
		"items": {
		  "type": "string"
		}
	  },
	  "role_map": %v,
	  "token": {"type": "string"},
	  "web_proxy_addr": {"type": "string"},
	  "tunnel_addr": {"type": "string"}%v
	}
  }`

// RoleMapSchema is a schema for role mappings of trusted clusters
const RoleMapSchema = `{
	"type": "array",
	"items": {
	  "type": "object",
	  "additionalProperties": false,
	  "properties": {
		"local": {
		  "type": "array",
		  "items": {
			 "type": "string"
		  }
		},
		"remote": {"type": "string"}
	  }
	}
  }`

// GetTrustedClusterSchema returns the schema with optionally injected
// schema for extensions.
func GetTrustedClusterSchema(extensionSchema string) string {
	var trustedClusterSchema string
	if extensionSchema == "" {
		trustedClusterSchema = fmt.Sprintf(TrustedClusterSpecSchemaTemplate, RoleMapSchema, "")
	} else {
		trustedClusterSchema = fmt.Sprintf(TrustedClusterSpecSchemaTemplate, RoleMapSchema, ","+extensionSchema)
	}
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, trustedClusterSchema, DefaultDefinitions)
}

// UnmarshalTrustedCluster unmarshals the TrustedCluster resource from JSON.
func UnmarshalTrustedCluster(bytes []byte, opts ...auth.MarshalOption) (TrustedCluster, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var trustedCluster TrustedClusterV2

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	if cfg.SkipValidation {
		if err := utils.FastUnmarshal(bytes, &trustedCluster); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	} else {
		err := utils.UnmarshalWithSchema(GetTrustedClusterSchema(""), &trustedCluster, bytes)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	}

	// DELETE IN(7.0)
	// temporarily allow to read trusted cluster with no role map
	// until users migrate from 6.0 OSS that had no role map present
	const allowEmptyRoleMap = true
	if err = auth.ValidateTrustedCluster(&trustedCluster, allowEmptyRoleMap); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		trustedCluster.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		trustedCluster.SetExpiry(cfg.Expires)
	}
	return &trustedCluster, nil
}

// MarshalTrustedCluster marshals the TrustedCluster resource to JSON.
func MarshalTrustedCluster(c TrustedCluster, opts ...auth.MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch resource := c.(type) {
	case *TrustedClusterV2:
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
