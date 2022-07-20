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

package gatewaytest

import (
	"fmt"
	"net"
	"testing"
	"time"

	apiutils "github.com/gravitational/teleport/api/utils"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// BlockUntilGatewayAcceptsConnections attempts to initiate a connection to the gateway on the given
// address. It will time out if that address doesn't respond after 1 second.
func BlockUntilGatewayAcceptsConnections(t *testing.T, address string) {
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

type MockTCPPortAllocator struct {
	PortsInUse    []string
	mockListeners []MockListener
	CallCount     int
}

// Listen accepts localPort as an argument but creates a listener on a random port. This lets us
// test code that attempt to set the port number to a specific value without risking that the actual
// port on the device running the tests is occupied.
//
// Listen returns a mock listener which forwards all methods to the real listener on the random port
// but its Addr function returns the port that was given as an argument to Listen.
func (m *MockTCPPortAllocator) Listen(localAddress, localPort string) (net.Listener, error) {
	m.CallCount++

	if apiutils.SliceContainsStr(m.PortsInUse, localPort) {
		return nil, trace.BadParameter("address already in use")
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", "localhost", "0"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mockListener := MockListener{
		realListener: listener,
		fakePort:     localPort,
	}

	m.mockListeners = append(m.mockListeners, mockListener)

	return mockListener, nil
}

func (m *MockTCPPortAllocator) RecentListener() *MockListener {
	if len(m.mockListeners) == 0 {
		return nil
	}
	return &m.mockListeners[len(m.mockListeners)-1]
}

// MockListener forwards almost all calls to the real listener. When asked about address, it will
// return the one pointing at the fake port.
//
// This lets us make calls to set the gateway port to a specific port without actually occupying
// those ports on the real system (which would lead to flaky tests otherwise).
type MockListener struct {
	realListener net.Listener
	fakePort     string
}

func (m MockListener) Accept() (net.Conn, error) {
	return m.realListener.Accept()
}

func (m MockListener) Close() error {
	return m.realListener.Close()
}

func (m MockListener) Addr() net.Addr {
	if m.fakePort == "0" {
		return m.realListener.Addr()
	}

	addr, err := net.ResolveTCPAddr("", fmt.Sprintf("%s:%s", "localhost", m.fakePort))

	if err != nil {
		panic(err)
	}

	return addr
}

func (m MockListener) RealAddr() net.Addr {
	return m.realListener.Addr()
}
