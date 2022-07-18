/*
Copyright 2021 Gravitational, Inc.

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

package helpers

import (
	"fmt"
	"net"
	"strconv"
	"testing"

	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/require"
)

// ports contains tcp ports allocated for all integration tests.
// TODO: Replace all usage of `Ports` with FD-injected sockets as per
//       https://github.com/gravitational/teleport/pull/13346
var ports utils.PortList

func init() {
	// Allocate tcp ports for all integration tests. 5000 should be plenty.
	var err error
	ports, err = utils.GetFreeTCPPorts(5000, utils.PortStartingNumber)
	if err != nil {
		panic(fmt.Sprintf("failed to allocate tcp ports for tests: %v", err))
	}
}

func NewPortValue() int {
	return ports.PopInt()
}

func NewPortStr() string {
	return ports.Pop()
}

func NewPortSlice(n int) []int {
	return ports.PopIntSlice(n)
}

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

type InstanceListenerSetupFunc func(*testing.T, *[]service.FileDescriptor) *InstanceListeners

func StandardListenerSetup(t *testing.T, fds *[]service.FileDescriptor) *InstanceListeners {
	return &InstanceListeners{
		Web:           NewListener(t, service.ListenerProxyWeb, fds),
		SSH:           NewListener(t, service.ListenerNodeSSH, fds),
		Auth:          NewListener(t, service.ListenerAuth, fds),
		SSHProxy:      NewListener(t, service.ListenerProxySSH, fds),
		ReverseTunnel: NewListener(t, service.ListenerProxyTunnel, fds),
		MySQL:         NewListener(t, service.ListenerProxyMySQL, fds),
	}
}

// SingleProxyPortSetupOn creates a constructor function that will in turn generate an
// InstanceConfig that allows proxying of multiple protocols over a single port when
// invoked.
func SingleProxyPortSetupOn(addr string) func(*testing.T, *[]service.FileDescriptor) *InstanceListeners {
	return func(t *testing.T, fds *[]service.FileDescriptor) *InstanceListeners {
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
func SingleProxyPortSetup(t *testing.T, fds *[]service.FileDescriptor) *InstanceListeners {
	return SingleProxyPortSetupOn("127.0.0.1")(t, fds)
}

func WebReverseTunnelMuxPortSetup(t *testing.T, fds *[]service.FileDescriptor) *InstanceListeners {
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

func SeparatePostgresPortSetup(t *testing.T, fds *[]service.FileDescriptor) *InstanceListeners {
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

func SeparateMongoPortSetup(t *testing.T, fds *[]service.FileDescriptor) *InstanceListeners {
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

func SeparateMongoAndPostgresPortSetup(t *testing.T, fds *[]service.FileDescriptor) *InstanceListeners {
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

func PortStr(t *testing.T, addr string) string {
	t.Helper()

	_, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	return portStr
}

func Port(t *testing.T, addr string) int {
	t.Helper()

	portStr := PortStr(t, addr)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)

	return port
}

// NewListener creates a new TCP listener on `hostAddr`:0, adds it to the
// FileDescriptor slice (with the specified type) and returns its actual local
// address as a string (for use in configuration). The idea is to subvert
// Teleport's file-descriptor injection mechanism (used to share ports between
// parent and child processes) to inject preconfigured listeners to Teleport
// instances under test. The ports are allocated and bound at runtime, so there
// should be no issues with port clashes on parallel tests.
//
// The resulting file descriptor is added to the `fds` slice, which can then be
// given to a teleport instance on startup in order to suppl
func NewListenerOn(t *testing.T, hostAddr string, ty service.ListenerType, fds *[]service.FileDescriptor) string {
	t.Helper()

	l, err := net.Listen("tcp", hostAddr+":0")
	require.NoError(t, err)
	defer l.Close()
	addr := l.Addr().String()

	// File() returns a dup of the listener's file descriptor as an *os.File, so
	// the original net.Listener still needs to be closed.
	lf, err := l.(*net.TCPListener).File()
	require.NoError(t, err)

	t.Logf("Listener %s for %s", addr, ty)

	// If the file descriptor slice ends up being passed to a TeleportProcess
	// that successfully starts, listeners will either get "imported" and used
	// or discarded and closed, this is just an extra safety measure that closes
	// the listener at the end of the test anyway (the finalizer would do that
	// anyway, in principle).
	t.Cleanup(func() { lf.Close() })

	*fds = append(*fds, service.FileDescriptor{
		Type:    string(ty),
		Address: addr,
		File:    lf,
	})

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
func NewListener(t *testing.T, ty service.ListenerType, fds *[]service.FileDescriptor) string {
	return NewListenerOn(t, "127.0.0.1", ty, fds)
}
