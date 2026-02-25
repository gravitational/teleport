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

package reversetunnelv3

import (
	"net"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
)

// ServerHandler handles an inbound net.Conn that arrives through the reverse
// tunnel for a specific service type. Implementations must block until the
// connection is fully consumed and closed.
type ServerHandler interface {
	HandleConnection(net.Conn)
}

// Agent is a live, successfully-handshaked reverse tunnel session to a single
// proxy. It is created by newAgent, which blocks until the AgentHello/ProxyHello
// exchange succeeds, and is valid until Done closes.
type Agent interface {
	// GetProxyID returns the UUID of the proxy this session is connected to,
	// as provided in the ProxyHello. Used for proxy-peering reporting.
	GetProxyID() string

	// Done returns a channel that is closed when the agent's yamux session
	// terminates for any reason (network error, graceful shutdown, Stop call).
	Done() <-chan struct{}

	// Stop closes the session immediately and releases the tracker lease.
	// Idempotent.
	Stop() error

	// IsTerminating returns true if the proxy sent ProxyControl{terminating:true},
	// signalling a graceful shutdown. The AgentPool uses this to decide whether
	// to open replacement connections before this one fully closes.
	IsTerminating() bool
}

// agentConfig holds the parameters for constructing a single agent.
type agentConfig struct {
	// addr is the address (host:port) of the proxy to connect to.
	addr string

	// hostID is the stable UUID of this Teleport instance.
	hostID string

	// clusterName is the name of the local cluster.
	clusterName string

	// version is the Teleport version string of this agent.
	version string

	// scope is the resource scope from this agent's certificate.
	scope string

	// services lists the TunnelType values to advertise in AgentHello.
	services []types.TunnelType

	// handlers maps each registered service type to the local ServerHandler
	// that accepts inbound dial connections for that service.
	handlers map[types.TunnelType]ServerHandler

	// tracker is shared across all agents in the pool. The agent calls
	// lease.Claim after a successful handshake to assert ownership of the
	// proxy it connected to.
	tracker *track.Tracker

	// lease is the tracker lease allocated for this connection attempt.
	// The agent is responsible for releasing it exactly once — either via
	// lease.Claim on success, or lease.Release on failure.
	lease *track.Lease
}
