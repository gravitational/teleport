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

// ClusterConfigSpecSchemaTemplate is a template for ClusterConfig schema.
const ClusterConfigSpecSchemaTemplate = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
	  "session_recording": {
		"type": "string"
	  },
	  "proxy_checks_host_keys": {
		"type": "string"
	  },
	  "cluster_id": {
		"type": "string"
	  },
	  "client_idle_timeout": {
		"type": "string"
	  },
	  "session_control_timeout": {
		"type": "string"
	  },
	  "disconnect_expired_cert": {
		"anyOf": [{"type": "string"}, { "type": "boolean"}]
	  },
	  "keep_alive_interval": {
		"type": "string"
	  },
	  "keep_alive_count_max": {
		"type": "number"
	  },
	  "local_auth": {
		"anyOf": [{"type": "string"}, { "type": "boolean"}]
	  },
	  "audit": {
		"type": "object",
		"additionalProperties": false,
		"properties": {
		  "type": {
			"type": "string"
		  },
		  "region": {
			"type": "string"
		  },
		  "audit_events_uri": {
			"anyOf": [
			  {"type": "string"},
			  {"type": "array",
			   "items": {
				 "type": "string"
			   }
			  }
			]
		  },
		  "audit_sessions_uri": {
			"type": "string"
		  },
		  "audit_table_name": {
			"type": "string"
		  }
		}
	  }%v
	}
  }`

// GetClusterConfigSchema returns the schema with optionally injected
// schema for extensions.
func GetClusterConfigSchema(extensionSchema string) string {
	var clusterConfigSchema string
	if clusterConfigSchema == "" {
		clusterConfigSchema = fmt.Sprintf(ClusterConfigSpecSchemaTemplate, "")
	} else {
		clusterConfigSchema = fmt.Sprintf(ClusterConfigSpecSchemaTemplate, ","+extensionSchema)
	}
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, clusterConfigSchema, DefaultDefinitions)
}

// UnmarshalClusterConfig unmarshals the ClusterConfig resource from JSON.
func UnmarshalClusterConfig(bytes []byte, opts ...auth.MarshalOption) (ClusterConfig, error) {
	var clusterConfig ClusterConfigV3

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.SkipValidation {
		if err := utils.FastUnmarshal(bytes, &clusterConfig); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	} else {
		err = utils.UnmarshalWithSchema(GetClusterConfigSchema(""), &clusterConfig, bytes)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	}

	err = clusterConfig.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		clusterConfig.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		clusterConfig.SetExpiry(cfg.Expires)
	}
	return &clusterConfig, nil
}

// MarshalClusterConfig marshals the ClusterConfig resource to JSON.
func MarshalClusterConfig(clusterConfig ClusterConfig, opts ...auth.MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch clusterConfig := clusterConfig.(type) {
	case *ClusterConfigV3:
		if version := clusterConfig.GetVersion(); version != V3 {
			return nil, trace.BadParameter("mismatched cluster config version %v and type %T", version, clusterConfig)
		}
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *clusterConfig
			copy.SetResourceID(0)
			clusterConfig = &copy
		}
		return utils.FastMarshal(clusterConfig)
	default:
		return nil, trace.BadParameter("unrecognized cluster config version %T", clusterConfig)
	}
}
