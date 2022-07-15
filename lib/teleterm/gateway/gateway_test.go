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
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestCLICommandUsesCLICommandProvider(t *testing.T) {
	gateway := Gateway{
		cfg: &Config{
			TargetName:            "foo",
			TargetSubresourceName: "bar",
			Protocol:              defaults.ProtocolPostgres,
			CLICommandProvider:    mockCLICommandProvider{},
			TCPPortAllocator:      &mockTCPPortAllocator{},
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

	gateway, err := New(
		Config{
			TargetName:         "foo",
			TargetURI:          uri.NewClusterURI("bar").AppendDB("foo").String(),
			TargetUser:         "alice",
			Protocol:           defaults.ProtocolPostgres,
			CertPath:           "../../../fixtures/certs/proxy1.pem",
			KeyPath:            "../../../fixtures/certs/proxy1-key.pem",
			Insecure:           true,
			WebProxyAddr:       hs.Listener.Addr().String(),
			CLICommandProvider: mockCLICommandProvider{},
			TCPPortAllocator:   &mockTCPPortAllocator{},
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() { gateway.Close() })
	gatewayAddress := net.JoinHostPort(gateway.LocalAddress(), gateway.LocalPort())

	require.NotEmpty(t, gateway.LocalPort())
	require.NotEqual(t, "0", gateway.LocalPort())

	serveErr := make(chan error)

	go func() {
		err := gateway.Serve()
		serveErr <- err
	}()

	blockUntilGatewayAcceptsConnections(t, gatewayAddress)

	err = gateway.Close()
	require.NoError(t, err)
	require.NoError(t, <-serveErr)
}

func TestSetLocalPortAndRestartStartsListenerOnNewPortIfPortIsFree(t *testing.T) {
	tcpPortAllocator := mockTCPPortAllocator{}
	gateway := serveGateway(t, &tcpPortAllocator)
	originalCloseContext := gateway.closeContext

	err := gateway.SetLocalPortAndRestart("12345")
	require.NoError(t, err)

	require.Equal(t, "12345", gateway.LocalPort())

	// Verify that the gateway is accepting connections on the new listener.
	//
	// mockTCPPortAllocator.Listen returns a listener which occupies a random port but its Addr method
	// reports the port that was passed to Listen. In order to actually dial the gateway we need to
	// obtain the real address from the listener.
	newGatewayAddress := tcpPortAllocator.RecentListener().RealAddr().String()
	blockUntilGatewayAcceptsConnections(t, newGatewayAddress)

	// Verify that the old context was canceled.
	//
	// What we really want to test is if the old listener was closed. Unfortunately, we don't seem to
	// have a straightforward way to test this as at this point another process might have started
	// listening on that port.
	require.ErrorIs(t, originalCloseContext.Err(), context.Canceled,
		"The listener on the old port wasn't closed after starting a listener on the new port.")
}

func TestSetLocalPortAndRestartDoesntStopGatewayIfNewPortIsOccupied(t *testing.T) {
	tcpPortAllocator := mockTCPPortAllocator{portsInUse: []string{"12345"}}
	gateway := serveGateway(t, &tcpPortAllocator)
	originalPort := gateway.LocalPort()
	originalCloseContext := gateway.closeContext

	err := gateway.SetLocalPortAndRestart("12345")
	require.ErrorContains(t, err, "address already in use")
	require.Equal(t, originalPort, gateway.LocalPort())

	// Verify that we don't stop the gateway if we failed to start a listener on the specified port.
	require.NoError(t, originalCloseContext.Err(),
		"The listener on the current port was closed even though we failed to start a listener on the new port.")
}

func TestSetLocalPortAndRestartIsNoopIfNewPortEqualsOldPort(t *testing.T) {
	tcpPortAllocator := mockTCPPortAllocator{}
	gateway := serveGateway(t, &tcpPortAllocator)
	port := gateway.LocalPort()
	gatewayAddress := tcpPortAllocator.RecentListener().RealAddr().String()
	originalCloseContext := gateway.closeContext

	err := gateway.SetLocalPortAndRestart(port)
	require.NoError(t, err)

	// Verify that we don't stop the gateway if the new port is equal to the old port.
	require.NoError(t, originalCloseContext.Err(),
		"The listener on the current port was closed even though the new port is equal to the old port.")
	blockUntilGatewayAcceptsConnections(t, gatewayAddress)
}

type mockCLICommandProvider struct{}

func (m mockCLICommandProvider) GetCommand(gateway *Gateway) (string, error) {
	command := fmt.Sprintf("%s/%s", gateway.TargetName(), gateway.TargetSubresourceName())
	return command, nil
}

type mockTCPPortAllocator struct {
	portsInUse    []string
	mockListeners []mockListener
}

// Listen accepts localPort as an argument but creates a listener on a random port. This lets us
// test code that attempt to set the port number to a specific value without risking that the actual
// port on the device running the tests is occupied.
//
// Listen returns a mock listener which forwards all methods to the real listener on the random port
// but its Addr function returns the port that was given as an argument to Listen.
func (m *mockTCPPortAllocator) Listen(localAddress, localPort string) (net.Listener, error) {
	if apiutils.SliceContainsStr(m.portsInUse, localPort) {
		return nil, trace.BadParameter("address already in use")
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", "localhost", "0"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mockListener := mockListener{
		realListener: listener,
		fakePort:     localPort,
	}

	m.mockListeners = append(m.mockListeners, mockListener)

	return mockListener, nil
}

func (m *mockTCPPortAllocator) RecentListener() *mockListener {
	if len(m.mockListeners) == 0 {
		return nil
	}
	return &m.mockListeners[len(m.mockListeners)-1]
}

// mockListener forwards almost all calls to the real listener. When asked about address, it will
// return the one pointing at the fake port.
//
// This lets us make calls to set the gateway port to a specific port without actually occupying
// those ports on the real system (which would lead to flaky tests otherwise).
type mockListener struct {
	realListener net.Listener
	fakePort     string
}

func (m mockListener) Accept() (net.Conn, error) {
	return m.realListener.Accept()
}

func (m mockListener) Close() error {
	return m.realListener.Close()
}

func (m mockListener) Addr() net.Addr {
	if m.fakePort == "0" {
		return m.realListener.Addr()
	}

	addr, err := net.ResolveTCPAddr("", fmt.Sprintf("%s:%s", "localhost", m.fakePort))

	if err != nil {
		panic(err)
	}

	return addr
}

func (m mockListener) RealAddr() net.Addr {
	return m.realListener.Addr()
}

// serveGateway starts a gateway and blocks until it accepts connections.
func serveGateway(t *testing.T, tcpPortAllocator TCPPortAllocator) *Gateway {
	hs := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))
	t.Cleanup(func() {
		hs.Close()
	})

	gateway, err := New(
		Config{
			TargetName:         "foo",
			TargetURI:          uri.NewClusterURI("bar").AppendDB("foo").String(),
			TargetUser:         "alice",
			Protocol:           defaults.ProtocolPostgres,
			CertPath:           "../../../fixtures/certs/proxy1.pem",
			KeyPath:            "../../../fixtures/certs/proxy1-key.pem",
			Insecure:           true,
			WebProxyAddr:       hs.Listener.Addr().String(),
			CLICommandProvider: mockCLICommandProvider{},
			TCPPortAllocator:   tcpPortAllocator,
		},
	)
	require.NoError(t, err)

	serveErr := make(chan error)
	go func() {
		err := gateway.Serve()
		serveErr <- err
	}()
	t.Cleanup(func() {
		gateway.Close()
		require.NoError(t, <-serveErr, "Gateway %s returned error from Serve()", gateway.URI())
	})

	gatewayAddress := net.JoinHostPort(gateway.LocalAddress(), gateway.LocalPort())
	blockUntilGatewayAcceptsConnections(t, gatewayAddress)

	return gateway
}

func blockUntilGatewayAcceptsConnections(t *testing.T, address string) {
	conn, err := net.DialTimeout("tcp", address, time.Second*1)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	err = conn.SetReadDeadline(time.Now().Add(time.Second))
	require.NoError(t, err)

	out := make([]byte, 1024)
	_, err = conn.Read(out)
	// Our "client" here is going to fail the handshake because it requests an application protocol
	// (typically teleport-<some db protocol>) that the target server (typically
	// httptest.NewTLSServer) doesn't support.
	//
	// So we just expect EOF here. In case of a timeout, this check will fail.
	require.True(t, trace.IsEOF(err), "expected EOF, got %v", err)

	err = conn.Close()
	require.NoError(t, err)
}
