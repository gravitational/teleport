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
package httprecorder

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/trace"
)

const (
	// maxChunkSize is the largest body payload stored in one chunk event.
	// These events are never sent to the audit log, so the limit is to allow
	// consumers to receive the message via streaming without being impacted
	// by the gRPC message size limit. The default gRPC limit is 4MB, so
	// 1MB is a safe choice.
	maxChunkSize = 1024 * 1024 // 1MB
)

// Config contains the inputs New needs to record one HTTP exchange.
type Config struct {
	// Request is the HTTP request to record. New wraps its body in place, so
	// callers should pass the request they will hand to the downstream handler.
	// Important: This request is not modified, so the caller must read from the
	// returned request instead of the original.
	Request *http.Request

	// ResponseWriter is wrapped so response headers and body writes can be
	// recorded before they reach the client.
	// Important: This writer is not modified, so the caller must write to the
	// returned writer instead of the original.
	ResponseWriter http.ResponseWriter

	// Recorder receives the prepared audit events.
	Recorder events.SessionPreparerRecorder

	// AppMetadata identifies the application being accessed.
	AppMetadata apievents.AppMetadata

	// UserMetadata identifies the user who initiated the session.
	UserMetadata apievents.UserMetadata

	// Logger reports recording failures. It is required so callers choose the
	// logging policy for this exchange.
	Logger *slog.Logger
}

// Validate checks that all required fields are set.
func (c *Config) Validate() error {
	if c.Request == nil {
		return trace.BadParameter("Request is required")
	}
	if c.ResponseWriter == nil {
		return trace.BadParameter("ResponseWriter is required")
	}
	if c.Recorder == nil {
		return trace.BadParameter("Recorder is required")
	}
	if c.AppMetadata.AppName == "" {
		return trace.BadParameter("AppMetadata.AppName is required")
	}
	if c.UserMetadata.User == "" {
		return trace.BadParameter("UserMetadata.User is required")
	}
	if c.Logger == nil {
		return trace.BadParameter("Logger is required")
	}
	return nil
}

// New wraps a request and response writer so a proxied HTTP exchange is
// recorded as audit events.
//
// It records AppSessionHTTPRequest before the exchange starts, records request
// body chunks as the downstream handler reads them, and returns a ResponseWriter
// that records the response head followed by response body chunks.
//
// The caller must invoke ResponseWriter.Finish once the handler returns so the
// final response chunk is emitted.
//
// Recording is fail-closed. If the initial request metadata event cannot be
// recorded, New returns an error before anything is proxied. Once proxying has
// started, failed request-body or response-body recording makes Read or Write
// return an error so unrecorded bytes do not continue through the exchange.
// Finish runs after the response is complete, so its final events are
// best-effort and failures are logged.
func New(
	cfg Config,
) (*http.Request, *ResponseWriter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	req := cfg.Request.WithContext(cfg.Request.Context())

	recorder := cfg.Recorder
	// Keep recording even if req.Context() is canceled. Client disconnects are
	// exactly when the tail of the exchange is useful to keep, and the
	// apievents.Emitter contract expects cancellation to be stripped.
	auditLogContext := context.WithoutCancel(req.Context())

	requestID := uuid.NewString()

	logger := cfg.Logger.With("request_id", requestID)

	// Treat recording as part of the exchange. If a required event cannot be
	// written, return the error so callers can stop proxying instead of allowing
	// unaudited traffic to keep flowing.
	record := func(e apievents.AuditEvent) error {
		if err := recordEvent(auditLogContext, e, recorder); err != nil {
			logger.WarnContext(auditLogContext, "Failed to record HTTP session event; failing the exchange closed",
				"error", err, "event_type", e.GetType())
			return trace.Wrap(err)
		}
		return nil
	}

	// If we cannot record request metadata, the exchange should not start.
	if err := record(newRequestEvent(cfg, requestID)); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Each downstream read records the bytes before they are handed to the
	// handler. A recording error aborts the read.
	recordRequestChunk := func(data []byte, index int64, isLast bool) error {
		return record(&apievents.AppSessionHTTPRequestBodyChunk{
			Metadata: apievents.Metadata{
				Type: events.AppSessionHTTPRequestBodyChunkEvent,
				Code: events.AppSessionHTTPRequestBodyChunkCode,
			},
			RequestId:  requestID,
			ChunkIndex: index,
			IsLast:     isLast,
			Data:       cloneChunk(data),
		})
	}

	// Wrap the request body so reads are recorded. If the body is nil or
	// http.NoBody, the handler will not read it, so no recording is needed.
	if req.Body != nil && req.Body != http.NoBody {
		req.Body = newBody(req.Body, recordRequestChunk)
	}

	// start is used to measure the time spent waiting for the response head.
	start := time.Now()

	// record the response head once the status and headers are known. A failure
	// is surfaced through ResponseWriter so it can stop the response body.
	onHeader := func(status int, header http.Header, waitMs int64) error {
		return record(&apievents.AppSessionHTTPResponse{
			Metadata: apievents.Metadata{
				Type: events.AppSessionHTTPResponseEvent,
				Code: events.AppSessionHTTPResponseCode,
			},
			AppMetadata: cfg.AppMetadata,
			RequestId:   requestID,
			StatusCode:  uint32(status),
			StatusText:  http.StatusText(status),
			HttpVersion: req.Proto,
			Headers:     filterHeaders(header),
			WaitTimeMs:  waitMs,
		})
	}

	// record each downstream write before it is sent to the client.
	emitChunk := func(data []byte, index int64, isLast bool) error {
		return record(&apievents.AppSessionHTTPResponseBodyChunk{
			Metadata: apievents.Metadata{
				Type: events.AppSessionHTTPResponseBodyChunkEvent,
				Code: events.AppSessionHTTPResponseBodyChunkCode,
			},
			RequestId:  requestID,
			ChunkIndex: index,
			IsLast:     isLast,
			Data:       cloneChunk(data),
		})
	}

	wrapped := newResponseWriter(
		cfg.ResponseWriter,
		start,
		onHeader,
		emitChunk,
	)
	return req, wrapped, nil
}

// redactedValue keeps sensitive headers visible in the audit log without
// storing their contents.
const redactedValue = "<redacted>"

// sensitiveHeaders lists headers whose values should never be stored in
// audit events.
var sensitiveHeaders = map[string]struct{}{
	"Authorization":        {},
	"Proxy-Authorization":  {},
	"Cookie":               {},
	"Set-Cookie":           {},
	"X-Api-Key":            {},
	"X-Auth-Token":         {},
	"X-Amz-Security-Token": {},
	"X-Csrf-Token":         {},
}

// filterHeaders flattens http.Header into audit-event header entries. Repeated
// values stay as separate entries, and known secrets values are redacted.
func filterHeaders(header http.Header) []*apievents.HTTPHeader {
	if len(header) == 0 {
		return nil
	}
	out := make([]*apievents.HTTPHeader, 0, len(header))
	for name, values := range header {
		_, redact := sensitiveHeaders[http.CanonicalHeaderKey(name)]
		for _, value := range values {
			if redact {
				value = redactedValue
			}
			out = append(out, &apievents.HTTPHeader{Name: name, Value: value})
		}
	}
	return out
}

// cloneChunk snapshots data before the caller's read/write buffer can be
// reused. Empty chunks are stored as nil so terminating chunks have no payload.
func cloneChunk(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	return bytes.Clone(data)
}

// chunkEmitter records one body chunk. Returning an error means the bytes were
// not recorded, so callers stop the Read or Write instead of passing them on.
type chunkEmitter func(data []byte, index int64, isLast bool) error

// body records bytes as they are read from an io.ReadCloser. It does not buffer
// or reshape the stream beyond splitting oversized reads.
//
// mu guards index, eofSeen, and emit calls. net/http may call Close to unblock a
// streaming Read on client disconnect, and that race should not corrupt the
// chunk sequence.
type body struct {
	inner io.ReadCloser
	emit  chunkEmitter

	mu      sync.Mutex
	index   int64
	eofSeen bool
}

// newBody records every read from inner. Reads larger than maxChunkSize are
// split across multiple emit calls, and one final empty chunk is emitted when
// EOF is seen or Close is called.
//
// The data passed to emit aliases the caller's read buffer and is only valid
// for the duration of the call; emit must copy it if it retains the bytes.
func newBody(inner io.ReadCloser, emit chunkEmitter) io.ReadCloser {
	return &body{inner: inner, emit: emit}
}

// Read passes through to the underlying body after recording the bytes it just
// read, splitting them at maxChunkSize when needed.
//
// If recording fails, Read returns the recording error and no bytes so the
// downstream reader never receives unaudited body data.
func (r *body) Read(p []byte) (int, error) {
	// inner.Read may block, so do not hold r.mu while a concurrent Close might
	// need to run.
	n, err := r.inner.Read(p)

	r.mu.Lock()
	defer r.mu.Unlock()
	if n > 0 {
		for chunk := range slices.Chunk(p[:n], maxChunkSize) {
			if recErr := r.emitChunkLocked(chunk, false); recErr != nil {
				return 0, recErr
			}
		}
	}
	if errors.Is(err, io.EOF) && !r.eofSeen {
		r.eofSeen = true
		if recErr := r.emitChunkLocked(nil, true); recErr != nil {
			return 0, recErr
		}
	}

	return n, err
}

// emitChunkLocked records a single body chunk and advances the chunk index. The
// caller holds r.mu.
func (r *body) emitChunkLocked(data []byte, isLast bool) error {
	err := r.emit(data, r.index, isLast)
	r.index++
	return err
}

// Close closes the underlying body. If EOF has not been seen, it records a final
// empty chunk so consumers know the stream ended.
func (r *body) Close() error {
	r.mu.Lock()
	var recErr error
	if !r.eofSeen {
		r.eofSeen = true
		recErr = r.emitChunkLocked(nil, true)
	}
	r.mu.Unlock()
	return errors.Join(recErr, r.inner.Close())
}

// headerEmitter records the response status and headers once they are known:
// either on WriteHeader or on the first Write's implicit 200. waitMs is the time
// spent waiting for that response head.
type headerEmitter func(status int, header http.Header, waitMs int64) error

// ResponseWriter records a response as it is sent to the client.
//
// It forwards Flusher and Hijacker when the wrapped writer supports them. It
// also exposes Unwrap so http.ResponseController can reach deadline and
// full-duplex controls on the underlying writer.
//
// It deliberately does not implement io.ReaderFrom. Otherwise io.Copy and
// net/http sendfile paths could bypass Write and skip recording.
type ResponseWriter struct {
	http.ResponseWriter
	onHeader   headerEmitter
	emit       chunkEmitter
	start      time.Time
	index      int64
	headerSent bool
	finished   bool
	// recordErr holds a response-head recording failure until the next Write,
	// because WriteHeader cannot return an error.
	recordErr error
}

// newResponseWriter wraps w with a response recorder.
func newResponseWriter(w http.ResponseWriter, start time.Time, onHeader headerEmitter, emit chunkEmitter) *ResponseWriter {
	return &ResponseWriter{ResponseWriter: w, start: start, onHeader: onHeader, emit: emit}
}

// Unwrap returns the wrapped writer for http.ResponseController.
func (w *ResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// emitHeader records response metadata exactly once, using the status and
// headers the client receives.
//
// 1xx informational responses are ignored here. A server may send several of
// them before the final status, and WriteHeader still forwards them to the
// client.
func (w *ResponseWriter) emitHeader(status int) error {
	if w.headerSent || (status >= 100 && status < 200) {
		return nil
	}
	w.headerSent = true
	return w.onHeader(status, w.Header().Clone(), time.Since(w.start).Milliseconds())
}

// WriteHeader records the response head before forwarding the status. If
// recording fails, the error is returned from the next Write and the body is not
// forwarded.
func (w *ResponseWriter) WriteHeader(status int) {
	if err := w.emitHeader(status); err != nil {
		w.recordErr = err
	}
	w.ResponseWriter.WriteHeader(status)
}

// emitChunk reports a single response body chunk and advances the chunk index.
func (w *ResponseWriter) emitChunk(data []byte, isLast bool) error {
	err := w.emit(data, w.index, isLast)
	w.index++
	return err
}

// Write records the payload before forwarding it to the underlying writer. If
// recording fails, Write returns the error and writes nothing.
func (w *ResponseWriter) Write(p []byte) (int, error) {
	// A Write without a preceding WriteHeader implies a 200 status.
	if err := w.emitHeader(http.StatusOK); err != nil {
		return 0, err
	}
	if w.recordErr != nil {
		return 0, w.recordErr
	}
	if !w.finished {
		if err := w.emitChunk(p, false); err != nil {
			return 0, err
		}
	}
	return w.ResponseWriter.Write(p)
}

// Flush forwards to the underlying writer when streaming is supported.
func (w *ResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack forwards to the underlying writer when connection upgrades are
// supported.
func (w *ResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Finish records response metadata if nothing wrote it yet, then records the
// final empty chunk. Call it once after the handler returns; later calls are
// ignored, so a deferred Finish is safe.
//
// Unlike Write, Finish is best-effort. It runs after the response has already
// been sent, so recording failures are logged instead of returned.
func (w *ResponseWriter) Finish() {
	if w.finished {
		return
	}
	w.finished = true
	_ = w.emitHeader(http.StatusOK)
	_ = w.emitChunk(nil, true)
}

// recordEvent prepares a session event (stamping session-related fields) and
// records it with the given recorder.
func recordEvent(ctx context.Context, e apievents.AuditEvent, recorder events.SessionPreparerRecorder) error {
	preparedEvent, err := recorder.PrepareSessionEvent(e)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(recorder.RecordEvent(ctx, preparedEvent))
}

// newRequestEvent builds the request metadata event for the exchange.
func newRequestEvent(cfg Config, requestID string) *apievents.AppSessionHTTPRequest {
	req := cfg.Request
	return &apievents.AppSessionHTTPRequest{
		Metadata: apievents.Metadata{
			Type: events.AppSessionHTTPRequestEvent,
			Code: events.AppSessionHTTPRequestCode,
		},
		UserMetadata: cfg.UserMetadata,
		AppMetadata:  cfg.AppMetadata,
		RequestId:    requestID,
		Method:       req.Method,
		// Url already contains the query string; RawQuery is duplicated here for
		// HAR compatibility so consumers can access it without re-parsing the URL.
		Url:         req.URL.String(),
		HttpVersion: req.Proto,
		Headers:     filterHeaders(req.Header),
		RawQuery:    req.URL.RawQuery,
	}
}
