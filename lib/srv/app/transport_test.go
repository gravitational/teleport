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
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
)

func TestNeedsPathRedirect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		uri          string
		requestPath  string
		wantRedirect bool
		wantLocation string
	}{
		{
			name:         "no path",
			uri:          "http://backend:9000",
			requestPath:  "/",
			wantRedirect: false,
		},
		{
			name:         "root path",
			uri:          "http://backend:9000/",
			requestPath:  "/",
			wantRedirect: false,
		},
		{
			name:         "path without trailing slash",
			uri:          "http://backend:9000/app",
			requestPath:  "/",
			wantRedirect: true,
			wantLocation: "https://public:443/app",
		},
		{
			name:         "path with trailing slash",
			uri:          "http://backend:9000/app/",
			requestPath:  "/",
			wantRedirect: true,
			wantLocation: "https://public:443/app/",
		},
		{
			name:         "deep path with trailing slash",
			uri:          "http://backend:9000/a/b/c/",
			requestPath:  "/",
			wantRedirect: true,
			wantLocation: "https://public:443/a/b/c/",
		},
		{
			name:         "non-root request",
			uri:          "http://backend:9000/app/",
			requestPath:  "/app/",
			wantRedirect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsedURI, err := url.Parse(tt.uri)
			require.NoError(t, err)

			tr := &transport{
				transportConfig: &transportConfig{
					app: &types.AppV3{
						Spec: types.AppSpecV3{
							PublicAddr: "public",
						},
					},
					publicPort: "443",
				},
				uri: parsedURI,
			}

			req := &http.Request{URL: &url.URL{Path: tt.requestPath}}
			location, ok := tr.needsPathRedirect(req)
			require.Equal(t, tt.wantRedirect, ok)
			if tt.wantRedirect {
				require.Equal(t, tt.wantLocation, location)
			} else {
				require.Empty(t, location)
			}
		})
	}
}

// TestTransport_LongRunningUpstreamCompletes verifies a 10-minute
// response-header delay completes under the 1-hour cap.
func TestTransport_LongRunningUpstreamCompletes(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		tr := newTestTransport(t, func(context.Context, string, string) (net.Conn, error) {
			return clientConn, nil
		})

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		upstreamDone := make(chan struct{})
		go func() {
			defer close(upstreamDone)
			fakeUpstream(ctx, serverConn, 10*time.Minute, "ok")
		}()

		req := newTestRequest(t)
		resp, err := tr.RoundTrip(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, "ok", string(body))

		cancel()
		<-upstreamDone
	})
}

// TestTransport_SanityCapFiresDeadlineExceededDiagnostic verifies the
// 1-hour cap fires and renders the Context Deadline Exceeded diagnostic
// page for HTTP/1 ResponseHeaderTimeout. HTTP/2 coverage for the
// net.Error.Timeout() case lives in errors_test.go.
func TestTransport_SanityCapFiresDeadlineExceededDiagnostic(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		tr := newTestTransport(t, func(context.Context, string, string) (net.Conn, error) {
			return clientConn, nil
		})

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		upstreamDone := make(chan struct{})
		go func() {
			defer close(upstreamDone)
			fakeUpstream(ctx, serverConn, 2*time.Hour, "should-not-be-read")
		}()

		req := newTestRequest(t)
		resp, err := tr.RoundTrip(req)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Contains(t, string(body), "Context Deadline Exceeded")

		cancel()
		<-upstreamDone
	})
}

// newTestTransport builds a transport with the minimum config that
// transportConfig.Check requires. The dial func is wired as the
// underlying http.Transport.DialContext so callers can inject a
// net.Pipe.
func newTestTransport(t *testing.T, dial func(context.Context, string, string) (net.Conn, error)) *transport {
	t.Helper()
	app, err := types.NewAppV3(types.Metadata{Name: "test"}, types.AppSpecV3{
		URI:        "http://upstream.invalid",
		PublicAddr: "test.example.com",
	})
	require.NoError(t, err)
	tr, err := newTransport(t.Context(), &transportConfig{
		app:                 app,
		publicPort:          "443",
		jwt:                 "test-jwt",
		clusterName:         "example",
		certAuthorityGetter: &emptyCertAuthorityGetter{},
	})
	require.NoError(t, err)
	tr.tr.(*http.Transport).DialContext = dial
	return tr
}

// newTestRequest builds a request with the SessionContext that
// transport.RoundTrip requires.
func newTestRequest(t *testing.T) *http.Request {
	t.Helper()
	app, err := types.NewAppV3(types.Metadata{Name: "test"}, types.AppSpecV3{
		URI: "http://upstream.invalid",
	})
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://upstream.invalid/", nil)
	require.NoError(t, err)
	return common.WithSessionContext(req, &common.SessionContext{
		App:   app,
		Audit: noopAudit{},
	})
}

// fakeUpstream reads one HTTP request, waits delay (or ctx cancel), then
// writes a minimal 200. The ctx hook lets tests cancel the upstream so
// synctest does not report a deadlock when delay exceeds the cap.
func fakeUpstream(ctx context.Context, conn net.Conn, delay time.Duration, body string) {
	defer conn.Close()
	_, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		return
	}
	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return
	}
	_, _ = fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\n\r\n%s", len(body), body)
}

// noopAudit satisfies common.Audit so RoundTrip's audit emission does
// not fail in tests.
type noopAudit struct{}

func (noopAudit) OnSessionStart(context.Context, string, *tlsca.Identity, types.Application) error {
	return nil
}

func (noopAudit) OnSessionEnd(context.Context, string, *tlsca.Identity, types.Application) error {
	return nil
}

func (noopAudit) OnSessionChunk(context.Context, string, string, *tlsca.Identity, types.Application) error {
	return nil
}

func (noopAudit) OnRequest(context.Context, *common.SessionContext, *http.Request, uint32, *common.AWSResolvedEndpoint) error {
	return nil
}

func (noopAudit) OnDynamoDBRequest(context.Context, *common.SessionContext, *http.Request, uint32, *common.AWSResolvedEndpoint) error {
	return nil
}

func (noopAudit) EmitEvent(context.Context, apievents.AuditEvent) error {
	return nil
}

type emptyCertAuthorityGetter struct{}

// GetCertAuthority returns cert authority by id.
func (emptyCertAuthorityGetter) GetCertAuthority(context.Context, types.CertAuthID, bool) (types.CertAuthority, error) {
	return nil, trace.NotFound("certificate authority not found")
}
