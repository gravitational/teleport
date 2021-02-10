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

// TunnelConnectionSpecV2Schema is JSON schema for reverse tunnel spec
const TunnelConnectionSpecV2Schema = `{
	"type": "object",
	"additionalProperties": false,
	"required": ["cluster_name", "proxy_name", "last_heartbeat"],
	"properties": {
	  "cluster_name": {"type": "string"},
	  "proxy_name": {"type": "string"},
	  "last_heartbeat": {"type": "string"},
	  "type": {"type": "string"}
	}
  }`

// GetTunnelConnectionSchema returns role schema with optionally injected
// schema for extensions
func GetTunnelConnectionSchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, TunnelConnectionSpecV2Schema, DefaultDefinitions)
}

// UnmarshalTunnelConnection unmarshals TunnelConnection resource from JSON or YAML,
// sets defaults and checks the schema
func UnmarshalTunnelConnection(data []byte, opts ...auth.MarshalOption) (TunnelConnection, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing tunnel connection data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h ResourceHeader
	err = utils.FastUnmarshal(data, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case V2:
		var r TunnelConnectionV2

		if cfg.SkipValidation {
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		} else {
			if err := utils.UnmarshalWithSchema(GetTunnelConnectionSchema(), &r, data); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		}

		if err := r.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			r.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			r.SetExpiry(cfg.Expires)
		}
		return &r, nil
	}
	return nil, trace.BadParameter("reverse tunnel version %v is not supported", h.Version)
}

// MarshalTunnelConnection marshals the TunnelConnection resource to JSON.
func MarshalTunnelConnection(tunnelConnection TunnelConnection, opts ...auth.MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch tunnelConnection := tunnelConnection.(type) {
	case *TunnelConnectionV2:
		if version := tunnelConnection.GetVersion(); version != V2 {
			return nil, trace.BadParameter("mismatched tunnel connection version %v and type %T", version, tunnelConnection)
		}
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *tunnelConnection
			copy.SetResourceID(0)
			tunnelConnection = &copy
		}
		return utils.FastMarshal(tunnelConnection)
	default:
		return nil, trace.BadParameter("unrecognized tunnel connection version %T", tunnelConnection)
	}
}
