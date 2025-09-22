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

package alpnproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/pingconn"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/srv/db/dbutils"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

// TestProxySSHHandler tests the ALPN routing. Connection with ALPN 'teleport-proxy-ssh' value should
// be forwarded and handled by custom ProtocolProxySSH ALPN protocol handler.
func TestProxySSHHandler(t *testing.T) {
	t.Parallel()
	const (
		handlerRespMessage = "response from proxy ssh handler"
	)
	suite := NewSuite(t)

	suite.router.Add(HandlerDecs{
		MatchFunc:  MatchByProtocol(common.ProtocolProxySSH),
		ForwardTLS: false,
		Handler: func(ctx context.Context, conn net.Conn) error {
			defer conn.Close()
			_, err := fmt.Fprint(conn, handlerRespMessage)
			require.NoError(t, err)
			return nil
		},
	})
	suite.Start(t)

	conn, err := tls.Dial("tcp", suite.GetServerAddress(), &tls.Config{
		NextProtos: []string{string(common.ProtocolProxySSH)},
		ServerName: "localhost",
		RootCAs:    suite.GetCertPool(),
	})
	require.NoError(t, err)

	mustReadFromConnection(t, conn, handlerRespMessage)
	mustCloseConnection(t, conn)
}

// TestProxyKubeHandler tests the SNI routing. HTTP request with 'kube' SNI
// prefix should be forwarded to Kube Proxy handler.
func TestProxyKubeHandler(t *testing.T) {
	t.Parallel()
	const (
		kubernetesHandlerResponse = "kubernetes handler response"
		kubeSNI                   = constants.KubeTeleportProxyALPNPrefix + "localhost"
	)
	suite := NewSuite(t)

	kubeCert := mustGenCertSignedWithCA(t, suite.ca)
	suite.router.AddKubeHandler(func(ctx context.Context, conn net.Conn) error {
		defer conn.Close()
		tlsConn := tls.Server(conn, &tls.Config{
			Certificates: []tls.Certificate{
				kubeCert,
			},
		})
		err := tlsConn.HandshakeContext(ctx)
		require.NoError(t, err)
		_, err = fmt.Fprint(tlsConn, kubernetesHandlerResponse)
		require.NoError(t, err)
		return nil
	})
	suite.Start(t)

	conn, err := tls.Dial("tcp", suite.GetServerAddress(), &tls.Config{
		NextProtos: []string{string(common.ProtocolHTTP2)},
		ServerName: kubeSNI,
		RootCAs:    suite.GetCertPool(),
	})
	require.NoError(t, err)

	mustReadFromConnection(t, conn, kubernetesHandlerResponse)
	mustCloseConnection(t, conn)
}

// TestProxyTLSDatabaseHandler tests TLS Database connection routing. Connection with cert issued
// by with `RouteToDatabase` identity filed  should be routed to Proxy database handler // in order to
// support legacy TLS multiplexing behavior.
// Connection established by Local ALPN proxy where TLS ALPN value is set to ProtocolMongoDB should be TLS terminated with
// underlying TLS database protocol and forwarded to database TLS handler.
func TestProxyTLSDatabaseHandler(t *testing.T) {
	t.Parallel()
	const (
		databaseHandleResponse = "database handler response"
	)

	suite := NewSuite(t)
	clientCert := mustGenCertSignedWithCA(t, suite.ca,
		withIdentity(tlsca.Identity{
			Username: "test-user",
			Groups:   []string{"test-group"},
			RouteToDatabase: tlsca.RouteToDatabase{
				ServiceName: "mongo-test-database",
			},
		}),
	)

	suite.router.AddDBTLSHandler(func(ctx context.Context, conn net.Conn) error {
		defer conn.Close()
		_, err := fmt.Fprint(conn, databaseHandleResponse)
		require.NoError(t, err)
		return nil
	})

	// Add HTTP handler to support empty values of NextProtos during DB connection.
	// Default handler needs to be returned because Databased routing is evaluated
	// after TLS termination.
	suite.router.Add(HandlerDecs{
		MatchFunc: MatchByProtocol(common.ProtocolHTTP),
		Handler: func(ctx context.Context, conn net.Conn) error {
			defer conn.Close()
			_, err := fmt.Fprint(conn, string(common.ProtocolHTTP))
			require.NoError(t, err)
			return nil
		},
	})
	suite.Start(t)

	t.Run("legacy tls database connection", func(t *testing.T) {
		conn, err := tls.Dial("tcp", suite.GetServerAddress(), &tls.Config{
			Certificates: []tls.Certificate{
				clientCert,
			},
			RootCAs:    suite.GetCertPool(),
			ServerName: "localhost",
		})
		require.NoError(t, err)

		mustReadFromConnection(t, conn, databaseHandleResponse)
		mustCloseConnection(t, conn)
	})

	t.Run("tls database connection wrapped with ALPN value", func(t *testing.T) {
		conn, err := tls.Dial("tcp", suite.GetServerAddress(), &tls.Config{
			NextProtos: []string{string(common.ProtocolMongoDB)},
			RootCAs:    suite.GetCertPool(),
			ServerName: "localhost",
		})
		require.NoError(t, err)

		tlsConn := tls.Client(conn, &tls.Config{
			Certificates: []tls.Certificate{
				clientCert,
			},
			RootCAs:    suite.GetCertPool(),
			ServerName: "localhost",
		})

		mustReadFromConnection(t, tlsConn, databaseHandleResponse)
		mustCloseConnection(t, conn)
	})

	t.Run("tls database connection wrapped with ALPN ping value", func(t *testing.T) {
		baseConn, err := tls.Dial("tcp", suite.GetServerAddress(), &tls.Config{
			NextProtos: []string{string(common.ProtocolWithPing(common.ProtocolMongoDB))},
			RootCAs:    suite.GetCertPool(),
			ServerName: "localhost",
		})
		require.NoError(t, err)

		conn := pingconn.NewTLS(baseConn)
		tlsConn := tls.Client(conn, &tls.Config{
			Certificates: []tls.Certificate{
				clientCert,
			},
			RootCAs:    suite.GetCertPool(),
			ServerName: "localhost",
		})

		mustReadFromConnection(t, tlsConn, databaseHandleResponse)
		mustCloseConnection(t, conn)
	})
}

// TestProxyRouteToDatabase tests db connection with protocol registered without any handler.
// ALPN router leverages empty handler to route the connection to DBHandler
// based on TLS RouteToDatabase identity entry.
func TestProxyRouteToDatabase(t *testing.T) {
	t.Parallel()
	const (
		databaseHandleResponse = "database handler response"
	)

	suite := NewSuite(t)
	clientCert := mustGenCertSignedWithCA(t, suite.ca,
		withIdentity(tlsca.Identity{
			Username: "test-user",
			Groups:   []string{"test-group"},
			RouteToDatabase: tlsca.RouteToDatabase{
				ServiceName: "mongo-test-database",
			},
		}),
	)

	suite.router.AddDBTLSHandler(func(ctx context.Context, conn net.Conn) error {
		defer conn.Close()
		_, err := fmt.Fprint(conn, databaseHandleResponse)
		require.NoError(t, err)
		return nil
	})
	suite.router.Add(HandlerDecs{
		MatchFunc: MatchByProtocol(common.ProtocolReverseTunnel),
	})

	suite.Start(t)

	t.Run("dial with user certs with RouteToDatabase info", func(t *testing.T) {
		conn, err := tls.Dial("tcp", suite.GetServerAddress(), &tls.Config{
			NextProtos: []string{string(common.ProtocolReverseTunnel)},
			RootCAs:    suite.GetCertPool(),
			ServerName: "localhost",
			Certificates: []tls.Certificate{
				clientCert,
			},
		})
		require.NoError(t, err)
		mustReadFromConnection(t, conn, databaseHandleResponse)
		mustCloseConnection(t, conn)
	})

	t.Run("dial with no user certs", func(t *testing.T) {
		conn, err := tls.Dial("tcp", suite.GetServerAddress(), &tls.Config{
			NextProtos: []string{string(common.ProtocolReverseTunnel)},
			RootCAs:    suite.GetCertPool(),
			ServerName: "localhost",
		})
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, conn.Close())
		})
	})
}

// TestLocalProxyPostgresProtocol tests Proxy Postgres connection  forwarded by LocalProxy.
// Client connects to LocalProxy with raw connection where downstream Proxy connection is upgraded to TLS with
// ALPN value set to ProtocolPostgres.
func TestLocalProxyPostgresProtocol(t *testing.T) {
	t.Parallel()
	const (
		databaseHandleResponse = "database handler response"
	)

	suite := NewSuite(t)
	suite.router.Add(HandlerDecs{
		MatchFunc: MatchByProtocol(common.ProtocolPostgres),
		Handler: func(ctx context.Context, conn net.Conn) error {
			defer conn.Close()
			_, err := fmt.Fprint(conn, databaseHandleResponse)
			require.NoError(t, err)
			return nil
		},
	})
	suite.Start(t)

	localProxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	localProxyConfig := LocalProxyConfig{
		RemoteProxyAddr:    suite.GetServerAddress(),
		Protocols:          []common.Protocol{common.ProtocolPostgres},
		Listener:           localProxyListener,
		SNI:                "localhost",
		ParentContext:      context.Background(),
		InsecureSkipVerify: true,
	}

	mustStartLocalProxy(t, localProxyConfig)

	conn, err := net.Dial("tcp", localProxyListener.Addr().String())
	require.NoError(t, err)

	mustReadFromConnection(t, conn, databaseHandleResponse)
	mustCloseConnection(t, conn)
}

// TestProxyHTTPConnection tests connection to http server where http proxy handler should forward and inject incoming
// connection by ListenerMuxWrapper to http.Server handler.
func TestProxyHTTPConnection(t *testing.T) {
	t.Parallel()

	suite := NewSuite(t)
	l := mustCreateLocalListener(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {})

	lw := NewMuxListenerWrapper(l, suite.serverListener)

	mustStartHTTPServer(lw)

	suite.router = NewRouter()
	suite.router.Add(HandlerDecs{
		MatchFunc: MatchByProtocol(common.ProtocolHTTP2, common.ProtocolHTTP),
		Handler:   lw.HandleConnection,
	})
	suite.Start(t)

	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				ServerName: "localhost",
				RootCAs:    suite.GetCertPool(),
			},
		},
	}

	mustSuccessfullyCallHTTPSServer(t, suite.GetServerAddress(), client)
}

// TestProxyMakeConnectionHandler creates a ConnectionHandler from the ALPN
// proxy server, and verifies ALPN protocol is properly handled through the
// ConnectionHandler.
func TestProxyMakeConnectionHandler(t *testing.T) {
	t.Parallel()

	suite := NewSuite(t)

	// Create a HTTP server and register the listener to ALPN server.
	lw := NewMuxListenerWrapper(nil, suite.serverListener)
	mustStartHTTPServer(lw)

	suite.router = NewRouter()
	suite.router.Add(HandlerDecs{
		MatchFunc: MatchByProtocol(common.ProtocolHTTP),
		Handler:   lw.HandleConnection,
	})

	svr := suite.CreateProxyServer(t)
	customCA := mustGenSelfSignedCert(t)

	// Create a ConnectionHandler from the proxy server.
	alpnConnHandler := svr.MakeConnectionHandler(
		&tls.Config{
			NextProtos: []string{string(common.ProtocolHTTP)},
			Certificates: []tls.Certificate{
				mustGenCertSignedWithCA(t, customCA),
			},
		},
		common.ConnHandlerSource(t.Name()),
	)

	// Prepare net.Conn to be used for the created alpnConnHandler.
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	// Let alpnConnHandler serve the connection in a separate go routine.
	handlerCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	go func() {
		defer cancel()
		alpnConnHandler(handlerCtx, serverConn)
	}()

	// Send client request.
	req, err := http.NewRequest("GET", "https://localhost/test", nil)
	require.NoError(t, err)

	// Use the customCA to validate default TLS config override.
	pool := x509.NewCertPool()
	pool.AddCert(customCA.Cert)

	clientTLSConn := tls.Client(clientConn, &tls.Config{
		NextProtos: []string{string(common.ProtocolHTTP)},
		RootCAs:    pool,
		ServerName: "localhost",
	})
	defer clientTLSConn.Close()

	require.NoError(t, clientTLSConn.HandshakeContext(context.Background()))
	checkGaugeValue(t, 1, proxyActiveConnections.WithLabelValues(string(common.ProtocolHTTP), t.Name()))
	require.Equal(t, string(common.ProtocolHTTP), clientTLSConn.ConnectionState().NegotiatedProtocol)
	require.NoError(t, req.Write(clientTLSConn))

	resp, err := http.ReadResponse(bufio.NewReader(clientTLSConn), req)
	require.NoError(t, err)

	// Always drain/close the body.
	io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Wait until handler is done. And verify context is canceled, NOT deadline exceeded.
	<-handlerCtx.Done()
	require.ErrorIs(t, handlerCtx.Err(), context.Canceled)

	// Check reporting.
	checkGaugeValue(t, 1, proxyConnectionsTotal.WithLabelValues(string(common.ProtocolHTTP), t.Name()))
	checkGaugeValue(t, 0, proxyActiveConnections.WithLabelValues(string(common.ProtocolHTTP), t.Name()))

	t.Run("on handler error", func(t *testing.T) {
		alpnConnHandler := svr.MakeConnectionHandler(
			&tls.Config{
				NextProtos: []string{string(common.ProtocolHTTP)},
				Certificates: []tls.Certificate{
					mustGenCertSignedWithCA(t, customCA),
				},
			},
			common.ConnHandlerSource(t.Name()),
		)

		serverConn, clientConn := net.Pipe()

		clientTLSConn := tls.Client(clientConn, &tls.Config{
			NextProtos: []string{"some-unknown-alpn"},
			RootCAs:    pool,
			ServerName: "localhost",
		})
		defer clientTLSConn.Close()

		// The handler should close this conn automatically on error.
		// Do a defer-close just in case the test fails earlier.
		trackServerConn := &closeTrackerConn{
			Conn: serverConn,
		}
		defer trackServerConn.Close()

		handlerCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		handlerErr := make(chan error, 1)
		go func() {
			defer cancel()
			handlerErr <- alpnConnHandler(handlerCtx, trackServerConn)
		}()

		// Now do the TLS handshake for server to handle.
		require.Error(t, clientTLSConn.HandshakeContext(context.Background()))
		select {
		case err := <-handlerErr:
			require.Error(t, err)
			require.True(t, trace.IsBadParameter(err))
			require.Contains(t, err.Error(), "failed to find ALPN handler")
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for handler error")
		}

		// Make sure passed in conn is closed.
		require.True(t, trackServerConn.closed.Load())
		// Check reporting.
		checkGaugeValue(t, 1, proxyConnectionsTotal.WithLabelValues("unknown", t.Name()))
		checkGaugeValue(t, 1, proxyConnectionErrorsTotal.WithLabelValues("unknown", t.Name()))
	})
}

func checkGaugeValue(t *testing.T, expected float64, gauge prometheus.Gauge) {
	t.Helper()

	var protoMetric dto.Metric
	err := gauge.Write(&protoMetric)
	assert.NoError(t, err)

	if err != nil {
		assert.InEpsilon(t, expected, protoMetric.GetGauge().GetValue(), float64(0))
	}
}

type closeTrackerConn struct {
	net.Conn
	closed atomic.Bool
}

func (c *closeTrackerConn) NetConn() net.Conn {
	return c.Conn
}
func (c *closeTrackerConn) Close() error {
	if c.closed.Load() {
		return io.ErrClosedPipe
	}
	c.closed.Store(true)
	return c.Conn.Close()
}

// TestProxyALPNProtocolsRouting tests the routing based on client TLS NextProtos values.
func TestProxyALPNProtocolsRouting(t *testing.T) {
	t.Parallel()

	makeHandler := func(protocol common.Protocol) HandlerDecs {
		return HandlerDecs{
			MatchFunc: MatchByProtocol(protocol),
			Handler: func(ctx context.Context, conn net.Conn) error {
				defer conn.Close()
				_, err := fmt.Fprint(conn, string(protocol))
				require.NoError(t, err)
				return nil
			},
		}
	}

	tests := []struct {
		name                string
		handlers            []HandlerDecs
		kubeHandler         HandlerDecs
		ServerName          string
		ClientNextProtos    []string
		wantProtocolHandler string
	}{
		{
			name: "one element - supported known protocol handler should be called",
			handlers: []HandlerDecs{
				makeHandler(common.ProtocolHTTP),
				makeHandler(common.ProtocolProxySSH),
			},
			ClientNextProtos:    []string{string(common.ProtocolProxySSH)},
			ServerName:          "localhost",
			wantProtocolHandler: string(common.ProtocolProxySSH),
		},
		{
			name: "supported protocol as last element",
			handlers: []HandlerDecs{
				makeHandler(common.ProtocolHTTP),
				makeHandler(common.ProtocolProxySSH),
			},
			ClientNextProtos: []string{
				"unknown-protocol1",
				"unknown-protocol2",
				"unknown-protocol3",
				string(common.ProtocolProxySSH),
			},
			ServerName:          "localhost",
			wantProtocolHandler: string(common.ProtocolProxySSH),
		},
		{
			name: "nil client next protos - default http handler should be called",
			handlers: []HandlerDecs{
				makeHandler(common.ProtocolHTTP),
				makeHandler(common.ProtocolProxySSH),
			},
			ClientNextProtos:    nil,
			ServerName:          "localhost",
			wantProtocolHandler: string(common.ProtocolHTTP),
		},
		{
			name:             "kube KubeTeleportProxyALPNPrefix prefix should route to kube handler",
			ClientNextProtos: nil,
			ServerName:       fmt.Sprintf("%s%s", constants.KubeTeleportProxyALPNPrefix, "localhost"),
			handlers: []HandlerDecs{
				makeHandler(common.ProtocolHTTP),
			},
			kubeHandler: HandlerDecs{
				Handler: func(ctx context.Context, conn net.Conn) error {
					defer conn.Close()
					_, err := fmt.Fprint(conn, "kube")
					require.NoError(t, err)
					return nil
				},
			},
			wantProtocolHandler: "kube",
		},
		{
			name:       "kubeapp app access should route to web handler",
			ServerName: "kubeapp.localhost",
			handlers: []HandlerDecs{
				makeHandler(common.ProtocolHTTP),
			},
			wantProtocolHandler: string(common.ProtocolHTTP),
		},
		{
			name:       "kubernetes servername prefix should route to web handler",
			ServerName: "kubernetes.localhost",
			handlers: []HandlerDecs{
				makeHandler(common.ProtocolHTTP),
			},
			wantProtocolHandler: string(common.ProtocolHTTP),
		},
		{
			name:             "kube ServerName prefix should route to kube handler",
			ClientNextProtos: nil,
			ServerName:       fmt.Sprintf("%s%s", constants.KubeTeleportProxyALPNPrefix, "localhost"),
			handlers: []HandlerDecs{
				makeHandler(common.ProtocolHTTP),
			},
			kubeHandler: HandlerDecs{
				Handler: func(ctx context.Context, conn net.Conn) error {
					defer conn.Close()
					_, err := fmt.Fprint(conn, "kube")
					require.NoError(t, err)
					return nil
				},
			},
			wantProtocolHandler: "kube",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suite := NewSuite(t)
			router := NewRouter()
			for _, r := range tc.handlers {
				router.Add(r)
			}
			router.kubeHandler = &tc.kubeHandler
			suite.router = router
			suite.Start(t)

			conn, err := tls.Dial("tcp", suite.GetServerAddress(), &tls.Config{
				NextProtos: tc.ClientNextProtos,
				ServerName: tc.ServerName,
				RootCAs:    suite.GetCertPool(),
			})
			require.NoError(t, err)
			defer conn.Close()

			mustReadFromConnection(t, conn, tc.wantProtocolHandler)
			mustCloseConnection(t, conn)
		})
	}
}

func TestMatchMySQLConn(t *testing.T) {
	encodeProto := func(version string) string {
		return string(common.ProtocolMySQLWithVerPrefix) + base64.StdEncoding.EncodeToString([]byte(version))
	}

	tests := []struct {
		name    string
		protos  []string
		version any
	}{
		{
			name:    "success",
			protos:  []string{encodeProto("8.0.12")},
			version: "8.0.12",
		},
		{
			name:    "protocol only",
			protos:  []string{string(common.ProtocolMySQL)},
			version: nil,
		},
		{
			name:    "random string",
			protos:  []string{encodeProto("MariaDB some version")},
			version: "MariaDB some version",
		},
		{
			name:    "missing -",
			protos:  []string{string(common.ProtocolMySQL) + base64.StdEncoding.EncodeToString([]byte("8.0.1"))},
			version: nil,
		},
		{
			name:    "missing version returns nothing",
			protos:  []string{encodeProto("")},
			version: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := ExtractMySQLEngineVersion(func(ctx context.Context, conn net.Conn) error {
				version := ctx.Value(dbutils.ContextMySQLServerVersion)
				require.Equal(t, tt.version, version)

				return nil
			})

			ctx := context.Background()
			connectionInfo := ConnectionInfo{
				ALPN: tt.protos,
			}

			err := fn(ctx, nil, connectionInfo)
			require.NoError(t, err)
		})
	}
}

func TestProxyPingConnections(t *testing.T) {
	dataWritten := "message ping connection"

	suite := NewSuite(t)
	clientCert := mustGenCertSignedWithCA(t, suite.ca,
		withIdentity(tlsca.Identity{
			Username: "test-user",
			Groups:   []string{"test-group"},
			RouteToDatabase: tlsca.RouteToDatabase{
				ServiceName: "mongo-test-database",
			},
		}),
	)
	handlerFunc := func(_ context.Context, conn net.Conn) error {
		defer conn.Close()
		_, err := fmt.Fprint(conn, dataWritten)
		require.NoError(t, err)
		return nil
	}

	// MatchByProtocol should match the corresponding Ping protocols.
	suite.router.Add(HandlerDecs{
		MatchFunc: MatchByProtocol(common.ProtocolsWithPingSupport...),
		Handler:   handlerFunc,
	})
	suite.router.AddDBTLSHandler(handlerFunc)
	suite.Start(t)

	for _, protocol := range common.ProtocolsWithPingSupport {
		t.Run(string(protocol), func(t *testing.T) {
			t.Parallel()

			localProxyListener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)

			localProxyConfig := LocalProxyConfig{
				RemoteProxyAddr:    suite.GetServerAddress(),
				Protocols:          []common.Protocol{common.ProtocolWithPing(protocol)},
				Listener:           localProxyListener,
				SNI:                "localhost",
				ParentContext:      context.Background(),
				InsecureSkipVerify: true,
				verifyUpstreamConnection: func(state tls.ConnectionState) error {
					if state.NegotiatedProtocol != string(common.ProtocolWithPing(protocol)) {
						return fmt.Errorf("expected negotiated protocol %q but got %q", common.ProtocolWithPing(protocol), state.NegotiatedProtocol)
					}
					return nil
				},
			}
			mustStartLocalProxy(t, localProxyConfig)

			conn, err := net.Dial("tcp", localProxyListener.Addr().String())
			require.NoError(t, err)

			if common.IsDBTLSProtocol(protocol) {
				conn = tls.Client(conn, &tls.Config{
					Certificates: []tls.Certificate{
						clientCert,
					},
					RootCAs:    suite.GetCertPool(),
					ServerName: "localhost",
				})
			}

			mustReadFromConnection(t, conn, dataWritten)
			mustCloseConnection(t, conn)
		})
	}
}
