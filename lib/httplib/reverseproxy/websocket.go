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

package reverseproxy

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"sync"

	"github.com/gravitational/trace"
)

// handleH2WebSocket handles the h2 extended CONNECT (RFC 8441) that Chrome
// sends for WebSocket over HTTP/2. the backend only speaks HTTP/1.1 WebSocket
// so we rewrite it, do the handshake, then pipe both ways until done.
func (f *Forwarder) handleH2WebSocket(w http.ResponseWriter, r *http.Request) error {
	f.logger.DebugContext(r.Context(), "h2 WebSocket CONNECT received",
		"path", r.URL.Path,
		"proto", r.Proto,
		"method", r.Method,
		":protocol", r.Header.Get(":protocol"),
	)

	// need full-duplex or we can't read and write the h2 stream at the same time
	rc := http.NewResponseController(w)
	if err := rc.EnableFullDuplex(); err != nil {
		return trace.Wrap(err, "enabling full-duplex mode for WebSocket over HTTP/2")
	}

	// rewrite into the HTTP/1.1 upgrade the backend expects
	outReq := r.Clone(r.Context())
	outReq.Method = http.MethodGet
	outReq.Proto = "HTTP/1.1"
	outReq.ProtoMajor = 1
	outReq.ProtoMinor = 1
	// Transport rejects outgoing requests with RequestURI set
	outReq.RequestURI = ""
	// upgrade is header-only, data is piped below. ContentLength must be 0
	// not -1 or Transport will reject it with a nil body
	outReq.Body = nil
	outReq.GetBody = nil
	outReq.ContentLength = 0
	outReq.Header.Del(":protocol")
	outReq.Header.Set("Connection", "Upgrade")
	outReq.Header.Set("Upgrade", "websocket")
	// RFC 8441 §4 says the client can omit these, but the backend usually
	// needs them for the HTTP/1.1 handshake, so fill them in if missing
	if outReq.Header.Get("Sec-Websocket-Key") == "" {
		key := make([]byte, 16)
		if _, err := rand.Read(key); err != nil {
			return trace.Wrap(err, "generating Sec-WebSocket-Key")
		}
		outReq.Header.Set("Sec-Websocket-Key", base64.StdEncoding.EncodeToString(key))
	}
	if outReq.Header.Get("Sec-Websocket-Version") == "" {
		outReq.Header.Set("Sec-Websocket-Version", "13")
	}

	f.logger.DebugContext(r.Context(), "forwarding WebSocket upgrade to backend",
		"path", outReq.URL.Path,
	)
	resp, err := f.transport.RoundTrip(outReq)
	if err != nil {
		f.logger.DebugContext(r.Context(), "WebSocket upgrade RoundTrip failed", "error", err)
		return trace.Wrap(err, "forwarding WebSocket upgrade request")
	}
	defer resp.Body.Close()

	f.logger.DebugContext(r.Context(), "got response from backend", "status", resp.StatusCode)
	if resp.StatusCode != http.StatusSwitchingProtocols {
		return trace.BadParameter("expected 101 Switching Protocols from backend, got %d", resp.StatusCode)
	}

	// after 101 the response body is the raw WebSocket conn
	backendConn, ok := resp.Body.(io.ReadWriteCloser)
	if !ok {
		return trace.BadParameter("backend WebSocket connection is not bidirectional")
	}

	// RFC 8441 wants 200 here, not 101
	f.logger.DebugContext(r.Context(), "sending 200 OK to browser, opening tunnel")
	w.WriteHeader(http.StatusOK)
	if err := rc.Flush(); err != nil {
		return trace.Wrap(err, "flushing 200 response")
	}

	// pipe both ways, must wait or returning here closes the h2 stream
	f.logger.DebugContext(r.Context(), "WebSocket tunnel up, piping", "path", r.URL.Path)
	wg := sync.WaitGroup{}
	wg.Add(2)
	wg.Go(func() {
		n, err := io.Copy(backendConn, r.Body)
		f.logger.DebugContext(r.Context(), "browser→backend done", "bytes", n, "error", err)
		backendConn.Close()
	})
	wg.Go(func() {
		n, err := io.Copy(&h2FlushWriter{w: w, rc: rc}, backendConn)
		f.logger.DebugContext(r.Context(), "backend→browser done", "bytes", n, "error", err)
	})
	wg.Wait()
	return nil
}

// h2FlushWriter flushes after every write so frames don't sit in the h2 buffer
type h2FlushWriter struct {
	w  http.ResponseWriter
	rc *http.ResponseController
}

func (fw *h2FlushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if err == nil {
		fw.rc.Flush()
	}
	return n, err
}
