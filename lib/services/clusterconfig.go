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

// DefaultClusterConfig is used as the default cluster configuration when
// one is not specified (record at node).
func DefaultClusterConfig() ClusterConfig {
	return &ClusterConfigV3{
		Kind:    KindClusterConfig,
		Version: V3,
		Metadata: Metadata{
			Name:      MetaNameClusterConfig,
			Namespace: defaults.Namespace,
		},
		Spec: ClusterConfigSpecV3{
			SessionRecording:    RecordAtNode,
			ProxyChecksHostKeys: HostKeyCheckYes,
			KeepAliveInterval:   NewDuration(defaults.KeepAliveInterval),
			KeepAliveCountMax:   int64(defaults.KeepAliveCountMax),
			LocalAuth:           NewBool(true),
		},
	}
}

// AuditConfigFromObject returns audit config from interface object
func AuditConfigFromObject(in interface{}) (*AuditConfig, error) {
	var cfg AuditConfig
	if in == nil {
		return &cfg, nil
	}
	if err := utils.ObjectToStruct(in, &cfg); err != nil {
		return nil, trace.Wrap(err)
	}
	return &cfg, nil
}

// IsRecordAtProxy returns true if recording is sync or async at proxy
func IsRecordAtProxy(mode string) bool {
	return mode == RecordAtProxy || mode == RecordAtProxySync
}

// IsRecordSync returns true if recording is sync or async for proxy or node
func IsRecordSync(mode string) bool {
	return mode == RecordAtProxySync || mode == RecordAtNodeSync
}

// ShouldUploadSessions returns whether audit config
// instructs server to upload sessions
func ShouldUploadSessions(a AuditConfig) bool {
	return a.AuditSessionsURI != ""
}

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

// UnmarshalClusterConfig unmarshals the ClusterConfig resource.
func UnmarshalClusterConfig(bytes []byte, opts ...MarshalOption) (ClusterConfig, error) {
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

// MarshalClusterConfig marshals the ClusterConfig resource.
func MarshalClusterConfig(c ClusterConfig, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch resource := c.(type) {
	case *ClusterConfigV3:
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
