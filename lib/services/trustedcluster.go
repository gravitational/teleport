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

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// ValidateTrustedCluster checks and sets Trusted Cluster defaults
func ValidateTrustedCluster(tc TrustedCluster, allowEmptyRolesOpts ...bool) error {
	if err := tc.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// DELETE IN (7.0)
	// This flag is used to allow reading trusted clusters with no role map.
	// This was possible in OSS before 6.0 release.
	allowEmptyRoles := false
	if len(allowEmptyRolesOpts) != 0 {
		allowEmptyRoles = allowEmptyRolesOpts[0]
	}
	// we are not mentioning Roles parameter because we are deprecating it
	if len(tc.GetRoles()) == 0 && len(tc.GetRoleMap()) == 0 {
		if !allowEmptyRoles {
			return trace.BadParameter("missing 'role_map' parameter")
		}
	}

	if _, err := parseRoleMap(tc.GetRoleMap()); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// RoleMapToString prints user friendly representation of role mapping
func RoleMapToString(r RoleMap) string {
	values, err := parseRoleMap(r)
	if err != nil {
		return fmt.Sprintf("<failed to parse: %v", err)
	}
	if len(values) != 0 {
		return fmt.Sprintf("%v", values)
	}
	return "<empty>"
}

func parseRoleMap(r RoleMap) (map[string][]string, error) {
	directMatch := make(map[string][]string)
	for i := range r {
		roleMap := r[i]
		if roleMap.Remote == "" {
			return nil, trace.BadParameter("missing 'remote' parameter for role_map")
		}
		_, err := utils.ReplaceRegexp(roleMap.Remote, "", "")
		if trace.IsBadParameter(err) {
			return nil, trace.BadParameter("failed to parse 'remote' parameter for role_map: %v", err.Error())
		}
		if len(roleMap.Local) == 0 {
			return nil, trace.BadParameter("missing 'local' parameter for 'role_map'")
		}
		for _, local := range roleMap.Local {
			if local == "" {
				return nil, trace.BadParameter("missing 'local' property of 'role_map' entry")
			}
			if local == Wildcard {
				return nil, trace.BadParameter("wildcard value is not supported for 'local' property of 'role_map' entry")
			}
		}
		_, ok := directMatch[roleMap.Remote]
		if ok {
			return nil, trace.BadParameter("remote role '%v' match is already specified", roleMap.Remote)
		}
		directMatch[roleMap.Remote] = roleMap.Local
	}
	return directMatch, nil
}

// MapRoles maps local roles to remote roles
func MapRoles(r RoleMap, remoteRoles []string) ([]string, error) {
	_, err := parseRoleMap(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var outRoles []string
	// when no remote roles are specified, assume that
	// there is a single empty remote role (that should match wildcards)
	if len(remoteRoles) == 0 {
		remoteRoles = []string{""}
	}
	for _, mapping := range r {
		expression := mapping.Remote
		for _, remoteRole := range remoteRoles {
			// never map default implicit role, it is always
			// added by default
			if remoteRole == constants.DefaultImplicitRole {
				continue
			}
			for _, replacementRole := range mapping.Local {
				replacement, err := utils.ReplaceRegexp(expression, replacementRole, remoteRole)
				switch {
				case err == nil:
					// empty replacement can occur when $2 expand refers
					// to non-existing capture group in match expression
					if replacement != "" {
						outRoles = append(outRoles, replacement)
					}
				case trace.IsNotFound(err):
					continue
				default:
					return nil, trace.Wrap(err)
				}
			}
		}
	}
	return outRoles, nil
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

// UnmarshalTrustedCluster unmarshals the TrustedCluster resource from JSON.
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

	// DELETE IN(7.0)
	// temporarily allow to read trusted cluster with no role map
	// until users migrate from 6.0 OSS that had no role map present
	const allowEmptyRoleMap = true
	if err = ValidateTrustedCluster(&trustedCluster, allowEmptyRoleMap); err != nil {
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
