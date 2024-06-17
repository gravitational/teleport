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

package gateway

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/gatewaytest"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestGatewayStart(t *testing.T) {
	hs := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))
	t.Cleanup(func() {
		hs.Close()
	})

	ca := gatewaytest.MustGenCACert(t)
	cert := gatewaytest.MustGenCertSignedWithCA(t, ca, tlsca.Identity{
		Username: "alice",
		Groups:   []string{"test-group"},
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: "foo",
			Protocol:    defaults.ProtocolPostgres,
			Username:    "alice",
		},
	})

	gateway, err := New(
		Config{
			TargetName:       "foo",
			TargetURI:        uri.NewClusterURI("bar").AppendDB("foo"),
			TargetUser:       "alice",
			Protocol:         defaults.ProtocolPostgres,
			Cert:             cert,
			Insecure:         true,
			WebProxyAddr:     hs.Listener.Addr().String(),
			TCPPortAllocator: &gatewaytest.MockTCPPortAllocator{},
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := gateway.Close(); err != nil {
			t.Logf("Ignoring error from gateway.Close() during cleanup, it appears the gateway was already closed. The error was: %s", err)
		}
	})
	gatewayAddress := net.JoinHostPort(gateway.LocalAddress(), gateway.LocalPort())

	require.NotEmpty(t, gateway.LocalPort())
	require.NotEqual(t, "0", gateway.LocalPort())

	serveErr := make(chan error)

	go func() {
		err := gateway.Serve()
		serveErr <- err
	}()

	gatewaytest.BlockUntilGatewayAcceptsConnections(t, gatewayAddress)

	err = gateway.Close()
	require.NoError(t, err)
	require.NoError(t, <-serveErr)
}

func TestNewWithLocalPortStartsListenerOnNewPortIfPortIsFree(t *testing.T) {
	tcpPortAllocator := gatewaytest.MockTCPPortAllocator{}
	oldGateway := createAndServeGateway(t, &tcpPortAllocator)

	newGateway, err := NewWithLocalPort(oldGateway, "12345")
	require.NoError(t, err)
	require.Equal(t, "12345", newGateway.LocalPort())
	require.Equal(t, oldGateway.URI(), newGateway.URI())

	// Verify that the gateway is accepting connections on the new listener.
	//
	// MockTCPPortAllocator.Listen returns a listener which occupies a random port but its Addr method
	// reports the port that was passed to Listen. In order to actually dial the gateway we need to
	// obtain the real address from the listener.
	newGatewayAddress := tcpPortAllocator.RecentListener().RealAddr().String()
	serveGatewayAndBlockUntilItAcceptsConnections(t, newGateway, newGatewayAddress)
}

func TestNewWithLocalPortReturnsErrorIfNewPortIsOccupied(t *testing.T) {
	tcpPortAllocator := gatewaytest.MockTCPPortAllocator{PortsInUse: []string{"12345"}}
	gateway := createAndServeGateway(t, &tcpPortAllocator)

	_, err := NewWithLocalPort(gateway, "12345")
	require.ErrorContains(t, err, "address already in use")
}

func TestNewWithLocalPortReturnsErrorIfNewPortEqualsOldPort(t *testing.T) {
	tcpPortAllocator := gatewaytest.MockTCPPortAllocator{}
	gateway := createAndServeGateway(t, &tcpPortAllocator)
	port := gateway.LocalPort()
	expectedErrMessage := fmt.Sprintf("port is already set to %s", port)

	_, err := NewWithLocalPort(gateway, port)
	require.True(t, trace.IsBadParameter(err), "Expected err to be a BadParameter error")
	require.ErrorContains(t, err, expectedErrMessage)
}

func createAndServeGateway(t *testing.T, tcpPortAllocator TCPPortAllocator) Gateway {
	gateway := createGateway(t, tcpPortAllocator)
	gatewayAddress := net.JoinHostPort(gateway.LocalAddress(), gateway.LocalPort())
	serveGatewayAndBlockUntilItAcceptsConnections(t, gateway, gatewayAddress)
	return gateway
}

func createGateway(t *testing.T, tcpPortAllocator TCPPortAllocator) Gateway {
	hs := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))
	t.Cleanup(func() {
		hs.Close()
	})

	ca := gatewaytest.MustGenCACert(t)
	cert := gatewaytest.MustGenCertSignedWithCA(t, ca, tlsca.Identity{
		Username: "alice",
		Groups:   []string{"test-group"},
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: "foo",
			Protocol:    defaults.ProtocolPostgres,
			Username:    "alice",
		},
	})

	gateway, err := New(
		Config{
			TargetName:       "foo",
			TargetURI:        uri.NewClusterURI("bar").AppendDB("foo"),
			TargetUser:       "alice",
			Protocol:         defaults.ProtocolPostgres,
			Cert:             cert,
			Insecure:         true,
			WebProxyAddr:     hs.Listener.Addr().String(),
			TCPPortAllocator: tcpPortAllocator,
		},
	)
	require.NoError(t, err)

	return gateway
}

// serveGateway starts a gateway and blocks until it accepts connections.
func serveGatewayAndBlockUntilItAcceptsConnections(t *testing.T, gateway Gateway, address string) {
	serveErr := make(chan error)
	go func() {
		err := gateway.Serve()
		serveErr <- err
	}()
	t.Cleanup(func() {
		if err := gateway.Close(); err != nil {
			t.Logf("Ignoring error from gateway.Close() during cleanup, it appears the gateway was already closed. The error was: %s", err)
		}
		require.NoError(t, <-serveErr, "Gateway %s returned error from Serve()", gateway.URI())
	})

	gatewaytest.BlockUntilGatewayAcceptsConnections(t, address)
}
