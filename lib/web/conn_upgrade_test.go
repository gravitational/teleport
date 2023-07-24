/*
Copyright 2022 Gravitational, Inc.

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
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/pingconn"
	"github.com/gravitational/teleport/lib/utils"
)

func TestWriteUpgradeResponse(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, writeUpgradeResponse(&buf, "custom"))

	resp, err := http.ReadResponse(bufio.NewReader(&buf), nil)
	require.NoError(t, err)

	// Always drain/close the body.
	io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	require.Equal(t, resp.StatusCode, http.StatusSwitchingProtocols)
	require.Equal(t, "custom", resp.Header.Get("Upgrade"))
}

func TestHandlerConnectionUpgrade(t *testing.T) {
	t.Parallel()

	expectedPayload := "hello@"
	expectedIP := "1.2.3.4"
	alpnHandler := func(_ context.Context, conn net.Conn) error {
		// Handles connection asynchronously to verify web handler waits until
		// connection is closed.
		go func() {
			defer conn.Close()

			clientIP, err := utils.ClientIPFromConn(conn)
			require.NoError(t, err)
			require.Equal(t, expectedIP, clientIP)

			n, err := conn.Write([]byte(expectedPayload))
			require.NoError(t, err)
			require.Equal(t, len(expectedPayload), n)
		}()
		return nil
	}

	// Cherry picked some attributes to create a Handler to test only the
	// connection upgrade portion.
	h := &Handler{
		cfg: Config{
			ALPNHandler: alpnHandler,
		},
		log:   newPackageLogger(),
		clock: clockwork.NewRealClock(),
	}

	tests := []struct {
		name                  string
		inputUpgradeHeaderKey string
		inputUpgradeType      string
		checkHandlerError     func(error) bool
		checkClientConnString func(*testing.T, net.Conn, string)
	}{
		{
			name:              "unsupported type",
			inputUpgradeType:  "unsupported-protocol",
			checkHandlerError: trace.IsNotFound,
		},
		{
			name:                  "upgraded to ALPN",
			inputUpgradeType:      constants.WebAPIConnUpgradeTypeALPN,
			checkClientConnString: mustReadClientConnString,
		},
		{
			name:                  "upgraded to ALPN with Ping",
			inputUpgradeType:      constants.WebAPIConnUpgradeTypeALPNPing,
			checkClientConnString: mustReadClientPingConnString,
		},
		{
			name:                  "upgraded to ALPN with Teleport-specific header",
			inputUpgradeHeaderKey: constants.WebAPIConnUpgradeTeleportHeader,
			inputUpgradeType:      constants.WebAPIConnUpgradeTypeALPN,
			checkClientConnString: mustReadClientConnString,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			serverConn, clientConn := net.Pipe()
			defer serverConn.Close()
			defer clientConn.Close()

			// serverConn will be hijacked.
			w := newResponseWriterHijacker(nil, serverConn)
			r := makeConnUpgradeRequest(t, test.inputUpgradeHeaderKey, test.inputUpgradeType, expectedIP)

			// Serve the handler with XForwardedFor middleware to set IPs.
			handlerErrChan := make(chan error, 1)
			go func() {
				connUpgradeHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err := h.connectionUpgrade(w, r, nil)
					handlerErrChan <- err

					// Catch the case where an error is not expected but still
					// happened. Close the connection to unblock the client
					// reader and print out the error.
					if test.checkHandlerError == nil {
						clientConn.Close()
						require.NoError(t, err)
					}
				})

				NewXForwardedForMiddleware(connUpgradeHandler).ServeHTTP(w, r)
			}()

			if test.checkHandlerError != nil {
				handlerErr := <-handlerErrChan
				require.Error(t, handlerErr)
				require.True(t, test.checkHandlerError(handlerErr))
				return
			}

			mustReadSwitchProtocolsResponse(t, r, clientConn, test.inputUpgradeType)
			test.checkClientConnString(t, clientConn, expectedPayload)
		})
	}
}

func makeConnUpgradeRequest(t *testing.T, upgradeHeaderKey, upgradeType, xForwardedFor string) *http.Request {
	t.Helper()

	if upgradeHeaderKey == "" {
		upgradeHeaderKey = constants.WebAPIConnUpgradeHeader
	}

	r, err := http.NewRequest("GET", "http://localhost/webapi/connectionupgrade", nil)
	require.NoError(t, err)
	r.Header.Add(upgradeHeaderKey, upgradeType)
	r.Header.Add("X-Forwarded-For", xForwardedFor)
	return r
}

func mustReadSwitchProtocolsResponse(t *testing.T, r *http.Request, clientConn net.Conn, upgradeType string) {
	t.Helper()

	resp, err := http.ReadResponse(bufio.NewReader(clientConn), r)
	require.NoError(t, err)

	// Always drain/close the body.
	io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	require.Equal(t, upgradeType, resp.Header.Get(constants.WebAPIConnUpgradeHeader))
	require.Equal(t, upgradeType, resp.Header.Get(constants.WebAPIConnUpgradeTeleportHeader))
	require.Equal(t, constants.WebAPIConnUpgradeConnectionType, resp.Header.Get(constants.WebAPIConnUpgradeConnectionHeader))
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
}

func mustReadClientConnString(t *testing.T, clientConn net.Conn, expectedPayload string) {
	t.Helper()

	receive, err := bufio.NewReader(clientConn).ReadString(byte('@'))
	require.NoError(t, err)
	require.Equal(t, expectedPayload, receive)
}

func mustReadClientPingConnString(t *testing.T, clientConn net.Conn, expectedPayload string) {
	t.Helper()

	mustReadClientConnString(t, pingconn.New(clientConn), expectedPayload)
}

// responseWriterHijacker is a mock http.ResponseWriter that also serves a
// net.Conn for http.Hijacker.
type responseWriterHijacker struct {
	http.ResponseWriter
	conn net.Conn
}

func newResponseWriterHijacker(w http.ResponseWriter, conn net.Conn) http.ResponseWriter {
	if w == nil {
		w = httptest.NewRecorder()
	}
	return &responseWriterHijacker{
		ResponseWriter: w,
		conn:           conn,
	}
}

func (h *responseWriterHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return h.conn, nil, nil
}
