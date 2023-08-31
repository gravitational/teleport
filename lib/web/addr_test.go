/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package web

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestNewXForwardedForMiddleware(t *testing.T) {
	t.Parallel()

	observeredAddr := utils.MustParseAddr("11.22.33.44:1234")
	xForwardedAddr := utils.MustParseAddr("55.66.77.88:5678")

	// Setup response writer with observeredAddr.
	fakeConn, _ := net.Pipe()

	// Setup request with observeredAddr.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(utils.ClientSrcAddrContext(req.Context(), observeredAddr))
	req.RemoteAddr = observeredAddr.String()

	// Setup other requests for testing.
	reqWithXFF := req.Clone(req.Context())
	reqWithXFF.Header.Set("X-Forwarded-For", xForwardedAddr.String())

	reqWithMultipleXFF := req.Clone(req.Context())
	reqWithMultipleXFF.Header.Add("X-Forwarded-For", xForwardedAddr.String())
	reqWithMultipleXFF.Header.Add("X-Forwarded-For", "88.77.66.55:8765")

	tests := []struct {
		name           string
		inputReq       *http.Request
		wantRemoteAddr string
		wantError      bool
	}{
		{
			name:           "no X-Forwarded-For header",
			inputReq:       req,
			wantRemoteAddr: observeredAddr.String(),
		},
		{
			name:      "multiple X-Forwarded-For values",
			inputReq:  reqWithMultipleXFF,
			wantError: true,
		},
		{
			name:           "using X-Forwarded-For header",
			inputReq:       reqWithXFF,
			wantRemoteAddr: xForwardedAddr.String(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checkRemoteAddr := http.HandlerFunc(func(outputRW http.ResponseWriter, outputReq *http.Request) {
				// Verify hijacked conn.
				hj, ok := outputRW.(http.Hijacker)
				require.True(t, ok)
				outputConn, _, err := hj.Hijack()
				require.NoError(t, err)
				require.Equal(t, test.wantRemoteAddr, outputConn.RemoteAddr().String())

				// Verify request.
				require.Equal(t, test.wantRemoteAddr, outputReq.RemoteAddr)

				// Verify request context.
				clientSrcAddr, _ := utils.ClientAddrFromContext(outputReq.Context())
				require.Equal(t, test.wantRemoteAddr, clientSrcAddr.String())

				outputRW.WriteHeader(http.StatusAccepted)
			})

			recorder := httptest.NewRecorder()
			inputResponseWriter := &responseWriterWithRemoteAddr{
				ResponseWriter: newResponseWriterHijacker(recorder, fakeConn),
				remoteAddr:     observeredAddr,
			}
			handler := NewXForwardedForMiddleware(checkRemoteAddr)
			handler.ServeHTTP(inputResponseWriter, test.inputReq)

			response := recorder.Result()
			defer response.Body.Close()
			if test.wantError {
				require.Equal(t, http.StatusBadRequest, response.StatusCode)
			} else {
				require.Equal(t, http.StatusAccepted, response.StatusCode)
			}
		})
	}

}

func TestParseXForwardedForHeaders(t *testing.T) {
	t.Parallel()

	inputObserveredAddr := "1.2.3.4:12345"
	tests := []struct {
		name                      string
		inputXForwardedForHeaders []string
		wantAddr                  string
		wantError                 func(error) bool
	}{
		{
			name:                      "empty",
			inputXForwardedForHeaders: []string{},
			wantError:                 trace.IsNotFound,
		},
		{
			name:                      "invalid X-Forwarded-For",
			inputXForwardedForHeaders: []string{"not-an-ip"},
			wantError:                 trace.IsBadParameter,
		},
		{
			name:                      "ipv4",
			inputXForwardedForHeaders: []string{"3.4.5.6"},
			wantAddr:                  "3.4.5.6:12345",
		},
		{
			name:                      "ipv4 with port",
			inputXForwardedForHeaders: []string{"3.4.5.6:22222"},
			wantAddr:                  "3.4.5.6:22222",
		},
		{
			name:                      "ipv6",
			inputXForwardedForHeaders: []string{"2001:db8::21f:5bff:febf:ce22:8a2e"},
			wantAddr:                  "[2001:db8:0:21f:5bff:febf:ce22:8a2e]:12345",
		},
		{
			name:                      "ipv6 with port",
			inputXForwardedForHeaders: []string{"[2001:db8::21f:5bff:febf:ce22:8a2e]:22222"},
			wantAddr:                  "[2001:db8:0:21f:5bff:febf:ce22:8a2e]:22222",
		},
		{
			name:                      "multiple IPs",
			inputXForwardedForHeaders: []string{"3.4.5.6, 7.8.9.10, 11.12.13.14"},
			wantError:                 trace.IsBadParameter,
		},
		{
			name:                      "multiple headers",
			inputXForwardedForHeaders: []string{"3.4.5.6", "7.8.9.10", "11.12.13.14"},
			wantError:                 trace.IsBadParameter,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualAddr, err := parseXForwardedForHeaders(inputObserveredAddr, test.inputXForwardedForHeaders)
			if test.wantError != nil {
				require.True(t, test.wantError(err))
			} else {
				require.NoError(t, err)
				require.Equal(t, test.wantAddr, actualAddr.String())
			}
		})
	}
}
