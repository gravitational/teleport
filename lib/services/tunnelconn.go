/*
Copyright 2015-2019 Gravitational, Inc.

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
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// LatestTunnelConnection returns latest tunnel connection from the list
// of tunnel connections, if no connections found, returns NotFound error
func LatestTunnelConnection(conns []TunnelConnection) (TunnelConnection, error) {
	var lastConn TunnelConnection
	for i := range conns {
		conn := conns[i]
		if lastConn == nil || conn.GetLastHeartbeat().After(lastConn.GetLastHeartbeat()) {
			lastConn = conn
		}
	}
	if lastConn == nil {
		return nil, trace.NotFound("no connections found")
	}
	return lastConn, nil
}

// TunnelConnectionStatus returns tunnel connection status based on the last
// heartbeat time recorded for a connection
func TunnelConnectionStatus(clock clockwork.Clock, conn TunnelConnection, offlineThreshold time.Duration) string {
	diff := clock.Now().Sub(conn.GetLastHeartbeat())
	if diff < offlineThreshold {
		return teleport.RemoteClusterStatusOnline
	}
	return teleport.RemoteClusterStatusOffline
}

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

// UnmarshalTunnelConnection unmarshals reverse tunnel from JSON or YAML,
// sets defaults and checks the schema
func UnmarshalTunnelConnection(data []byte, opts ...MarshalOption) (TunnelConnection, error) {
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

// MarshalTunnelConnection marshals tunnel connection
func MarshalTunnelConnection(rt TunnelConnection, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch resource := rt.(type) {
	case *TunnelConnectionV2:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *resource
			copy.SetResourceID(0)
			resource = &copy
		}
		return utils.FastMarshal(resource)
	default:
		return nil, trace.BadParameter("unrecognized resource version %T", rt)
	}
}
