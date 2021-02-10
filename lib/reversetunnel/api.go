/*
Copyright 2016 Gravitational, Inc.

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

package reversetunnel

import (
	"context"

	"fmt"
	"net"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/teleagent"
)

// DialParams is a list of parameters used to Dial to a node within a cluster.
type DialParams struct {
	// From is the source address.
	From net.Addr

	// To is the destination address.
	To net.Addr

	// GetUserAgent gets an SSH agent for use in connecting to the remote host. Used by the
	// forwarding proxy.
	GetUserAgent teleagent.Getter

	// Address is used by the forwarding proxy to generate a host certificate for
	// the target node. This is needed because while dialing occurs via IP
	// address, tsh thinks it's connecting via DNS name and that's how it
	// validates the host certificate.
	Address string

	// Principals are additional principals that need to be added to the host
	// certificate. Used by the recording proxy to correctly generate a host
	// certificate.
	Principals []string

	// ServerID the hostUUID.clusterName of a Teleport node. Used with nodes
	// that are connected over a reverse tunnel.
	ServerID string

	// ConnType is the type of connection requested, either node or application.
	// Only used when connecting through a tunnel.
	ConnType services.TunnelType
}

func (params DialParams) String() string {
	to := params.To.String()
	if to == "" {
		to = params.ServerID
	}
	return fmt.Sprintf("from: %q to: %q", params.From, to)
}

// RemoteSite represents remote teleport site that can be accessed via
// teleport tunnel or directly by proxy
//
// There are two implementations of this interface: local and remote sites.
type RemoteSite interface {
	// DialAuthServer returns a net.Conn to the Auth Server of a site.
	DialAuthServer() (net.Conn, error)
	// Dial dials any address within the site network, in terminating
	// mode it uses local instance of forwarding server to terminate
	// and record the connection
	Dial(DialParams) (net.Conn, error)
	// DialTCP dials any address within the site network,
	// ignores recording mode and always uses TCP dial, used
	// in components that need direct dialer.
	DialTCP(DialParams) (net.Conn, error)
	// GetLastConnected returns last time the remote site was seen connected
	GetLastConnected() time.Time
	// GetName returns site name (identified by authority domain's name)
	GetName() string
	// GetStatus returns status of this site (either offline or connected)
	GetStatus() string
	// GetClient returns client connected to remote auth server
	GetClient() (client.ClientI, error)
	// CachingAccessPoint returns access point that is lightweight
	// but is resilient to auth server crashes
	CachingAccessPoint() (auth.AccessPoint, error)
	// GetTunnelsCount returns the amount of active inbound tunnels
	// from the remote cluster
	GetTunnelsCount() int
	// IsClosed reports whether this RemoteSite has been closed and should no
	// longer be used.
	IsClosed() bool
}

// Tunnel provides access to connected local or remote clusters
// using unified interface.
type Tunnel interface {
	// GetSites returns a list of connected remote sites
	GetSites() ([]RemoteSite, error)
	// GetSite returns remote site this node belongs to
	GetSite(domainName string) (RemoteSite, error)
}

// Server is a TCP/IP SSH server which listens on an SSH endpoint and remote/local
// sites connect and register with it.
type Server interface {
	Tunnel
	// Start starts server
	Start() error
	// Close closes server's operations immediately
	Close() error
	// Shutdown performs graceful server shutdown
	Shutdown(context.Context) error
	// Wait waits for server to close all outstanding operations
	Wait()
}
