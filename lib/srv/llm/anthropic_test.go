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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/app/common"
)

func TestStreamBedrockEvents(t *testing.T) {
	for name, tc := range map[string]struct {
		input          string
		expectedErr    require.ErrorAssertionFunc
		expectedOutput require.ValueAssertionFunc
	}{
		"single event": {
			input:       "event: message_start\ndata: {\"type\":\"message_start\"}\n\n",
			expectedErr: require.NoError,
			expectedOutput: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(tt, "event: message_start\ndata: {\"type\":\"message_start\"}\n\ndata: \n\n", i1, i2...)
			},
		},
		"multiple events": {
			input:       "event: message_start\ndata: {\"type\":\"message_start\"}\n\nevent: message_stop\ndata: {}\n\n",
			expectedErr: require.NoError,
			expectedOutput: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(tt, "event: message_start\ndata: {\"type\":\"message_start\"}\n\ndata: \n\nevent: message_stop\ndata: {}\n\ndata: \n\n", i1, i2...)
			},
		},
		"multi-line data": {
			input:       "event: content_block_delta\ndata: line1\ndata: line2\n\n",
			expectedErr: require.NoError,
			expectedOutput: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(tt, "event: content_block_delta\ndata: line1\n\ndata: line2\n\ndata: \n\n", i1, i2...)
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
			err := streamBedrockEvents(w, resp)
			tc.expectedErr(t, err)
			tc.expectedOutput(t, w.Body.String())
		})
	}
}

func TestHandleAnthropic(t *testing.T) {
	for name, tc := range map[string]struct {
		reqBody             string
		respStatus          int
		respBody            string
		expectedStatus      int
		expectedAuditStatus uint32
		expectedBody        require.ValueAssertionFunc
	}{
		"success non-streaming": {
			reqBody:             `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello"}]}`,
			respStatus:          http.StatusOK,
			respBody:            `{"id":"msg_123","type":"message","content":[{"type":"text","text":"Hello"}]}`,
			expectedStatus:      http.StatusOK,
			expectedAuditStatus: uint32(http.StatusOK),
			expectedBody: func(tt require.TestingT, i1 any, i2 ...any) {
				require.JSONEq(tt, `{"id":"msg_123","type":"message","content":[{"type":"text","text":"Hello"}]}`, i1.(string), i2...)
			},
		},
		"api error non-401": {
			reqBody:             `{}`,
			respStatus:          http.StatusBadRequest,
			respBody:            `{"type":"invalid_request_error","message":"invalid model"}`,
			expectedStatus:      http.StatusBadRequest,
			expectedAuditStatus: uint32(http.StatusBadRequest),
			expectedBody: func(tt require.TestingT, i1 any, i2 ...any) {
				require.JSONEq(tt, `{"type":"invalid_request_error","message":"invalid model"}`, i1.(string), i2...)
			},
		},
		"auth error 401 rewrites message": {
			reqBody:             `{}`,
			respStatus:          http.StatusUnauthorized,
			respBody:            `{"type":"authentication_error","message":"invalid api key"}`,
			expectedStatus:      http.StatusOK, // 401 path writes a JSON body without calling WriteHeader
			expectedAuditStatus: uint32(http.StatusUnauthorized),
			expectedBody: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Contains(tt, i1, "Teleport", i2...)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.respStatus)
				io.WriteString(w, tc.respBody)
			}))
			t.Cleanup(mockServer.Close)

			var auditStatus uint32
			audit := newTestAudit(t, func(pe apievents.PreparedSessionEvent) {
				if evt, ok := pe.GetAuditEvent().(*apievents.AppSessionRequest); ok {
					auditStatus = evt.StatusCode
				}
			})

			h := newTestHandler(t, mockServer.URL, "")
			app := newTestApp(t, types.LLM_FORMAT_ANTHROPIC, types.LLM_PROVIDER_ANTHROPIC)
			sessionCtx := &common.SessionContext{App: app, Audit: audit}
			req := newTestSessionRequest(
				t,
				http.MethodPost,
				"/v1/messages",
				strings.NewReader(tc.reqBody),
				sessionCtx,
			)
			w := httptest.NewRecorder()

			err := h.handleAnthropic(sessionCtx, w, req)
			require.NoError(t, err)
			require.Equal(t, tc.expectedStatus, w.Code)
			require.Equal(t, tc.expectedAuditStatus, auditStatus)
			tc.expectedBody(t, w.Body.String())
		})
	}
}

func TestHandleAnthropic_Streaming(t *testing.T) {
	sseBody := "event: message_start\ndata: {\"type\":\"message_start\"}\n\nevent: message_stop\ndata: {}\n\n"
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, sseBody)
	}))
	t.Cleanup(mockServer.Close)

	var auditStatus uint32
	audit := newTestAudit(t, func(pe apievents.PreparedSessionEvent) {
		if evt, ok := pe.GetAuditEvent().(*apievents.AppSessionRequest); ok {
			auditStatus = evt.StatusCode
		}
	})

	h := newTestHandler(t, mockServer.URL, "")
	app := newTestApp(t, types.LLM_FORMAT_ANTHROPIC, types.LLM_PROVIDER_ANTHROPIC)
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

	err := h.handleAnthropic(sessionCtx, w, req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "event: message_start")
	require.Contains(t, w.Body.String(), "event: message_stop")
	require.Equal(t, uint32(http.StatusOK), auditStatus)
}

// TestHandleAnthropic_UnsupportedProvider verifies that a provider not
// handled by the Anthropic path returns a BadParameter error.
func TestHandleAnthropic_UnsupportedProvider(t *testing.T) {
	audit := newTestAudit(t, func(apievents.PreparedSessionEvent) {})
	h := newTestHandler(t, "", "")
	// Azure is a valid provider for Anthropic format (passes validation)
	// but the Anthropic handler doesn't support it.
	app := newTestApp(t, types.LLM_FORMAT_ANTHROPIC, types.LLM_PROVIDER_AZURE)
	sessionCtx := &common.SessionContext{App: app, Audit: audit}
	req := newTestSessionRequest(t, http.MethodPost, "/v1/messages", strings.NewReader(`{}`), sessionCtx)
	w := httptest.NewRecorder()

	err := h.handleAnthropic(sessionCtx, w, req)
	require.Error(t, err)
	require.ErrorContains(t, err, "unsupported provider")
}

func TestHandleAnthropic_BedrockModelMapping(t *testing.T) {
	for name, tc := range map[string]struct {
		mappings     []*types.LLM_ModelMap
		defaultModel string
		reqModel     string
		wantModel    string
	}{
		"mapping rewrites model": {
			mappings: []*types.LLM_ModelMap{
				{From: "claude-sonnet", To: "us.anthropic.claude-sonnet-v2:0"},
			},
			defaultModel: "us.anthropic.claude-default:0",
			reqModel:     "claude-sonnet",
			wantModel:    "us.anthropic.claude-sonnet-v2:0",
		},
		"no match uses default model": {
			mappings: []*types.LLM_ModelMap{
				{From: "claude-opus", To: "us.anthropic.claude-opus:0"},
			},
			defaultModel: "us.anthropic.claude-default:0",
			reqModel:     "claude-sonnet",
			wantModel:    "us.anthropic.claude-default:0",
		},
		"case insensitive match": {
			mappings: []*types.LLM_ModelMap{
				{From: "claude-sonnet", To: "us.anthropic.claude-sonnet-v2:0"},
			},
			defaultModel: "us.anthropic.claude-default:0",
			reqModel:     "Claude-Sonnet",
			wantModel:    "us.anthropic.claude-sonnet-v2:0",
		},
	} {
		t.Run(name, func(t *testing.T) {
			// We assert the model via the URL path rather than the
			// request body because the Bedrock SDK middleware strips
			// the "model" field from the JSON body and embeds it in
			// the URL path as /model/{model}/invoke.
			var gotModelPath string
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotModelPath = r.URL.Path

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				io.WriteString(w, `{"id":"msg_123","type":"message","content":[]}`)
			}))
			t.Cleanup(mockServer.Close)

			audit := newTestAudit(t, func(apievents.PreparedSessionEvent) {})
			h := newTestBedrockHandler(t, mockServer.URL)
			app, err := types.NewAppV3(types.Metadata{
				Name: "test-bedrock-app",
			}, types.AppSpecV3{
				LLM: &types.LLM{
					Format:        types.LLM_FORMAT_ANTHROPIC,
					Provider:      types.LLM_PROVIDER_AWS_BEDROCK,
					DefaultModel:  tc.defaultModel,
					ModelMappings: tc.mappings,
				},
			})
			require.NoError(t, err)

			sessionCtx := &common.SessionContext{App: app, Audit: audit}
			reqBody := `{"model":"` + tc.reqModel + `","messages":[{"role":"user","content":"Hi"}]}`
			req := newTestSessionRequest(t, http.MethodPost, "/v1/messages", strings.NewReader(reqBody), sessionCtx)
			w := httptest.NewRecorder()

			err = h.handleAnthropic(sessionCtx, w, req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, "/model/"+tc.wantModel+"/invoke", gotModelPath)
		})
	}
}

func TestHandleAnthropic_AuthErrorBody(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"type":"authentication_error","message":"original"}`)
	}))
	t.Cleanup(mockServer.Close)

	audit := newTestAudit(t, func(apievents.PreparedSessionEvent) {})
	h := newTestHandler(t, mockServer.URL, "")
	app := newTestApp(t, types.LLM_FORMAT_ANTHROPIC, types.LLM_PROVIDER_ANTHROPIC)
	sessionCtx := &common.SessionContext{App: app, Audit: audit}
	req := newTestSessionRequest(
		t,
		http.MethodPost,
		"/v1/messages",
		strings.NewReader(`{}`),
		sessionCtx,
	)
	w := httptest.NewRecorder()

	err := h.handleAnthropic(sessionCtx, w, req)
	require.NoError(t, err)

	var errResp struct {
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	require.Contains(t, errResp.Message, "Teleport")
}
