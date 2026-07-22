// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package servicecfg

import "net/netip"

// RelayConfig contains the configuration for the Relay service.
type RelayConfig struct {
	Enabled bool

	// RelayGroup is the Relay group name, required if the Relay service is
	// enabled.
	RelayGroup string

	// TargetConnectionCount is the connection count that agents are supposed to
	// maintain when connecting to the Relay group of this instance.
	TargetConnectionCount int32

	// PublicHostnames is the list of DNS names and IP addresses that the Relay
	// service credentials should be authoritative for.
	PublicHostnames []string

	// TransportListenAddr is the listen address for the transport listener.
	TransportListenAddr netip.AddrPort

	// TransportPROXYProtocol is set if the transport listener should expect a
	// PROXY protocol header in incoming connections.
	TransportPROXYProtocol bool

	// PeerListenAddr is the listen address for the peer listener.
	PeerListenAddr netip.AddrPort

	// PeerPublicAddr, if set, is the public address for the peer listener, in
	// host:port format.
	PeerPublicAddr string

	// TunnelListenAddr is the listen address for the tunnel listener.
	TunnelListenAddr netip.AddrPort

	// TunnelPROXYProtocol is set if the tunnel listener should expect a PROXY
	// protocol header in incoming connections.
	TunnelPROXYProtocol bool
}
