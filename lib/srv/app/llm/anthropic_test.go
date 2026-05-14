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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"github.com/anthropics/anthropic-sdk-go/shared"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream/eventstreamapi"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/app/common"
)

type anthropicProviderRequest struct {
	Model string `json:"model"`
}

func TestHandleAnthropic(t *testing.T) {
	for name, tc := range map[string]struct {
		reqBody               string
		respStatus            int
		respBody              string
		configureLLM          func(*types.LLM)
		expectedStatus        int
		expectProviderRequest require.ValueAssertionFunc
		expectAuditEvent      require.ValueAssertionFunc
		expectedBody          require.ValueAssertionFunc
	}{
		"success non-streaming": {
			reqBody:        `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello"}]}`,
			respStatus:     http.StatusOK,
			respBody:       `{"id":"msg_123","type":"message","content":[{"type":"text","text":"Hello"}]}`,
			expectedStatus: http.StatusOK,
			expectProviderRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(t, i1, i2...)
				req := i1.(*anthropicProviderRequest)
				require.Equal(tt, "claude-sonnet-4-20250514", req.Model, i2...)
			},
			expectAuditEvent: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(t, i1, i2...)
				evt := i1.(*apievents.AppSessionLLMRequest)
				require.True(tt, evt.Status.Success, i2...)
			},
			expectedBody: func(tt require.TestingT, i1 any, i2 ...any) {
				require.JSONEq(tt, `{"id":"msg_123","type":"message","content":[{"type":"text","text":"Hello"}]}`, i1.(string), i2...)
			},
		},
		"api invalid request includes original message": {
			reqBody:               `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello"}]}`,
			respStatus:            http.StatusBadRequest,
			respBody:              `{"error":{"type":"invalid_request_error","message":"invalid model"}}`,
			expectedStatus:        http.StatusBadRequest,
			expectProviderRequest: require.NotNil,
			expectAuditEvent: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(t, i1, i2...)
				evt := i1.(*apievents.AppSessionLLMRequest)
				require.False(tt, evt.Status.Success, i2...)
			},
			expectedBody: func(tt require.TestingT, i1 any, i2 ...any) {
				// expects Anthropic-compatible error message.
				bodyStr, _ := i1.(string)
				anthropicErr := requireAnthropicError(tt, bodyStr, i2...)
				require.Equal(tt, "invalid_request_error", string(anthropicErr.Type), i2...)
				require.Contains(tt, anthropicErr.Message, "invalid model", i2...)
			},
		},
		"auth error 401 rewrites message": {
			reqBody:               `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello"}]}`,
			respStatus:            http.StatusUnauthorized,
			respBody:              `{"error":{"type":"authentication_error","message":"original provider secret"}}`,
			expectedStatus:        http.StatusUnauthorized,
			expectProviderRequest: require.NotNil,
			expectAuditEvent: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(t, i1, i2...)
				evt := i1.(*apievents.AppSessionLLMRequest)
				require.False(tt, evt.Status.Success, i2...)
				require.NotContains(tt, evt.Status.Error, "original provider secret", i2...)
			},
			expectedBody: func(tt require.TestingT, i1 any, i2 ...any) {
				// expects Anthropic-compatible error message.
				bodyStr, _ := i1.(string)
				anthropicErr := requireAnthropicError(tt, bodyStr, i2...)
				require.Equal(tt, "authentication_error", string(anthropicErr.Type), i2...)
				require.Contains(tt, anthropicErr.Message, "Teleport", i2...)
				require.NotContains(tt, anthropicErr.Message, "original provider secret", i2...)
			},
		},
		"rewrites configured model and audit": {
			reqBody:    `{"model":"claude-sonnet","messages":[{"role":"user","content":"Hello"}]}`,
			respStatus: http.StatusOK,
			respBody:   `{"id":"msg_123","type":"message"}`,
			configureLLM: func(llm *types.LLM) {
				llm.Models = []*types.LLM_Model{
					{Name: "claude-sonnet", ProviderName: "provider-sonnet"},
					{Name: "claude-haiku", ProviderName: "provider-haiku"},
				}
			},
			expectedStatus: http.StatusOK,
			expectProviderRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(t, i1, i2...)
				req := i1.(*anthropicProviderRequest)
				require.Equal(tt, "provider-sonnet", req.Model, i2...)
			},
			expectAuditEvent: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(t, i1, i2...)
				evt := i1.(*apievents.AppSessionLLMRequest)
				require.True(tt, evt.Status.Success, i2...)
				require.Equal(tt, "provider-sonnet", evt.Model, i2...)
				require.Equal(tt, "claude-sonnet", evt.RequestedModel, i2...)
			},
			expectedBody: func(tt require.TestingT, i1 any, i2 ...any) {
				require.JSONEq(tt, `{"id":"msg_123","type":"message"}`, i1.(string), i2...)
			},
		},
		"uses fallback model and audits requested model": {
			reqBody:    `{"model":"unknown-model","messages":[{"role":"user","content":"Hello"}]}`,
			respStatus: http.StatusOK,
			respBody:   `{"id":"msg_123","type":"message"}`,
			configureLLM: func(llm *types.LLM) {
				llm.Models = []*types.LLM_Model{
					{Name: "claude-sonnet", ProviderName: "provider-sonnet"},
					{Name: "claude-haiku", ProviderName: "provider-haiku"},
				}
				llm.FallbackModel = "claude-haiku"
			},
			expectedStatus: http.StatusOK,
			expectProviderRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(t, i1, i2...)
				req := i1.(*anthropicProviderRequest)
				require.Equal(tt, "provider-haiku", req.Model, i2...)
			},
			expectAuditEvent: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(t, i1, i2...)
				evt := i1.(*apievents.AppSessionLLMRequest)
				require.True(tt, evt.Status.Success, i2...)
				require.Equal(tt, "provider-haiku", evt.Model, i2...)
				require.Equal(tt, "unknown-model", evt.RequestedModel, i2...)
			},
			expectedBody: func(tt require.TestingT, i1 any, i2 ...any) {
				require.JSONEq(tt, `{"id":"msg_123","type":"message"}`, i1.(string), i2...)
			},
		},
		"rejects unsupported model": {
			reqBody:    `{"model":"denied-model","messages":[{"role":"user","content":"Hello"}]}`,
			respStatus: http.StatusOK,
			configureLLM: func(llm *types.LLM) {
				llm.Models = []*types.LLM_Model{
					{Name: "allowed-model", ProviderName: "provider-model"},
				}
			},
			expectedStatus:        http.StatusBadRequest,
			expectProviderRequest: require.Nil,
			expectAuditEvent: func(tt require.TestingT, i1 any, i2 ...any) {
				require.NotNil(t, i1, i2...)
				evt := i1.(*apievents.AppSessionLLMRequest)
				require.False(tt, evt.Status.Success, i2...)
				require.Equal(tt, "denied-model", evt.RequestedModel, i2...)
			},
			expectedBody: func(tt require.TestingT, i1 any, i2 ...any) {
				anthropicErr := requireAnthropicError(tt, i1.(string), i2...)
				require.Equal(tt, "invalid_request_error", string(anthropicErr.Type), i2...)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			var providerReq *anthropicProviderRequest
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				providerReq = &anthropicProviderRequest{}
				require.NoError(t, json.NewDecoder(r.Body).Decode(providerReq))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.respStatus)
				io.WriteString(w, tc.respBody)
			}))
			t.Cleanup(mockServer.Close)
			t.Setenv("ANTHROPIC_BASE_URL", mockServer.URL)

			var auditEvent *apievents.AppSessionLLMRequest
			audit := newTestAudit(t, func(pe apievents.PreparedSessionEvent) {
				if evt, ok := pe.GetAuditEvent().(*apievents.AppSessionLLMRequest); ok {
					auditEvent = evt
				}
			})

			h := newTestHandler(t)
			app := newTestApp(t, &types.LLM{Format: types.LLMFormatAnthropic, Provider: types.LLMProviderAnthropic})
			if tc.configureLLM != nil {
				tc.configureLLM(app.GetLLM())
			}
			sessionCtx := &common.SessionContext{App: app, Audit: audit}
			req := newTestSessionRequest(
				t,
				http.MethodPost,
				"/v1/messages",
				strings.NewReader(tc.reqBody),
				sessionCtx,
			)

			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			require.Equal(t, tc.expectedStatus, w.Code)
			tc.expectedBody(t, w.Body.String())
			tc.expectProviderRequest(t, providerReq)
			require.NotNil(t, auditEvent)
			tc.expectAuditEvent(t, auditEvent)
		})
	}
}

func TestHandleAnthropicStreaming(t *testing.T) {
	sseBody := "event: message_start\ndata: {\"type\":\"message_start\"}\n\nevent: message_stop\ndata: {}\n\n"
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, sseBody)
	}))
	t.Cleanup(mockServer.Close)
	t.Setenv("ANTHROPIC_BASE_URL", mockServer.URL)

	var (
		emittedAuditEvent bool
		statusSuccess     bool
	)
	audit := newTestAudit(t, func(pe apievents.PreparedSessionEvent) {
		if evt, ok := pe.GetAuditEvent().(*apievents.AppSessionLLMRequest); ok {
			emittedAuditEvent = true
			statusSuccess = evt.Status.Success
		}
	})

	h := newTestHandler(t)
	app := newTestApp(t, &types.LLM{Format: types.LLMFormatAnthropic, Provider: types.LLMProviderAnthropic})
	sessionCtx := &common.SessionContext{App: app, Audit: audit}
	req := newTestSessionRequest(
		t,
		http.MethodPost,
		"/v1/messages",
		strings.NewReader(
			`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello"}],"stream":true}`,
		),
		sessionCtx,
	)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "event: message_start")
	require.Contains(t, w.Body.String(), "event: message_stop")
	require.True(t, emittedAuditEvent)
	require.True(t, statusSuccess)
}

func TestAnthropicStreamEvents(t *testing.T) {
	for name, tc := range map[string]struct {
		input          string
		expectedErr    require.ErrorAssertionFunc
		expectedOutput require.ValueAssertionFunc
	}{
		"single event": {
			input:       "event: message_start\ndata: {\"type\":\"message_start\"}\n\n",
			expectedErr: require.NoError,
			expectedOutput: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(tt, "event: message_start\ndata: {\"type\":\"message_start\"}\n\n", i1, i2...)
			},
		},
		"multiple events": {
			input:       "event: message_start\ndata: {\"type\":\"message_start\"}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
			expectedErr: require.NoError,
			expectedOutput: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(tt, "event: message_start\ndata: {\"type\":\"message_start\"}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", i1, i2...)
			},
		},
		"multi-line data": {
			input:       "event: content_block_delta\ndata: line1\ndata: line2\n\n",
			expectedErr: require.NoError,
			expectedOutput: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(tt, "event: content_block_delta\ndata: line1\ndata: line2\n\n", i1, i2...)
			},
		},
		"stream with errors": {
			input:       "event: message_start\ndata: {\"type\":\"message_start\"}\n\nevent: error\ndata: {\"type\": \"error\", \"error\": {\"type\": \"overloaded_error\", \"message\": \"Overloaded\"}}\n\n",
			expectedErr: require.Error,
			expectedOutput: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(tt, "event: message_start\ndata: {\"type\":\"message_start\"}\n\nevent: error\ndata: {\"type\":\"error\",\"error\":{\"message\":\"The inference provider returned an unexpected error. Contact your Teleport administrator\",\"type\":\"api_error\"}}\n\n", i1, i2...)
			},
		},
		"empty stream": {
			input:          "",
			expectedErr:    require.NoError,
			expectedOutput: require.Empty,
		},
	} {
		t.Run(name, func(t *testing.T) {
			resp := &http.Response{
				Header: http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:   io.NopCloser(strings.NewReader(tc.input)),
			}
			w := httptest.NewRecorder()
			written, err := streamAnthropicSSEEvents(t.Context(), slog.Default(), w, resp, &mockReporter{})
			tc.expectedErr(t, err)
			tc.expectedOutput(t, w.Body.String())
			require.Equal(t, int64(w.Body.Len()), written)
		})
	}
}

func TestAnthropicStreamBedrockEvents(t *testing.T) {
	enc := eventstream.NewEncoder()

	eventMessageTypeHeader := eventstream.Header{Name: eventstreamapi.MessageTypeHeader, Value: eventstream.StringValue(eventstreamapi.EventMessageType)}
	eventTypeChunkHeader := eventstream.Header{Name: eventstreamapi.EventTypeHeader, Value: eventstream.StringValue("chunk")}
	exceptionMessageTypeHeader := eventstream.Header{Name: eventstreamapi.MessageTypeHeader, Value: eventstream.StringValue(eventstreamapi.ExceptionMessageType)}
	exceptionTypeHeader := eventstream.Header{Name: eventstreamapi.ExceptionTypeHeader, Value: eventstream.StringValue("overloaded_error")}

	var singleEvent bytes.Buffer
	require.NoError(t, enc.Encode(&singleEvent, eventstream.Message{
		Headers: eventstream.Headers{eventMessageTypeHeader, eventTypeChunkHeader},
		Payload: []byte(`{"bytes": "` + base64.StdEncoding.EncodeToString([]byte(`{"type":"message_start"}`)) + `"}`),
	}))

	var multiEvents bytes.Buffer
	require.NoError(t, enc.Encode(&multiEvents, eventstream.Message{
		Headers: eventstream.Headers{eventMessageTypeHeader, eventTypeChunkHeader},
		Payload: []byte(`{"bytes": "` + base64.StdEncoding.EncodeToString([]byte(`{"type":"message_start"}`)) + `"}`),
	}))
	require.NoError(t, enc.Encode(&multiEvents, eventstream.Message{
		Headers: eventstream.Headers{eventMessageTypeHeader, eventTypeChunkHeader},
		Payload: []byte(`{"bytes": "` + base64.StdEncoding.EncodeToString([]byte(`{"type":"message_stop"}`)) + `"}`),
	}))

	var errorEvents bytes.Buffer
	require.NoError(t, enc.Encode(&errorEvents, eventstream.Message{
		Headers: eventstream.Headers{eventMessageTypeHeader, eventTypeChunkHeader},
		Payload: []byte(`{"bytes": "` + base64.StdEncoding.EncodeToString([]byte(`{"type":"message_start"}`)) + `"}`),
	}))
	require.NoError(t, enc.Encode(&errorEvents, eventstream.Message{
		Headers: eventstream.Headers{exceptionMessageTypeHeader, exceptionTypeHeader},
		Payload: []byte(`{"bytes": "` + base64.StdEncoding.EncodeToString([]byte(`{"type": "error", "error": {"type": "overloaded_error", "message": "Overloaded"}}`)) + `"}`),
	}))
	require.NoError(t, enc.Encode(&errorEvents, eventstream.Message{
		Headers: eventstream.Headers{eventMessageTypeHeader, eventTypeChunkHeader},
		Payload: []byte(`{"bytes": "` + base64.StdEncoding.EncodeToString([]byte(`{"type":"message_start"}`)) + `"}`),
	}))

	for name, tc := range map[string]struct {
		input          *bytes.Buffer
		expectedErr    require.ErrorAssertionFunc
		expectedOutput require.ValueAssertionFunc
	}{
		"single event": {
			input:       &singleEvent,
			expectedErr: require.NoError,
			expectedOutput: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(tt, "event: message_start\ndata: {\"type\":\"message_start\"}\n\n", i1, i2...)
			},
		},
		"multiple events": {
			input:       &multiEvents,
			expectedErr: require.NoError,
			expectedOutput: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(tt, "event: message_start\ndata: {\"type\":\"message_start\"}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", i1, i2...)
			},
		},
		"stream with errors": {
			input:       &errorEvents,
			expectedErr: require.Error,
			expectedOutput: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(tt, "event: message_start\ndata: {\"type\":\"message_start\"}\n\n", i1, i2...)
			},
		},
		"empty stream": {
			input:          &bytes.Buffer{},
			expectedErr:    require.NoError,
			expectedOutput: require.Empty,
		},
	} {
		t.Run(name, func(t *testing.T) {
			resp := &http.Response{
				Header: http.Header{"Content-Type": []string{"application/vnd.amazon.eventstream"}},
				Body:   io.NopCloser(tc.input),
			}
			w := httptest.NewRecorder()
			written, err := streamAnthropicSSEEvents(t.Context(), slog.Default(), w, resp, &mockReporter{})
			tc.expectedErr(t, err)
			tc.expectedOutput(t, w.Body.String())
			require.Equal(t, int64(w.Body.Len()), written)
		})
	}
}

type mockReporter struct{}

func (m *mockReporter) Report(context.Context, types.LLMFormat, usageReport) error { return nil }

// recordingReporter captures every Report call so tests can assert on call
// count, format, and usage payload.
type recordingReporter struct {
	mu      sync.Mutex
	calls   []recordedReport
	err     error
	errOnce bool
}

type recordedReport struct {
	format types.LLMFormat
	usage  usageReport
}

func (r *recordingReporter) Report(_ context.Context, format types.LLMFormat, usage usageReport) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, recordedReport{format: format, usage: usage})
	if r.errOnce {
		r.errOnce = false
		return r.err
	}
	return nil
}

func (r *recordingReporter) snapshot() []recordedReport {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedReport, len(r.calls))
	copy(out, r.calls)
	return out
}

func TestAnthropicStreamUsageReport(t *testing.T) {
	const (
		startWithUsage = "event: message_start\n" +
			`data: {"type":"message_start","message":{"id":"msg_1","usage":{"input_tokens":42,"output_tokens":0}}}` + "\n\n"
		deltaOutput10 = "event: message_delta\n" +
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":10}}` + "\n\n"
		deltaOutput25 = "event: message_delta\n" +
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":25}}` + "\n\n"
		messageStop    = "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
		errorEvent     = "event: error\n" +
			`data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}` + "\n\n"
		startNoUsage = "event: message_start\n" +
			`data: {"type":"message_start","message":{"id":"msg_1"}}` + "\n\n"
		startMalformed = "event: message_start\n" +
			`data: {"type":"message_start","message":"not-an-object"}` + "\n\n"
	)

	for name, tc := range map[string]struct {
		input         string
		expectErr     require.ErrorAssertionFunc
		expectedUsage usageReport
	}{
		"message_start then message_delta then message_stop": {
			input:         startWithUsage + deltaOutput25 + messageStop,
			expectErr:     require.NoError,
			expectedUsage: usageReport{InputTokens: 42, OutputTokens: 25},
		},
		"two message_delta events: last value wins (not summed)": {
			input:         startWithUsage + deltaOutput10 + deltaOutput25 + messageStop,
			expectErr:     require.NoError,
			expectedUsage: usageReport{InputTokens: 42, OutputTokens: 25},
		},
		"message_start only leaves output at zero": {
			input:         startWithUsage + messageStop,
			expectErr:     require.NoError,
			expectedUsage: usageReport{InputTokens: 42, OutputTokens: 0},
		},
		"message_delta only leaves input at zero": {
			input:         deltaOutput10 + messageStop,
			expectErr:     require.NoError,
			expectedUsage: usageReport{InputTokens: 0, OutputTokens: 10},
		},
		"error event mid-stream still reports usage seen so far": {
			input:         startWithUsage + errorEvent,
			expectErr:     require.Error,
			expectedUsage: usageReport{InputTokens: 42, OutputTokens: 0},
		},
		"empty stream still reports zero usage once": {
			input:         "",
			expectErr:     require.NoError,
			expectedUsage: usageReport{},
		},
		"message_start without usage key is treated as zero": {
			input:         startNoUsage + deltaOutput10 + messageStop,
			expectErr:     require.NoError,
			expectedUsage: usageReport{InputTokens: 0, OutputTokens: 10},
		},
		"malformed message payload does not panic and reports zero input": {
			input:         startMalformed + deltaOutput10 + messageStop,
			expectErr:     require.NoError,
			expectedUsage: usageReport{InputTokens: 0, OutputTokens: 10},
		},
	} {
		t.Run(name, func(t *testing.T) {
			resp := &http.Response{
				Header: http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:   io.NopCloser(strings.NewReader(tc.input)),
			}
			reporter := &recordingReporter{}
			w := httptest.NewRecorder()

			_, err := streamAnthropicSSEEvents(t.Context(), slog.Default(), w, resp, reporter)
			tc.expectErr(t, err)

			calls := reporter.snapshot()
			require.Len(t, calls, 1, "reporter must be called exactly once via defer, regardless of stream outcome")
			require.Equal(t, types.LLMFormatAnthropic, calls[0].format)
			require.Equal(t, tc.expectedUsage, calls[0].usage)
		})
	}

	t.Run("reporter error is swallowed and does not fail the stream", func(t *testing.T) {
		resp := &http.Response{
			Header: http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:   io.NopCloser(strings.NewReader(startWithUsage + messageStop)),
		}
		reporter := &recordingReporter{err: errors.New("boom"), errOnce: true}
		w := httptest.NewRecorder()

		_, err := streamAnthropicSSEEvents(t.Context(), slog.Default(), w, resp, reporter)
		require.NoError(t, err, "reporter errors must be logged, not surfaced to the client")
		require.Len(t, reporter.snapshot(), 1)
	})
}

func TestHandleAnthropicUnsupportedEndpoints(t *testing.T) {
	for name, tc := range map[string]struct {
		method string
		path   string
	}{
		"unknown path": {
			method: http.MethodGet,
			path:   "/v1/unknown/endpoint",
		},
		"unsupported method on supported path": {
			method: http.MethodPatch,
			path:   "/v1/responses",
		},
		"models endpoint": {
			method: http.MethodGet,
			path:   "/v1/models",
		},
		"skills endpoint": {
			method: http.MethodGet,
			path:   "/v1/skills",
		},
		"files endpoint": {
			method: http.MethodGet,
			path:   "/v1/files",
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("upstream server should not be called for unsupported endpoints")
			}))
			t.Cleanup(mockServer.Close)
			t.Setenv("ANTHROPIC_BASE_URL", mockServer.URL)

			var (
				emittedAuditEvent bool
				statusSuccess     bool
			)
			audit := newTestAudit(t, func(pe apievents.PreparedSessionEvent) {
				if evt, ok := pe.GetAuditEvent().(*apievents.AppSessionLLMRequest); ok {
					emittedAuditEvent = true
					statusSuccess = evt.Status.Success
				}
			})

			h := newTestHandler(t)
			app := newTestApp(t, &types.LLM{Format: types.LLMFormatAnthropic, Provider: types.LLMProviderAnthropic})
			sessionCtx := &common.SessionContext{App: app, Audit: audit}
			req := newTestSessionRequest(t, tc.method, tc.path, nil, sessionCtx)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, req)
			require.Equal(t, http.StatusNotFound, w.Code)
			require.True(t, emittedAuditEvent)
			require.False(t, statusSuccess)
		})
	}
}

func TestWriteAnthropicSSEEvent(t *testing.T) {
	for name, tc := range map[string]struct {
		evt      ssestream.Event
		expected string
	}{
		"type and data": {
			evt:      ssestream.Event{Type: "message_start", Data: []byte(`{"type":"message_start"}`)},
			expected: "event: message_start\ndata: {\"type\":\"message_start\"}\n\n",
		},
		"data without type": {
			evt:      ssestream.Event{Data: []byte(`{"foo":"bar"}`)},
			expected: "data: {\"foo\":\"bar\"}\n\n",
		},
		"type without data": {
			evt:      ssestream.Event{Type: "ping"},
			expected: "event: ping\n\n",
		},
		"empty event": {
			evt:      ssestream.Event{},
			expected: "\n",
		},
		"multi-line data": {
			evt:      ssestream.Event{Type: "delta", Data: []byte("line1\nline2")},
			expected: "event: delta\ndata: line1\ndata: line2\n\n",
		},
		"trailing newline on data is trimmed": {
			evt:      ssestream.Event{Type: "delta", Data: []byte("line1\n")},
			expected: "event: delta\ndata: line1\n\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			n, err := writeAnthropicSSEEvent(&buf, tc.evt)
			require.NoError(t, err)
			require.Equal(t, tc.expected, buf.String())
			require.Equal(t, int64(buf.Len()), n)
		})
	}
}

func TestWriteAnthropicSSEEventWriteError(t *testing.T) {
	evt := ssestream.Event{Type: "delta", Data: []byte("line1\nline2")}
	wantErr := errors.New("err")

	for name, failAfter := range map[string]int{
		"fails on event line":       0,
		"fails on first data line":  len("event: delta\n"),
		"fails on second data line": len("event: delta\ndata: line1\n"),
		"fails on terminator":       len("event: delta\ndata: line1\ndata: line2\n"),
	} {
		t.Run(name, func(t *testing.T) {
			fw := &failingWriter{failAfter: failAfter, err: wantErr}
			n, err := writeAnthropicSSEEvent(fw, evt)
			require.ErrorIs(t, err, wantErr)
			require.Equal(t, int64(fw.written), n)
		})
	}
}

// failingWriter returns an error after writing failAfter bytes.
type failingWriter struct {
	written   int
	failAfter int
	err       error
}

func (fw *failingWriter) Write(p []byte) (int, error) {
	if fw.written >= fw.failAfter {
		return 0, fw.err
	}
	remaining := fw.failAfter - fw.written
	if len(p) <= remaining {
		fw.written += len(p)
		return len(p), nil
	}
	fw.written = fw.failAfter
	return remaining, fw.err
}

func requireAnthropicError(tt require.TestingT, body string, msgAndArgs ...any) shared.APIErrorObject {
	var errResp struct {
		Type  string                `json:"type"`
		Error shared.APIErrorObject `json:"error"`
	}
	require.NoError(tt, json.Unmarshal([]byte(body), &errResp), msgAndArgs...)
	require.Equal(tt, "error", errResp.Type, msgAndArgs...)
	return errResp.Error
}

func TestHandleAnthropicBedrock(t *testing.T) {
	for name, tc := range map[string]struct {
		reqBody          string
		bedrockHandler   http.HandlerFunc
		expectedStatus   int
		expectAuditEvent require.ValueAssertionFunc
		expectedBody     require.ValueAssertionFunc
	}{
		"non-streaming success": {
			reqBody: `{"model":"anthropic.claude-3-5-sonnet-20241022-v2:0","messages":[{"role":"user","content":"Hi"}]}`,
			bedrockHandler: func(w http.ResponseWriter, r *http.Request) {
				// The Anthropic SDK's bedrock middleware rewrites /v1/messages
				// to /model/{modelId}/invoke and SigV4-signs the request.
				require.Contains(t, r.URL.Path, "/model/")
				require.Contains(t, r.URL.Path, "/invoke")
				require.NotContains(t, r.URL.Path, "invoke-with-response-stream")
				require.Contains(t, r.Header.Get("Authorization"), "AWS4-HMAC-SHA256")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				io.WriteString(w, `{"id":"msg_123","type":"message","content":[{"type":"text","text":"Hi"}]}`)
			},
			expectedStatus: http.StatusOK,
			expectAuditEvent: func(tt require.TestingT, i1 any, i2 ...any) {
				evt := i1.(*apievents.AppSessionLLMRequest)
				require.True(tt, evt.Status.Success, i2...)
				require.Equal(tt, types.LLMProviderAWSBedrock, evt.Provider, i2...)
			},
			expectedBody: func(tt require.TestingT, i1 any, i2 ...any) {
				require.JSONEq(tt, `{"id":"msg_123","type":"message","content":[{"type":"text","text":"Hi"}]}`, i1.(string), i2...)
			},
		},
		"streaming success converts eventstream to SSE": {
			reqBody: `{"model":"anthropic.claude-3-5-sonnet-20241022-v2:0","messages":[{"role":"user","content":"Hi"}],"stream":true}`,
			bedrockHandler: func(w http.ResponseWriter, r *http.Request) {
				require.Contains(t, r.URL.Path, "invoke-with-response-stream")
				require.Contains(t, r.Header.Get("Authorization"), "AWS4-HMAC-SHA256")

				w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
				w.WriteHeader(http.StatusOK)

				enc := eventstream.NewEncoder()
				headers := eventstream.Headers{
					{Name: eventstreamapi.MessageTypeHeader, Value: eventstream.StringValue(eventstreamapi.EventMessageType)},
					{Name: eventstreamapi.EventTypeHeader, Value: eventstream.StringValue("chunk")},
				}
				require.NoError(t, enc.Encode(w, eventstream.Message{
					Headers: headers,
					Payload: []byte(`{"bytes":"` + base64.StdEncoding.EncodeToString([]byte(`{"type":"message_start"}`)) + `"}`),
				}))
				require.NoError(t, enc.Encode(w, eventstream.Message{
					Headers: headers,
					Payload: []byte(`{"bytes":"` + base64.StdEncoding.EncodeToString([]byte(`{"type":"message_stop"}`)) + `"}`),
				}))
			},
			expectedStatus: http.StatusOK,
			expectAuditEvent: func(tt require.TestingT, i1 any, i2 ...any) {
				evt := i1.(*apievents.AppSessionLLMRequest)
				require.True(tt, evt.Status.Success, i2...)
				require.Equal(tt, types.LLMProviderAWSBedrock, evt.Provider, i2...)
			},
			expectedBody: func(tt require.TestingT, i1 any, i2 ...any) {
				body := i1.(string)
				require.Contains(tt, body, "event: message_start", i2...)
				require.Contains(tt, body, `data: {"type":"message_start"}`, i2...)
				require.Contains(tt, body, "event: message_stop", i2...)
			},
		},
		"upstream 403 is sanitized": {
			reqBody: `{"model":"anthropic.claude-3-5-sonnet-20241022-v2:0","messages":[{"role":"user","content":"Hi"}]}`,
			bedrockHandler: func(w http.ResponseWriter, r *http.Request) {
				// Bedrock surfaces errors using the AWS REST/JSON protocol.
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				io.WriteString(w, `{"message":"User: arn:aws:iam::1234:role/leak is not authorized to perform bedrock:InvokeModel"}`)
			},
			expectedStatus: http.StatusForbidden,
			expectAuditEvent: func(tt require.TestingT, i1 any, i2 ...any) {
				evt := i1.(*apievents.AppSessionLLMRequest)
				require.False(tt, evt.Status.Success, i2...)
			},
			expectedBody: func(tt require.TestingT, i1 any, i2 ...any) {
				anthropicErr := requireAnthropicError(tt, i1.(string), i2...)
				require.NotContains(tt, anthropicErr.Message, "arn:aws:iam", i2...)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockServer := httptest.NewServer(tc.bedrockHandler)
			t.Cleanup(mockServer.Close)

			target, err := url.Parse(mockServer.URL)
			require.NoError(t, err)

			provider := awsconfig.ProviderFunc(func(_ context.Context, _ string, _ ...awsconfig.OptionsFn) (aws.Config, error) {
				return aws.Config{
					Region:      "us-east-1",
					Credentials: credentials.NewStaticCredentialsProvider("test-key", "test-secret", ""),
				}, nil
			})

			var auditEvent *apievents.AppSessionLLMRequest
			audit := newTestAudit(t, func(pe apievents.PreparedSessionEvent) {
				if evt, ok := pe.GetAuditEvent().(*apievents.AppSessionLLMRequest); ok {
					auditEvent = evt
				}
			})

			h, err := NewHandler(t.Context(), HandlerConfig{
				Log:               slog.Default(),
				AWSConfigProvider: provider,
				HTTPClient:        &http.Client{Transport: &redirectingTransport{target: target}},
			})
			require.NoError(t, err)

			app := newTestApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAWSBedrock,
			})
			sessionCtx := &common.SessionContext{App: app, Audit: audit}
			req := newTestSessionRequest(t, http.MethodPost, "/v1/messages", strings.NewReader(tc.reqBody), sessionCtx)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, req)

			require.Equal(t, tc.expectedStatus, w.Code)
			tc.expectedBody(t, w.Body.String())
			require.NotNil(t, auditEvent)
			tc.expectAuditEvent(t, auditEvent)
		})
	}
}

// redirectingTransport rewrites the host/scheme of outgoing requests to point
// at a test server. The Anthropic SDK's bedrock middleware sets the request
// URL to https://bedrock-runtime.{region}.amazonaws.com after signing. This
// transport reroutes the connection to the local httptest server while
// keeping the SigV4-signed headers intact.
type redirectingTransport struct {
	target *url.URL
}

func (t *redirectingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.URL.Scheme = t.target.Scheme
	r.URL.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(r)
}

func TestReadAnthropicRequest(t *testing.T) {
	for name, tc := range map[string]struct {
		input         string
		expectErr     require.ErrorAssertionFunc
		expectRequest require.ValueAssertionFunc
	}{
		"minimal valid request": {
			input:     `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello"}]}`,
			expectErr: require.NoError,
			expectRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*anthropicRequest)
				require.Equal(tt, "claude-sonnet-4-20250514", req.model)
			},
		},
		"valid request with stream": {
			input:     `{"model":"claude-sonnet-4-20250514","stream":true,"messages":[{"role":"user","content":"Hello"}]}`,
			expectErr: require.NoError,
			expectRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*anthropicRequest)
				require.Equal(tt, "claude-sonnet-4-20250514", req.model)
				require.True(tt, req.streaming)
			},
		},
		"valid request with max tokens": {
			input:     `{"model":"claude-sonnet-4-20250514","max_tokens":1024,"messages":[{"role":"user","content":"Hello"}]}`,
			expectErr: require.NoError,
			expectRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*anthropicRequest)
				require.Equal(tt, "claude-sonnet-4-20250514", req.model)
				require.Equal(tt, req.maxTokens, int64(1024))
			},
		},
		"invalid model duplicates": {
			input:         `{"model":"claude-sonnet-4-20250514","model":"claude-opus-4-7","messages":[{"role":"user","content":"Hello"}]}`,
			expectErr:     require.Error,
			expectRequest: require.Nil,
		},
		"invalid stream duplicates": {
			input:         `{"model":"claude-sonnet-4-20250514","stream":true,"stream":false,"messages":[{"role":"user","content":"Hello"}]}`,
			expectErr:     require.Error,
			expectRequest: require.Nil,
		},
		"invalid max tokens duplicates": {
			input:         `{"model":"claude-sonnet-4-20250514","max_tokens":1024,"max_tokens":9999,"messages":[{"role":"user","content":"Hello"}]}`,
			expectErr:     require.Error,
			expectRequest: require.Nil,
		},
		"stream invalid format": {
			input:         `{"model":"claude-sonnet-4-20250514","stream":"yes","messages":[{"role":"user","content":"Hello"}]}`,
			expectErr:     require.Error,
			expectRequest: require.Nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := readAnthropicRequest([]byte(tc.input))
			tc.expectErr(t, err)
			tc.expectRequest(t, req)
		})
	}
}
