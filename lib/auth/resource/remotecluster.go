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

// RemoteClusterV3SchemaTemplate is a template JSON Schema for V3 style objects
const RemoteClusterV3SchemaTemplate = `{
	"type": "object",
	"additionalProperties": false,
	"required": ["kind", "metadata", "version"],
	"properties": {
	  "kind": {"type": "string"},
	  "version": {"type": "string", "default": "v3"},
	  "metadata": %v,
	  "status": %v
	}
  }`

// RemoteClusterV3StatusSchema is a template for remote cluster
const RemoteClusterV3StatusSchema = `{
	"type": "object",
	"additionalProperties": false,
	"required": ["connection", "last_heartbeat"],
	"properties": {
	  "connection": {"type": "string"},
	  "last_heartbeat": {"type": "string"}
	}
  }`

// GetRemoteClusterSchema returns the schema for remote cluster
func GetRemoteClusterSchema() string {
	return fmt.Sprintf(RemoteClusterV3SchemaTemplate, MetadataSchema, RemoteClusterV3StatusSchema)
}

// UnmarshalRemoteCluster unmarshals the RemoteCluster resource from JSON.
func UnmarshalRemoteCluster(bytes []byte, opts ...auth.MarshalOption) (RemoteCluster, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var cluster RemoteClusterV3

	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	if cfg.SkipValidation {
		err := utils.FastUnmarshal(bytes, &cluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		err = utils.UnmarshalWithSchema(GetRemoteClusterSchema(), &cluster, bytes)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	}

	err = cluster.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		cluster.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		cluster.SetExpiry(cfg.Expires)
	}

	return &cluster, nil
}

// MarshalRemoteCluster marshals the RemoteCluster resource to JSON.
func MarshalRemoteCluster(remoteCluster RemoteCluster, opts ...auth.MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch remoteCluster := remoteCluster.(type) {
	case *RemoteClusterV3:
		if version := remoteCluster.GetVersion(); version != V3 {
			return nil, trace.BadParameter("mismatched remote cluster version %v and type %T", version, remoteCluster)
		}
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *remoteCluster
			copy.SetResourceID(0)
			remoteCluster = &copy
		}
		return utils.FastMarshal(remoteCluster)
	default:
		return nil, trace.BadParameter("unrecognized remote cluster version %T", remoteCluster)
	}
}
