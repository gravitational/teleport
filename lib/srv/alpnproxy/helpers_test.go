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
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/cryptopatch"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

type Suite struct {
	serverListener net.Listener
	router         *Router
	ca             *tlsca.CertAuthority
	authServer     *auth.TestAuthServer
	tlsServer      *auth.TestTLSServer
	accessPoint    *authclient.Client
}

func NewSuite(t *testing.T) *Suite {
	ca := mustGenSelfSignedCert(t)
	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		l.Close()
	})

	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		ClusterName: "root.example.com",
		Dir:         t.TempDir(),
		Clock:       clockwork.NewFakeClockAt(time.Now()),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		err := authServer.Close()
		require.NoError(t, err)
	})
	tlsServer, err := authServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() {
		err := tlsServer.Close()
		require.NoError(t, err)
	})

	ap, err := tlsServer.NewClient(auth.TestBuiltin(types.RoleProxy))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, ap.Close())
	})

	router := NewRouter()

	return &Suite{
		tlsServer:      tlsServer,
		authServer:     authServer,
		accessPoint:    ap,
		ca:             ca,
		serverListener: l,
		router:         router,
	}
}

func (s *Suite) GetServerAddress() string {
	return s.serverListener.Addr().String()
}

func (s *Suite) GetCertPool() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(s.ca.Cert)
	return pool
}

func (s *Suite) CreateProxyServer(t *testing.T) *Proxy {
	serverCert := mustGenCertSignedWithCA(t, s.ca)
	tlsConfig := &tls.Config{
		NextProtos: common.ProtocolsToString(common.SupportedProtocols),
		ClientAuth: tls.VerifyClientCertIfGiven,
		ClientCAs:  s.GetCertPool(),
		Certificates: []tls.Certificate{
			serverCert,
		},
	}

	proxyConfig := ProxyConfig{
		Listener:          s.serverListener,
		WebTLSConfig:      tlsConfig,
		Router:            s.router,
		Log:               logrus.New(),
		AccessPoint:       s.accessPoint,
		IdentityTLSConfig: tlsConfig,
		ClusterName:       "root",
	}

	svr, err := New(proxyConfig)
	require.NoError(t, err)
	// Reset GetConfigForClient to simplify test setup.
	svr.cfg.IdentityTLSConfig.GetConfigForClient = nil
	return svr
}

func (s *Suite) Start(t *testing.T) {
	svr := s.CreateProxyServer(t)

	go func() {
		err := svr.Serve(context.Background())
		require.NoError(t, err)
	}()

	t.Cleanup(func() {
		err := svr.Close()
		require.NoError(t, err)
	})
}

func mustGenSelfSignedCert(t *testing.T) *tlsca.CertAuthority {
	caKey, caCert, err := tlsca.GenerateSelfSignedCA(pkix.Name{
		CommonName: "localhost",
	}, []string{"localhost"}, defaults.CATTL)
	require.NoError(t, err)

	ca, err := tlsca.FromKeys(caCert, caKey)
	require.NoError(t, err)
	return ca
}

type signOptions struct {
	identity tlsca.Identity
	clock    clockwork.Clock
}

func withIdentity(identity tlsca.Identity) signOptionsFunc {
	return func(o *signOptions) {
		o.identity = identity
	}
}

func withClock(clock clockwork.Clock) signOptionsFunc {
	return func(o *signOptions) {
		o.clock = clock
	}
}

type signOptionsFunc func(o *signOptions)

func mustGenCertSignedWithCA(t *testing.T, ca *tlsca.CertAuthority, opts ...signOptionsFunc) tls.Certificate {
	options := signOptions{
		identity: tlsca.Identity{Username: "test-user"},
		clock:    clockwork.NewRealClock(),
	}

	for _, opt := range opts {
		opt(&options)
	}

	subj, err := options.identity.Subject()
	require.NoError(t, err)

	privateKey, err := cryptopatch.GenerateRSAKey(rand.Reader, 2048)
	require.NoError(t, err)

	tlsCert, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     options.clock,
		PublicKey: privateKey.Public(),
		Subject:   subj,
		NotAfter:  options.clock.Now().UTC().Add(time.Minute),
		DNSNames:  []string{"localhost", "*.localhost"},
	})
	require.NoError(t, err)

	keyRaw := x509.MarshalPKCS1PrivateKey(privateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyRaw})
	cert, err := tls.X509KeyPair(tlsCert, keyPEM)
	require.NoError(t, err)
	leaf, err := utils.TLSCertLeaf(cert)
	require.NoError(t, err)
	cert.Leaf = leaf
	return cert
}

func mustReadFromConnection(t *testing.T, conn net.Conn, want string) {
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second*5)))
	buff, err := io.ReadAll(conn)
	require.NoError(t, err)
	require.NoError(t, conn.SetReadDeadline(time.Time{}))
	require.Equal(t, want, string(buff))
}

func mustCloseConnection(t *testing.T, conn net.Conn) {
	err := conn.Close()
	require.NoError(t, err)
}

func mustCreateLocalListener(t *testing.T) net.Listener {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		l.Close()
	})
	return l
}

func mustCreateCertGenListener(t *testing.T, ca tls.Certificate) net.Listener {
	listener, err := NewCertGenListener(CertGenListenerConfig{
		ListenAddr: "localhost:0",
		CA:         ca,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		listener.Close()
	})
	return listener
}

func mustSuccessfullyCallHTTPSServer(t *testing.T, addr string, client http.Client) {
	mustCallHTTPSServerAndReceiveCode(t, addr, client, http.StatusOK)
}

func mustCallHTTPSServerAndReceiveCode(t *testing.T, addr string, client http.Client, expectStatusCode int) {
	resp, err := client.Get(fmt.Sprintf("https://%s", addr))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, expectStatusCode, resp.StatusCode)
}

func mustStartHTTPServer(t *testing.T, l net.Listener) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {})
	go http.Serve(l, mux)
}

func mustStartLocalProxy(t *testing.T, config LocalProxyConfig) {
	lp, err := NewLocalProxy(config)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = lp.Close()
		require.NoError(t, err)
	})
	go func() {
		err := lp.Start(context.Background())
		require.NoError(t, err)
	}()
}

func httpsClientWithProxyURL(proxyAddr string, caPem []byte) *http.Client {
	rootCAs := x509.NewCertPool()
	rootCAs.AppendCertsFromPEM(caPem)

	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(&url.URL{
				Scheme: "http",
				Host:   proxyAddr,
			}),

			TLSClientConfig: &tls.Config{
				RootCAs: rootCAs,
			},
		},
	}
}
