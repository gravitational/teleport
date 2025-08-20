/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
			return nil, trace.BadParameter("%s", err)
		}

		if err := r.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
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
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch tunnelConnection := tunnelConnection.(type) {
	case *types.TunnelConnectionV2:
		if err := tunnelConnection.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, tunnelConnection))
	default:
		return nil, trace.BadParameter("unrecognized tunnel connection version %T", tunnelConnection)
	}
}
