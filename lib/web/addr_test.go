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

package web

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/authz"
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
	req = req.WithContext(authz.ContextWithClientSrcAddr(req.Context(), observeredAddr))
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
				clientSrcAddr, err := authz.ClientSrcAddrFromContext(outputReq.Context())
				require.NoError(t, err)
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
