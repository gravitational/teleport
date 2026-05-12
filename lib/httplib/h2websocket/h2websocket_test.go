/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package h2websocket

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// TestWrap_PassthroughNonWebSocket verifies that non-WebSocket requests
// flow through unmodified. Pre-fix failure mode: if Wrap rewrote every
// request, this test would observe r.Method == "GET" but with the
// synthetic Upgrade headers planted on it.
func TestWrap_PassthroughNonWebSocket(t *testing.T) {
	var seen *http.Request
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r
		w.WriteHeader(http.StatusTeapot)
	})
	srv := httptest.NewServer(Wrap(inner, Options{}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusTeapot, resp.StatusCode)
	require.NotNil(t, seen)
	require.Equal(t, http.MethodGet, seen.Method)
	require.Empty(t, seen.Header.Get("Upgrade"))
	require.Empty(t, seen.Header.Get("Sec-WebSocket-Key"))
}

// TestWrap_PassthroughH1WebSocket verifies that an HTTP/1.1 WebSocket
// Upgrade is not rewritten and the inner gorilla.Upgrader handles it.
// Pre-fix failure mode: a bug in isH2WebSocketConnect that returned true
// for h1 would route the request through the synthetic path and gorilla
// would see headers it has already received, double-stuffed.
func TestWrap_PassthroughH1WebSocket(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := websocket.Upgrader{}
		ws, err := up.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer ws.Close()
		require.NoError(t, ws.WriteMessage(websocket.BinaryMessage, []byte("hi")))
	})
	srv := httptest.NewServer(Wrap(inner, Options{}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	typ, payload, err := ws.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.BinaryMessage, typ)
	require.Equal(t, []byte("hi"), payload)
}

// TestRewriteAsH1Upgrade verifies the request rewrite: CONNECT becomes
// GET, the :protocol pseudo-header is dropped, Upgrade/Connection
// headers are set, Sec-WebSocket-* defaults are synthesised when
// missing, and reserved headers are stripped. Pre-fix failure: any
// missing piece prevents gorilla.Upgrader.Upgrade from accepting the
// request inside the inner handler.
func TestRewriteAsH1Upgrade(t *testing.T) {
	req := httptest.NewRequest(http.MethodConnect, "/ws", nil)
	req.ProtoMajor = 2
	req.Header.Set(":protocol", "websocket")
	req.Header.Set("X-Teleport-Aws-Assumed-Role", "evil")
	req.Header.Set("Authorization", "Bearer keep-me")

	got, err := rewriteAsH1Upgrade(req, canonicalSet([]string{
		"X-Teleport-Aws-Assumed-Role",
	}))
	require.NoError(t, err)

	require.Equal(t, http.MethodGet, got.Method)
	require.Equal(t, 1, got.ProtoMajor)
	require.Equal(t, "websocket", got.Header.Get("Upgrade"))
	require.Equal(t, "Upgrade", got.Header.Get("Connection"))
	require.Equal(t, "13", got.Header.Get("Sec-WebSocket-Version"))
	require.NotEmpty(t, got.Header.Get("Sec-WebSocket-Key"))
	require.Empty(t, got.Header.Get(":protocol"))
	require.Empty(t, got.Header.Get("X-Teleport-Aws-Assumed-Role"),
		"reserved header must be stripped")
	require.Equal(t, "Bearer keep-me", got.Header.Get("Authorization"),
		"non-reserved client header must survive")
}

// TestStripUpgradeWriter_DropsLeadingHeader verifies the incremental
// stripper discards the synthetic "HTTP/1.1 101 ..." header and
// forwards everything after the "\r\n\r\n" terminator. Pre-fix failure:
// without the stripper, the 101 bytes flow onto the h2 stream and
// confuse the browser's WebSocket frame parser.
func TestStripUpgradeWriter_DropsLeadingHeader(t *testing.T) {
	var sink bytes.Buffer
	w := &stripUpgradeWriter{inner: &sink}

	header := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\n" +
		"Connection: Upgrade\r\nSec-WebSocket-Accept: x\r\n\r\n"
	payload := []byte{0x82, 0x05, 'h', 'e', 'l', 'l', 'o'}

	_, err := w.Write([]byte(header))
	require.NoError(t, err)
	require.Empty(t, sink.Bytes(), "header bytes must not reach the wire")

	_, err = w.Write(payload)
	require.NoError(t, err)
	require.Equal(t, payload, sink.Bytes())
}

// TestStripUpgradeWriter_CoalescedWrite verifies the stripper handles a
// Write that contains both the trailing "\r\n\r\n" and the first frame
// bytes in one call. Pre-fix failure: a buffer-then-flush stripper that
// only forwards on a *separate* Write call would drop the inline frame.
func TestStripUpgradeWriter_CoalescedWrite(t *testing.T) {
	var sink bytes.Buffer
	w := &stripUpgradeWriter{inner: &sink}

	combined := []byte("HTTP/1.1 101 OK\r\nA: b\r\n\r\n" + "WS-FRAME")
	_, err := w.Write(combined)
	require.NoError(t, err)
	require.Equal(t, "WS-FRAME", sink.String())
}

// TestStripUpgradeWriter_ByteByByte verifies the incremental parser
// when the terminator is split across Write calls one byte at a time.
// Pre-fix failure: a stripper that scans only within a single Write
// would miss a "\r\n\r\n" that straddles two Writes.
func TestStripUpgradeWriter_ByteByByte(t *testing.T) {
	var sink bytes.Buffer
	w := &stripUpgradeWriter{inner: &sink}

	header := []byte("HTTP/1.1 101 OK\r\n\r\n")
	for _, b := range header {
		_, err := w.Write([]byte{b})
		require.NoError(t, err)
	}
	require.Empty(t, sink.Bytes())

	_, err := w.Write([]byte("X"))
	require.NoError(t, err)
	require.Equal(t, "X", sink.String())
}

// TestStripUpgradeWriter_LongHeader verifies the stripper has no fixed
// buffer cap: a Sec-WebSocket-Protocol response with many subprotocols
// can exceed any kilobyte-sized buffer. Pre-fix failure mode (against
// an earlier draft that used a 4 KiB buffer): the test would error with
// "buffer exceeded" before the terminator was reached.
func TestStripUpgradeWriter_LongHeader(t *testing.T) {
	var sink bytes.Buffer
	w := &stripUpgradeWriter{inner: &sink}

	longProtos := strings.Repeat("subproto-x,", 1000)
	header := "HTTP/1.1 101 OK\r\nSec-WebSocket-Protocol: " +
		longProtos + "last\r\n\r\nFRAMES"
	_, err := w.Write([]byte(header))
	require.NoError(t, err)
	require.Equal(t, "FRAMES", sink.String())
}

// TestCanonicalSet verifies the reserved-header lookup is
// case-insensitive in the way http.Header.Del expects.
func TestCanonicalSet(t *testing.T) {
	got := canonicalSet([]string{"x-teleport-foo", "X-Teleport-Bar"})
	_, ok := got["X-Teleport-Foo"]
	require.True(t, ok)
	_, ok = got["X-Teleport-Bar"]
	require.True(t, ok)
	require.Nil(t, canonicalSet(nil))
}

// TestH2StreamConn_CloseIdempotent verifies double-close is a no-op.
// Pre-fix failure: re-closing the request body twice would panic on
// some HTTP/2 stream implementations.
func TestH2StreamConn_CloseIdempotent(t *testing.T) {
	c := &h2StreamConn{r: io.NopCloser(strings.NewReader(""))}
	require.NoError(t, c.Close())
	require.NoError(t, c.Close())
}
