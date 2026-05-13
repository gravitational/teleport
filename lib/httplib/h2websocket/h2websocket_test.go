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
	"errors"
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

func TestWrap_PassthroughNonWebSocket(t *testing.T) {
	type observed struct {
		method   string
		upgrade  string
		wsKey    string
		gotInner bool
	}
	ch := make(chan observed, 1)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ch <- observed{
			method:   r.Method,
			upgrade:  r.Header.Get("Upgrade"),
			wsKey:    r.Header.Get("Sec-WebSocket-Key"),
			gotInner: true,
		}
		w.WriteHeader(http.StatusTeapot)
	})
	srv := httptest.NewServer(Wrap(inner, Options{}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusTeapot, resp.StatusCode)
	got := <-ch
	require.True(t, got.gotInner)
	require.Equal(t, http.MethodGet, got.method)
	require.Empty(t, got.upgrade)
	require.Empty(t, got.wsKey)
}

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

func TestStripUpgradeWriter_CoalescedWrite(t *testing.T) {
	var sink bytes.Buffer
	w := &stripUpgradeWriter{inner: &sink}

	combined := []byte("HTTP/1.1 101 OK\r\nA: b\r\n\r\n" + "WS-FRAME")
	_, err := w.Write(combined)
	require.NoError(t, err)
	require.Equal(t, "WS-FRAME", sink.String())
}

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

// TestNextScanState exhausts the three transitions of the matcher
// (match-byte, restart-on-prefix, full-reset) so a regression on any
// branch surfaces independently of the integration cases above.
func TestNextScanState(t *testing.T) {
	tests := []struct {
		name    string
		scanned int
		b       byte
		want    int
	}{
		{name: "match first \\r", scanned: 0, b: '\r', want: 1},
		{name: "match \\n after \\r", scanned: 1, b: '\n', want: 2},
		{name: "match second \\r", scanned: 2, b: '\r', want: 3},
		{name: "match final \\n", scanned: 3, b: '\n', want: 4},
		{name: "mismatch at start", scanned: 0, b: 'a', want: 0},
		{name: "mismatch after \\r, byte is \\r (self-overlap)", scanned: 1, b: '\r', want: 1},
		{name: "mismatch after \\r\\n\\r, byte is \\r (self-overlap)", scanned: 3, b: '\r', want: 1},
		{name: "mismatch after \\r\\n, byte is unrelated", scanned: 2, b: 'a', want: 0},
		{name: "mismatch after \\r\\n\\r, byte is unrelated", scanned: 3, b: 'a', want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, nextScanState(tc.scanned, tc.b))
		})
	}
}

func TestCanonicalSet(t *testing.T) {
	got := canonicalSet([]string{"X-TELEPORT-foo", "x-teleport-BAR"})
	require.True(t, got["X-Teleport-Foo"])
	require.True(t, got["X-Teleport-Bar"])
	require.Nil(t, canonicalSet(nil))
}

func TestH2StreamConn_CloseIdempotent(t *testing.T) {
	c := &h2StreamConn{r: io.NopCloser(strings.NewReader(""))}
	require.NoError(t, c.Close())
	require.NoError(t, c.Close())
}

func TestH2StreamConn_ReadAfterClose(t *testing.T) {
	c := &h2StreamConn{r: io.NopCloser(strings.NewReader(""))}
	require.NoError(t, c.Close())

	_, err := c.Read(make([]byte, 4))
	require.ErrorIs(t, err, net.ErrClosed)
}

// TestFlushWriter exercises the three Flush outcomes: a successful
// flush returns (n, nil); ErrNotSupported is suppressed; any other
// Flush error is propagated with n=0 so callers cannot mistake
// buffered bytes for durably committed ones.
func TestFlushWriter(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		w := &flushingRW{}
		f := &flushWriter{w: w, rc: http.NewResponseController(w)}
		n, err := f.Write([]byte("hi"))
		require.NoError(t, err)
		require.Equal(t, 2, n)
		require.Equal(t, 1, w.flushes)
	})
	t.Run("ErrNotSupported is suppressed", func(t *testing.T) {
		// flushingRW with flushErr=ErrNotSupported still reports the
		// underlying Write count without surfacing the error.
		w := &flushingRW{flushErr: http.ErrNotSupported}
		f := &flushWriter{w: w, rc: http.NewResponseController(w)}
		n, err := f.Write([]byte("hi"))
		require.NoError(t, err)
		require.Equal(t, 2, n)
	})
	t.Run("real flush error surfaces with n=0", func(t *testing.T) {
		sentinel := errors.New("h2 stream reset")
		w := &flushingRW{flushErr: sentinel}
		f := &flushWriter{w: w, rc: http.NewResponseController(w)}
		n, err := f.Write([]byte("hi"))
		require.ErrorIs(t, err, sentinel)
		require.Equal(t, 0, n, "Write must report 0 when Flush failed")
	})
}

// TestServe_FailsClosedWhenHandlerDoesNotHijack verifies that an inner
// handler that returns without hijacking has its writes silenced and
// its body closed. This protects the h2 stream from being addressable
// as an opaque WebSocket payload echo by non-WebSocket routes.
func TestServe_FailsClosedWhenHandlerDoesNotHijack(t *testing.T) {
	body := &closableBuffer{r: strings.NewReader("client frames")}
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
	require.Equal(t, []int{http.StatusOK}, rec.statuses,
		"only the bridge's 200 reaches the writer; the inner handler's WriteHeader is gated")
	require.Empty(t, rec.body.Bytes(),
		"handler bytes must not leak onto the h2 stream as opaque payload")
}

func TestServe_FullDuplexUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodConnect, "/", nil)
	req.ProtoMajor = 2
	req.Header.Set(":protocol", "websocket")

	// Test-controlled signal: duplexWriter explicitly errors on
	// EnableFullDuplex. Avoids relying on httptest.NewRecorder
	// happening to lack the interface.
	rec := &duplexWriter{header: http.Header{}, enableFullDuplexErr: http.ErrNotSupported}
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	serve(rec, req, inner, nil)

	require.Equal(t, []int{http.StatusInternalServerError}, rec.statuses)
	require.False(t, called, "inner handler must not run when full duplex is unavailable")
}

// closableBuffer is a request body that tracks Close calls.
type closableBuffer struct {
	r      io.Reader
	closed atomic.Bool
}

func (b *closableBuffer) Read(p []byte) (int, error) { return b.r.Read(p) }
func (b *closableBuffer) Close() error               { b.closed.Store(true); return nil }

// duplexWriter is a minimal http.ResponseWriter that satisfies the
// full-duplex and flush hooks the bridge expects. Tests configure
// enableFullDuplexErr to drive the error branch explicitly; statuses
// records every WriteHeader call so tests can assert exact sequencing
// rather than relying on real net/http's "first WriteHeader wins"
// semantics.
type duplexWriter struct {
	header              http.Header
	body                bytes.Buffer
	statuses            []int
	enableFullDuplexErr error
}

func (d *duplexWriter) Header() http.Header  { return d.header }
func (d *duplexWriter) WriteHeader(code int) { d.statuses = append(d.statuses, code) }
func (d *duplexWriter) Write(p []byte) (int, error) {
	if len(d.statuses) == 0 {
		d.statuses = append(d.statuses, http.StatusOK)
	}
	return d.body.Write(p)
}

func (d *duplexWriter) EnableFullDuplex() error { return d.enableFullDuplexErr }
func (d *duplexWriter) Flush()                  {}

// flushingRW is a minimal http.ResponseWriter that records every
// Write/Flush call and reports a configurable Flush error.
type flushingRW struct {
	body     bytes.Buffer
	flushes  int
	flushErr error
}

func (w *flushingRW) Header() http.Header         { return http.Header{} }
func (w *flushingRW) WriteHeader(int)             {}
func (w *flushingRW) Write(p []byte) (int, error) { return w.body.Write(p) }
func (w *flushingRW) FlushError() error           { w.flushes++; return w.flushErr }
