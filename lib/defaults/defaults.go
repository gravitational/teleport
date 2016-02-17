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
package defaults

import (
	"fmt"

	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/utils"
)

// Default port numbers used by all teleport tools
const (
	// Web UI over HTTP
	HTTPListenPort = 3080

	// Web UI over HTTPS
	HTTPSecureListenPort = 3081

	// When running in "SSH Server" mode behind a proxy, this
	// listening port will be used to connect users to:
	SSHServerListenPort = 3022

	// When running in "SSH Proxy" role this port will be used to
	// accept incoming client connections and proxy them to SSHServerListenPort of
	// one of many SSH nodes
	SSHProxyListenPort = 3023

	// When running in "SSH Proxy" role this port will be used for incoming
	// connections from SSH nodes who wish to use "reverse tunnell" (when they
	// run behind an environment/firewall which only allows outgoing connections)
	SSHProxyTunnelListenPort = 3024

	// When running in "SSH Proxy" role this port will be used for incoming
	// connections from Hangouts nodes to register in the proxy
	SSHProxyHangoutsListenPort = 3026

	// When running as a "SSH Proxy" this port will be used to
	// serve auth requests.
	AuthListenPort = 3025

	// Default DB to use for persisting state. Another options is "etcd"
	BackendType = "bolt"

	// Default path to teleport config file
	ConfigFilePath = "/etc/teleport.yaml"

	// This is where all mutable data is stored (user keys, recorded sessions,
	// registered SSH servers, etc):
	DataDir = "/var/lib/teleport"

	// By default SSH server (and SSH proxy) will bind to this IP
	BindIP = "0.0.0.0"
)

// Default connection limits, they can be applied separately on any of the Teleport
// services (SSH, auth, proxy)
const (
	// Number of max. simultaneous connections to a service
	LimiterMaxConnections = 25

	// Number of max. simultaneous connected users/logins
	LimiterMaxConcurrentUsers = 25
)

// list of roles teleport service can run as:
const (
	RoleNode        = "node"
	RoleProxy       = "proxy"
	RoleAuthService = "auth"
)

var (
	// Default roles teleport assumes when started via 'start' command
	StartRoles = []string{RoleProxy, RoleNode, RoleAuthService}
)

const (
	initError = "failure initializing default values"
)

// ConfigureLimiter assigns the default parameters to a connection throttler (AKA limiter)
func ConfigureLimiter(lc *limiter.LimiterConfig) {
	lc.MaxConnections = LimiterMaxConnections
	lc.MaxNumberOfUsers = LimiterMaxConcurrentUsers
}

// AuthListenAddr returns the default listening address for the Auth service
func AuthListenAddr() *utils.NetAddr {
	return makeAddr(BindIP, AuthListenPort)
}

// AuthConnectAddr returns the default address to search for auth. service on
func AuthConnectAddr() *utils.NetAddr {
	return makeAddr("127.0.0.1", AuthListenPort)
}

// ProxyListenAddr returns the default listening address for the SSH Proxy service
func ProxyListenAddr() *utils.NetAddr {
	return makeAddr(BindIP, SSHProxyListenPort)
}

// ProxyWebListenAddr returns the default listening address for the Web-based SSH Proxy service
func ProxyWebListenAddr() *utils.NetAddr {
	return makeAddr(BindIP, HTTPListenPort)
}

// SSHServerListenAddr returns the default listening address for the Web-based SSH Proxy service
func SSHServerListenAddr() *utils.NetAddr {
	return makeAddr(BindIP, SSHServerListenPort)
}

// ReverseTunnellListenAddr returns the default listening address for the SSH Proxy service used
// by the SSH nodes to establish proxy<->ssh_node connection from behind a firewall which
// blocks inbound connecions to ssh_nodes
func ReverseTunnellListenAddr() *utils.NetAddr {
	return makeAddr(BindIP, SSHProxyTunnelListenPort)
}

// ReverseTunnellListenAddr returns the default listening address for the SSH Proxy service used
// by the Hangout nodes to establish hangouts_server<->proxy connection from the hangout
// owner side
func HangoutsListenAddr() *utils.NetAddr {
	return makeAddr(BindIP, SSHProxyHangoutsListenPort)
}

func ReverseTunnellConnectAddr() *utils.NetAddr {
	return makeAddr("127.0.0.1", SSHProxyTunnelListenPort)
}

func makeAddr(host string, port int16) *utils.NetAddr {
	addrSpec := fmt.Sprintf("tcp://%s:%d", host, port)
	retval, err := utils.ParseAddr(addrSpec)
	if err != nil {
		panic(fmt.Sprintf("%s: error parsing '%v'", initError, addrSpec))
	}
	return retval
}
