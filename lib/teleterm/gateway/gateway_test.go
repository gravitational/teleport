// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

func TestCLICommandUsesCLICommandProvider(t *testing.T) {
	gateway := Gateway{
		cfg: &Config{
			TargetName:            "foo",
			TargetSubresourceName: "bar",
			Protocol:              defaults.ProtocolPostgres,
			CLICommandProvider:    mockCLICommandProvider{},
			TCPPortAllocator:      &gatewaytest.MockTCPPortAllocator{},
		},
	}

	command, err := gateway.CLICommand()
	require.NoError(t, err)

	require.Equal(t, "foo/bar", command)
}

func TestGatewayStart(t *testing.T) {
	hs := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))
	t.Cleanup(func() {
		hs.Close()
	})

	keyPairPaths := gatewaytest.MustGenAndSaveCert(t, tlsca.Identity{
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
			TargetName:         "foo",
			TargetURI:          uri.NewClusterURI("bar").AppendDB("foo").String(),
			TargetUser:         "alice",
			Protocol:           defaults.ProtocolPostgres,
			CertPath:           keyPairPaths.CertPath,
			KeyPath:            keyPairPaths.KeyPath,
			Insecure:           true,
			WebProxyAddr:       hs.Listener.Addr().String(),
			CLICommandProvider: mockCLICommandProvider{},
			TCPPortAllocator:   &gatewaytest.MockTCPPortAllocator{},
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

type mockCLICommandProvider struct{}

func (m mockCLICommandProvider) GetCommand(gateway *Gateway) (string, error) {
	command := fmt.Sprintf("%s/%s", gateway.TargetName(), gateway.TargetSubresourceName())
	return command, nil
}

func createAndServeGateway(t *testing.T, tcpPortAllocator TCPPortAllocator) *Gateway {
	gateway := createGateway(t, tcpPortAllocator)
	gatewayAddress := net.JoinHostPort(gateway.LocalAddress(), gateway.LocalPort())
	serveGatewayAndBlockUntilItAcceptsConnections(t, gateway, gatewayAddress)
	return gateway
}

func createGateway(t *testing.T, tcpPortAllocator TCPPortAllocator) *Gateway {
	hs := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))
	t.Cleanup(func() {
		hs.Close()
	})

	keyPairPaths := gatewaytest.MustGenAndSaveCert(t, tlsca.Identity{
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
			TargetName:         "foo",
			TargetURI:          uri.NewClusterURI("bar").AppendDB("foo").String(),
			TargetUser:         "alice",
			Protocol:           defaults.ProtocolPostgres,
			CertPath:           keyPairPaths.CertPath,
			KeyPath:            keyPairPaths.KeyPath,
			Insecure:           true,
			WebProxyAddr:       hs.Listener.Addr().String(),
			CLICommandProvider: mockCLICommandProvider{},
			TCPPortAllocator:   tcpPortAllocator,
		},
	)
	require.NoError(t, err)

	return gateway
}

// serveGateway starts a gateway and blocks until it accepts connections.
func serveGatewayAndBlockUntilItAcceptsConnections(t *testing.T, gateway *Gateway, address string) {
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
