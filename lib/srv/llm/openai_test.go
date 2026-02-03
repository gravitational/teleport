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

func TestHandleOpenAI(t *testing.T) {
	for name, tc := range map[string]struct {
		reqBody             string
		reqPath             string
		respStatus          int
		respBody            string
		expectedStatus      int
		expectedAuditStatus uint32
		expectedBody        require.ValueAssertionFunc
	}{
		"success": {
			reqPath:    "/v1/responses",
			reqBody:    `{"model":"gpt-5-2","input":"Tell me a joke"}`,
			respStatus: http.StatusOK,
			respBody: `{
				"id":"resp_123",
				"object":"response",
				"output":[{"type":"message","content":[{"type":"output_text","text":"Why did the chicken cross the road?"}]}]
			}`,
			expectedStatus:      http.StatusOK,
			expectedAuditStatus: uint32(http.StatusOK),
			expectedBody: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Contains(tt, i1, "resp_123", i2...)
			},
		},
		"api error non-401": {
			reqPath:             "/v1/responses",
			reqBody:             `{"model":"invalid"}`,
			respStatus:          http.StatusBadRequest,
			respBody:            `{"error":{"code":"invalid_request_error","message":"invalid model","param":"model","type":"invalid_request_error"}}`,
			expectedStatus:      http.StatusBadRequest,
			expectedAuditStatus: uint32(http.StatusBadRequest),
			expectedBody: func(tt require.TestingT, i1 any, i2 ...any) {
				require.JSONEq(tt, `{"error":{"code":"invalid_request_error","message":"invalid model","param":"model","type":"invalid_request_error"}}`, i1.(string), i2...)
			},
		},
		"auth error 401 rewrites message": {
			reqPath:             "/v1/responses",
			reqBody:             `{"model":"gpt-5-2","input":"hello"}`,
			respStatus:          http.StatusUnauthorized,
			respBody:            `{"error":{"code":"invalid_api_key","message":"invalid api key","param":null,"type":"invalid_request_error"}}`,
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

			h := newTestHandler(t, "", mockServer.URL)
			app := newTestApp(t, types.LLM_FORMAT_OPENAI, types.LLM_PROVIDER_OPENAI)
			sessionCtx := &common.SessionContext{App: app, Audit: audit}
			req := newTestSessionRequest(
				t,
				http.MethodPost,
				tc.reqPath,
				strings.NewReader(tc.reqBody),
				sessionCtx,
			)
			w := httptest.NewRecorder()

			err := h.handleOpenAI(sessionCtx, w, req)
			require.NoError(t, err)
			require.Equal(t, tc.expectedStatus, w.Code)
			require.Equal(t, tc.expectedAuditStatus, auditStatus)
			tc.expectedBody(t, w.Body.String())
		})
	}
}

func TestHandleOpenAI_AuthErrorBody(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"code":"invalid_api_key","message":"original","param":null,"type":"invalid_request_error"}}`)
	}))
	t.Cleanup(mockServer.Close)

	audit := newTestAudit(t, func(apievents.PreparedSessionEvent) {})
	h := newTestHandler(t, "", mockServer.URL)
	app := newTestApp(t, types.LLM_FORMAT_OPENAI, types.LLM_PROVIDER_OPENAI)
	sessionCtx := &common.SessionContext{App: app, Audit: audit}
	req := newTestSessionRequest(
		t,
		http.MethodPost,
		"/v1/responses",
		strings.NewReader(`{"model":"gpt-5-2","input":"hello"}`),
		sessionCtx,
	)
	w := httptest.NewRecorder()

	err := h.handleOpenAI(sessionCtx, w, req)
	require.NoError(t, err)

	var errResp struct {
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	require.Contains(t, errResp.Message, "Teleport")
}
