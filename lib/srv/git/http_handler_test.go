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

package git

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	gitserverv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/gitserver/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	testAccessToken  = "ghu_test_token_abc123"
	testGitServer    = "github-my-org"
	testOrg          = "my-org"
	testIntegration  = "github-my-org"
	testSessionID    = "test-session-id-123"
	testUsername      = "alice"
)

type fakeCredentialsClient struct {
	token string
	err   error
}

func (f *fakeCredentialsClient) GenerateGitHubAppToken(_ context.Context, _ *gitserverv1.GenerateGitHubAppTokenRequest, _ ...grpc.CallOption) (*gitserverv1.GenerateGitHubAppTokenResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	resp := gitserverv1.GenerateGitHubAppTokenResponse_builder{
		AccessToken: f.token,
	}.Build()
	return resp, nil
}

type fakeGitServerGetter struct{}

func (f *fakeGitServerGetter) GetGitServer(_ context.Context, name string) (types.Server, error) {
	server, err := types.NewGitHubServer(types.GitHubServerMetadata{
		Organization: testOrg,
		Integration:  testIntegration,
	})
	if err != nil {
		return nil, err
	}
	return server, nil
}

type capturedEvent struct {
	event apievents.AuditEvent
}

type fakeEmitter struct {
	mu     sync.Mutex
	events []capturedEvent
}

func (f *fakeEmitter) EmitAuditEvent(_ context.Context, event apievents.AuditEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, capturedEvent{event: event})
	return nil
}

func (f *fakeEmitter) getEvents() []capturedEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]capturedEvent, len(f.events))
	copy(out, f.events)
	return out
}

type capturedUpstreamRequest struct {
	method string
	host   string
	path   string
	auth   string
}

type fakeTransport struct {
	mu       sync.Mutex
	requests []capturedUpstreamRequest
	status   int
	body     string
}

func (f *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, capturedUpstreamRequest{
		method: req.Method,
		host:   req.URL.Host,
		path:   req.URL.Path,
		auth:   req.Header.Get("Authorization"),
	})
	status := f.status
	if status == 0 {
		status = http.StatusOK
	}
	body := f.body
	if body == "" {
		body = "ok"
	}
	return &http.Response{
		StatusCode: status,
		Body:       http.NoBody,
		Header:     make(http.Header),
	}, nil
}

func (f *fakeTransport) getRequests() []capturedUpstreamRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]capturedUpstreamRequest, len(f.requests))
	copy(out, f.requests)
	return out
}

type fakeSessionRecorder struct {
	mu     sync.Mutex
	events []apievents.AuditEvent
	*events.DiscardRecorder
}

func newFakeSessionRecorder() *fakeSessionRecorder {
	return &fakeSessionRecorder{DiscardRecorder: events.NewDiscardRecorder()}
}

func (r *fakeSessionRecorder) PrepareSessionEvent(event apievents.AuditEvent) (apievents.PreparedSessionEvent, error) {
	return &preparedEvent{event: event}, nil
}

func (r *fakeSessionRecorder) RecordEvent(_ context.Context, pe apievents.PreparedSessionEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, pe.GetAuditEvent())
	return nil
}

func (r *fakeSessionRecorder) getEvents() []apievents.AuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]apievents.AuditEvent, len(r.events))
	copy(out, r.events)
	return out
}

type preparedEvent struct {
	event apievents.AuditEvent
}

func (p *preparedEvent) GetAuditEvent() apievents.AuditEvent {
	return p.event
}

type testHandlerSetup struct {
	handler  *HTTPHandler
	emitter  *fakeEmitter
	upstream *fakeTransport
	recorder *fakeSessionRecorder
}

func newTestHandler(t *testing.T, transport *fakeTransport, emitter *fakeEmitter) *testHandlerSetup {
	t.Helper()

	if transport == nil {
		transport = &fakeTransport{}
	}
	if emitter == nil {
		emitter = &fakeEmitter{}
	}

	recorder := newFakeSessionRecorder()

	handler, err := NewHTTPHandler(context.Background(), HTTPHandlerConfig{
		GitCredentialsClient: &fakeCredentialsClient{token: testAccessToken},
		GitServerGetter:      &fakeGitServerGetter{},
		Emitter:              emitter,
		HostID:               "test-proxy-id",
		ClusterName:          "test.teleport.sh",
		Transport:            transport,
		NewSessionRecorder: func(_ context.Context, _ string) (events.SessionPreparerRecorder, error) {
			return recorder, nil
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { handler.Close() })
	return &testHandlerSetup{
		handler:  handler,
		emitter:  emitter,
		upstream: transport,
		recorder: recorder,
	}
}

func makeTestCert(t *testing.T, identity tlsca.Identity) *x509.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	subject, err := identity.Subject()
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      subject,
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(1 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)
	return cert
}

func makeGitRequest(t *testing.T, method, host, path string, identity *tlsca.Identity) *http.Request {
	t.Helper()

	req := httptest.NewRequest(method, "https://"+host+path, nil)
	req.Host = host

	if identity != nil {
		cert := makeTestCert(t, *identity)
		req.TLS = &tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{cert},
		}
	}
	return req
}

func defaultGitIdentity() tlsca.Identity {
	return tlsca.Identity{
		Username:       testUsername,
		Groups:         []string{"access"},
		TeleportCluster: "test.teleport.sh",
		RouteToGit: tlsca.RouteToGit{
			GitServerName: testGitServer,
			SessionID:     testSessionID,
		},
	}
}

func TestHTTPHandler_GitClone(t *testing.T) {
	t.Parallel()
	transport := &fakeTransport{}
	emitter := &fakeEmitter{}
	setup := newTestHandler(t, transport, emitter)
	handler := setup.handler

	identity := defaultGitIdentity()

	// Discovery request (GET /info/refs).
	req := makeGitRequest(t, "GET", "github.com", "/my-org/my-repo.git/info/refs?service=git-upload-pack", &identity)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	upstream := transport.getRequests()
	require.Len(t, upstream, 1)
	assert.Equal(t, "github.com", upstream[0].host)
	assert.Equal(t, "/my-org/my-repo.git/info/refs", upstream[0].path)
	// github.com should use Basic auth with x-access-token.
	assert.Contains(t, upstream[0].auth, "Basic")

	// Upload-pack request (POST).
	req2 := makeGitRequest(t, "POST", "github.com", "/my-org/my-repo.git/git-upload-pack", &identity)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)

	upstream2 := transport.getRequests()
	require.Len(t, upstream2, 2)
	assert.Contains(t, upstream2[1].auth, "Basic")

	// Should have emitted a GitCommand event for the POST (non-discovery).
	evts := emitter.getEvents()
	var gitCmdEvents []*apievents.GitCommand
	for _, e := range evts {
		if cmd, ok := e.event.(*apievents.GitCommand); ok {
			gitCmdEvents = append(gitCmdEvents, cmd)
		}
	}
	require.NotEmpty(t, gitCmdEvents)
	assert.Equal(t, events.GitCommandCode, gitCmdEvents[0].Code)
	assert.Equal(t, "git-upload-pack", gitCmdEvents[0].Service)
}

func TestHTTPHandler_GitCloneFailure(t *testing.T) {
	t.Parallel()
	transport := &fakeTransport{status: http.StatusForbidden}
	emitter := &fakeEmitter{}
	setup := newTestHandler(t, transport, emitter)
	handler := setup.handler

	identity := defaultGitIdentity()

	// Discovery request that returns 403.
	req := makeGitRequest(t, "GET", "github.com", "/my-org/denied-repo.git/info/refs?service=git-upload-pack", &identity)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)

	// Failed discovery should emit a failure GitCommand event.
	evts := emitter.getEvents()
	var failureEvents []*apievents.GitCommand
	for _, e := range evts {
		if cmd, ok := e.event.(*apievents.GitCommand); ok && cmd.Code == events.GitCommandFailureCode {
			failureEvents = append(failureEvents, cmd)
		}
	}
	require.NotEmpty(t, failureEvents)
	assert.Equal(t, uint32(http.StatusForbidden), failureEvents[0].HttpStatusCode)
}

func TestHTTPHandler_APIRequest(t *testing.T) {
	t.Parallel()
	transport := &fakeTransport{}
	emitter := &fakeEmitter{}
	setup := newTestHandler(t, transport, emitter)
	handler := setup.handler

	identity := defaultGitIdentity()

	req := makeGitRequest(t, "GET", "api.github.com", "/user", &identity)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	upstream := transport.getRequests()
	require.Len(t, upstream, 1)
	assert.Equal(t, "api.github.com", upstream[0].host)
	// api.github.com should use Bearer token auth.
	assert.Equal(t, "token "+testAccessToken, upstream[0].auth)
}

func TestHTTPHandler_GraphQLCapture(t *testing.T) {
	t.Parallel()
	transport := &fakeTransport{}
	emitter := &fakeEmitter{}
	setup := newTestHandler(t, transport, emitter)
	handler := setup.handler

	identity := defaultGitIdentity()
	graphqlBody := `{"query":"{ viewer { login } }"}`

	req := makeGitRequest(t, "POST", "api.github.com", "/graphql", &identity)
	req.Body = io.NopCloser(strings.NewReader(graphqlBody))
	req.ContentLength = int64(len(graphqlBody))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// The GraphQL query should be captured in a session recording event.
	evts := emitter.getEvents()
	var found bool
	for _, e := range evts {
		if chunk, ok := e.event.(*apievents.GitSessionChunk); ok {
			assert.Equal(t, testSessionID, chunk.SessionID)
			found = true
		}
	}
	assert.True(t, found, "expected GitSessionChunk event")
}

func TestHTTPHandler_UnsupportedHost(t *testing.T) {
	t.Parallel()
	setup := newTestHandler(t, nil, nil)
	handler := setup.handler
	identity := defaultGitIdentity()

	req := makeGitRequest(t, "GET", "evil.com", "/some/path", &identity)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestHTTPHandler_MissingCert(t *testing.T) {
	t.Parallel()
	setup := newTestHandler(t, nil, nil)
	handler := setup.handler

	// No TLS state at all.
	req := makeGitRequest(t, "GET", "github.com", "/my-org/my-repo.git/info/refs?service=git-upload-pack", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestHTTPHandler_MissingRouteToGit(t *testing.T) {
	t.Parallel()
	setup := newTestHandler(t, nil, nil)
	handler := setup.handler

	// Identity with no RouteToGit.
	identity := tlsca.Identity{
		Username:        testUsername,
		Groups:          []string{"access"},
		TeleportCluster: "test.teleport.sh",
	}
	req := makeGitRequest(t, "GET", "github.com", "/my-org/my-repo.git/info/refs?service=git-upload-pack", &identity)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestHTTPHandler_BrowserBlocked(t *testing.T) {
	t.Parallel()
	setup := newTestHandler(t, nil, nil)
	handler := setup.handler
	identity := defaultGitIdentity()

	req := makeGitRequest(t, "GET", "github.com", "/my-org/my-repo.git/info/refs?service=git-upload-pack", &identity)
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestHTTPHandler_InvalidGitPath(t *testing.T) {
	t.Parallel()
	setup := newTestHandler(t, nil, nil)
	handler := setup.handler
	identity := defaultGitIdentity()

	req := makeGitRequest(t, "GET", "github.com", "/my-org/my-repo/tree/main", &identity)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}
