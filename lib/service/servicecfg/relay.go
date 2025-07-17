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

import "time"

// RelayConfig contains the configuration for the Relay service.
type RelayConfig struct {
	Enabled bool

	// RelayGroup is the Relay group name, required if the Relay service is
	// enabled.
	RelayGroup string

	// TargetConnectionCount is the connection count that agents are supposed to
	// maintain when connecting to the Relay group of this instance.
	TargetConnectionCount int32

	// ShutdownDelay is a minimum time to wait after a shutdown signal is
	// received and the terminating status is advertised in heartbeats and to
	// the connected agents but before stopping listeners and tunnels. Can be
	// used to give enough time to the agents connected to this Relay service to
	// connect to other Relay instances and advertise their connectivity. If not
	// set to a positive value, no delay is applied.
	ShutdownDelay time.Duration

	// APIPublicHostnames is the list of DNS names and IP addresses that the
	// Relay service credentials should be authoritative for.
	APIPublicHostnames []string

	// APIListenAddr is the listen address for the API listener, in addr:port
	// format. The default port used by the client if unspecified is 3040.
	APIListenAddr string

	// TunnelListenAddr is the listen address for the tunnel listener, in
	// addr:port format. There is no default port expected by clients, but port
	// 3042 is the intended default.
	TunnelListenAddr string
	// TunnelPublicAddr is the address that will be used by agents to connect to
	// the tunnel service load balancer.
	TunnelPublicAddr string
}
