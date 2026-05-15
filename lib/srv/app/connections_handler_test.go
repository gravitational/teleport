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
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/utils"
)

func TestNewHTTPServer(t *testing.T) {
	t.Parallel()

	c := &ConnectionsHandler{
		cfg: &ConnectionsHandlerConfig{
			ServiceComponent: teleport.ComponentApp,
		},
	}

	srv := c.newHTTPServer("test-cluster")

	// The HTTP server no longer wraps a limiter (limiting is applied at
	// the connection level in handleConnection). Verify that requests
	// are never rejected with 429 by the HTTP handler itself.
	for i := range 5 {
		rec := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		require.NotEqual(t, http.StatusTooManyRequests, rec.Code,
			"request %d should not be rate-limited", i+1)
	}
}

// newTestHandler creates a ConnectionsHandler with a limiter for testing
// handleConnection. The handler is minimal - only the limiter wiring is
// functional; TLS and auth are not configured.
func newTestHandler(t *testing.T, cfg limiter.Config) *ConnectionsHandler {
	t.Helper()
	lim, err := limiter.NewLimiter(cfg)
	require.NoError(t, err)

	return &ConnectionsHandler{
		cfg: &ConnectionsHandlerConfig{
			Clock:            clockwork.NewFakeClock(),
			ServiceComponent: teleport.ComponentApp,
		},
		closeContext: t.Context(),
		limiter:      lim,
		log:          slog.Default(),
	}
}

// newConnWithIP creates a net.Pipe-backed connection whose RemoteAddr
// returns the given IP and port. Close the returned client side when
// done; the server side is used by handleConnection.
func newConnWithIP(t *testing.T, ip string, port int) (server, client net.Conn) {
	t.Helper()
	server, client = net.Pipe()
	t.Cleanup(func() {
		server.Close()
		client.Close()
	})
	addr := &net.TCPAddr{IP: net.ParseIP(ip), Port: port}
	server = utils.NewConnWithSrcAddr(server, addr)
	return server, client
}

func TestHandleConnection_MaxConnections(t *testing.T) {
	t.Parallel()

	c := newTestHandler(t, limiter.Config{MaxConnections: 1})

	const clientIP = "10.0.0.1"

	// Pre-fill the limiter to capacity for this IP so the next
	// handleConnection call from the same IP is rejected.
	release, err := c.limiter.RegisterRequestAndConnection(clientIP)
	require.NoError(t, err)
	defer release()

	conn, client := newConnWithIP(t, clientIP, 10001)
	client.Close() // prevent blocking on TLS handshake

	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	_, err = c.handleConnection(ctx, cancel, conn)
	require.True(t, trace.IsLimitExceeded(err), "expected LimitExceeded error, got: %v", err)
}

func TestHandleConnection_MaxConnectionsRelease(t *testing.T) {
	t.Parallel()

	c := newTestHandler(t, limiter.Config{MaxConnections: 1})

	const clientIP = "10.0.0.1"

	// Acquire and immediately release the slot so the limiter
	// has capacity when handleConnection runs.
	release, err := c.limiter.RegisterRequestAndConnection(clientIP)
	require.NoError(t, err)
	release()

	conn, client := newConnWithIP(t, clientIP, 10001)
	client.Close()

	// handleConnection should pass the limiter and fail later
	// (at TLS handshake). The error must not be about limits.
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	_, err = c.handleConnection(ctx, cancel, conn)
	require.Error(t, err)
	require.False(t, trace.IsLimitExceeded(err), "unexpected LimitExceeded error: %v", err)
}

func TestHandleConnection_RateLimiting(t *testing.T) {
	t.Parallel()

	c := newTestHandler(t, limiter.Config{
		Rates: []limiter.Rate{{
			Period:  time.Minute,
			Average: 1,
			Burst:   1,
		}},
	})

	const clientIP = "10.0.0.1"

	// Consume the one allowed request so the next is rate-limited.
	require.NoError(t, c.limiter.RegisterRequest(clientIP))

	conn, client := newConnWithIP(t, clientIP, 10001)
	client.Close()

	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	_, err := c.handleConnection(ctx, cancel, conn)
	require.True(t, trace.IsLimitExceeded(err), "expected LimitExceeded error, got: %v", err)
}

func TestHandleConnection_PipeSkipsLimiter(t *testing.T) {
	t.Parallel()

	// net.Pipe connections (whose RemoteAddr cannot be parsed as
	// host:port) skip limiting entirely. Configure a real limit and
	// pre-fill it so that a normal connection would be rejected.
	c := newTestHandler(t, limiter.Config{MaxConnections: 1})

	release, err := c.limiter.RegisterRequestAndConnection("10.0.0.1")
	require.NoError(t, err)
	defer release()

	server, client := net.Pipe()
	t.Cleanup(func() { server.Close() })
	client.Close()

	// handleConnection should bypass the limiter (which is full)
	// and fail later at TLS instead.
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	_, err = c.handleConnection(ctx, cancel, server)
	require.Error(t, err)
	require.False(t, trace.IsLimitExceeded(err), "unexpected LimitExceeded error: %v", err)
}

func TestClassifyUserAgent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                      string
		ua                        string
		wantIsBrowser             bool
		wantIsPossiblyAppleMobile bool
	}{
		{
			name:          "empty",
			ua:            "",
			wantIsBrowser: false,
		},
		{
			name:          "tsh",
			ua:            "tsh/17.0.0",
			wantIsBrowser: false,
		},
		{
			name:          "curl",
			ua:            "curl/8.4.0",
			wantIsBrowser: false,
		},
		{
			name:          "Go http client",
			ua:            "Go-http-client/1.1",
			wantIsBrowser: false,
		},
		{
			name:          "Mozilla prefix without engine token",
			ua:            "Mozilla/5.0 compatible",
			wantIsBrowser: false,
		},
		{
			name:                      "Chrome on macOS",
			ua:                        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
			wantIsBrowser:             true,
			wantIsPossiblyAppleMobile: true,
		},
		{
			name:                      "Safari on macOS",
			ua:                        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.1",
			wantIsBrowser:             true,
			wantIsPossiblyAppleMobile: true,
		},
		{
			name:                      "Safari on iPhone",
			ua:                        "Mozilla/5.0 (iPhone; CPU iPhone OS 8_4 like Mac OS X) AppleWebKit/600.1.4 (KHTML, like Gecko) Version/8.0 Mobile/12H143 Safari/600.1.4",
			wantIsBrowser:             true,
			wantIsPossiblyAppleMobile: true,
		},
		{
			name:                      "Firefox on Linux",
			ua:                        "Mozilla/5.0 (Linux; U; Linux 2.6; en-US; rv:1.9.9.2) Gecko/20100722 Firefox/3.6.8",
			wantIsBrowser:             true,
			wantIsPossiblyAppleMobile: false,
		},
		{
			name:                      "Firefox on iOS",
			ua:                        "Mozilla/5.0 (iPhone; CPU iPhone OS 12_0_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/7.0.4 Mobile/16A404 Safari/605.1.15",
			wantIsBrowser:             true,
			wantIsPossiblyAppleMobile: true,
		},
		{
			name:                      "Chrome on iOS",
			ua:                        "Mozilla/5.0 (iPhone; CPU iPhone OS 8_2 like Mac OS X) AppleWebKit/600.1.4 (KHTML, like Gecko) CriOS/44.0.2403.67 Mobile/12D508 Safari/600.1.4",
			wantIsBrowser:             true,
			wantIsPossiblyAppleMobile: true,
		},
		{
			name:                      "Edge on Windows",
			ua:                        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36 Edg/147.0.3912.72",
			wantIsBrowser:             true,
			wantIsPossiblyAppleMobile: false,
		},
		{
			name:                      "Opera on macOS",
			ua:                        "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_3_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 OPR/108.0.0.0",
			wantIsBrowser:             true,
			wantIsPossiblyAppleMobile: true,
		},
		{
			name:                      "Chrome on Android",
			ua:                        "Mozilla/5.0 (Linux; Android 16) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.7727.50 Mobile Safari/537.36",
			wantIsBrowser:             true,
			wantIsPossiblyAppleMobile: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isBrowser, isPossiblyAppleMobile := classifyUserAgent(tc.ua)
			require.Equal(t, tc.wantIsBrowser, isBrowser, "isBrowser")
			require.Equal(t, tc.wantIsPossiblyAppleMobile, isPossiblyAppleMobile, "isPossiblyAppleMobile")
		})
	}
}

func TestWriteTrustedDeviceRequired(t *testing.T) {
	t.Parallel()

	const (
		linuxBrowserUA       = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"
		appleMobileBrowserUA = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_7_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Mobile/15E148 Safari/604.1"
	)

	tests := []struct {
		name            string
		userAgent       string
		proxyHost       string
		proxyPort       string
		wantContentType string
		wantBodyParts   []string
		wantNoBodyPart  string
	}{
		{
			name:            "desktop Linux browser gets HTML with Web UI and app access guides only",
			userAgent:       linuxBrowserUA,
			proxyHost:       "teleport.example.com",
			proxyPort:       "443",
			wantContentType: "text/html; charset=utf-8",
			wantBodyParts: []string{
				`<a href="` + trustedDeviceRequiredWebUIDocsURL + `"`,
				`<a href="` + trustedDeviceRequiredAppAccessDocsURL + `"`,
			},
			wantNoBodyPart: "/web/account/security",
		},
		{
			name:            "Apple mobile browser gets HTML with Account Settings link (port 443 stripped)",
			userAgent:       appleMobileBrowserUA,
			proxyHost:       "teleport.example.com",
			proxyPort:       "443",
			wantContentType: "text/html; charset=utf-8",
			wantBodyParts: []string{
				`<a href="` + trustedDeviceRequiredWebUIDocsURL + `"`,
				`<a href="https://teleport.example.com/web/account/security"`,
				`<a href="` + trustedDeviceRequiredAppAccessDocsURL + `"`,
			},
		},
		{
			name:            "Apple mobile browser gets HTML with Account Settings link (non-443 port kept)",
			userAgent:       appleMobileBrowserUA,
			proxyHost:       "teleport.example.com",
			proxyPort:       "3080",
			wantContentType: "text/html; charset=utf-8",
			wantBodyParts: []string{
				`<a href="https://teleport.example.com:3080/web/account/security"`,
			},
		},
		{
			name:            "Apple mobile browser without proxy host gets HTML without Account Settings link",
			userAgent:       appleMobileBrowserUA,
			proxyHost:       "",
			proxyPort:       "443",
			wantContentType: "text/html; charset=utf-8",
			wantBodyParts: []string{
				`<a href="` + trustedDeviceRequiredWebUIDocsURL + `"`,
				`<a href="` + trustedDeviceRequiredAppAccessDocsURL + `"`,
			},
			wantNoBodyPart: "/web/account/security",
		},
		{
			name:            "non-browser gets plain text with URL but no HTML link",
			userAgent:       "tsh/17.0.0",
			proxyHost:       "teleport.example.com",
			proxyPort:       "443",
			wantContentType: "text/plain; charset=utf-8",
			wantBodyParts:   []string{trustedDeviceRequiredDocsURL},
			wantNoBodyPart:  "<a ",
		},
		{
			name:            "empty UA gets plain text with URL but no HTML link",
			userAgent:       "",
			proxyHost:       "teleport.example.com",
			proxyPort:       "443",
			wantContentType: "text/plain; charset=utf-8",
			wantBodyParts:   []string{trustedDeviceRequiredDocsURL},
			wantNoBodyPart:  "<a ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tc.userAgent != "" {
				req.Header.Set("User-Agent", tc.userAgent)
			}
			rec := httptest.NewRecorder()

			writeTrustedDeviceRequired(rec, req, http.StatusForbidden, tc.proxyHost, tc.proxyPort)

			require.Equal(t, http.StatusForbidden, rec.Code)
			require.Equal(t, tc.wantContentType, rec.Header().Get("Content-Type"))
			for _, want := range tc.wantBodyParts {
				require.Contains(t, rec.Body.String(), want)
			}
			if tc.wantNoBodyPart != "" {
				require.NotContains(t, rec.Body.String(), tc.wantNoBodyPart)
			}
		})
	}
}
