/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package bigquery

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

func TestResolveEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		uri          string
		wantEndpoint string
	}{
		{
			name:         "empty URI defaults to BigQuery API",
			uri:          "",
			wantEndpoint: "https://bigquery.googleapis.com",
		},
		{
			name:         "host:port gets HTTPS prefix",
			uri:          "bigquery.googleapis.com:443",
			wantEndpoint: "https://bigquery.googleapis.com:443",
		},
		{
			name:         "full HTTPS URL passthrough",
			uri:          "https://custom-bigquery.example.com",
			wantEndpoint: "https://custom-bigquery.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint := resolveEndpoint(tt.uri)
			require.Equal(t, tt.wantEndpoint, endpoint)
		})
	}
}

func TestDatabaseUserToGCPServiceAccount(t *testing.T) {
	t.Parallel()

	db, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-bigquery",
	}, types.DatabaseSpecV3{
		Protocol: "bigquery",
		URI:      "bigquery.googleapis.com:443",
		GCP:      types.GCPCloudSQL{ProjectID: "my-project"},
	})
	require.NoError(t, err)

	tests := []struct {
		name         string
		databaseUser string
		want         string
	}{
		{
			name:         "simple user",
			databaseUser: "bq-reader",
			want:         "bq-reader@my-project.iam.gserviceaccount.com",
		},
		{
			name:         "already fully qualified",
			databaseUser: "svc",
			want:         "svc@my-project.iam.gserviceaccount.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionCtx := &common.Session{
				Database:     db,
				DatabaseUser: tt.databaseUser,
			}
			got := databaseUserToGCPServiceAccount(sessionCtx)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestExtractQuery(t *testing.T) {
	t.Parallel()

	e := &Engine{}

	tests := []struct {
		name  string
		path  string
		body  map[string]interface{}
		want  string
	}{
		{
			name: "jobs.query endpoint",
			path: "/bigquery/v2/projects/my-project/queries",
			body: map[string]interface{}{
				"query": "SELECT 1 as test",
			},
			want: "SELECT 1 as test",
		},
		{
			name: "jobs.insert endpoint with configuration",
			path: "/bigquery/v2/projects/my-project/jobs",
			body: map[string]interface{}{
				"configuration": map[string]interface{}{
					"query": map[string]interface{}{
						"query": "SELECT * FROM dataset.table",
					},
				},
			},
			want: "SELECT * FROM dataset.table",
		},
		{
			name: "non-query endpoint returns empty",
			path: "/bigquery/v2/projects/my-project/datasets",
			body: map[string]interface{}{
				"query": "SELECT 1",
			},
			want: "",
		},
		{
			name: "no query field returns empty",
			path: "/bigquery/v2/projects/my-project/queries",
			body: map[string]interface{}{
				"useLegacySql": false,
			},
			want: "",
		},
		{
			name: "invalid body returns empty",
			path: "/bigquery/v2/projects/my-project/queries",
			body: nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.body != nil {
				body, _ = json.Marshal(tt.body)
			} else {
				body = []byte("not json")
			}
			got := e.extractQuery(tt.path, body)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSendError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   bool
	}{
		{
			name:       "trace not found maps to 404",
			err:        trace.NotFound("db not found"),
			wantStatus: http.StatusNotFound,
			wantBody:   true,
		},
		{
			name:       "trace access denied maps to 403",
			err:        trace.AccessDenied("unauthorized"),
			wantStatus: http.StatusForbidden,
			wantBody:   true,
		},
		{
			name:       "generic error maps to 500",
			err:        trace.Errorf("something broke"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientConn, serverConn := net.Pipe()
			defer clientConn.Close()
			defer serverConn.Close()

			engine := &Engine{
				EngineConfig: common.EngineConfig{
					Context: context.Background(),
					Log:     slog.Default(),
				},
				clientConn: serverConn,
			}

			go engine.SendError(tt.err)

			resp, err := http.ReadResponse(bufio.NewReader(clientConn), nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, tt.wantStatus, resp.StatusCode)
			require.Equal(t, "application/json", resp.Header.Get("Content-Type"))

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var errResp map[string]interface{}
			require.NoError(t, json.Unmarshal(body, &errResp))
			errorObj, ok := errResp["error"].(map[string]interface{})
			require.True(t, ok, "expected 'error' object in response")
			require.NotEmpty(t, errorObj["message"])
			require.NotZero(t, errorObj["code"])
		})
	}
}

func TestSendErrorNilConn(t *testing.T) {
	t.Parallel()

	engine := &Engine{
		EngineConfig: common.EngineConfig{
			Context: context.Background(),
			Log:     slog.Default(),
		},
		clientConn: nil,
	}
	// Should not panic.
	engine.SendError(trace.AccessDenied("test"))
}

func TestSendErrorNilErr(t *testing.T) {
	t.Parallel()

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	engine := &Engine{
		EngineConfig: common.EngineConfig{
			Context: context.Background(),
			Log:     slog.Default(),
		},
		clientConn: serverConn,
	}
	// Should not write anything for nil error.
	engine.SendError(nil)
}

// TestRoundTripBodyLimit verifies that the audit peek (capped at maxAuditBytes) does not
// truncate the request body forwarded to the BigQuery upstream — the full body must be sent.
func TestRoundTripBodyLimit(t *testing.T) {
	t.Parallel()

	fullSize := maxAuditBytes + 1024
	var receivedBodyLen int
	upstream := startTestUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBodyLen = len(body)
		w.WriteHeader(http.StatusOK)
	})

	targetURL, err := url.Parse(upstream.URL)
	require.NoError(t, err)

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	e := newTestEngine(serverConn, &fakeTokenSource{token: "tok"})

	largeBody := strings.Repeat("x", fullSize)
	req, err := http.NewRequest(http.MethodPost, upstream.URL+"/bigquery/v2/projects/p/queries",
		strings.NewReader(largeBody))
	require.NoError(t, err)

	go io.Copy(io.Discard, clientConn)

	err = e.roundTrip(context.Background(), &http.Client{Transport: http.DefaultTransport}, req, targetURL, testMsgCounter())
	require.NoError(t, err)
	// Upstream must receive the full body even though the audit only reads the first maxAuditBodySize bytes.
	require.Equal(t, fullSize, receivedBodyLen)
}

// newTestEngine creates a minimal Engine wired to serverConn for use in roundTrip tests.
func newTestEngine(serverConn net.Conn, ts oauth2.TokenSource) *Engine {
	return &Engine{
		tokenSource: ts,
		clientConn:  serverConn,
		sessionCtx:  &common.Session{},
		EngineConfig: common.EngineConfig{
			Log:   slog.Default(),
			Audit: &noopAuditor{},
		},
	}
}

// testMsgCounter returns a new unregistered prometheus counter for use in roundTrip test calls.
func testMsgCounter() prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{Name: "test_msg"})
}

// TestRoundTripBearerTokenSent verifies that the Bearer token from tokenSource
// reaches the upstream BigQuery API on each request.
func TestRoundTripBearerTokenSent(t *testing.T) {
	t.Parallel()

	var receivedAuth string
	upstream := startTestUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"kind":"bigquery#queryResponse","jobComplete":true}`))
	})

	targetURL, err := url.Parse(upstream.URL)
	require.NoError(t, err)

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	e := newTestEngine(serverConn, &fakeTokenSource{token: "test-access-token-12345"})

	req, err := http.NewRequest(http.MethodGet, upstream.URL+"/bigquery/v2/projects/p/datasets", nil)
	require.NoError(t, err)

	go io.Copy(io.Discard, clientConn)

	err = e.roundTrip(context.Background(), &http.Client{Transport: http.DefaultTransport}, req, targetURL, testMsgCounter())
	require.NoError(t, err)
	require.Equal(t, "Bearer test-access-token-12345", receivedAuth)
}

// TestRoundTripTokenError verifies that a token fetch failure is returned
// directly as an error — the upstream is never called.
func TestRoundTripTokenError(t *testing.T) {
	t.Parallel()

	var upstreamCalled bool
	upstream := startTestUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		w.WriteHeader(http.StatusOK)
	})

	targetURL, err := url.Parse(upstream.URL)
	require.NoError(t, err)

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	tokenErr := trace.AccessDenied("IAM permission denied: cannot generate access token")
	e := newTestEngine(serverConn, &fakeTokenSource{err: tokenErr})

	req, err := http.NewRequest(http.MethodPost, upstream.URL+"/bigquery/v2/projects/p/queries",
		strings.NewReader(`{"query":"SELECT 1"}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	err = e.roundTrip(context.Background(), &http.Client{Transport: http.DefaultTransport}, req, targetURL, testMsgCounter())
	require.ErrorContains(t, err, "IAM permission denied")
	require.False(t, upstreamCalled, "upstream must not be called when token fails")
}

// TestRoundTripForwardedHeadersStripped verifies that X-Forwarded-* headers
// are removed before the request reaches BigQuery.
func TestRoundTripForwardedHeadersStripped(t *testing.T) {
	t.Parallel()

	var receivedHeaders http.Header
	upstream := startTestUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	})

	targetURL, err := url.Parse(upstream.URL)
	require.NoError(t, err)

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	e := newTestEngine(serverConn, &fakeTokenSource{token: "tok"})

	req, err := http.NewRequest(http.MethodGet, upstream.URL+"/bigquery/v2/projects/p/datasets", nil)
	require.NoError(t, err)
	req.Header.Set("X-Forwarded-For", "attacker.com")
	req.Header.Set("X-Forwarded-Host", "evil.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")

	go io.Copy(io.Discard, clientConn)

	err = e.roundTrip(context.Background(), &http.Client{Transport: http.DefaultTransport}, req, targetURL, testMsgCounter())
	require.NoError(t, err)
	require.Empty(t, receivedHeaders.Get("X-Forwarded-For"))
	require.Empty(t, receivedHeaders.Get("X-Forwarded-Host"))
	require.Empty(t, receivedHeaders.Get("X-Forwarded-Proto"))
}

// startTestUpstream starts a plain HTTP server and registers cleanup.
func startTestUpstream(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// fakeTokenSource implements oauth2.TokenSource for tests.
type fakeTokenSource struct {
	token string
	err   error
}

func (f *fakeTokenSource) Token() (*oauth2.Token, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &oauth2.Token{AccessToken: f.token}, nil
}

// noopAuditor implements common.Audit for tests.
type noopAuditor struct{}

var _ common.Audit = (*noopAuditor)(nil)

func (n *noopAuditor) OnSessionStart(_ context.Context, _ *common.Session, _ error)                  {}
func (n *noopAuditor) OnSessionEnd(_ context.Context, _ *common.Session)                              {}
func (n *noopAuditor) OnQuery(_ context.Context, _ *common.Session, _ common.Query)                   {}
func (n *noopAuditor) OnResult(_ context.Context, _ *common.Session, _ common.Result)                 {}
func (n *noopAuditor) EmitEvent(_ context.Context, _ apievents.AuditEvent)                            {}
func (n *noopAuditor) RecordEvent(_ context.Context, _ apievents.AuditEvent)                          {}
func (n *noopAuditor) OnPermissionsUpdate(_ context.Context, _ *common.Session, _ []apievents.DatabasePermissionEntry) {}
func (n *noopAuditor) OnDatabaseUserCreate(_ context.Context, _ *common.Session, _ error)             {}
func (n *noopAuditor) OnDatabaseUserDeactivate(_ context.Context, _ *common.Session, _ bool, _ error) {}
