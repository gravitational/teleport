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

package app

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
)

// requestIDContextKey is the context key carrying the per-request recording ID.
type requestIDContextKey struct{}

// withRequestID returns a copy of ctx carrying id as the per-request recording
// ID. The transport reads it back to correlate its HTTP audit events with the
// body chunks recorded by the handler.
func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, id)
}

// requestIDFromContext returns the per-request recording ID, or "" if HTTP
// recording is disabled for this request.
func requestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}

// chunkEmitter is called for each payload produced by a Read call.
// data is the raw bytes; index is the zero-based chunk sequence number;
// isLast is true for the final chunk.
type chunkEmitter func(data []byte, index int64, isLast bool)

// recordingBody wraps an io.ReadCloser and calls emit for every non-empty
// Read result. Chunk boundaries follow OS-level read sizes — no artificial
// splitting or buffering.
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
		isLast := err == io.EOF
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

// recordingResponseWriter wraps an http.ResponseWriter and calls emit for every
// non-empty Write, capturing the response body exactly as it is sent to the
// client. Chunk boundaries follow the writes performed by the proxy — no
// artificial splitting or buffering.
//
// It forwards the optional Flusher and Hijacker interfaces so the reverse
// proxy's response streaming and protocol upgrades (e.g. websockets) keep
// working when the writer is wrapped.
type recordingResponseWriter struct {
	http.ResponseWriter
	emit  chunkEmitter
	index int64
}

func newRecordingResponseWriter(w http.ResponseWriter, emit chunkEmitter) *recordingResponseWriter {
	return &recordingResponseWriter{ResponseWriter: w, emit: emit}
}

func (w *recordingResponseWriter) Write(p []byte) (int, error) {
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

// finish emits a final zero-byte chunk with isLast=true so consumers know the
// response body ended. It must be called once after the handler returns.
func (w *recordingResponseWriter) finish() {
	w.emit(nil, w.index, true)
}
