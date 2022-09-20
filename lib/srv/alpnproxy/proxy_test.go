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

package alpnproxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/stretchr/testify/require"
)

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
		kubeSNI                   = "kube.localhost"
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
		err := tlsConn.Handshake()
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
		Protocol:           common.ProtocolPostgres,
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

	mustStartHTTPServer(t, lw)

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
				string(common.ProtocolProxySSH)},
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
			name:             "kube ServerName prefix should route to kube handler",
			ClientNextProtos: nil,
			ServerName:       fmt.Sprintf("%s%s", constants.KubeSNIPrefix, "localhost"),
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
