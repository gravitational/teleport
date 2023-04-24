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

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestMaybeUpdateClientSrvAddr(t *testing.T) {
	t.Parallel()

	observeredAddr := utils.MustParseAddr("11.22.33.44:1234")
	xForwardedAddr := utils.MustParseAddr("55.66.77.88:5678")

	// Setup response writer with observeredAddr.
	fakeConn, _ := net.Pipe()
	rw := &responseWriterWithRemoteAddr{
		ResponseWriter: newResponseWriterHijacker(nil, fakeConn),
		remoteAddr:     observeredAddr,
	}

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
		name                  string
		inputUseXForwardedFor bool
		inputRW               http.ResponseWriter
		inputReq              *http.Request
		wantRemoteAddr        string
		wantError             bool
	}{
		{
			name:           "UseXForwardedFor not enabled",
			inputRW:        rw,
			inputReq:       req,
			wantRemoteAddr: observeredAddr.String(),
		},
		{
			name:                  "no X-Forwarded-For header",
			inputRW:               rw,
			inputReq:              req,
			inputUseXForwardedFor: true,
			wantRemoteAddr:        observeredAddr.String(),
		},
		{
			name:                  "multiple X-Forwarded-For values",
			inputRW:               rw,
			inputReq:              reqWithMultipleXFF,
			inputUseXForwardedFor: true,
			wantError:             true,
		},
		{
			name:                  "using X-Forwarded-For header",
			inputRW:               rw,
			inputReq:              reqWithXFF,
			inputUseXForwardedFor: true,
			wantRemoteAddr:        xForwardedAddr.String(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := &Handler{
				cfg: Config{
					UseXForwardedFor: test.inputUseXForwardedFor,
				},
			}

			outputRW, outputReq, err := h.maybeUpdateClientSrcAddr(test.inputRW, test.inputReq)
			if test.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

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
		})
	}

}

func TestParseForwardedAddr(t *testing.T) {
	t.Parallel()

	inputObserveredAddr := "1.2.3.4:12345"
	tests := []struct {
		name               string
		inputForwardedAddr string
		wantAddr           string
		wantError          bool
	}{
		{
			name:               "empty",
			inputForwardedAddr: "",
			wantError:          true,
		},
		{
			name:               "invalid X-Forwarded-For",
			inputForwardedAddr: "not-an-ip",
			wantError:          true,
		},
		{
			name:               "ipv4",
			inputForwardedAddr: "3.4.5.6",
			wantAddr:           "3.4.5.6:12345",
		},
		{
			name:               "ipv4 with port",
			inputForwardedAddr: "3.4.5.6:22222",
			wantAddr:           "3.4.5.6:22222",
		},
		{
			name:               "ipv6",
			inputForwardedAddr: "2001:db8::21f:5bff:febf:ce22:8a2e",
			wantAddr:           "[2001:db8:0:21f:5bff:febf:ce22:8a2e]:12345",
		},
		{
			name:               "ipv6 with port",
			inputForwardedAddr: "[2001:db8::21f:5bff:febf:ce22:8a2e]:22222",
			wantAddr:           "[2001:db8:0:21f:5bff:febf:ce22:8a2e]:22222",
		},
		{
			name:               "multiple IPs",
			inputForwardedAddr: "3.4.5.6, 7.8.9.10, 11.12.13.14",
			wantError:          true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualAddr, err := parseForwardedAddr(inputObserveredAddr, test.inputForwardedAddr)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.wantAddr, actualAddr.String())
			}
		})
	}
}
