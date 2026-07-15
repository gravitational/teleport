// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package llm

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/app/common"
)

// TestHandleRequest covers `handleRequest` function which is responsible for
// "orchestrating" the LLM handling. Instead of verifying provider-specific
// implementations (which are covered on their respective package) we test with
// mocks, ensure the flow is correct and that it generates audit events
// correctly.
func TestHandleRequest(t *testing.T) {
	expectString := func(str string) require.ValueAssertionFunc {
		return func(tt require.TestingT, i1 any, i2 ...any) {
			require.Equal(tt, str, i1, i2...)
		}
	}

	for name, tc := range map[string]struct {
		transport            *http.Transport
		newRequestFunc       func(http.ResponseWriter, *httptest.Server) NewUpstreamRequestFunc
		modifyRecorderFunc   func(*mockUpstreamRecorder)
		writeErrorFunc       WriteErrorFunc
		upstreamBody         string
		expectUpstreamCalled bool
		expectAuditEvent     require.ValueAssertionFunc
		expectedResponse     require.ValueAssertionFunc
	}{
		"success request": {
			newRequestFunc: func(w http.ResponseWriter, s *httptest.Server) NewUpstreamRequestFunc {
				return func(_ types.Application, _ *http.Request) (*http.Request, RequestInfo, error) {
					req, err := http.NewRequest(http.MethodPost, s.URL, nil)
					if err != nil {
						return nil, nil, trace.Wrap(err)
					}

					return req, &mockRequestInfo{
						requestedModel: "requested",
						providerModel:  "provider",
					}, nil
				}
			},
			modifyRecorderFunc: func(r *mockUpstreamRecorder) {
				r.err = nil
				r.inputTokensCount = 10
				r.outputTokensCount = 20
			},
			writeErrorFunc: func(w http.ResponseWriter, err error) error {
				return nil
			},
			upstreamBody:         "REPLY",
			expectUpstreamCalled: true,
			expectAuditEvent: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(tt, i1, i2...)
				evt := i1.(*apievents.AppSessionLLMRequest)
				require.True(tt, evt.Status.Success, i2...)
				require.Equal(tt, "requested", evt.RequestedModel, i2...)
				require.Equal(tt, "provider", evt.Model, i2...)
				require.Equal(tt, int64(10), evt.InputTokenCount, i2...)
				require.Equal(tt, int64(20), evt.OutputTokenCount, i2...)
			},
			expectedResponse: expectString("REPLY"),
		},
		"new request error": {
			newRequestFunc: func(w http.ResponseWriter, s *httptest.Server) NewUpstreamRequestFunc {
				return func(_ types.Application, _ *http.Request) (*http.Request, RequestInfo, error) {
					return nil, nil, trace.BadParameter("invalid request")
				}
			},
			writeErrorFunc: func(w http.ResponseWriter, err error) error {
				_, werr := io.WriteString(w, err.Error())
				return trace.Wrap(werr)
			},
			expectUpstreamCalled: false,
			expectAuditEvent: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(tt, i1, i2...)
				evt := i1.(*apievents.AppSessionLLMRequest)
				require.False(tt, evt.Status.Success, i2...)
				require.Empty(tt, evt.RequestedModel, i2...)
				require.Empty(tt, evt.Model, i2...)
				require.Empty(tt, evt.InputTokenCount, i2...)
				require.Empty(tt, evt.OutputTokenCount, i2...)
			},
			expectedResponse: expectString("invalid request"),
		},
		"new request error with partial info": {
			newRequestFunc: func(w http.ResponseWriter, s *httptest.Server) NewUpstreamRequestFunc {
				return func(_ types.Application, _ *http.Request) (*http.Request, RequestInfo, error) {
					return nil, &mockRequestInfo{
						requestedModel: "requested",
						providerModel:  "provider",
					}, trace.BadParameter("invalid request")
				}
			},
			writeErrorFunc: func(w http.ResponseWriter, err error) error {
				_, werr := io.WriteString(w, err.Error())
				return trace.Wrap(werr)
			},
			expectUpstreamCalled: false,
			expectAuditEvent: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(tt, i1, i2...)
				evt := i1.(*apievents.AppSessionLLMRequest)
				require.False(tt, evt.Status.Success, i2...)
				require.Equal(tt, "requested", evt.RequestedModel, i2...)
				require.Equal(tt, "provider", evt.Model, i2...)
				require.Empty(tt, evt.InputTokenCount, i2...)
				require.Empty(tt, evt.OutputTokenCount, i2...)
			},
			expectedResponse: expectString("invalid request"),
		},
		"successful request with recorder error": {
			newRequestFunc: func(w http.ResponseWriter, s *httptest.Server) NewUpstreamRequestFunc {
				return func(_ types.Application, _ *http.Request) (*http.Request, RequestInfo, error) {
					req, err := http.NewRequest(http.MethodPost, s.URL, nil)
					if err != nil {
						return nil, nil, trace.Wrap(err)
					}

					return req, &mockRequestInfo{
						requestedModel: "requested",
						providerModel:  "provider",
					}, nil
				}
			},
			modifyRecorderFunc: func(r *mockUpstreamRecorder) {
				r.err = trace.AccessDenied("model denied")
			},
			writeErrorFunc: func(w http.ResponseWriter, err error) error {
				return nil
			},
			upstreamBody:         "REPLY",
			expectUpstreamCalled: true,
			expectAuditEvent: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(tt, i1, i2...)
				evt := i1.(*apievents.AppSessionLLMRequest)
				require.False(tt, evt.Status.Success, i2...)
				require.Equal(tt, "requested", evt.RequestedModel, i2...)
				require.Equal(tt, "provider", evt.Model, i2...)
				require.Empty(tt, evt.InputTokenCount, i2...)
				require.Empty(tt, evt.OutputTokenCount, i2...)
			},
			expectedResponse: expectString("REPLY"),
		},
		// This case covers scenarios where upstream is not reachable.
		"upstream forward error": {
			newRequestFunc: func(w http.ResponseWriter, s *httptest.Server) NewUpstreamRequestFunc {
				return func(_ types.Application, _ *http.Request) (*http.Request, RequestInfo, error) {
					req, err := http.NewRequest(http.MethodPost, s.URL, nil)
					if err != nil {
						return nil, nil, trace.Wrap(err)
					}

					return req, &mockRequestInfo{
						requestedModel: "requested",
						providerModel:  "provider",
					}, nil
				}
			},
			modifyRecorderFunc: func(r *mockUpstreamRecorder) {
				r.err = nil
				r.inputTokensCount = 10
				r.outputTokensCount = 20
			},
			writeErrorFunc: func(w http.ResponseWriter, err error) error {
				_, werr := io.WriteString(w, err.Error())
				return trace.Wrap(werr)
			},
			transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return nil, trace.ConnectionProblem(nil, "failed to connect to upstream")
				},
			},
			expectUpstreamCalled: false,
			expectAuditEvent: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(tt, i1, i2...)
				evt := i1.(*apievents.AppSessionLLMRequest)
				require.False(tt, evt.Status.Success, i2...)
				require.Equal(tt, "requested", evt.RequestedModel, i2...)
				require.Equal(tt, "provider", evt.Model, i2...)
				require.Equal(tt, int64(10), evt.InputTokenCount, i2...)
				require.Equal(tt, int64(20), evt.OutputTokenCount, i2...)
			},
			expectedResponse: expectString("failed to connect to upstream"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			var upstreamCalled bool
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				io.WriteString(w, tc.upstreamBody)
				upstreamCalled = true
			}))
			t.Cleanup(mockServer.Close)

			var auditEvent *apievents.AppSessionLLMRequest
			audit := newTestAudit(t, func(pe apievents.PreparedSessionEvent) {
				if evt, ok := pe.GetAuditEvent().(*apievents.AppSessionLLMRequest); ok {
					auditEvent = evt
				}
			})

			h := newTestHandler(t, tc.transport)
			app := newTestApp(t, &types.LLM{Format: types.LLMFormatAnthropic, Provider: types.LLMProviderAnthropic})
			sessionCtx := &common.SessionContext{App: app, Audit: audit}
			req := newTestSessionRequest(
				t,
				http.MethodPost,
				mockServer.URL,
				"/",
				nil,
				sessionCtx,
			)

			w := httptest.NewRecorder()
			h.handleRequest(
				sessionCtx,
				w,
				req,
				tc.newRequestFunc(w, mockServer),
				func(_ *slog.Logger, w http.ResponseWriter) (UpstreamRecorder, error) {
					rec := &mockUpstreamRecorder{ResponseWriter: w}
					tc.modifyRecorderFunc(rec)
					return rec, nil
				},
				tc.writeErrorFunc,
			)
			require.Equal(t, tc.expectUpstreamCalled, upstreamCalled)
			require.NotNil(t, auditEvent)
			tc.expectAuditEvent(t, auditEvent)
			tc.expectedResponse(t, w.Body.String())
		})
	}
}

type mockUpstreamRecorder struct {
	http.ResponseWriter

	err               error
	inputTokensCount  int
	outputTokensCount int
	written           int
}

// Close implements [UpstreamRecorder].
func (m *mockUpstreamRecorder) Close() error {
	return nil
}

// Err implements [UpstreamRecorder].
func (m *mockUpstreamRecorder) Err() error {
	return m.err
}

// InputTokensCount implements [UpstreamRecorder].
func (m *mockUpstreamRecorder) InputTokensCount() int {
	return m.inputTokensCount
}

// OutputTokensCount implements [UpstreamRecorder].
func (m *mockUpstreamRecorder) OutputTokensCount() int {
	return m.outputTokensCount
}

// Written implements [UpstreamRecorder].
func (m *mockUpstreamRecorder) Written() int {
	return m.written
}

type mockRequestInfo struct {
	RequestInfo

	requestedModel string
	providerModel  string
	stream         bool
	requestSize    int
}

func (r *mockRequestInfo) RequestedModel() string { return r.requestedModel }
func (r *mockRequestInfo) ProviderModel() string  { return r.providerModel }
func (r *mockRequestInfo) IsStream() bool         { return r.stream }
func (r *mockRequestInfo) RequestSize() int       { return r.requestSize }

// streamRecorder adapts an apievents.Stream to an events.SessionRecorder
// by adding a no-op io.Writer (required by the interface).
type streamRecorder struct {
	apievents.Stream
}

func (s *streamRecorder) Write(p []byte) (int, error) { return len(p), nil }

// newTestAudit creates a common.Audit that calls onRecord for each recorded event.
func newTestAudit(t *testing.T, onRecord func(apievents.PreparedSessionEvent)) common.Audit {
	t.Helper()
	streamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
		Inner: events.NewDiscardStreamer(),
		OnRecordEvent: func(_ context.Context, _ session.ID, pe apievents.PreparedSessionEvent) error {
			onRecord(pe)
			return nil
		},
	})
	require.NoError(t, err)
	stream, err := streamer.CreateAuditStream(t.Context(), "test-session")
	require.NoError(t, err)
	audit, err := common.NewAudit(common.AuditConfig{
		Emitter:  events.NewDiscardEmitter(),
		Recorder: events.WithNoOpPreparer(&streamRecorder{stream}),
	})
	require.NoError(t, err)
	return audit
}

// newTestApp creates a types.Application configured with the given LLM options.
func newTestApp(t *testing.T, llm *types.LLM) types.Application {
	t.Helper()
	app, err := types.NewAppV3(types.Metadata{
		Name: "test-llm-app",
	}, types.AppSpecV3{
		LLM: llm,
	})
	require.NoError(t, err)
	return app
}

// newTestSessionRequest creates an *http.Request with a SessionContext attached.
func newTestSessionRequest(t *testing.T, method, addr string, path string, body io.Reader, sessionCtx *common.SessionContext) *http.Request {
	t.Helper()
	target, err := url.JoinPath(addr, path)
	require.NoError(t, err)
	req := httptest.NewRequest(method, target, body)
	return common.WithSessionContext(req, sessionCtx)
}

func newTestHandler(t *testing.T, tr *http.Transport) *Handler {
	t.Helper()
	h, err := NewHandler(t.Context(), HandlerConfig{
		Log:       slog.Default(),
		Transport: tr,
	})
	require.NoError(t, err)
	return h
}
