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

package httplib

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gravitational/roundtrip"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/observability/tracing"
)

type netError struct{}

func (e *netError) Error() string   { return "net" }
func (e *netError) Timeout() bool   { return true }
func (e *netError) Temporary() bool { return true }

func TestConvertResponse(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name: "url error",
			err: &url.Error{
				Op:  "POST",
				URL: "http://localhost",
				Err: errors.New("error goes here"),
			},
			expected: "error goes here",
		},
		{
			name: "url with path error",
			err: &url.Error{
				Op:  "POST",
				URL: "http://localhost?path%20foobar",
				Err: errors.New("error goes here"),
			},
			expected: "error goes here",
		},
		{
			name:     "timeout error",
			err:      &netError{},
			expected: "unable to complete the request due to a timeout, please try again in a few minutes",
		},
		{
			name:     "normal error",
			err:      errors.New("this is a normal error"),
			expected: "this is a normal error",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			_, err := ConvertResponse(&roundtrip.Response{}, test.err)
			require.Error(t, err)
			require.Equal(t, test.expected, err.Error())
		})
	}

}

func TestRewritePaths(t *testing.T) {
	handler := newTestHandler()
	server := httptest.NewServer(
		RewritePaths(handler,
			Rewrite("/v1/sessions/([^/]+)/(.*)", "/v1/namespaces/default/sessions/$1/$2")))
	defer server.Close()
	re, err := http.Post(server.URL+"/v1/sessions/s1/stream", "text/json", nil)
	require.NoError(t, err)
	defer re.Body.Close()
	require.Equal(t, http.StatusOK, re.StatusCode)
	require.Equal(t, "default", handler.capturedNamespace)
	require.Equal(t, "s1", handler.capturedID)

	re, err = http.Post(server.URL+"/v1/namespaces/system/sessions/s2/stream", "text/json", nil)
	require.NoError(t, err)
	defer re.Body.Close()
	require.Equal(t, http.StatusOK, re.StatusCode)
	require.Equal(t, "system", handler.capturedNamespace)
	require.Equal(t, "s2", handler.capturedID)
}

type testHandler struct {
	httprouter.Router
	capturedNamespace string
	capturedID        string
}

func newTestHandler() *testHandler {
	h := &testHandler{}
	h.Router = *httprouter.New()
	h.Router.UseRawPath = true
	h.POST("/v1/sessions/:id/stream", MakeHandler(h.postSessionChunkOriginal))
	h.POST("/v1/namespaces/:namespace/sessions/:id/stream", MakeHandler(h.postSessionChunkNamespace))
	return h
}

func (h *testHandler) postSessionChunkOriginal(_ http.ResponseWriter, _ *http.Request, _ httprouter.Params) (interface{}, error) {
	return "ok", nil
}

func (h *testHandler) postSessionChunkNamespace(_ http.ResponseWriter, _ *http.Request, p httprouter.Params) (interface{}, error) {
	h.capturedNamespace = p.ByName("namespace")
	h.capturedID = p.ByName("id")
	return "ok", nil
}

func TestReadJSON_ContentType(t *testing.T) {
	t.Parallel()

	type TestJSON struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	testCases := []struct {
		name        string
		contentType string
		wantErr     bool
	}{
		{
			name:        "empty value",
			contentType: "",
			wantErr:     true,
		},
		{
			name:        "invalid type",
			contentType: "multipart/form-data",
			wantErr:     true,
		},
		{
			name:        "just type/subtype",
			contentType: "application/json",
		},
		{
			name:        "type/subtype with params",
			contentType: "application/json; charset=utf-8",
		},
	}

	body := TestJSON{Name: "foo", Age: 60}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payloadBuf := new(bytes.Buffer)
			require.NoError(t, json.NewEncoder(payloadBuf).Encode(body))

			httpReq, err := http.NewRequest("", "", payloadBuf)
			require.NoError(t, err)
			httpReq.Header.Add("Content-Type", tc.contentType)

			output := TestJSON{}
			err = ReadJSON(httpReq, &output)
			if tc.wantErr {
				require.True(t, strings.Contains(err.Error(), "invalid request"))
				require.Empty(t, output)
			} else {
				require.NoError(t, err)
				require.Equal(t, body, output)
			}
		})
	}
}

func TestMakeTracingHandler(t *testing.T) {
	t.Parallel()

	newRequest := func(t *testing.T) *http.Request {
		req, err := http.NewRequest(http.MethodGet, "", nil)
		require.NoError(t, err)

		return req
	}

	cases := []struct {
		name            string
		req             func(t *testing.T) *http.Request
		headerAssertion func(t *testing.T, req *http.Request)
	}{
		{
			name: "no tracing context provided",
			req:  newRequest,
			headerAssertion: func(t *testing.T, req *http.Request) {
				require.Empty(t, req.Header.Get(tracing.TraceParent))
			},
		},
		{
			name: "tracing context provided via header",
			req: func(t *testing.T) *http.Request {
				req := newRequest(t)
				req.Header.Add(tracing.TraceParent, "test")
				return req
			},
			headerAssertion: func(t *testing.T, req *http.Request) {
				require.Equal(t, "test", req.Header.Get(tracing.TraceParent))
			},
		},
		{
			name: "tracing context provided via parameter",
			req: func(t *testing.T) *http.Request {
				req := newRequest(t)
				q := req.URL.Query()
				q.Set(tracing.TraceParent, "test")
				req.URL.RawQuery = q.Encode()
				return req
			},
			headerAssertion: func(t *testing.T, req *http.Request) {
				require.Equal(t, "test", req.Header.Get(tracing.TraceParent))
			},
		},
		{
			name: "header has priority",
			req: func(t *testing.T) *http.Request {
				req := newRequest(t)
				q := req.URL.Query()
				req.Header.Add(tracing.TraceParent, "header")
				q.Set(tracing.TraceParent, "parameter")
				req.URL.RawQuery = q.Encode()
				return req
			},
			headerAssertion: func(t *testing.T, req *http.Request) {
				require.Equal(t, "header", req.Header.Get(tracing.TraceParent))
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			handler := MakeTracingHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.headerAssertion(t, r)
			}), teleport.ComponentProxy)

			handler.ServeHTTP(httptest.NewRecorder(), tt.req(t))
		})
	}

}

func TestSetIndexContentSecurityPolicy(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name            string
		features        proto.Features
		urlPath         string
		expectedCspVals map[string]string
	}{
		{
			name:     "default (no stripe or wasm)",
			features: proto.Features{},
			urlPath:  "/web/index.js",
			expectedCspVals: map[string]string{
				"default-src":     "'self'",
				"base-uri":        "'self'",
				"form-action":     "'self'",
				"frame-ancestors": "'none'",
				"object-src":      "'none'",
				"script-src":      "'self'",
				"style-src":       "'self' 'unsafe-inline'",
				"img-src":         "'self' data: blob:",
				"font-src":        "'self' data:",
				"connect-src":     "'self' wss:",
			},
		},
		{
			name:     "for cloud based usage, Stripe managed product (with stripe, no wasm)",
			features: proto.Features{Cloud: true, IsUsageBased: true, IsStripeManaged: true},
			urlPath:  "/web/index.js",
			expectedCspVals: map[string]string{
				"default-src":     "'self'",
				"base-uri":        "'self'",
				"form-action":     "'self'",
				"frame-ancestors": "'none'",
				"frame-src":       "https://js.stripe.com",
				"object-src":      "'none'",
				"script-src":      "'self' https://js.stripe.com",
				"style-src":       "'self' 'unsafe-inline'",
				"img-src":         "'self' data: blob:",
				"font-src":        "'self' data:",
				"connect-src":     "'self' wss:",
			},
		},
		{
			name:     "for cloud based usage, EUB product (no stripe or wasm)",
			features: proto.Features{Cloud: true, IsUsageBased: true, ProductType: proto.ProductType_PRODUCT_TYPE_EUB},
			urlPath:  "/web/index.js",
			expectedCspVals: map[string]string{
				"default-src":     "'self'",
				"base-uri":        "'self'",
				"form-action":     "'self'",
				"frame-ancestors": "'none'",
				"object-src":      "'none'",
				"style-src":       "'self' 'unsafe-inline'",
				"img-src":         "'self' data: blob:",
				"font-src":        "'self' data:",
				"connect-src":     "'self' wss:",
			},
		},
		{
			name:     "for desktop session (no stripe, with wasm)",
			features: proto.Features{},
			urlPath:  "/web/cluster/:clusterId/desktops/:desktopName/:username",
			expectedCspVals: map[string]string{
				"default-src":     "'self'",
				"base-uri":        "'self'",
				"form-action":     "'self'",
				"frame-ancestors": "'none'",
				"object-src":      "'none'",
				"script-src":      "'self' 'wasm-unsafe-eval'",
				"style-src":       "'self' 'unsafe-inline'",
				"img-src":         "'self' data: blob:",
				"font-src":        "'self' data:",
				"connect-src":     "'self' wss:",
			},
		},
		{
			name:     "for cloud based usage & desktop session, Stripe managed product (with stripe, with wasm)",
			features: proto.Features{Cloud: true, IsUsageBased: true, IsStripeManaged: true},
			urlPath:  "/web/cluster/:clusterId/desktops/:desktopName/:username",
			expectedCspVals: map[string]string{
				"default-src":     "'self'",
				"base-uri":        "'self'",
				"form-action":     "'self'",
				"frame-ancestors": "'none'",
				"object-src":      "'none'",
				"script-src":      "'self' https://js.stripe.com 'wasm-unsafe-eval'",
				"frame-src":       "https://js.stripe.com",
				"style-src":       "'self' 'unsafe-inline'",
				"img-src":         "'self' data: blob:",
				"font-src":        "'self' data:",
				"connect-src":     "'self' wss:",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := make(http.Header)
			SetIndexContentSecurityPolicy(h, tt.features, tt.urlPath)
			actualCsp := h.Get("Content-Security-Policy")
			for k, v := range tt.expectedCspVals {
				expectedCspSubString := fmt.Sprintf("%s %s;", k, v)
				require.Contains(t, actualCsp, expectedCspSubString)
			}
		})
	}
}

func TestSetAppLaunchContentSecurityPolicy(t *testing.T) {
	t.Parallel()

	applicationURL := "https://example.com"

	expectedCspVals := map[string]string{
		"default-src":     "'self'",
		"base-uri":        "'self'",
		"form-action":     "'self'",
		"frame-ancestors": "'none'",
		"object-src":      "'none'",
		"style-src":       "'self' 'unsafe-inline'",
		"img-src":         "'self' data: blob:",
		"font-src":        "'self' data:",
		"connect-src":     fmt.Sprintf("'self' %s", applicationURL),
	}

	h := make(http.Header)
	SetAppLaunchContentSecurityPolicy(h, applicationURL)
	actualCsp := h.Get("Content-Security-Policy")
	for k, v := range expectedCspVals {
		expectedCspSubString := fmt.Sprintf("%s %s;", k, v)
		require.Contains(t, actualCsp, expectedCspSubString)
	}
}

func TestSetRedirectPageContentSecurityPolicy(t *testing.T) {
	t.Parallel()

	scriptSrc := "nonce-123456789abcdefg"

	expectedCspVals := map[string]string{
		"default-src":     "'self'",
		"base-uri":        "'self'",
		"form-action":     "'self'",
		"frame-ancestors": "'none'",
		"object-src":      "'none'",
		"style-src":       "'self' 'unsafe-inline'",
		"img-src":         "'self' data: blob:",
		"script-src":      fmt.Sprintf("'self' '%s'", scriptSrc),
	}

	h := make(http.Header)
	SetRedirectPageContentSecurityPolicy(h, scriptSrc)
	actualCsp := h.Get("Content-Security-Policy")
	for k, v := range expectedCspVals {
		expectedCspSubString := fmt.Sprintf("%s %s;", k, v)
		require.Contains(t, actualCsp, expectedCspSubString)
	}
}

func TestOriginLocalRedirectURI(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		expected string
		errCheck require.ErrorAssertionFunc
	}{
		{
			name:     "empty",
			input:    "",
			expected: "/",
			errCheck: require.NoError,
		},
		{
			name:     "simple path",
			input:    "/foo",
			expected: "/foo",
			errCheck: require.NoError,
		},
		{
			name:     "host only",
			input:    "https://localhost",
			expected: "/",
			errCheck: require.NoError,
		},
		{
			name:     "host and simple path",
			input:    "https://localhost/bar",
			expected: "/bar",
			errCheck: require.NoError,
		},
		{
			name:     "double slash redirect with host",
			input:    "https://localhost//goteleport.com/",
			expected: "",
			errCheck: require.Error,
		},
		{
			name:     "basic auth redirect with host",
			input:    "https://localhost/@goteleport.com/",
			expected: "",
			errCheck: require.Error,
		},
		{
			name:     "ftp scheme",
			input:    "ftp://localhost",
			expected: "",
			errCheck: require.Error,
		},
		{
			name:     "invalid url",
			input:    "https://foo com",
			expected: "",
			errCheck: require.Error,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := OriginLocalRedirectURI(tc.input)
			require.Equal(t, tc.expected, result)
			tc.errCheck(t, err)
		})
	}
}
