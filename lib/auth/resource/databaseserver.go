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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// DatabaseServerSpecV3Schema is JSON schema for a database server spec.
const DatabaseServerSpecV3Schema = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
	  "description": {"type": "string"},
	  "protocol": {"type": "string"},
	  "uri": {"type": "string"},
	  "ca_cert": {"type": "string"},
	  "aws": {
		"type": "object",
		"additionalProperties": false,
		"properties": {
		  "region": {"type": "string"}
		}
	  },
	  "gcp": {
		"type": "object",
		"additionalProperties": false,
		"properties": {
		  "project_id": {"type": "string"},
		  "instance_id": {"type": "string"}
		}
	  },
	  "version": {"type": "string"},
	  "hostname": {"type": "string"},
	  "host_id": {"type": "string"},
	  "dynamic_labels": {
		"type": "object",
		"additionalProperties": false,
		"patternProperties": {
		  "^.*$": {
			"type": "object",
			"additionalProperties": false,
			"required": ["command"],
			"properties": {
			  "command": {"type": "array", "items": {"type": "string"}},
			  "period": {"type": "string"},
			  "result": {"type": "string"}
			}
		  }
		}
	  },
	  "rotation": %v
	}
  }`

// GetDatabaseServerSchema returns full database server JSON schema.
func GetDatabaseServerSchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, fmt.Sprintf(DatabaseServerSpecV3Schema, RotationSchema), DefaultDefinitions)
}

// MarshalDatabaseServer marshals the DatabaseServer resource to JSON.
func MarshalDatabaseServer(databaseServer types.DatabaseServer, opts ...auth.MarshalOption) ([]byte, error) {
	if err := databaseServer.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch databaseServer := databaseServer.(type) {
	case *types.DatabaseServerV3:
		if version := databaseServer.GetVersion(); version != V3 {
			return nil, trace.BadParameter("mismatched database server version %v and type %T", version, databaseServer)
		}
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *databaseServer
			copy.SetResourceID(0)
			databaseServer = &copy
		}
		return utils.FastMarshal(databaseServer)
	default:
		return nil, trace.BadParameter("unrecognized database server version %T", databaseServer)
	}
}

// UnmarshalDatabaseServer unmarshals the DatabaseServer resource from JSON.
func UnmarshalDatabaseServer(data []byte, opts ...auth.MarshalOption) (types.DatabaseServer, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing database server data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case V3:
		var s types.DatabaseServerV3
		if cfg.SkipValidation {
			if err := utils.FastUnmarshal(data, &s); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		} else {
			if err := utils.UnmarshalWithSchema(GetDatabaseServerSchema(), &s, data); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		}
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			s.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			s.SetExpiry(cfg.Expires)
		}
		return &s, nil
	}
	return nil, trace.BadParameter("database server resource version %q is not supported", h.Version)
}
