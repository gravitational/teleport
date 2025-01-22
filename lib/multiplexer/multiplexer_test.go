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

package multiplexer

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgproto3/v2"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/multiplexer/test"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/cert"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// TestMux tests multiplexing protocols
// using the same listener.
func TestMux(t *testing.T) {
	_, signer, err := cert.CreateCertificate("foo", ssh.HostCert)
	require.NoError(t, err)

	// TestMux tests basic use case of multiplexing TLS
	// and SSH on the same listener socket
	t.Run("TLSSSH", func(t *testing.T) {
		t.Parallel()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		mux, err := New(Config{
			Listener: listener,
		})
		require.NoError(t, err)
		go mux.Serve()
		defer mux.Close()

		backend1 := &httptest.Server{
			Listener: mux.TLS(),
			Config: &http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprintf(w, "backend 1")
				}),
			},
		}
		backend1.StartTLS()
		defer backend1.Close()

		go startSSHServer(t, mux.SSH())

		clt, err := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         time.Second,
		})
		require.NoError(t, err)
		defer clt.Close()

		// Make sure the SSH connection works correctly
		ok, response, err := clt.SendRequest("echo", true, []byte("beep"))
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "beep", string(response))

		client := testClient(backend1)
		re, err := client.Get(backend1.URL)
		require.NoError(t, err)
		defer re.Body.Close()
		bytes, err := io.ReadAll(re.Body)
		require.NoError(t, err)
		require.Equal(t, "backend 1", string(bytes))

		// Close mux, new requests should fail
		mux.Close()
		mux.Wait()

		// use new client to use new connection pool
		client = testClient(backend1)
		re, err = client.Get(backend1.URL)
		if err == nil {
			re.Body.Close()
		}
		require.Error(t, err)
	})
	t.Run("HTTP", func(t *testing.T) {
		t.Parallel()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		mux, err := New(Config{
			Listener: listener,
		})
		require.NoError(t, err)
		go mux.Serve()
		defer mux.Close()

		backend1 := &httptest.Server{
			Listener: mux.HTTP(),
			Config: &http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprintf(w, "backend 1")
				}),
			},
		}
		backend1.Start()
		defer backend1.Close()

		re, err := http.Get(backend1.URL)
		require.NoError(t, err)
		defer re.Body.Close()
		bytes, err := io.ReadAll(re.Body)
		require.NoError(t, err)
		require.Equal(t, "backend 1", string(bytes))

		// Close mux, new requests should fail
		mux.Close()
		mux.Wait()

		// Use new client to use new connection pool
		client := &http.Client{Transport: &http.Transport{}}
		re, err = client.Get(backend1.URL)
		if err == nil {
			re.Body.Close()
		}
		require.Error(t, err)
	})
	// ProxyLine tests proxy line protocol
	t.Run("ProxyLines", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			desc            string
			proxyLine       []byte
			expectedAddress string
		}{
			{
				desc:            "PROXY protocol v1",
				proxyLine:       []byte(sampleProxyV1Line),
				expectedAddress: "127.0.0.1:12345",
			},
			{
				desc:            "PROXY protocol v2 LOCAL command",
				proxyLine:       sampleProxyV2LineLocal,
				expectedAddress: "", // Shouldn't be changed
			},
			{
				desc:            "PROXY protocol v2 PROXY command",
				proxyLine:       sampleProxyV2Line,
				expectedAddress: "127.0.0.1:12345",
			},
		}

		for _, tt := range testCases {
			t.Run(tt.desc, func(t *testing.T) {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)

				mux, err := New(Config{
					Listener:          listener,
					PROXYProtocolMode: PROXYProtocolOn,
				})
				require.NoError(t, err)
				go mux.Serve()
				defer mux.Close()

				backend1 := &httptest.Server{
					Listener: mux.TLS(),
					Config: &http.Server{
						Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							fmt.Fprint(w, r.RemoteAddr)
						}),
					},
				}
				backend1.StartTLS()
				defer backend1.Close()

				parsedURL, err := url.Parse(backend1.URL)
				require.NoError(t, err)

				conn, err := net.Dial("tcp", parsedURL.Host)
				require.NoError(t, err)
				defer conn.Close()
				// send proxy line first before establishing TLS connection
				_, err = conn.Write(tt.proxyLine)
				require.NoError(t, err)

				// upgrade connection to TLS
				tlsConn := tls.Client(conn, clientConfig(backend1))
				defer tlsConn.Close()

				// make sure the TLS call succeeded and we got remote address correctly
				out, err := utils.RoundtripWithConn(tlsConn)
				require.NoError(t, err)
				if tt.expectedAddress != "" {
					require.Equal(t, tt.expectedAddress, out)
				} else {
					require.Equal(t, tlsConn.LocalAddr().String(), out)
				}
			})
		}
	})

	// TestDisabledProxy makes sure the connection gets dropped
	// when Proxy line support protocol is turned off
	t.Run("DisabledProxy", func(t *testing.T) {
		t.Parallel()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		mux, err := New(Config{
			Listener:          listener,
			PROXYProtocolMode: PROXYProtocolOff,
		})
		require.NoError(t, err)
		go mux.Serve()
		defer mux.Close()

		backend1 := &httptest.Server{
			Listener: mux.TLS(),
			Config: &http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, r.RemoteAddr)
				}),
			},
		}
		backend1.StartTLS()
		defer backend1.Close()

		remoteAddr := net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8000}
		proxyLine := ProxyLine{
			Protocol:    TCP4,
			Source:      remoteAddr,
			Destination: net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9000},
		}

		parsedURL, err := url.Parse(backend1.URL)
		require.NoError(t, err)

		conn, err := net.Dial("tcp", parsedURL.Host)
		require.NoError(t, err)
		defer conn.Close()
		// send proxy line first before establishing TLS connection
		_, err = fmt.Fprint(conn, proxyLine.String())
		require.NoError(t, err)

		// upgrade connection to TLS
		tlsConn := tls.Client(conn, clientConfig(backend1))
		defer tlsConn.Close()

		// make sure the TLS call failed
		_, err = utils.RoundtripWithConn(tlsConn)
		require.Error(t, err)
	})

	// makes sure the connection gets dropped
	// when PROXY protocol is 'on' but PROXY line isn't received
	t.Run("required PROXY line", func(t *testing.T) {
		t.Parallel()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		mux, err := New(Config{
			Listener:              listener,
			PROXYProtocolMode:     PROXYProtocolOn,
			IgnoreSelfConnections: true,
		})
		require.NoError(t, err)
		go mux.Serve()
		defer mux.Close()

		backend1 := &httptest.Server{
			Listener: mux.TLS(),
			Config: &http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, r.RemoteAddr)
				}),
			},
		}
		backend1.StartTLS()
		defer backend1.Close()

		parsedURL, err := url.Parse(backend1.URL)
		require.NoError(t, err)

		conn, err := net.Dial("tcp", parsedURL.Host)
		require.NoError(t, err)
		defer conn.Close()

		// upgrade connection to TLS
		tlsConn := tls.Client(conn, clientConfig(backend1))
		defer tlsConn.Close()

		// make sure the TLS call failed
		_, err = utils.RoundtripWithConn(tlsConn)
		require.Error(t, err)
	})

	// makes sure the connection get port set to 0
	// when PROXY protocol is unspecified
	t.Run("source port set to 0 in unspecified PROXY mode", func(t *testing.T) {
		t.Parallel()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		mux, err := New(Config{
			Listener:              listener,
			PROXYProtocolMode:     PROXYProtocolUnspecified,
			IgnoreSelfConnections: true,
		})
		require.NoError(t, err)
		go mux.Serve()
		defer mux.Close()

		backend1 := &httptest.Server{
			Listener: mux.TLS(),
			Config: &http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, r.RemoteAddr)
				}),
			},
		}
		backend1.StartTLS()
		defer backend1.Close()

		parsedURL, err := url.Parse(backend1.URL)
		require.NoError(t, err)

		conn, err := net.Dial("tcp", parsedURL.Host)
		require.NoError(t, err)
		defer conn.Close()

		// Write PROXY line into connection to simulate PROXY protocol
		_, err = conn.Write([]byte(sampleProxyV1Line))
		require.NoError(t, err)

		// upgrade connection to TLS
		tlsConn := tls.Client(conn, clientConfig(backend1))
		defer tlsConn.Close()

		res, err := utils.RoundtripWithConn(tlsConn)
		require.NoError(t, err)

		// Make sure that server saw our connection with source port set to 0
		require.Equal(t, "127.0.0.1:0", res)
	})

	// Timeout test makes sure that multiplexer respects read deadlines.
	t.Run("Timeout", func(t *testing.T) {
		t.Parallel()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		config := Config{
			Listener: listener,
			// Set read deadline in the past to remove reliance on real time
			// and simulate scenario when read deadline has elapsed.
			DetectTimeout: -time.Millisecond,
		}
		mux, err := New(config)
		require.NoError(t, err)
		go mux.Serve()
		defer mux.Close()

		backend1 := &httptest.Server{
			Listener: mux.TLS(),
			Config: &http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, r.RemoteAddr)
				}),
			},
		}
		backend1.StartTLS()
		defer backend1.Close()

		parsedURL, err := url.Parse(backend1.URL)
		require.NoError(t, err)

		conn, err := net.Dial("tcp", parsedURL.Host)
		require.NoError(t, err)
		defer conn.Close()

		// upgrade connection to TLS
		tlsConn := tls.Client(conn, clientConfig(backend1))
		defer tlsConn.Close()

		// roundtrip should fail on the timeout
		_, err = utils.RoundtripWithConn(tlsConn)
		require.Error(t, err)
	})

	// UnknownProtocol make sure that multiplexer closes connection
	// with unknown protocol
	t.Run("UnknownProtocol", func(t *testing.T) {
		t.Parallel()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		mux, err := New(Config{
			Listener: listener,
		})
		require.NoError(t, err)
		go mux.Serve()
		defer mux.Close()

		conn, err := net.Dial("tcp", listener.Addr().String())
		require.NoError(t, err)
		defer conn.Close()

		// try plain HTTP
		_, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n")
		require.NoError(t, err)

		// connection should be closed
		_, err = conn.Read(make([]byte, 1))
		require.Equal(t, err, io.EOF)
	})

	// DisableSSH disables SSH
	t.Run("DisableSSH", func(t *testing.T) {
		t.Parallel()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		mux, err := New(Config{
			Listener: listener,
		})
		require.NoError(t, err)
		go mux.Serve()
		defer mux.Close()

		backend1 := &httptest.Server{
			Listener: mux.TLS(),
			Config: &http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprintf(w, "backend 1")
				}),
			},
		}
		backend1.StartTLS()
		defer backend1.Close()

		_, err = ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{
			Auth:            []ssh.AuthMethod{ssh.Password("abcdef123456")},
			Timeout:         time.Second,
			HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()),
		})
		require.Error(t, err)

		// TLS requests will succeed
		client := testClient(backend1)
		re, err := client.Get(backend1.URL)
		require.NoError(t, err)
		defer re.Body.Close()
		bytes, err := io.ReadAll(re.Body)
		require.NoError(t, err)
		require.Equal(t, "backend 1", string(bytes))

		// Close mux, new requests should fail
		mux.Close()
		mux.Wait()

		// use new client to use new connection pool
		client = testClient(backend1)
		re, err = client.Get(backend1.URL)
		if err == nil {
			re.Body.Close()
		}
		require.Error(t, err)
	})

	// TestDisableTLS tests scenario with disabled TLS
	t.Run("DisableTLS", func(t *testing.T) {
		t.Parallel()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		mux, err := New(Config{
			Listener: listener,
		})
		require.NoError(t, err)
		go mux.Serve()
		defer mux.Close()

		backend1 := &httptest.Server{
			Listener: &noopListener{addr: listener.Addr()},
			Config: &http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprintf(w, "backend 1")
				}),
			},
		}
		backend1.StartTLS()
		defer backend1.Close()

		go startSSHServer(t, mux.SSH())

		clt, err := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         time.Second,
		})
		require.NoError(t, err)
		defer clt.Close()

		// Make sure the SSH connection works correctly
		ok, response, err := clt.SendRequest("echo", true, []byte("beep"))
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "beep", string(response))

		client := testClient(backend1)
		re, err := client.Get(backend1.URL)
		if err == nil {
			re.Body.Close()
		}
		require.Error(t, err)

		// Close mux, new requests should fail
		mux.Close()
		mux.Wait()
	})

	// NextProto tests multiplexing using NextProto selector
	t.Run("NextProto", func(t *testing.T) {
		t.Parallel()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		mux, err := New(Config{
			Listener: listener,
		})
		require.NoError(t, err)
		go mux.Serve()
		defer mux.Close()

		cfg, err := fixtures.LocalTLSConfig()
		require.NoError(t, err)

		tlsLis, err := NewTLSListener(TLSListenerConfig{
			Listener: tls.NewListener(mux.TLS(), cfg.TLS),
		})
		require.NoError(t, err)
		go tlsLis.Serve()

		opts := []grpc.ServerOption{
			grpc.Creds(&httplib.TLSCreds{
				Config: cfg.TLS,
			}),
		}
		s := grpc.NewServer(opts...)
		test.RegisterPingerServer(s, &server{})

		errCh := make(chan error, 2)

		go func() {
			errCh <- s.Serve(tlsLis.HTTP2())
		}()

		httpServer := http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, "http backend")
			}),
		}
		go func() {
			err := httpServer.Serve(tlsLis.HTTP())
			if err == nil || errors.Is(err, http.ErrServerClosed) {
				errCh <- nil
				return
			}
			errCh <- err
		}()

		url := fmt.Sprintf("https://%s", listener.Addr())
		client := cfg.NewClient()
		re, err := client.Get(url)
		require.NoError(t, err)
		defer re.Body.Close()
		bytes, err := io.ReadAll(re.Body)
		require.NoError(t, err)
		require.Equal(t, "http backend", string(bytes))

		creds := credentials.NewClientTLSFromCert(cfg.CertPool, "")

		// Set up a connection to the server.
		conn, err := grpc.Dial(listener.Addr().String(), grpc.WithTransportCredentials(creds), grpc.WithBlock())
		require.NoError(t, err)
		defer conn.Close()

		gclient := test.NewPingerClient(conn)

		out, err := gclient.Ping(context.TODO(), &test.Request{})
		require.NoError(t, err)
		require.Equal(t, "grpc backend", out.GetPayload())

		// Close mux, new requests should fail
		mux.Close()
		mux.Wait()

		// use new client to use new connection pool
		client = cfg.NewClient()
		re, err = client.Get(url)
		if err == nil {
			re.Body.Close()
		}
		require.Error(t, err)

		httpServer.Close()
		s.Stop()
		// wait for both servers to finish
		for i := 0; i < 2; i++ {
			err := <-errCh
			require.NoError(t, err)
		}
	})

	t.Run("PostgresProxy", func(t *testing.T) {
		t.Parallel()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		mux, err := New(Config{
			Context:  ctx,
			Listener: listener,
		})
		require.NoError(t, err)
		go mux.Serve()
		defer mux.Close()

		// register listener before establishing frontend connection
		dblistener := mux.DB()

		check := func(t *testing.T, expectedAddr string, proxyLine []byte) {
			// Connect to the listener and send Postgres SSLRequest which is what
			// psql or other Postgres client will do.
			conn, err := net.Dial("tcp", listener.Addr().String())
			require.NoError(t, err)
			defer conn.Close()

			_, err = conn.Write(sampleProxyV2Line)
			require.NoError(t, err)

			frontend := pgproto3.NewFrontend(pgproto3.NewChunkReader(conn), conn)
			err = frontend.Send(&pgproto3.SSLRequest{})
			require.NoError(t, err)

			// This should not hang indefinitely since we set timeout on the mux context above.
			dbConn, err := dblistener.Accept()
			require.NoError(t, err, "detected Postgres connection")
			require.Equal(t, ProtoPostgres, dbConn.(*Conn).Protocol())
			if expectedAddr != "" {
				require.Equal(t, expectedAddr, dbConn.RemoteAddr().String())
			}
		}

		t.Run("without proxy line", func(t *testing.T) {
			check(t, "", nil)
		})

		t.Run("with proxy line", func(t *testing.T) {
			check(t, "127.0.0.1:0", sampleProxyV2Line)
		})
	})

	// WebListener verifies web listener correctly multiplexes connections
	// between web and database listeners based on the client certificate.
	t.Run("WebListener", func(t *testing.T) {
		t.Parallel()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		mux, err := New(Config{
			Listener: listener,
		})
		require.NoError(t, err)
		go mux.Serve()
		defer mux.Close()

		// register listener before establishing frontend connection
		tlslistener := mux.TLS()

		// Generate self-signed CA.
		caKey, caCert, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: "test-ca"}, nil, time.Hour)
		require.NoError(t, err)
		ca, err := tlsca.FromKeys(caCert, caKey)
		require.NoError(t, err)
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(caCert)

		// Sign server certificate.
		serverRSAKey, err := native.GenerateRSAPrivateKey()
		require.NoError(t, err)
		serverPEM, err := ca.GenerateCertificate(tlsca.CertificateRequest{
			Subject:   pkix.Name{CommonName: "localhost"},
			PublicKey: serverRSAKey.Public(),
			NotAfter:  time.Now().Add(time.Hour),
			DNSNames:  []string{"127.0.0.1"},
		})
		require.NoError(t, err)
		serverKeyPEM, err := keys.MarshalPrivateKey(serverRSAKey)
		require.NoError(t, err)
		serverCert, err := tls.X509KeyPair(serverPEM, serverKeyPEM)
		require.NoError(t, err)

		// Sign client certificate with database access identity.
		clientRSAKey, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
		require.NoError(t, err)
		subject, err := (&tlsca.Identity{
			Username: "alice",
			Groups:   []string{"admin"},
			RouteToDatabase: tlsca.RouteToDatabase{
				ServiceName: "postgres",
			},
		}).Subject()
		require.NoError(t, err)
		clientPEM, err := ca.GenerateCertificate(tlsca.CertificateRequest{
			Subject:   subject,
			PublicKey: clientRSAKey.Public(),
			NotAfter:  time.Now().Add(time.Hour),
		})
		require.NoError(t, err)
		clientKeyPEM, err := keys.MarshalPrivateKey(clientRSAKey)
		require.NoError(t, err)
		clientCert, err := tls.X509KeyPair(clientPEM, clientKeyPEM)
		require.NoError(t, err)

		webLis, err := NewWebListener(WebListenerConfig{
			Listener: tls.NewListener(tlslistener, &tls.Config{
				ClientCAs:    certPool,
				ClientAuth:   tls.VerifyClientCertIfGiven,
				Certificates: []tls.Certificate{serverCert},
			}),
		})
		require.NoError(t, err)
		go webLis.Serve()
		defer webLis.Close()

		go func() {
			conn, err := webLis.Web().Accept()
			require.NoError(t, err)
			defer conn.Close()
			conn.Write([]byte("web listener"))
		}()

		go func() {
			conn, err := webLis.DB().Accept()
			require.NoError(t, err)
			defer conn.Close()
			conn.Write([]byte("db listener"))
		}()

		webConn, err := tls.Dial("tcp", listener.Addr().String(), &tls.Config{
			RootCAs: certPool,
		})
		require.NoError(t, err)
		defer webConn.Close()

		webBytes, err := io.ReadAll(webConn)
		require.NoError(t, err)
		require.Equal(t, "web listener", string(webBytes))

		dbConn, err := tls.Dial("tcp", listener.Addr().String(), &tls.Config{
			RootCAs:      certPool,
			Certificates: []tls.Certificate{clientCert},
		})
		require.NoError(t, err)
		defer dbConn.Close()

		dbBytes, err := io.ReadAll(dbConn)
		require.NoError(t, err)
		require.Equal(t, "db listener", string(dbBytes))
	})

	// Ensures that we can correctly send and verify signed PROXY header
	t.Run("signed PROXYv2 headers", func(t *testing.T) {
		t.Parallel()

		const clusterName = "teleport-test"
		tlsProxyCert, casGetter, jwtSigner := getTestCertCAsGetterAndSigner(t, clusterName)

		listener4, err := net.Listen("tcp", "127.0.0.1:")
		require.NoError(t, err)

		// If listener for IPv6 will fail to be created we'll skip IPv6 portion of test.
		listener6, _ := net.Listen("tcp6", "[::1]:0")

		server := muxServer{
			certAuthorityGetter: casGetter,
		}

		mux4, backend4, err := server.startServing(listener4, clusterName)
		require.NoError(t, err)
		defer mux4.Close()
		defer backend4.Close()

		var backend6 *httptest.Server
		var mux6 *Mux
		if listener6 != nil {
			mux6, backend6, err = server.startServing(listener6, clusterName)
			require.NoError(t, err)
			defer mux6.Close()
			defer backend6.Close()
		}

		addr1 := net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 444}
		addr2 := net.TCPAddr{IP: net.ParseIP("5.4.3.2"), Port: 555}
		addrV6 := net.TCPAddr{IP: net.ParseIP("::1"), Port: 999}

		t.Run("single signed PROXY header", func(t *testing.T) {
			conn, err := net.Dial("tcp", listener4.Addr().String())
			require.NoError(t, err)
			defer conn.Close()

			signedHeader, err := signPROXYHeader(signPROXYHeaderInput{
				source:      &addr1,
				destination: &addr2,
				clusterName: clusterName,
				signingCert: tlsProxyCert,
				signer:      jwtSigner,
			})
			require.NoError(t, err)

			_, err = conn.Write(signedHeader)
			require.NoError(t, err)

			clt := tls.Client(conn, clientConfig(backend4))

			out, err := utils.RoundtripWithConn(clt)
			require.NoError(t, err)
			require.Equal(t, addr1.String(), out)
		})
		t.Run("single signed PROXY header on IPv6", func(t *testing.T) {
			if listener6 == nil {
				t.Skip("Skipping since IPv6 listener is not available")
			}
			conn, err := net.Dial("tcp6", listener6.Addr().String())
			require.NoError(t, err)

			defer conn.Close()

			signedHeader, err := signPROXYHeader(signPROXYHeaderInput{
				source:         &addrV6,
				destination:    &addrV6,
				clusterName:    clusterName,
				signingCert:    tlsProxyCert,
				signer:         jwtSigner,
				allowDowngrade: false,
			})
			require.NoError(t, err)

			_, err = conn.Write(signedHeader)
			require.NoError(t, err)

			clt := tls.Client(conn, clientConfig(backend6))

			out, err := utils.RoundtripWithConn(clt)
			require.NoError(t, err)
			require.Equal(t, addrV6.String(), out)
		})
		t.Run("single signed PROXY header from IPv6 to IPv4", func(t *testing.T) {
			conn, err := net.Dial("tcp", listener4.Addr().String())
			require.NoError(t, err)

			defer conn.Close()

			signedHeader, err := signPROXYHeader(signPROXYHeaderInput{
				source:         &addrV6,
				destination:    &addr1,
				clusterName:    clusterName,
				signingCert:    tlsProxyCert,
				signer:         jwtSigner,
				allowDowngrade: true,
			})
			require.NoError(t, err)

			_, err = conn.Write(signedHeader)
			require.NoError(t, err)

			clt := tls.Client(conn, clientConfig(backend4))

			out, err := utils.RoundtripWithConn(clt)
			require.NoError(t, err)

			require.Equal(t, addrV6.String(), out)
		})
		t.Run("single signed PROXY header from IPv6 to IPv4, no downgrade", func(t *testing.T) {
			conn, err := net.Dial("tcp", listener4.Addr().String())
			require.NoError(t, err)

			defer conn.Close()

			_, err = signPROXYHeader(signPROXYHeaderInput{
				source:      &addrV6,
				destination: &addr1,
				clusterName: clusterName,
				signingCert: tlsProxyCert,
				signer:      jwtSigner,
			})
			require.Error(t, err)
		})
		t.Run("two signed PROXY headers", func(t *testing.T) {
			conn, err := net.Dial("tcp", listener4.Addr().String())
			require.NoError(t, err)
			defer conn.Close()

			signedHeader, err := signPROXYHeader(signPROXYHeaderInput{
				source:      &addr1,
				destination: &addr2,
				clusterName: clusterName,
				signingCert: tlsProxyCert,
				signer:      jwtSigner,
			})
			require.NoError(t, err)

			_, err = conn.Write(signedHeader)
			require.NoError(t, err)
			_, err = conn.Write(signedHeader)
			require.NoError(t, err)

			clt := tls.Client(conn, clientConfig(backend4))

			_, err = utils.RoundtripWithConn(clt)
			require.Error(t, err)
		})
		t.Run("two signed PROXY headers, one signed for wrong cluster", func(t *testing.T) {
			conn, err := net.Dial("tcp", listener4.Addr().String())
			require.NoError(t, err)
			defer conn.Close()

			signedHeader, err := signPROXYHeader(signPROXYHeaderInput{
				source:      &addr1,
				destination: &addr2,
				clusterName: clusterName,
				signingCert: tlsProxyCert,
				signer:      jwtSigner,
			})
			require.NoError(t, err)
			signedHeader2, err := signPROXYHeader(signPROXYHeaderInput{
				source:      &addr2,
				destination: &addr1,
				clusterName: clusterName + "wrong",
				signingCert: tlsProxyCert,
				signer:      jwtSigner,
			})
			require.NoError(t, err)

			_, err = conn.Write(signedHeader)
			require.NoError(t, err)
			_, err = conn.Write(signedHeader2)
			require.NoError(t, err)

			clt := tls.Client(conn, clientConfig(backend4))

			_, err = utils.RoundtripWithConn(clt)
			require.Error(t, err)
		})
		t.Run("first unsigned then signed PROXY headers", func(t *testing.T) {
			conn, err := net.Dial("tcp", listener4.Addr().String())
			require.NoError(t, err)
			defer conn.Close()

			signedHeader, err := signPROXYHeader(signPROXYHeaderInput{
				source:      &addr1,
				destination: &addr2,
				clusterName: clusterName,
				signingCert: tlsProxyCert,
				signer:      jwtSigner,
			})
			require.NoError(t, err)

			pl := ProxyLine{
				Protocol:    TCP4,
				Source:      addr2,
				Destination: addr1,
			}

			b, err := pl.Bytes()
			require.NoError(t, err)

			_, err = conn.Write(b)
			require.NoError(t, err)
			_, err = conn.Write(signedHeader)
			require.NoError(t, err)

			clt := tls.Client(conn, clientConfig(backend4))

			out, err := utils.RoundtripWithConn(clt)
			require.NoError(t, err)
			require.Equal(t, addr1.String(), out)
		})
		t.Run("first signed then unsigned PROXY headers", func(t *testing.T) {
			conn, err := net.Dial("tcp", listener4.Addr().String())
			require.NoError(t, err)
			defer conn.Close()

			signedHeader, err := signPROXYHeader(signPROXYHeaderInput{
				source:      &addr1,
				destination: &addr2,
				clusterName: clusterName,
				signingCert: tlsProxyCert,
				signer:      jwtSigner,
			})
			require.NoError(t, err)

			pl := ProxyLine{
				Protocol:    TCP4,
				Source:      addr2,
				Destination: addr1,
			}

			b, err := pl.Bytes()
			require.NoError(t, err)

			_, err = conn.Write(signedHeader)
			require.NoError(t, err)
			_, err = conn.Write(b)
			require.NoError(t, err)

			clt := tls.Client(conn, clientConfig(backend4))

			_, err = utils.RoundtripWithConn(clt)
			require.Error(t, err)
		})
		t.Run("two unsigned PROXY headers, gets an error", func(t *testing.T) {
			conn, err := net.Dial("tcp", listener4.Addr().String())
			require.NoError(t, err)
			defer conn.Close()

			pl := ProxyLine{
				Protocol:    TCP4,
				Source:      addr2,
				Destination: addr1,
			}

			b, err := pl.Bytes()
			require.NoError(t, err)

			_, err = conn.Write(b)
			require.NoError(t, err)
			_, err = conn.Write(b)
			require.NoError(t, err)

			clt := tls.Client(conn, clientConfig(backend4))

			_, err = utils.RoundtripWithConn(clt)
			require.Error(t, err)
		})
		t.Run("proxy line with non-teleport TLV", func(t *testing.T) {
			conn, err := net.Dial("tcp", listener4.Addr().String())
			require.NoError(t, err)
			defer conn.Close()

			awsProxyLine := ProxyLine{
				Protocol:    TCP4,
				Source:      addr1,
				Destination: addr2,
				TLVs: []TLV{
					{
						Type:  PP2TypeAWS,
						Value: []byte{0x42, 0x84, 0x42, 0x84},
					},
				},
				IsVerified: false,
			}

			header, err := awsProxyLine.Bytes()
			require.NoError(t, err)

			_, err = conn.Write(header)
			require.NoError(t, err)

			clt := tls.Client(conn, clientConfig(backend4))

			out, err := utils.RoundtripWithConn(clt)
			require.NoError(t, err)
			require.Equal(t, addr1.IP.String()+":0", out)
		})
		t.Run("PROXY header signed by non local cluster get an error", func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:")
			require.NoError(t, err)

			// start multiplexer with wrong cluster name specified
			mux, backend, err := server.startServing(listener, "different-cluster")
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, mux.Close())
				backend.Close()
			})

			conn, err := net.Dial("tcp", listener.Addr().String())
			require.NoError(t, err)
			defer conn.Close()

			signedHeader, err := signPROXYHeader(signPROXYHeaderInput{
				source:      &addr1,
				destination: &addr2,
				clusterName: clusterName,
				signingCert: tlsProxyCert,
				signer:      jwtSigner,
			})
			require.NoError(t, err)

			_, err = conn.Write(signedHeader)
			require.NoError(t, err)

			clt := tls.Client(conn, clientConfig(backend))

			_, err = utils.RoundtripWithConn(clt)
			require.Error(t, err)
		})
	})
}

func TestProtocolString(t *testing.T) {
	for i := -1; i < len(protocolStrings)+1; i++ {
		got := Protocol(i).String()
		switch i {
		case -1, len(protocolStrings) + 1:
			require.Equal(t, "", got)
		default:
			require.Equal(t, protocolStrings[Protocol(i)], got)
		}
	}
}

// server is used to implement test.PingerServer
type server struct {
	test.UnimplementedPingerServer
}

func (s *server) Ping(ctx context.Context, req *test.Request) (*test.Response, error) {
	return &test.Response{Payload: "grpc backend"}, nil
}

// clientConfig returns tls client config from test http server
// set up to listen on TLS
func clientConfig(srv *httptest.Server) *tls.Config {
	cert, err := x509.ParseCertificate(srv.TLS.Certificates[0].Certificate[0])
	if err != nil {
		panic(err)
	}

	certpool := x509.NewCertPool()
	certpool.AddCert(cert)
	return &tls.Config{
		RootCAs:    certpool,
		ServerName: fmt.Sprintf("%v", cert.IPAddresses[0].String()),
	}
}

// testClient is a test HTTP client set up for TLS
func testClient(srv *httptest.Server) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: clientConfig(srv),
		},
	}
}

type noopListener struct {
	addr net.Addr
}

func (noopListener) Accept() (net.Conn, error) {
	return nil, errors.New("noop")
}

func (noopListener) Close() error {
	return nil
}

func (l noopListener) Addr() net.Addr {
	return l.addr
}

func TestIsHTTP(t *testing.T) {
	t.Parallel()
	for _, verb := range httpMethods {
		t.Run(fmt.Sprintf("Accept %v", string(verb)), func(t *testing.T) {
			data := fmt.Sprintf("%v /some/path HTTP/1.1", string(verb))
			require.True(t, isHTTP([]byte(data)))
		})
	}

	rejectedInputs := []string{
		"some random junk",
		"FAKE /some/path HTTP/1.1",
		// This case checks for a bug where the arguments to bytes.HasPrefix are reversed.
		"GE",
	}
	for _, input := range rejectedInputs {
		t.Run(fmt.Sprintf("Reject %q", input), func(t *testing.T) {
			require.False(t, isHTTP([]byte(input)))
		})
	}
}

func getTestCertCAsGetterAndSigner(t testing.TB, clusterName string) ([]byte, CertAuthorityGetter, JWTPROXYSigner) {
	t.Helper()
	caPriv, caCert, err := tlsca.GenerateSelfSignedCA(pkix.Name{
		CommonName: clusterName, Organization: []string{clusterName}}, []string{clusterName}, time.Hour)
	require.NoError(t, err)

	tlsCA, err := tlsca.FromKeys(caCert, caPriv)
	require.NoError(t, err)

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: clusterName,
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{
				{
					Cert: caCert,
					Key:  caPriv,
				},
			},
		},
	})
	require.NoError(t, err)

	mockCAGetter := func(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
		return ca, nil
	}
	proxyPriv, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
	require.NoError(t, err)

	// Create host identity with role "Proxy"
	identity := tlsca.Identity{
		TeleportCluster: clusterName,
		Username:        "proxy1",
		Groups:          []string{string(types.RoleProxy)},
		Expires:         time.Now().Add(time.Hour),
	}

	subject, err := identity.Subject()
	require.NoError(t, err)
	certReq := tlsca.CertificateRequest{
		PublicKey: proxyPriv.Public(),
		Subject:   subject,
		NotAfter:  time.Now().Add(time.Hour),
		DNSNames:  []string{"localhost", "127.0.0.1"},
	}
	tlsProxyCertPEM, err := tlsCA.GenerateCertificate(certReq)
	require.NoError(t, err)
	clock := clockwork.NewFakeClockAt(time.Now())
	jwtSigner, err := jwt.New(&jwt.Config{
		Clock:       clock,
		Algorithm:   defaults.ApplicationTokenAlgorithm,
		ClusterName: clusterName,
		PrivateKey:  proxyPriv,
	})
	require.NoError(t, err)

	tlsProxyCertDER, err := tlsca.ParseCertificatePEM(tlsProxyCertPEM)
	require.NoError(t, err)

	return tlsProxyCertDER.Raw, mockCAGetter, jwtSigner
}

func startSSHServer(t *testing.T, listener net.Listener) {
	nConn, err := listener.Accept()
	assert.NoError(t, err)

	t.Cleanup(func() { nConn.Close() })

	block, _ := pem.Decode(fixtures.LocalhostKey)
	pkey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	assert.NoError(t, err)

	signer, err := ssh.NewSignerFromKey(pkey)
	assert.NoError(t, err)

	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)

	conn, _, reqs, err := ssh.NewServerConn(nConn, config)
	assert.NoError(t, err)
	if err != nil {
		return
	}
	t.Cleanup(func() { conn.Close() })

	go func() {
		for newReq := range reqs {
			if newReq.Type == "echo" {
				err := newReq.Reply(true, newReq.Payload)
				assert.NoError(t, err)
				continue
			}
			err := newReq.Reply(false, nil)
			assert.NoError(t, err)
		}
	}()
}

func BenchmarkMux_ProxyV2Signature(b *testing.B) {
	const clusterName = "test-teleport"
	tlsProxyCert, caGetter, jwtSigner := getTestCertCAsGetterAndSigner(b, clusterName)
	ip := "1.2.3.4"
	sAddr := net.TCPAddr{IP: net.ParseIP(ip), Port: 444}
	dAddr := net.TCPAddr{IP: net.ParseIP(ip), Port: 555}
	listener4, err := net.Listen("tcp", "127.0.0.1:")
	require.NoError(b, err)

	server := muxServer{
		disableTLS:          true,
		certAuthorityGetter: caGetter,
	}
	mux4, backend4, err := server.startServing(listener4, clusterName)
	require.NoError(b, err)
	defer mux4.Close()
	defer backend4.Close()

	b.Run("simulation of signing and verifying PROXY header", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			conn, err := net.Dial("tcp", listener4.Addr().String())
			require.NoError(b, err)
			defer conn.Close()

			signedHeader, err := signPROXYHeader(signPROXYHeaderInput{
				source:      &sAddr,
				destination: &dAddr,
				clusterName: clusterName,
				signingCert: tlsProxyCert,
				signer:      jwtSigner,
			})
			require.NoError(b, err)

			_, err = conn.Write(signedHeader)
			require.NoError(b, err)

			out, err := utils.RoundtripWithConn(conn)
			require.NoError(b, err)
			require.Equal(b, sAddr.String(), out)
		}
	})
}

func Test_GetTcpAddr(t *testing.T) {
	testCases := []struct {
		input    net.Addr
		expected string
	}{
		{
			input: &utils.NetAddr{
				Addr:        "127.0.0.1:24998",
				AddrNetwork: "tcp",
				Path:        "",
			},
			expected: "127.0.0.1:24998",
		},
		{
			input:    nil,
			expected: ":0",
		},
		{
			input: &net.TCPAddr{
				IP:   net.ParseIP("8.8.8.8"),
				Port: 25000,
			},
			expected: "8.8.8.8:25000",
		},
		{
			input: &net.TCPAddr{
				IP:   net.ParseIP("::1"),
				Port: 25000,
			},
			expected: "[::1]:25000",
		},
	}

	for _, tt := range testCases {
		result := getTCPAddr(tt.input)
		require.Equal(t, tt.expected, result.String())
	}
}

func TestIsDifferentTCPVersion(t *testing.T) {
	testCases := []struct {
		addr1    string
		addr2    string
		expected bool
	}{
		{
			addr1:    "8.8.8.8:42",
			addr2:    "8.8.8.8:42",
			expected: false,
		},
		{
			addr1:    "[2601:602:8700:4470:a3:813c:1d8c:30b9]:42",
			addr2:    "[2607:f8b0:4005:80a::200e]:42",
			expected: false,
		},
		{
			addr1:    "127.0.0.1:42",
			addr2:    "[::1]:42",
			expected: true,
		},
		{
			addr1:    "[::1]:42",
			addr2:    "127.0.0.1:42",
			expected: true,
		},
		{
			addr1:    "::ffff:39.156.68.48:42",
			addr2:    "39.156.68.48:42",
			expected: true,
		},
		{
			addr1:    "[2607:f8b0:4005:80a::200e]:42",
			addr2:    "1.1.1.1:42",
			expected: true,
		},
		{
			addr1:    "127.0.0.1:42",
			addr2:    "[2607:f8b0:4005:80a::200e]:42",
			expected: true,
		},
		{
			addr1:    "::ffff:39.156.68.48:42",
			addr2:    "[2607:f8b0:4005:80a::200e]:42",
			expected: false,
		},
	}

	for _, tt := range testCases {
		addr1 := getTCPAddr(utils.MustParseAddr(tt.addr1))
		addr2 := getTCPAddr(utils.MustParseAddr(tt.addr2))
		require.Equal(t, tt.expected, isDifferentTCPVersion(addr1, addr2),
			fmt.Sprintf("Unexpected result for %q, %q", tt.addr1, tt.addr2))
	}
}

type muxServer struct {
	certAuthorityGetter func(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
	disableTLS          bool
}

func (m *muxServer) startServing(muxListener net.Listener, cluster string) (*Mux, *httptest.Server, error) {
	mux, err := New(Config{
		Listener:            muxListener,
		PROXYProtocolMode:   PROXYProtocolUnspecified,
		CertAuthorityGetter: m.certAuthorityGetter,
		Clock:               clockwork.NewFakeClockAt(time.Now()),
		LocalClusterName:    cluster,
	})
	if err != nil {
		return mux, &httptest.Server{}, err
	}

	if m.disableTLS {
		muxListener = mux.HTTP()
	} else {
		muxListener = mux.TLS()
	}

	go mux.Serve()

	backend := &httptest.Server{
		Listener: muxListener,

		Config: &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, r.RemoteAddr)
			}),
		},
	}

	if m.disableTLS {
		backend.Start()
	} else {
		backend.StartTLS()
	}

	return mux, backend, nil
}
