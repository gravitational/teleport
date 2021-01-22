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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// ValidateTrustedCluster checks and sets Trusted Cluster defaults
func ValidateTrustedCluster(tc TrustedCluster) error {
	if err := tc.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// we are not mentioning Roles parameter because we are deprecating it
	if len(tc.GetRoles()) == 0 && len(tc.GetRoleMap()) == 0 {
		if err := modules.GetModules().EmptyRolesHandler(); err != nil {
			return trace.Wrap(err)
		}
		// OSS teleport uses 'admin' by default:
		tc.SetRoleMap(RoleMap{
			RoleMapping{
				Remote: teleport.AdminRoleName,
				Local:  []string{teleport.AdminRoleName},
			},
		})
	}

	return nil
}

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

// UnmarshalTrustedCluster unmarshals the TrustedCluster resource.
func UnmarshalTrustedCluster(bytes []byte, opts ...MarshalOption) (TrustedCluster, error) {
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

	err = trustedCluster.CheckAndSetDefaults()
	if err != nil {
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

// MarshalTrustedCluster marshals the TrustedCluster resource.
func MarshalTrustedCluster(c TrustedCluster, opts ...MarshalOption) ([]byte, error) {
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
