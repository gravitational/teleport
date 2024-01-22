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
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
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
			require.Len(t, expectedPayload, n)
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
				})

				NewXForwardedForMiddleware(connUpgradeHandler).ServeHTTP(w, r)
			}()

			select {
			case handlerErr := <-handlerErrChan:
				if test.checkHandlerError != nil {
					require.Error(t, handlerErr)
					require.True(t, test.checkHandlerError(handlerErr))
				} else {
					require.NoError(t, handlerErr)
				}

			case <-w.hijackedCtx.Done():
				mustReadSwitchProtocolsResponse(t, r, clientConn, test.inputUpgradeType)
				test.checkClientConnString(t, clientConn, expectedPayload)

			case <-time.After(5 * time.Second):
				require.Fail(t, "timed out waiting for handler to serve")
			}
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

	// hijackedCtx is canceled when Hijack is called
	hijackedCtx       context.Context
	hijackedCtxCancel context.CancelFunc
}

func newResponseWriterHijacker(w http.ResponseWriter, conn net.Conn) *responseWriterHijacker {
	hijackedCtx, hijackedCtxCancel := context.WithCancel(context.Background())
	if w == nil {
		w = httptest.NewRecorder()
	}
	return &responseWriterHijacker{
		ResponseWriter:    w,
		conn:              conn,
		hijackedCtx:       hijackedCtx,
		hijackedCtxCancel: hijackedCtxCancel,
	}
}

func (h *responseWriterHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijackedCtxCancel()
	return h.conn, nil, nil
}
