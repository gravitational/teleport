/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package app

import (
	"context"
	"crypto/x509/pkix"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestTCPServerHandleConnectionUpstreamTLS(t *testing.T) {
	t.Run("plain upstream without tls block", func(t *testing.T) {
		message := "plain-upstream-response"
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, message)
		}))
		defer upstream.Close()

		app := newTCPTestApp(t, tcpURIFromURL(t, upstream.URL), nil)
		server := newTestTCPServer()

		body, err := sendHTTPRequestThroughTCPProxy(t, server, app)
		require.NoError(t, err)
		require.Equal(t, message, strings.TrimSpace(body))
	})

	t.Run("tls verify-full mode", func(t *testing.T) {
		clock := clockwork.NewRealClock()
		setup := newMTLSUpstreamSetup(t)
		setup.generateAndWriteClientCert(t, clock, time.Now().Add(1*time.Hour))

		message := "verify-full-upstream-response"
		upstream := setup.startUpstreamServer(t, clock, message, []string{"127.0.0.1"}, nil /* ca */)
		defer upstream.Close()

		app := newTCPTestApp(t, tcpURIFromURL(t, upstream.URL), &types.AppTLS{
			CaPath:   setup.caPath,
			CertPath: setup.certPath,
			KeyPath:  setup.keyPath,
			Mode:     types.AppTLS_MODE_VERIFY_FULL,
		})
		server := newTestTCPServer()

		body, err := sendHTTPRequestThroughTCPProxy(t, server, app)
		require.NoError(t, err)
		require.Equal(t, message, strings.TrimSpace(body))
	})

	t.Run("tls verify-ca mode", func(t *testing.T) {
		clock := clockwork.NewRealClock()
		setup := newMTLSUpstreamSetup(t)
		setup.generateAndWriteClientCert(t, clock, time.Now().Add(1*time.Hour))

		message := "verify-ca-upstream-response"
		upstream := setup.startUpstreamServer(t, clock, message, []string{"my-random-domain.com"}, nil /* ca */)
		defer upstream.Close()

		app := newTCPTestApp(t, tcpURIFromURL(t, upstream.URL), &types.AppTLS{
			CaPath:   setup.caPath,
			CertPath: setup.certPath,
			KeyPath:  setup.keyPath,
			Mode:     types.AppTLS_MODE_VERIFY_CA,
		})
		server := newTestTCPServer()

		body, err := sendHTTPRequestThroughTCPProxy(t, server, app)
		require.NoError(t, err)
		require.Equal(t, message, strings.TrimSpace(body))
	})

	t.Run("tls insecure mode", func(t *testing.T) {
		serverCAKey, serverCACertPEM, err := tlsca.GenerateSelfSignedCA(
			pkix.Name{Organization: []string{"test-alternative-server-ca"}},
			nil,
			1*time.Hour,
		)
		require.NoError(t, err)
		serverCA, err := tlsca.FromKeys(serverCACertPEM, serverCAKey)
		require.NoError(t, err)

		clock := clockwork.NewRealClock()
		setup := newMTLSUpstreamSetup(t)
		setup.generateAndWriteClientCert(t, clock, time.Now().Add(1*time.Hour))

		message := "insecure-upstream-response"
		upstream := setup.startUpstreamServer(t, clock, message, []string{"my-random-domain.com"}, serverCA)
		defer upstream.Close()

		app := newTCPTestApp(t, tcpURIFromURL(t, upstream.URL), &types.AppTLS{
			CaPath:   setup.caPath,
			CertPath: setup.certPath,
			KeyPath:  setup.keyPath,
			Mode:     types.AppTLS_MODE_INSECURE,
		})
		server := newTestTCPServer()

		body, err := sendHTTPRequestThroughTCPProxy(t, server, app)
		require.NoError(t, err)
		require.Equal(t, message, strings.TrimSpace(body))
	})
}

func sendHTTPRequestThroughTCPProxy(t *testing.T, tcpServer *tcpServer, app types.Application) (string, error) {
	t.Helper()

	acceptListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer acceptListener.Close()

	errCh := make(chan error, 1)
	go func() {
		clientConn, err := acceptListener.Accept()
		if err != nil {
			errCh <- err
			return
		}
		identity := &tlsca.Identity{
			Username: "test-user",
			RouteToApp: tlsca.RouteToApp{
				SessionID:   "test-session-id",
				ClusterName: "root.example.com",
			},
		}
		errCh <- tcpServer.handleConnection(t.Context(), clientConn, identity, app)
	}()

	transport := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			d := &net.Dialer{Timeout: 2 * time.Second}
			return d.DialContext(ctx, "tcp", acceptListener.Addr().String())
		},
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, "http://proxy.test/", nil)
	require.NoError(t, err)
	req.Close = true

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if err := <-errCh; err != nil {
		return "", err
	}

	return string(body), nil
}

func newTCPTestApp(t *testing.T, uri string, tlsConfig *types.AppTLS) types.Application {
	t.Helper()

	appSpec := types.AppSpecV3{
		URI:        uri,
		PublicAddr: "tcp-app.example.com",
	}
	if tlsConfig != nil {
		appSpec.TLS = *tlsConfig
	}

	app, err := types.NewAppV3(types.Metadata{Name: "tcp-app"}, appSpec)
	require.NoError(t, err)
	return app
}

func newTestTCPServer() *tcpServer {
	return &tcpServer{
		emitter: events.NewDiscardEmitter(),
		hostID:  "test-host-id",
		log:     slog.Default(),
	}
}

func tcpURIFromURL(t *testing.T, rawURL string) string {
	t.Helper()

	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return "tcp://" + u.Host
}
