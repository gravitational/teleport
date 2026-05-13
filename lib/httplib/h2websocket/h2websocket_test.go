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
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// TestWrap_PassthroughNonWebSocket verifies that non-WebSocket requests
// flow through the wrapper unmodified.
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
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()
	defer resp.Body.Close()

	typ, payload, err := ws.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.BinaryMessage, typ)
	require.Equal(t, []byte("hi"), payload)
}

// TestIsH2WebSocketConnect covers the gate that decides whether to
// route a request through the bridge. Non-WS extended CONNECTs
// (:protocol=bytes, :protocol=connect-udp) and h2 GET requests must
// not be rewritten.
func TestIsH2WebSocketConnect(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		major    int
		protocol string
		want     bool
	}{
		{name: "h2 CONNECT websocket", method: http.MethodConnect, major: 2, protocol: "websocket", want: true},
		{name: "h2 CONNECT bytes", method: http.MethodConnect, major: 2, protocol: "bytes", want: false},
		{name: "h2 CONNECT connect-udp", method: http.MethodConnect, major: 2, protocol: "connect-udp", want: false},
		{name: "h2 CONNECT no protocol", method: http.MethodConnect, major: 2, protocol: "", want: false},
		{name: "h2 GET websocket", method: http.MethodGet, major: 2, protocol: "websocket", want: false},
		{name: "h1 CONNECT websocket", method: http.MethodConnect, major: 1, protocol: "websocket", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/ws", nil)
			req.ProtoMajor = tc.major
			if tc.protocol != "" {
				req.Header.Set(":protocol", tc.protocol)
			}
			require.Equal(t, tc.want, isH2WebSocketConnect(req))
		})
	}
}

// TestRewriteAsH1Upgrade verifies the request rewrite: CONNECT becomes
// GET, the :protocol pseudo-header is dropped, Upgrade/Connection
// headers are set, Sec-WebSocket-* defaults are synthesized when
// missing, and reserved headers are stripped.
func TestRewriteAsH1Upgrade(t *testing.T) {
	req := httptest.NewRequest(http.MethodConnect, "/ws", nil)
	req.ProtoMajor = 2
	req.Header.Set(":protocol", "websocket")
	req.Header.Set("X-Teleport-Aws-Assumed-Role", "evil")
	req.Header.Set("Authorization", "Bearer keep-me")

	got, err := rewriteAsH1Upgrade(req, canonicalSet([]string{"X-Teleport-Aws-Assumed-Role"}))
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

// TestRewriteAsH1Upgrade_PreservesClientWebsocketHeaders verifies that
// a client-supplied Sec-WebSocket-Key and Sec-WebSocket-Version survive
// the rewrite verbatim instead of being overwritten by the defaults.
func TestRewriteAsH1Upgrade_PreservesClientWebsocketHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodConnect, "/ws", nil)
	req.ProtoMajor = 2
	req.Header.Set(":protocol", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Sec-WebSocket-Version", "8")

	got, err := rewriteAsH1Upgrade(req, nil)
	require.NoError(t, err)
	require.Equal(t, "dGhlIHNhbXBsZSBub25jZQ==", got.Header.Get("Sec-WebSocket-Key"))
	require.Equal(t, "8", got.Header.Get("Sec-WebSocket-Version"))
}

// TestStripUpgradeWriter_DropsLeadingHeader verifies the incremental
// stripper discards the synthetic "HTTP/1.1 101 ..." header and
// forwards everything after the "\r\n\r\n" terminator.
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
// bytes in one call.
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
// can exceed any kilobyte-sized buffer.
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

// TestStripUpgradeWriter_SelfOverlap covers the KMP-style restart in
// the matcher: on input that contains "\r\r\n\r\n" the second '\r'
// must not be discarded as a plain mismatch byte, because it starts
// the real terminator. A naive reset-to-zero matcher would skip past
// it and miss the terminator.
func TestStripUpgradeWriter_SelfOverlap(t *testing.T) {
	var sink bytes.Buffer
	w := &stripUpgradeWriter{inner: &sink}

	header := "HTTP/1.1 101 OK\r\nX-Stray-CR: \r\r\n\r\nFRAMES"
	_, err := w.Write([]byte(header))
	require.NoError(t, err)
	require.Equal(t, "FRAMES", sink.String())
}

// TestCanonicalSet verifies the reserved-header lookup is
// case-insensitive in the way http.Header.Del expects, including
// pathological mixed-case inputs.
func TestCanonicalSet(t *testing.T) {
	got := canonicalSet([]string{"X-TELEPORT-foo", "x-teleport-BAR"})
	require.True(t, got["X-Teleport-Foo"])
	require.True(t, got["X-Teleport-Bar"])
	require.Nil(t, canonicalSet(nil))
}

// TestH2StreamConn_CloseIdempotent verifies double-close is a no-op
// and that Read after Close returns net.ErrClosed.
func TestH2StreamConn_CloseIdempotent(t *testing.T) {
	c := &h2StreamConn{r: io.NopCloser(strings.NewReader(""))}
	require.NoError(t, c.Close())
	require.NoError(t, c.Close())

	_, err := c.Read(make([]byte, 4))
	require.ErrorIs(t, err, net.ErrClosed)
}

// TestServe_FailsClosedWhenHandlerDoesNotHijack verifies that an inner
// handler that returns without hijacking has its writes silenced and
// its body closed. This protects the h2 stream from being addressable
// as an opaque WebSocket payload echo by non-WebSocket routes.
func TestServe_FailsClosedWhenHandlerDoesNotHijack(t *testing.T) {
	body := &closableBuffer{Reader: strings.NewReader("client frames")}
	req := httptest.NewRequest(http.MethodConnect, "/", body)
	req.ProtoMajor = 2
	req.Header.Set(":protocol", "websocket")

	rec := &duplexWriter{header: http.Header{}}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a non-WebSocket route that writes a JSON body.
		_, _ = w.Write([]byte(`{"error":"not a websocket route"}`))
	})

	serve(rec, req, inner, nil)

	require.True(t, body.closed.Load(), "request body must be closed")
	require.Equal(t, http.StatusOK, rec.status, "status is committed at the tunnel open")
	require.Empty(t, rec.body.Bytes(),
		"handler bytes must not leak onto the h2 stream as opaque payload")
}

// TestServe_FullDuplexUnavailable verifies that serve fails with 500
// and does not dispatch the inner handler when EnableFullDuplex
// reports the writer cannot stream both directions.
func TestServe_FullDuplexUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodConnect, "/", nil)
	req.ProtoMajor = 2
	req.Header.Set(":protocol", "websocket")

	rec := httptest.NewRecorder()
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	serve(rec, req, inner, nil)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.False(t, called, "inner handler must not run when full duplex is unavailable")
}

// closableBuffer is a request body that tracks Close calls.
type closableBuffer struct {
	io.Reader
	closed atomic.Bool
}

func (b *closableBuffer) Close() error {
	b.closed.Store(true)
	return nil
}

// duplexWriter is a minimal http.ResponseWriter that satisfies the
// full-duplex and flush hooks the bridge expects. It lets tests run
// serve end-to-end without standing up a real HTTP/2 server.
type duplexWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (d *duplexWriter) Header() http.Header { return d.header }
func (d *duplexWriter) WriteHeader(code int) {
	if d.status == 0 {
		d.status = code
	}
}

func (d *duplexWriter) Write(p []byte) (int, error) {
	if d.status == 0 {
		d.status = http.StatusOK
	}
	return d.body.Write(p)
}

func (d *duplexWriter) EnableFullDuplex() error { return nil }
func (d *duplexWriter) Flush()                  {}
