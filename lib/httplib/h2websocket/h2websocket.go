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
// by tearing down the HTTP/2 stream. The 200 status is already committed
// to open the tunnel, so closing the stream is the only signal left.
// The wrapped ResponseWriter also silences Write and WriteHeader before
// hijack so a non-WebSocket route reached via extended CONNECT cannot
// leak its response body onto the stream as opaque WebSocket payload.
//
// Sec-WebSocket-Protocol negotiation is not propagated. The wrapped
// HTTP/1.1 upgrader writes the negotiated value into the synthetic 101
// preamble that the stripper discards, and RFC 8441 §5.1 places
// subprotocol negotiation in the H2 response HEADERS frame instead.
// Until the bridge lifts the negotiated subprotocol onto the outer
// response headers before committing the 200, routes that depend on
// gorilla.Upgrader.Subprotocols (kube exec, desktop, conn upgrade)
// must not be reached over HTTP/2.
package h2websocket

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
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
	// Teleport-injected identity headers (e.g. X-Teleport-Jwt-Assertion,
	// X-Teleport-Aws-Assumed-Role) so that a malicious client cannot
	// plant them on an extended CONNECT request and have them reach a
	// backend that treats them as control input. The check is
	// case-insensitive (http.Header.Del is canonicalized).
	//
	// Do not include generic X-Forwarded-* headers here: the XFF
	// middleware that runs inside the wrapped chain needs to see them
	// to resolve the real client address. Stripping them at this layer
	// would point clientSrcAddr at the load balancer for h2 traffic
	// while h1 traffic on the same listener resolves the real IP.
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

func serve(w http.ResponseWriter, r *http.Request, next http.Handler, reservedSet map[string]bool) {
	rc := http.NewResponseController(w)
	if err := rc.EnableFullDuplex(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// The 200 commits before the inner handler runs, so any handler
	// rejection (auth failure, route not found) shows up in proxy
	// access logs as 200 rather than 4xx. Alerting that pages on 4xx
	// spikes will miss h2-ws auth failures; log from the inner handler
	// if the metric matters.
	w.WriteHeader(http.StatusOK)
	if err := rc.Flush(); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return
	}

	r2, err := rewriteAsH1Upgrade(r, reservedSet)
	if err != nil {
		return
	}

	hw := &hijackResponseWriter{
		ResponseWriter: w,
		body:           r.Body,
		remoteAddr:     r.RemoteAddr,
		rc:             rc,
	}
	next.ServeHTTP(hw, r2)

	// Handler returned without hijacking: close the stream to signal
	// "wrong route" (the 200 status is already committed). The
	// hijackResponseWriter gate above also discards any Write the
	// handler made before returning, so opaque bytes do not leak.
	if !hw.hijacked.Load() {
		_ = r.Body.Close()
	}
}

// rewriteAsH1Upgrade returns a copy of r that looks like an HTTP/1.1
// WebSocket Upgrade request: method GET, Upgrade / Connection /
// Sec-WebSocket-* headers populated, and reserved headers stripped.
// The body is left alone (it carries WebSocket frames once the
// handshake completes).
func rewriteAsH1Upgrade(r *http.Request, reservedSet map[string]bool) (*http.Request, error) {
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
		// RFC 8441 §5.1 lets the client omit Sec-WebSocket-Key under
		// extended CONNECT because the h2 stream already protects
		// request integrity. The synthesized key is never observed by
		// the h2 client; it exists only so the gorilla upgrader's
		// HTTP/1.1 path accepts the synthetic request.
		key := make([]byte, 16)
		if _, err := rand.Read(key); err != nil {
			return nil, trace.Wrap(err, "generating Sec-WebSocket-Key")
		}
		hdr.Set("Sec-WebSocket-Key", base64.StdEncoding.EncodeToString(key))
	}
	r2.Header = hdr
	return r2, nil
}

func canonicalSet(names []string) map[string]bool {
	if len(names) == 0 {
		return nil
	}
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[http.CanonicalHeaderKey(n)] = true
	}
	return out
}

// hijackResponseWriter implements http.Hijacker on top of an HTTP/2
// ResponseWriter so an HTTP/1.1 upgrader can take ownership of the
// underlying stream as a net.Conn. Write and WriteHeader are silenced
// before hijack so a handler that returns without hijacking (e.g. a
// non-WebSocket route reached via extended CONNECT) cannot leak its
// response body onto the h2 stream as opaque WebSocket payload.
//
// Header() returns the embedded writer's header map. Mutations before
// hijack are no-ops on the wire because the 200 status is already
// committed; rely on the hijacked conn or context for error signaling.
type hijackResponseWriter struct {
	http.ResponseWriter
	body       io.ReadCloser
	remoteAddr string
	rc         *http.ResponseController

	hijacked atomic.Bool
}

func (h *hijackResponseWriter) Write(p []byte) (int, error) {
	if !h.hijacked.Load() {
		return len(p), nil
	}
	return h.ResponseWriter.Write(p)
}

func (h *hijackResponseWriter) WriteHeader(code int) {
	if !h.hijacked.Load() {
		return
	}
	h.ResponseWriter.WriteHeader(code)
}

func (h *hijackResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if !h.hijacked.CompareAndSwap(false, true) {
		return nil, nil, http.ErrHijacked
	}
	conn := newH2StreamConn(h.body, h.ResponseWriter, h.remoteAddr, h.rc)
	brw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	return conn, brw, nil
}

// h2StreamConn presents an HTTP/2 request body + flushed response writer
// pair as a net.Conn. Deadlines forward to the http.ResponseController.
// LocalAddr returns a placeholder because net/http's HTTP/2 server
// does not expose stream-level local addresses on the hijacked conn;
// RemoteAddr forwards the request's RemoteAddr when set.
type h2StreamConn struct {
	r          io.ReadCloser
	w          io.Writer
	remoteAddr string

	rc     *http.ResponseController
	closed atomic.Bool
}

func newH2StreamConn(body io.ReadCloser, w http.ResponseWriter, remoteAddr string, rc *http.ResponseController) *h2StreamConn {
	c := &h2StreamConn{
		r:          body,
		remoteAddr: remoteAddr,
		rc:         rc,
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
	// Swap serializes Close to a single caller; subsequent calls
	// short-circuit before touching the request body.
	if c.closed.Swap(true) {
		return nil
	}
	// Closing the request body cancels the HTTP/2 stream so the peer
	// observes a clean EOF rather than a hung connection.
	return c.r.Close()
}

func (c *h2StreamConn) LocalAddr() net.Addr { return placeholderAddr("h2-local") }

func (c *h2StreamConn) RemoteAddr() net.Addr {
	// netip.ParseAddrPort accepts only literal IP:port forms, so this
	// cannot trigger a synchronous DNS lookup if r.RemoteAddr is ever
	// rewritten to a hostname-style value (matches lib/web/addr.go).
	if c.remoteAddr != "" {
		if ap, err := netip.ParseAddrPort(c.remoteAddr); err == nil {
			return net.TCPAddrFromAddrPort(ap)
		}
	}
	return placeholderAddr("h2-remote")
}

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

// placeholderAddr stands in when the request's RemoteAddr is missing
// or unparseable. The HTTP/2 server does not expose stream-level
// addresses on the hijacked conn; the request's RemoteAddr is the only
// peer information available, and a placeholder keeps callers that log
// conn.RemoteAddr() from panicking on a nil addr.
type placeholderAddr string

func (placeholderAddr) Network() string  { return "h2-websocket" }
func (a placeholderAddr) String() string { return string(a) }

// flushWriter flushes after every Write so a single WebSocket frame
// ends up on the wire as a single h2 DATA frame instead of being
// buffered. Flush errors are propagated so a stream reset (Write
// succeeds into the buffer, Flush fails) surfaces to the WebSocket
// handler instead of being silently dropped. ErrNotSupported is
// suppressed because not every ResponseWriter implements Flusher.
type flushWriter struct {
	w  http.ResponseWriter
	rc *http.ResponseController
}

func (f *flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if err != nil {
		return n, err
	}
	if err := f.rc.Flush(); err != nil && !errors.Is(err, http.ErrNotSupported) {
		// Per io.Writer, n must reflect bytes durably committed; on
		// Flush failure the bytes sit in the h2 send buffer and never
		// reach the wire, so report zero.
		return 0, err
	}
	return n, nil
}

// stripUpgradeWriter discards bytes up to and including the first
// "\r\n\r\n" sequence (the synthetic HTTP/1.1 response preamble the
// wrapped upgrader emits before owning the conn), then forwards
// everything afterward. The parser is incremental: it never
// accumulates more than the four bytes needed to recognize the
// terminator, so there is no fixed buffer cap that could silently
// truncate a long Sec-WebSocket-Protocol header.
type stripUpgradeWriter struct {
	inner   io.Writer
	scanned int // number of consecutive bytes of upgradeHeaderTerminator matched so far
	done    bool
}

const upgradeHeaderTerminator = "\r\n\r\n"

func (s *stripUpgradeWriter) Write(p []byte) (int, error) {
	if s.done {
		return s.inner.Write(p)
	}
	for i := range p {
		s.scanned = nextScanState(s.scanned, p[i])
		if s.scanned == len(upgradeHeaderTerminator) {
			s.done = true
			rest := p[i+1:]
			if len(rest) > 0 {
				if _, err := s.inner.Write(rest); err != nil {
					return 0, err
				}
			}
			return len(p), nil
		}
	}
	return len(p), nil
}

// nextScanState advances the upgradeHeaderTerminator matcher by one
// byte. The pattern has self-overlap on '\r' (positions 0 and 2), so
// on a mismatch the failing byte may itself start a new match: e.g.
// the second '\r' in "\r\r\n\r\n" must not be discarded. This is the
// KMP failure function for "\r\n\r\n" collapsed to a single comparison.
func nextScanState(scanned int, b byte) int {
	if b == upgradeHeaderTerminator[scanned] {
		return scanned + 1
	}
	if b == upgradeHeaderTerminator[0] {
		return 1
	}
	return 0
}
