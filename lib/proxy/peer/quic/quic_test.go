// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package quic

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	peerdial "github.com/gravitational/teleport/lib/proxy/peer/dial"
	"github.com/gravitational/teleport/lib/proxy/peer/internal"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestCertificateVerification(t *testing.T) {
	t.Parallel()

	clientTransport := newTransport(t)

	correctCA := newCA(t)
	wrongCA := newCA(t)

	correctProxyCert := newCert(t, correctCA, tlsca.Identity{
		Username: "correctproxy.testcluster",
		Groups:   []string{string(types.RoleProxy)},
	})
	clientProxyCert := newCert(t, correctCA, tlsca.Identity{
		Username: "clientproxy.testcluster",
		Groups:   []string{string(types.RoleProxy)},
	})

	correctCAPool := x509.NewCertPool()
	correctCAPool.AddCert(correctCA.Cert)

	type testCase struct {
		name string

		serverCert tls.Certificate
		clientCAs  *x509.CertPool

		clientCert tls.Certificate
		rootCAs    *x509.CertPool

		check require.ErrorAssertionFunc
	}

	testCases := []testCase{
		{
			name:       "success",
			serverCert: correctProxyCert,
			clientCAs:  correctCAPool,
			clientCert: clientProxyCert,
			rootCAs:    correctCAPool,
			check:      require.NoError,
		},
		{
			name: "wrong proxy",
			serverCert: newCert(t, correctCA, tlsca.Identity{
				Username: "wrongproxy.testcluster",
				Groups:   []string{string(types.RoleProxy)},
			}),
			clientCAs:  correctCAPool,
			clientCert: clientProxyCert,
			rootCAs:    correctCAPool,
			check: func(t require.TestingT, err error, msgAndArgs ...interface{}) {
				require.Error(t, err, msgAndArgs...)
				require.ErrorIs(t, err, internal.WrongProxyError{}, msgAndArgs...)
			},
		},
		{
			name: "server not a proxy",
			serverCert: newCert(t, correctCA, tlsca.Identity{
				Username: "notproxy.testcluster",
				Groups:   []string{string(types.RoleNode)},
			}),
			clientCAs:  correctCAPool,
			clientCert: clientProxyCert,
			rootCAs:    correctCAPool,
			check: func(t require.TestingT, err error, msgAndArgs ...interface{}) {
				require.Error(t, err, msgAndArgs...)
				require.NotErrorIs(t, err, internal.WrongProxyError{}, msgAndArgs...)
				require.ErrorAs(t, err, new(*trace.AccessDeniedError), msgAndArgs...)
			},
		},
		{
			name: "bad server certificate",
			serverCert: newCert(t, wrongCA, tlsca.Identity{
				Username: "correctproxy.testcluster",
				Groups:   []string{string(types.RoleProxy)},
			}),
			clientCAs:  correctCAPool,
			clientCert: clientProxyCert,
			rootCAs:    correctCAPool,
			check: func(t require.TestingT, err error, msgAndArgs ...interface{}) {
				require.Error(t, err, msgAndArgs...)
				require.ErrorAs(t, err, new(*tls.CertificateVerificationError))
			},
		},
		{
			name:       "unknown client",
			serverCert: correctProxyCert,
			clientCAs:  correctCAPool,
			clientCert: newCert(t, wrongCA, tlsca.Identity{
				Username: "clientproxy.testcluster",
				Groups:   []string{string(types.RoleProxy)},
			}),
			rootCAs: correctCAPool,
			check: func(t require.TestingT, err error, msgAndArgs ...interface{}) {
				require.Error(t, err, msgAndArgs...)
				var transportError *quic.TransportError
				require.ErrorAs(t, err, &transportError, msgAndArgs...)
				require.True(t, transportError.Remote, msgAndArgs...)
				// RFC 8446 (TLS 1.3) section 6 (Alert Protocol)
				const alertUnknownCA = 48
				// quic-go surfaces TLS alerts as error code 0x100 plus the alert code
				const quicAlertUnknownCA quic.TransportErrorCode = alertUnknownCA + 0x100
				require.Equal(t, quicAlertUnknownCA, transportError.ErrorCode, msgAndArgs...)
			},
		},
		{
			name:       "client not a proxy",
			serverCert: correctProxyCert,
			clientCAs:  correctCAPool,
			clientCert: newCert(t, correctCA, tlsca.Identity{
				Username: "notproxy.testcluster",
				Groups:   []string{string(types.RoleNode)},
			}),
			rootCAs: correctCAPool,
			check: func(t require.TestingT, err error, msgAndArgs ...interface{}) {
				require.Error(t, err, msgAndArgs...)
				var transportError *quic.TransportError
				require.ErrorAs(t, err, &transportError, msgAndArgs...)
				require.True(t, transportError.Remote, msgAndArgs...)
				// RFC 8446 (TLS 1.3) section 6 (Alert Protocol)
				const alertBadCertificate = 42
				// quic-go surfaces TLS alerts as error code 0x100 plus the alert code
				const quicAlertBadCertificate quic.TransportErrorCode = alertBadCertificate + 0x100
				require.Equal(t, quicAlertBadCertificate, transportError.ErrorCode, msgAndArgs...)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			serverTransport := newTransport(t)
			server, err := NewServer(ServerConfig{
				Dialer: noDialer,
				GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
					return &tc.serverCert, nil
				},
				GetClientCAs: func(*tls.ClientHelloInfo) (*x509.CertPool, error) {
					return tc.clientCAs, nil
				},
			})
			require.NoError(t, err)
			go server.Serve(serverTransport)
			t.Cleanup(func() { _ = server.Close() })

			client, err := NewClientConn(ClientConnConfig{
				Log:         slog.Default(),
				PeerID:      "correctproxy",
				PeerAddr:    serverTransport.Conn.LocalAddr().String(),
				ClusterName: "testcluster",
				GetTLSCertificate: func() (*tls.Certificate, error) {
					return &tc.clientCert, nil
				},
				GetTLSRoots: func() (*x509.CertPool, error) {
					return tc.rootCAs, nil
				},
				Transport: clientTransport,
			})
			require.NoError(t, err)
			t.Cleanup(func() { _ = client.Close() })

			err = client.Ping(context.Background())
			tc.check(t, err)
		})
	}
}

func TestBasicFunctionality(t *testing.T) {
	t.Skip("disabled until quic-go/quic-go#4303 is fixed (data race)")

	t.Parallel()

	hostCA := newCA(t)
	hostCAPool := x509.NewCertPool()
	hostCAPool.AddCert(hostCA.Cert)

	serverHostID := uuid.NewString()
	serverCert := newCert(t, hostCA, tlsca.Identity{
		Username: serverHostID + ".testcluster",
		Groups:   []string{string(types.RoleProxy)},
	})
	clientHostID := uuid.NewString()
	clientCert := newCert(t, hostCA, tlsca.Identity{
		Username: clientHostID + ".testcluster",
		Groups:   []string{string(types.RoleProxy)},
	})

	serverTransport := newTransport(t)
	server, err := NewServer(ServerConfig{
		Dialer: noDialer,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return &serverCert, nil
		},
		GetClientCAs: func(*tls.ClientHelloInfo) (*x509.CertPool, error) {
			return hostCAPool, nil
		},
	})
	require.NoError(t, err)
	go server.Serve(serverTransport)
	t.Cleanup(func() { _ = server.Close() })

	client, err := NewClientConn(ClientConnConfig{
		Log:         slog.Default(),
		PeerID:      serverHostID,
		PeerAddr:    serverTransport.Conn.LocalAddr().String(),
		ClusterName: "testcluster",
		GetTLSCertificate: func() (*tls.Certificate, error) {
			return &clientCert, nil
		},
		GetTLSRoots: func() (*x509.CertPool, error) {
			return hostCAPool, nil
		},
		Transport: newTransport(t),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	t.Run("ping", func(t *testing.T) {
		require.NoError(t, client.Ping(context.Background()))
	})

	clientDialRandomNode := func() (net.Conn, error) {
		return client.Dial(
			uuid.NewString()+".testcluster",
			&utils.NetAddr{
				AddrNetwork: "tcp",
				Addr:        "1.2.3.4:56",
			}, &utils.NetAddr{
				AddrNetwork: "tcp",
				Addr:        "7.8.9.0:12",
			},
			types.NodeTunnel,
		)
	}

	t.Run("error translation", func(t *testing.T) {
		conn, err := clientDialRandomNode()
		if !assert.Error(t, err) {
			_ = conn.Close()
			t.FailNow()
		}
		require.ErrorAs(t, err, new(*trace.NotImplementedError))

		server.dialer = peerDialerFunc(func(clusterName string, request peerdial.DialParams) (net.Conn, error) {
			return nil, trace.NotFound("nope")
		})
		conn, err = clientDialRandomNode()
		if !assert.Error(t, err) {
			_ = conn.Close()
			t.FailNow()
		}
		require.ErrorAs(t, err, new(*trace.NotFoundError))
	})

	t.Run("successful dial", func(t *testing.T) {
		pipeClosed := make(chan struct{})
		server.dialer = peerDialerFunc(func(clusterName string, request peerdial.DialParams) (net.Conn, error) {
			if clusterName != "echo" {
				return nil, trace.BadParameter("expected echo cluster")
			}
			if request.ServerID != "echo.echo" {
				return nil, trace.NotFound("only echo.echo is here")
			}
			if request.ConnType != "echo" {
				return nil, trace.CompareFailed("the only conntype is echo")
			}
			p1, p2 := net.Pipe()
			go func() {
				defer close(pipeClosed)
				defer p2.Close()
				// send all the data received from the pipe back to the pipe, until
				// the other end of the pipe is closed
				io.Copy(p2, p2)
			}()
			return p1, nil
		})

		conn, err := client.Dial(
			"echo.echo",
			&utils.NetAddr{
				AddrNetwork: "tcp",
				Addr:        "1.2.3.4:56",
			}, &utils.NetAddr{
				AddrNetwork: "tcp",
				Addr:        "7.8.9.0:12",
			},
			"echo",
		)
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })

		n, err := conn.Write([]byte("abcd"))
		require.NoError(t, err)
		require.Equal(t, 4, n)

		buf := make([]byte, 4)
		n, err = io.ReadFull(conn, buf)
		require.NoError(t, err)
		require.Equal(t, 4, n)
		require.Equal(t, "abcd", string(buf))

		require.NoError(t, conn.Close())
		t.Log("closed")
		<-pipeClosed
	})
}

func newTransport(t *testing.T) *quic.Transport {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)

	transport := &quic.Transport{
		Conn: pc,
	}
	t.Cleanup(func() { _ = transport.Close() })

	return transport
}

func newCA(t *testing.T) *tlsca.CertAuthority {
	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	cert, err := tlsca.GenerateSelfSignedCAWithSigner(
		key, pkix.Name{}, nil, defaults.CATTL,
	)
	require.NoError(t, err)

	ca, err := tlsca.FromCertAndSigner(cert, key)
	require.NoError(t, err)

	return ca
}

func newCert(t *testing.T, ca *tlsca.CertAuthority, ident tlsca.Identity) tls.Certificate {
	subj, err := ident.Subject()
	require.NoError(t, err)

	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	keyPEM, err := keys.MarshalPrivateKey(key)
	require.NoError(t, err)

	clock := clockwork.NewRealClock()

	request := tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: key.Public(),
		Subject:   subj,
		NotAfter:  clock.Now().Add(time.Hour),
		DNSNames:  []string{apiutils.EncodeClusterName("testcluster")},
	}
	certPEM, err := ca.GenerateCertificate(request)
	require.NoError(t, err)

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	return cert
}

// peerDialerFunc is a function that implements [peerdial.Dialer].
type peerDialerFunc func(clusterName string, request peerdial.DialParams) (net.Conn, error)

// Dial implements [peerdial.Dialer].
func (f peerDialerFunc) Dial(clusterName string, request peerdial.DialParams) (net.Conn, error) {
	return f(clusterName, request)
}

var noDialer peerDialerFunc = func(clusterName string, request peerdial.DialParams) (net.Conn, error) {
	return nil, trace.NotImplemented("no dialer")
}
