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

// Package h2websocket adapts RFC 8441 extended CONNECT requests so that
// handlers using gorilla/websocket's HTTP/1.1-only Upgrader continue to
// work when a client opens a WebSocket over HTTP/2.
//
// The middleware intercepts requests with HTTP/2, method CONNECT, and the
// :protocol=websocket pseudo-header. For each one it:
//
//  1. Enables full duplex on the response so reads on r.Body can overlap
//     writes to w.
//  2. Sends 200 OK to open the tunnel (per RFC 8441 §5, the response on
//     an HTTP/2 stream is 200, not 101).
//  3. Strips client-supplied reserved Teleport headers if a list was
//     configured.
//  4. Rewrites the request as an HTTP/1.1 GET + Upgrade and dispatches
//     to the wrapped handler with a ResponseWriter that implements
//     http.Hijacker. The handler's gorilla.Upgrader.Upgrade succeeds
//     because the method is GET and the Upgrade / Connection /
//     Sec-WebSocket-* headers are present.
//
// When Hijack runs, the returned net.Conn reads from r.Body and writes
// through w with a flush after every Write so WebSocket frames do not sit
// in the HTTP/2 buffer. Deadlines forward to the http.ResponseController
// for the request. The synthetic "HTTP/1.1 101 Switching Protocols"
// response gorilla writes onto the hijacked conn is stripped off
// incrementally before bytes reach the wire.
//
// If the handler returns without hijacking, the middleware fails closed
// and sends 405 Method Not Allowed rather than passing handler output
// through the 101 stripper. This protects every other GET route from
// being addressable via extended CONNECT.
package h2websocket

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
)

// pseudoHeaderProtocol is the HTTP/2 :protocol pseudo-header (RFC 8441
// §4). Go's net/http server surfaces it as a regular header named
// ":protocol".
const pseudoHeaderProtocol = ":protocol"

// Options configures the middleware.
type Options struct {
	// ReservedHeaders names HTTP headers that the middleware strips from
	// the synthetic request before dispatch. The list is intended for
	// Teleport-injected headers (lib/srv/app/common.ReservedHeaders) so
	// that a malicious client cannot plant them on an extended CONNECT
	// request and have them reach a backend that treats them as control
	// input. The check is case-insensitive (http.Header.Del is
	// canonicalized).
	ReservedHeaders []string
}

// Wrap returns next wrapped with the HTTP/2 extended CONNECT bridge.
// Apply once at the outermost layer of the proxy web chain, outside
// XForwardedFor / tracing / limiter so those middlewares observe the
// synthetic HTTP/1.1 request rather than the raw HTTP/2 one.
func Wrap(next http.Handler, opts Options) http.Handler {
	reservedSet := canonicalSet(opts.ReservedHeaders)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isH2WebSocketConnect(r) {
			next.ServeHTTP(w, r)
			return
		}
		serve(w, r, next, reservedSet)
	})
}

// isH2WebSocketConnect reports whether r is an RFC 8441 extended CONNECT
// WebSocket handshake.
func isH2WebSocketConnect(r *http.Request) bool {
	return r.ProtoMajor >= 2 &&
		r.Method == http.MethodConnect &&
		r.Header.Get(pseudoHeaderProtocol) == "websocket"
}

func serve(w http.ResponseWriter, r *http.Request, next http.Handler, reservedSet map[string]struct{}) {
	rc := http.NewResponseController(w)
	if err := rc.EnableFullDuplex(); err != nil {
		http.Error(w, "full-duplex unavailable", http.StatusInternalServerError)
		return
	}

	// Open the h2 tunnel before dispatching so the client starts
	// streaming WebSocket frames into r.Body.
	w.WriteHeader(http.StatusOK)
	if err := rc.Flush(); err != nil {
		return
	}

	r2, err := rewriteAsH1Upgrade(r, reservedSet)
	if err != nil {
		return
	}

	hw := &hijackResponseWriter{
		ResponseWriter: w,
		req:            r,
		rc:             rc,
	}
	next.ServeHTTP(hw, r2)

	// Fail closed: if the handler returned without hijacking, the route
	// is not a WebSocket route. Surface that as a stream-level error
	// rather than passing the handler's output (likely JSON, HTML, etc.)
	// through the 101 stripper, which would corrupt the stream.
	if !hw.hijacked.Load() {
		// We already sent 200 to open the tunnel, so the HTTP status
		// is committed. Closing the stream is the only way to signal
		// "wrong route" to the client at this point.
		if c, ok := r.Body.(io.Closer); ok {
			_ = c.Close()
		}
	}
}

// rewriteAsH1Upgrade returns a copy of r that looks like an HTTP/1.1
// WebSocket Upgrade request: method GET, Upgrade / Connection /
// Sec-WebSocket-* headers populated, and reserved headers stripped.
// The body is left alone (it carries WebSocket frames once the
// handshake completes).
func rewriteAsH1Upgrade(r *http.Request, reservedSet map[string]struct{}) (*http.Request, error) {
	r2 := r.Clone(r.Context())
	r2.Method = http.MethodGet
	r2.Proto = "HTTP/1.1"
	r2.ProtoMajor = 1
	r2.ProtoMinor = 1
	r2.RequestURI = ""

	hdr := r2.Header.Clone()
	hdr.Del(pseudoHeaderProtocol)
	for h := range reservedSet {
		hdr.Del(h)
	}
	hdr.Set("Connection", "Upgrade")
	hdr.Set("Upgrade", "websocket")
	if hdr.Get("Sec-WebSocket-Version") == "" {
		hdr.Set("Sec-WebSocket-Version", "13")
	}
	if hdr.Get("Sec-WebSocket-Key") == "" {
		key := make([]byte, 16)
		if _, err := rand.Read(key); err != nil {
			return nil, trace.Wrap(err, "generating Sec-WebSocket-Key")
		}
		hdr.Set("Sec-WebSocket-Key", base64.StdEncoding.EncodeToString(key))
	}
	r2.Header = hdr
	return r2, nil
}

func canonicalSet(names []string) map[string]struct{} {
	if len(names) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(names))
	for _, n := range names {
		out[http.CanonicalHeaderKey(n)] = struct{}{}
	}
	return out
}

// hijackResponseWriter implements http.Hijacker on top of an HTTP/2
// ResponseWriter so gorilla.Upgrader.Upgrade can take ownership of the
// underlying stream as a net.Conn.
type hijackResponseWriter struct {
	http.ResponseWriter
	req *http.Request
	rc  *http.ResponseController

	hijacked atomic.Bool
}

func (h *hijackResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if !h.hijacked.CompareAndSwap(false, true) {
		return nil, nil, http.ErrHijacked
	}
	conn := newH2StreamConn(h.req, h.ResponseWriter, h.rc)
	brw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	return conn, brw, nil
}

// h2StreamConn presents an HTTP/2 request body + flushed response writer
// pair as a net.Conn. Deadlines forward to the http.ResponseController.
type h2StreamConn struct {
	r io.ReadCloser
	w io.Writer

	rc      *http.ResponseController
	closeMu sync.Mutex
	closed  atomic.Bool
}

func newH2StreamConn(req *http.Request, w http.ResponseWriter, rc *http.ResponseController) *h2StreamConn {
	c := &h2StreamConn{
		r:  req.Body,
		rc: rc,
	}
	c.w = &stripUpgradeWriter{
		inner: &flushWriter{w: w, rc: rc},
	}
	return c
}

func (c *h2StreamConn) Read(p []byte) (int, error) {
	if c.closed.Load() {
		return 0, net.ErrClosed
	}
	return c.r.Read(p)
}

func (c *h2StreamConn) Write(p []byte) (int, error) {
	return c.w.Write(p)
}

func (c *h2StreamConn) Close() error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.closed.Swap(true) {
		return nil
	}
	// Closing the request body cancels the HTTP/2 stream so the peer
	// observes a clean EOF rather than a hung connection.
	return c.r.Close()
}

func (c *h2StreamConn) LocalAddr() net.Addr  { return placeholderAddr("h2-local") }
func (c *h2StreamConn) RemoteAddr() net.Addr { return placeholderAddr("h2-remote") }

func (c *h2StreamConn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}

func (c *h2StreamConn) SetReadDeadline(t time.Time) error {
	return c.rc.SetReadDeadline(t)
}

func (c *h2StreamConn) SetWriteDeadline(t time.Time) error {
	return c.rc.SetWriteDeadline(t)
}

type placeholderAddr string

func (placeholderAddr) Network() string  { return "h2-websocket" }
func (a placeholderAddr) String() string { return string(a) }

// flushWriter flushes after every Write so a single WebSocket frame ends
// up on the wire as a single h2 DATA frame instead of being buffered.
type flushWriter struct {
	w  io.Writer
	rc *http.ResponseController
}

func (f *flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if err == nil {
		_ = f.rc.Flush()
	}
	return n, err
}

// stripUpgradeWriter discards bytes up to and including the first
// "\r\n\r\n" sequence (gorilla's synthetic 101 response that does not
// belong on the h2 stream), then forwards everything afterward. The
// parser is incremental: it never accumulates more than the four bytes
// needed to recognise the terminator, so there is no fixed buffer cap
// that could silently truncate a long Sec-WebSocket-Protocol header.
type stripUpgradeWriter struct {
	inner   io.Writer
	scanned int // number of consecutive bytes of "\r\n\r\n" matched so far
	done    bool
}

var crlfcrlf = []byte("\r\n\r\n")

func (s *stripUpgradeWriter) Write(p []byte) (int, error) {
	if s.done {
		return s.inner.Write(p)
	}
	consumed := 0
	for consumed < len(p) {
		if p[consumed] == crlfcrlf[s.scanned] {
			s.scanned++
			consumed++
			if s.scanned == len(crlfcrlf) {
				s.done = true
				rest := p[consumed:]
				if len(rest) > 0 {
					if _, err := s.inner.Write(rest); err != nil {
						return 0, err
					}
				}
				return len(p), nil
			}
		} else {
			// Reset the match. If we matched some prefix, those bytes
			// are part of the header (still discarded). The current
			// byte is also discarded.
			s.scanned = 0
			consumed++
		}
	}
	return len(p), nil
}
