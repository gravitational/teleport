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

package gatewaytest

import (
	"crypto/tls"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
)

const timeout = time.Second * 5

// BlockUntilGatewayAcceptsConnections attempts to initiate a connection to the gateway on the given
// address. It will time out if that address doesn't respond in time.
func BlockUntilGatewayAcceptsConnections(t *testing.T, address string) {
	conn, err := net.DialTimeout("tcp", address, timeout)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	err = conn.SetReadDeadline(time.Now().Add(timeout))
	require.NoError(t, err)

	out := make([]byte, 1024)
	_, err = conn.Read(out)
	// Our "client" here is going to fail the handshake because it requests an application protocol
	// (typically teleport-<some db protocol>) that the target server (typically
	// httptest.NewTLSServer) doesn't support.
	//
	// So we just expect EOF here. In case of a timeout, this check will fail.
	require.ErrorIs(t, err, io.EOF, "expected EOF, got %v", err)

	err = conn.Close()
	require.NoError(t, err)
}

type MockTCPPortAllocator struct {
	PortsInUse    []string
	mockListeners []*MockListener
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

	if slices.Contains(m.PortsInUse, localPort) {
		return nil, trace.BadParameter("address already in use")
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", "localhost", "0"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mockListener := &MockListener{
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
	return m.mockListeners[len(m.mockListeners)-1]
}

// MockListener forwards almost all calls to the real listener. When asked about address, it will
// return the one pointing at the fake port.
//
// This lets us make calls to set the gateway port to a specific port without actually occupying
// those ports on the real system (which would lead to flaky tests otherwise).
type MockListener struct {
	realListener   net.Listener
	fakePort       string
	CloseCallCount int
}

func (m *MockListener) Accept() (net.Conn, error) {
	return m.realListener.Accept()
}

func (m *MockListener) Close() error {
	m.CloseCallCount++
	return m.realListener.Close()
}

func (m *MockListener) Addr() net.Addr {
	if m.fakePort == "0" {
		return m.realListener.Addr()
	}

	addr, err := net.ResolveTCPAddr("", fmt.Sprintf("%s:%s", "localhost", m.fakePort))

	if err != nil {
		panic(err)
	}

	return addr
}

func (m *MockListener) RealAddr() net.Addr {
	return m.realListener.Addr()
}

func MustGenCACert(t *testing.T) *tlsca.CertAuthority {
	t.Helper()
	caKey, caCert, err := tlsca.GenerateSelfSignedCA(pkix.Name{
		CommonName: "localhost",
	}, []string{"localhost"}, defaults.CATTL)
	require.NoError(t, err)

	ca, err := tlsca.FromKeys(caCert, caKey)
	require.NoError(t, err)
	return ca
}

func MustGenCertSignedWithCA(t *testing.T, ca *tlsca.CertAuthority, identity tlsca.Identity) tls.Certificate {
	t.Helper()
	clock := clockwork.NewRealClock()
	subj, err := identity.Subject()
	require.NoError(t, err)

	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	tlsCert, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: privateKey.Public(),
		Subject:   subj,
		NotAfter:  clock.Now().UTC().Add(time.Minute),
		DNSNames:  []string{"localhost", "*.localhost"},
	})
	require.NoError(t, err)

	keyPEM, err := keys.MarshalPrivateKey(privateKey)
	require.NoError(t, err)
	cert, err := tls.X509KeyPair(tlsCert, keyPEM)
	require.NoError(t, err)
	return cert
}
