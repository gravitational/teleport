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
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// LatestTunnelConnection returns latest tunnel connection from the list
// of tunnel connections, if no connections found, returns NotFound error
func LatestTunnelConnection(conns []types.TunnelConnection) (types.TunnelConnection, error) {
	var lastConn types.TunnelConnection
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
func TunnelConnectionStatus(clock clockwork.Clock, conn types.TunnelConnection, offlineThreshold time.Duration) string {
	diff := clock.Now().Sub(conn.GetLastHeartbeat())
	if diff < offlineThreshold {
		return teleport.RemoteClusterStatusOnline
	}
	return teleport.RemoteClusterStatusOffline
}

// UnmarshalTunnelConnection unmarshals TunnelConnection resource from JSON or YAML,
// sets defaults and checks the schema
func UnmarshalTunnelConnection(data []byte, opts ...MarshalOption) (types.TunnelConnection, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing tunnel connection data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	err = utils.FastUnmarshal(data, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V2:
		var r types.TunnelConnectionV2

		if err := utils.FastUnmarshal(data, &r); err != nil {
			return nil, trace.BadParameter(err.Error())
		}

		if err := r.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			r.SetResourceID(cfg.ID)
		}
		if cfg.Revision != "" {
			r.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			r.SetExpiry(cfg.Expires)
		}
		return &r, nil
	}
	return nil, trace.BadParameter("reverse tunnel version %v is not supported", h.Version)
}

// MarshalTunnelConnection marshals the TunnelConnection resource to JSON.
func MarshalTunnelConnection(tunnelConnection types.TunnelConnection, opts ...MarshalOption) ([]byte, error) {
	if err := tunnelConnection.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch tunnelConnection := tunnelConnection.(type) {
	case *types.TunnelConnectionV2:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *tunnelConnection
			copy.SetResourceID(0)
			copy.SetRevision("")
			tunnelConnection = &copy
		}
		return utils.FastMarshal(tunnelConnection)
	default:
		return nil, trace.BadParameter("unrecognized tunnel connection version %T", tunnelConnection)
	}
}
