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

package helpers

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// InstanceListeners represents the listener configuration for a test cluster.
// Each address field is expected to be hull host:port pair.
type InstanceListeners struct {
	Web               string
	SSH               string
	SSHProxy          string
	Auth              string
	ReverseTunnel     string
	MySQL             string
	Postgres          string
	Mongo             string
	IsSinglePortSetup bool
}

// InstanceListenerSetupFunc defines a function type used for specifying the
// listener setup for a given test. InstanceListenerSetupFuncs are useful when
// you need to have some distance between the test configuration and actually
// executing the listener setup.
type InstanceListenerSetupFunc func(*testing.T, *[]*servicecfg.FileDescriptor) *InstanceListeners

// StandardListenerSetupOn returns a InstanceListenerSetupFunc that will create
// a new InstanceListeners configured with each service listening on its own
// port, all bound to the supplied address
func StandardListenerSetupOn(addr string) func(t *testing.T, fds *[]*servicecfg.FileDescriptor) *InstanceListeners {
	return func(t *testing.T, fds *[]*servicecfg.FileDescriptor) *InstanceListeners {
		return &InstanceListeners{
			Web:           NewListenerOn(t, addr, service.ListenerProxyWeb, fds),
			SSH:           NewListenerOn(t, addr, service.ListenerNodeSSH, fds),
			Auth:          NewListenerOn(t, addr, service.ListenerAuth, fds),
			SSHProxy:      NewListenerOn(t, addr, service.ListenerProxySSH, fds),
			ReverseTunnel: NewListenerOn(t, addr, service.ListenerProxyTunnel, fds),
			MySQL:         NewListenerOn(t, addr, service.ListenerProxyMySQL, fds),
		}
	}
}

// StandardListenerSetup creates an InstanceListeners configures with each service
// listening on its own port, all bound to the loopback address
func StandardListenerSetup(t *testing.T, fds *[]*servicecfg.FileDescriptor) *InstanceListeners {
	return StandardListenerSetupOn(Loopback)(t, fds)
}

// SingleProxyPortSetupOn creates a constructor function that will in turn generate an
// InstanceConfig that allows proxying of multiple protocols over a single port when
// invoked.
func SingleProxyPortSetupOn(addr string) func(*testing.T, *[]*servicecfg.FileDescriptor) *InstanceListeners {
	return func(t *testing.T, fds *[]*servicecfg.FileDescriptor) *InstanceListeners {
		ssh := NewListenerOn(t, addr, service.ListenerProxyWeb, fds)
		return &InstanceListeners{
			Web:               ssh,
			SSH:               NewListenerOn(t, addr, service.ListenerNodeSSH, fds),
			Auth:              NewListenerOn(t, addr, service.ListenerAuth, fds),
			SSHProxy:          ssh,
			ReverseTunnel:     ssh,
			MySQL:             ssh,
			IsSinglePortSetup: true,
		}
	}
}

// SingleProxyPortSetup generates an InstanceConfig that allows proxying of multiple protocols
// over a single port.
func SingleProxyPortSetup(t *testing.T, fds *[]*servicecfg.FileDescriptor) *InstanceListeners {
	return SingleProxyPortSetupOn(Loopback)(t, fds)
}

// WebReverseTunnelMuxPortSetup generates a listener config using the same port for web and
// tunnel, and independent ports for all other services.
func WebReverseTunnelMuxPortSetup(t *testing.T, fds *[]*servicecfg.FileDescriptor) *InstanceListeners {
	web := NewListener(t, service.ListenerProxyTunnelAndWeb, fds)
	return &InstanceListeners{
		Web:           web,
		ReverseTunnel: web,
		SSH:           NewListener(t, service.ListenerNodeSSH, fds),
		SSHProxy:      NewListener(t, service.ListenerProxySSH, fds),
		MySQL:         NewListener(t, service.ListenerProxyMySQL, fds),
		Auth:          NewListener(t, service.ListenerAuth, fds),
	}
}

// SeparatePostgresPortSetup generates a listener config with a defined port for Postgres
func SeparatePostgresPortSetup(t *testing.T, fds *[]*servicecfg.FileDescriptor) *InstanceListeners {
	return &InstanceListeners{
		Web:           NewListener(t, service.ListenerProxyWeb, fds),
		SSH:           NewListener(t, service.ListenerNodeSSH, fds),
		Auth:          NewListener(t, service.ListenerAuth, fds),
		SSHProxy:      NewListener(t, service.ListenerProxySSH, fds),
		ReverseTunnel: NewListener(t, service.ListenerProxyTunnel, fds),
		MySQL:         NewListener(t, service.ListenerProxyMySQL, fds),
		Postgres:      NewListener(t, service.ListenerProxyPostgres, fds),
	}
}

// SeparateMongoPortSetup generates a listener config with a defined port for MongoDB
func SeparateMongoPortSetup(t *testing.T, fds *[]*servicecfg.FileDescriptor) *InstanceListeners {
	return &InstanceListeners{
		Web:           NewListener(t, service.ListenerProxyWeb, fds),
		SSH:           NewListener(t, service.ListenerNodeSSH, fds),
		Auth:          NewListener(t, service.ListenerAuth, fds),
		SSHProxy:      NewListener(t, service.ListenerProxySSH, fds),
		ReverseTunnel: NewListener(t, service.ListenerProxyTunnel, fds),
		MySQL:         NewListener(t, service.ListenerProxyMySQL, fds),
		Mongo:         NewListener(t, service.ListenerProxyMongo, fds),
	}
}

// SeparateMongoAndPostgresPortSetup generates a listener config with a defined port for Postgres and Mongo
func SeparateMongoAndPostgresPortSetup(t *testing.T, fds *[]*servicecfg.FileDescriptor) *InstanceListeners {
	return &InstanceListeners{
		Web:           NewListener(t, service.ListenerProxyWeb, fds),
		SSH:           NewListener(t, service.ListenerNodeSSH, fds),
		Auth:          NewListener(t, service.ListenerAuth, fds),
		SSHProxy:      NewListener(t, service.ListenerProxySSH, fds),
		ReverseTunnel: NewListener(t, service.ListenerProxyTunnel, fds),
		MySQL:         NewListener(t, service.ListenerProxyMySQL, fds),
		Mongo:         NewListener(t, service.ListenerProxyMongo, fds),
		Postgres:      NewListener(t, service.ListenerProxyPostgres, fds),
	}
}

// PortStr extracts the port number from the supplied string, which is assumed
// to be a host:port pair. The port is returned as a string. Any errors result
// in an immediately failed test.
func PortStr(t *testing.T, addr string) string {
	t.Helper()

	_, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	return portStr
}

// Port extracts the port number from the supplied string, which is assumed
// to be a host:port pair. The port value is returned as an integer. Any errors
// result in an immediately failed test.
func Port(t *testing.T, addr string) int {
	t.Helper()

	portStr := PortStr(t, addr)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	return port
}

// NewListenerOn creates a new TCP listener on `hostAddr`:0, adds it to the
// FileDescriptor slice (with the specified type) and returns its actual local
// address as a string (for use in configuration). The idea is to subvert
// Teleport's file-descriptor injection mechanism (used to share ports between
// parent and child processes) to inject preconfigured listeners to Teleport
// instances under test. The ports are allocated and bound at runtime, so there
// should be no issues with port clashes on parallel tests.
//
// The resulting file descriptor is added to the `fds` slice, which can then be
// given to a teleport instance on startup in order to suppl
func NewListenerOn(t *testing.T, hostAddr string, ty service.ListenerType, fds *[]*servicecfg.FileDescriptor) string {
	t.Helper()

	l, err := net.Listen("tcp", net.JoinHostPort(hostAddr, "0"))
	require.NoError(t, err)
	defer l.Close()
	addr := net.JoinHostPort(hostAddr, PortStr(t, l.Addr().String()))

	// File() returns a dup of the listener's file descriptor as an *os.File, so
	// the original net.Listener still needs to be closed.
	lf, err := l.(*net.TCPListener).File()
	require.NoError(t, err)
	fd := &servicecfg.FileDescriptor{
		Type:    string(ty),
		Address: addr,
		File:    lf,
	}

	// If the file descriptor slice ends up being passed to a TeleportProcess
	// that successfully starts, listeners will either get "imported" and used
	// or discarded and closed, this is just an extra safety measure that closes
	// the listener at the end of the test anyway (the finalizer would do that
	// anyway, in principle).
	t.Cleanup(func() { require.NoError(t, fd.Close()) })

	*fds = append(*fds, fd)
	return addr
}

// NewListener creates a new TCP listener on 127.0.0.1:0, adds it to the
// FileDescriptor slice (with the specified type) and returns its actual local
// address as a string (for use in configuration). The idea is to subvert
// Teleport's file-descriptor injection mechanism (used to share ports between
// parent and child processes) to inject preconfigured listeners to Teleport
// instances under test. The ports are allocated and bound at runtime, so there
// should be no issues with port clashes on parallel tests.
//
// The resulting file descriptor is added to the `fds` slice, which can then be
// given to a teleport instance on startup in order to suppl
func NewListener(t *testing.T, ty service.ListenerType, fds *[]*servicecfg.FileDescriptor) string {
	return NewListenerOn(t, Loopback, ty, fds)
}

// DynamicServiceAddr collects listeners addresses and sockets descriptors allowing to create and network listeners
// and pass the file descriptors to teleport service.
// This is usefully when Teleport service is created from config file where a port is allocated by OS.
type DynamicServiceAddr struct {
	// Descriptors ia a list of descriptors associated with listens.
	Descriptors []*servicecfg.FileDescriptor
	// WebAddr is a Teleport Proxy Web Address.
	WebAddr string
	// TunnelAddr is a Teleport Proxy Tunnel Address.
	TunnelAddr string
	// AuthAddr is a Teleport Auth Address.
	AuthAddr string
	// TunnelAddr is a Teleport Proxy SSH Address
	ProxySSHAddr string
	// TunnelAddr is a Teleport node SSH Address.
	NodeSSHAddr string
}

// NewDynamicServiceAddr creates an instance of DynamicServiceAddr.
func NewDynamicServiceAddr(t *testing.T) *DynamicServiceAddr {
	var fds []*servicecfg.FileDescriptor
	webAddr := NewListener(t, service.ListenerProxyWeb, &fds)
	tunnelAddr := NewListener(t, service.ListenerProxyTunnel, &fds)
	authAddr := NewListener(t, service.ListenerAuth, &fds)
	proxySSHAddr := NewListener(t, service.ListenerProxySSH, &fds)
	nodeSSHAddr := NewListener(t, service.ListenerNodeSSH, &fds)

	return &DynamicServiceAddr{
		Descriptors:  fds,
		WebAddr:      webAddr,
		TunnelAddr:   tunnelAddr,
		AuthAddr:     authAddr,
		ProxySSHAddr: proxySSHAddr,
		NodeSSHAddr:  nodeSSHAddr,
	}
}
