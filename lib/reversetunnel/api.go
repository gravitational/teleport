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

reversetunnel package allows teleport clusters to connect to each
other and to allow users of one cluster to get access to machines
inside of another cluster.

This capability is called "Trusted Clusters": see Teleport documentation.
The words "site" and "clusters" are used in the code interchangeably.

Every cluster, in order to be accessible by other trusted clusters,
must register itself with the reverse tunnel server.

Reverse tunnel server: the TCP/IP server which accepts remote connections
(tunnels) and keeps track of them. There are two types of tunnels:
	- Direct (local)
	- Remote

Direct sites/tunnels are tunnels to itself, i.e. within the same cluster.
Remote sites/tunnels are, well, remote.
*/
package reversetunnel

import (
	"net"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"golang.org/x/crypto/ssh"
)

// RemoteSite represents remote teleport site that can be accessed via
// teleport tunnel or directly by proxy
//
// There are two implementations of this interface: local and remote sites.
type RemoteSite interface {
	// ConnectToServer allows to SSH into remote teleport server
	ConnectToServer(addr, user string, auth []ssh.AuthMethod) (*ssh.Client, error)
	// Dial dials any address within the site network
	Dial(fromAddr, toAddr net.Addr) (net.Conn, error)
	// GetLastConnected returns last time the remote site was seen connected
	GetLastConnected() time.Time
	// GetName returns site name (identified by authority domain's name)
	GetName() string
	// GetStatus returns status of this site (either offline or connected)
	GetStatus() string
	// GetClient returns client connected to remote auth server
	GetClient() (auth.ClientI, error)
}

// Server is a TCP/IP SSH server which listens on an SSH endpoint and remote/local
// sites connect and register with it.
type Server interface {
	// GetSites returns a list of connected remote sites
	GetSites() []RemoteSite
	// GetSite returns remote site this node belongs to
	GetSite(domainName string) (RemoteSite, error)
	// RemoveSite removes the site with the specified name from the list of connected sites
	RemoveSite(domainName string) error
	// Start starts server
	Start() error
	// CLose closes server's socket
	Close() error
	// Wait waits for server to close all outstanding operations
	Wait()
}
