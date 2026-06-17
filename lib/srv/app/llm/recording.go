// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package llm

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

// chunkEmitter is called for each payload produced by a Read or Write call.
// data is the raw bytes; index is the zero-based chunk sequence number;
// isLast is true for the final chunk.
type chunkEmitter func(data []byte, index int64, isLast bool)

// recordingBody wraps an io.ReadCloser and calls emit for every non-empty
// Read result. Chunk boundaries follow OS-level read sizes — no artificial
// splitting or buffering. It is wrapped around the downstream request body so
// the recording captures exactly what the client sent, before the provider
// request is built from it.
type recordingBody struct {
	inner   io.ReadCloser
	emit    chunkEmitter
	index   int64
	eofSeen bool
}

func newRecordingBody(inner io.ReadCloser, emit chunkEmitter) *recordingBody {
	return &recordingBody{inner: inner, emit: emit}
}

func (r *recordingBody) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	if n > 0 {
		isLast := errors.Is(err, io.EOF)
		if isLast {
			r.eofSeen = true
		}
		r.emit(p[:n], r.index, isLast)
		r.index++
	}
	return n, err
}

// Close closes the underlying body. If the body was abandoned before EOF,
// emits a final zero-byte chunk with isLast=true so consumers know the
// stream ended.
func (r *recordingBody) Close() error {
	if !r.eofSeen {
		r.emit(nil, r.index, true)
	}
	return r.inner.Close()
}

// headerEmitter is called once, when the response status and headers first
// become known (on WriteHeader, or the implicit-200 first Write). It reports
// what the client receives, along with the time elapsed waiting for the
// response.
type headerEmitter func(status int, header http.Header, waitMs int64)

// recordingResponseWriter wraps an http.ResponseWriter and records the response
// exactly as it is sent to the client. It is wrapped beneath the provider
// response recorder, so the captured status, headers and body bytes reflect the
// final, downstream-facing response (including any error rewriting the provider
// recorder performs).
//
// It forwards the optional Flusher and Hijacker interfaces so the reverse
// proxy's response streaming and protocol upgrades (e.g. websockets) keep
// working when the writer is wrapped.
type recordingResponseWriter struct {
	http.ResponseWriter
	onHeader   headerEmitter
	emit       chunkEmitter
	start      time.Time
	index      int64
	headerSent bool
}

func newRecordingResponseWriter(w http.ResponseWriter, start time.Time, onHeader headerEmitter, emit chunkEmitter) *recordingResponseWriter {
	return &recordingResponseWriter{ResponseWriter: w, start: start, onHeader: onHeader, emit: emit}
}

// emitHeader fires the onHeader callback exactly once, snapshotting the status
// and headers the client receives.
func (w *recordingResponseWriter) emitHeader(status int) {
	if w.headerSent {
		return
	}
	w.headerSent = true
	w.onHeader(status, w.Header().Clone(), time.Since(w.start).Milliseconds())
}

func (w *recordingResponseWriter) WriteHeader(status int) {
	w.emitHeader(status)
	w.ResponseWriter.WriteHeader(status)
}

func (w *recordingResponseWriter) Write(p []byte) (int, error) {
	// A Write without a preceding WriteHeader implies a 200 status.
	w.emitHeader(http.StatusOK)
	n, err := w.ResponseWriter.Write(p)
	if n > 0 {
		w.emit(p[:n], w.index, false)
		w.index++
	}
	return n, err
}

// Flush forwards to the underlying ResponseWriter so the reverse proxy's
// periodic flushing keeps working when streaming responses.
func (w *recordingResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack forwards to the underlying ResponseWriter so connection upgrades
// (e.g. websockets) keep working.
func (w *recordingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// finish emits the response metadata event (if no bytes were ever written) and
// a final zero-byte chunk with isLast=true so consumers know the response body
// ended. It must be called once after the handler returns.
func (w *recordingResponseWriter) finish() {
	w.emitHeader(http.StatusOK)
	w.emit(nil, w.index, true)
}
