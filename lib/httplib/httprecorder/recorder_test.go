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
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
)

// newCaptureRecorder returns a capturing recorder with a no-op event preparer.
func newCaptureRecorder() (*eventstest.MockRecorderEmitter, events.SessionPreparerRecorder) {
	c := &eventstest.MockRecorderEmitter{}
	return c, events.WithNoOpPreparer(c)
}

func TestBodyNormalRead(t *testing.T) {
	t.Parallel()
	type chunk struct {
		index  int64
		isLast bool
		data   []byte
	}
	var chunks []chunk
	body := io.NopCloser(strings.NewReader("hello world"))
	rb := newBody(body, func(data []byte, index int64, isLast bool) error {
		cp := slices.Clone(data)
		chunks = append(chunks, chunk{index, isLast, cp})
		return nil
	})

	buf := make([]byte, 5)
	n, err := rb.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 5, n)

	n, err = rb.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 5, n)

	n, _ = rb.Read(buf)
	require.Equal(t, 1, n)

	require.NoError(t, rb.Close())

	var got []byte
	for _, c := range chunks {
		got = append(got, c.data...)
	}
	require.Equal(t, "hello world", string(got))

	require.NotEmpty(t, chunks)
	for i, c := range chunks[:len(chunks)-1] {
		require.False(t, c.isLast, "chunk %d must not be last", i)
	}
	require.True(t, chunks[len(chunks)-1].isLast, "last chunk must have isLast=true")

	for i, c := range chunks {
		require.Equal(t, int64(i), c.index)
	}
}

// TestBodyChunkSplitting covers a read large enough to span several audit
// events.
func TestBodyChunkSplitting(t *testing.T) {
	t.Parallel()
	// 2.5 * maxChunkSize so the read splits into 3 chunks (1MB, 1MB, 0.5MB).
	size := maxChunkSize*2 + maxChunkSize/2
	src := bytes.Repeat([]byte("x"), size)

	var dataChunks [][]byte
	var lastSeen bool
	rb := newBody(io.NopCloser(bytes.NewReader(src)), func(data []byte, _ int64, isLast bool) error {
		if len(data) > 0 {
			require.LessOrEqual(t, len(data), maxChunkSize, "no chunk may exceed maxChunkSize")
			dataChunks = append(dataChunks, bytes.Clone(data))
		}
		if isLast {
			lastSeen = true
		}
		return nil
	})

	// read everything at once so the split happens inside the recorder, not
	// across multiple reads.
	buf := make([]byte, size)
	n, _ := io.ReadFull(rb, buf)
	require.Equal(t, size, n)
	_, _ = io.ReadAll(rb)
	require.NoError(t, rb.Close())

	require.GreaterOrEqual(t, len(dataChunks), 3, "data larger than maxChunkSize must split")
	require.True(t, lastSeen)
	require.Equal(t, src, bytes.Join(dataChunks, nil))
}

func TestBodyEmptyBody(t *testing.T) {
	t.Parallel()
	type chunk struct {
		index  int64
		isLast bool
		data   []byte
	}
	var chunks []chunk
	body := io.NopCloser(bytes.NewReader(nil))
	rb := newBody(body, func(data []byte, index int64, isLast bool) error {
		cp := make([]byte, len(data))
		copy(cp, data)
		chunks = append(chunks, chunk{index, isLast, cp})
		return nil
	})

	buf := make([]byte, 8)
	n, err := rb.Read(buf)
	require.Equal(t, 0, n)
	require.ErrorIs(t, err, io.EOF)

	require.NoError(t, rb.Close())

	// even an empty body gets a final chunk from Close.
	require.Len(t, chunks, 1)
	require.True(t, chunks[0].isLast)
	require.Empty(t, chunks[0].data)
}

func TestBodyCloseBeforeEOF(t *testing.T) {
	t.Parallel()
	type chunk struct {
		index  int64
		isLast bool
		data   []byte
	}
	var chunks []chunk
	body := io.NopCloser(strings.NewReader("not fully read"))
	rb := newBody(body, func(data []byte, index int64, isLast bool) error {
		cp := make([]byte, len(data))
		copy(cp, data)
		chunks = append(chunks, chunk{index, isLast, cp})
		return nil
	})

	buf := make([]byte, 3)
	_, err := rb.Read(buf)
	require.NoError(t, err)

	require.NoError(t, rb.Close())

	// closing before EOF still marks the stream complete.
	require.NotEmpty(t, chunks)
	last := chunks[len(chunks)-1]
	require.True(t, last.isLast)
}

func TestResponseWriter(t *testing.T) {
	t.Parallel()
	type chunk struct {
		index  int64
		isLast bool
		data   []byte
	}
	var chunks []chunk
	rec := httptest.NewRecorder()
	rw := newResponseWriter(rec, time.Now(),
		func(int, http.Header, int64) error { return nil },
		func(data []byte, index int64, isLast bool) error {
			cp := slices.Clone(data)
			chunks = append(chunks, chunk{index, isLast, cp})
			return nil
		})

	rw.WriteHeader(http.StatusCreated)
	_, err := rw.Write([]byte("hello "))
	require.NoError(t, err)
	_, err = rw.Write([]byte("world"))
	require.NoError(t, err)
	require.NoError(t, rw.Finish())

	// the wrapped writer should leave the client-visible response unchanged.
	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "hello world", rec.Body.String())

	var got []byte
	for _, c := range chunks {
		got = append(got, c.data...)
	}
	require.Equal(t, "hello world", string(got))

	for i, c := range chunks {
		require.Equal(t, int64(i), c.index)
	}

	for i, c := range chunks[:len(chunks)-1] {
		require.False(t, c.isLast, "chunk %d must not be last", i)
	}
	last := chunks[len(chunks)-1]
	require.True(t, last.isLast)
	require.Empty(t, last.data)
}

// TestResponseWriterChunkSplitting covers a single Write large enough to span
// several audit events, mirroring TestBodyChunkSplitting for the response path.
func TestResponseWriterChunkSplitting(t *testing.T) {
	t.Parallel()
	// 2.5 * maxChunkSize so the write splits into 3 chunks (1MB, 1MB, 0.5MB).
	size := maxChunkSize*2 + maxChunkSize/2
	src := bytes.Repeat([]byte("x"), size)

	var dataChunks [][]byte
	rec := httptest.NewRecorder()
	rw := newResponseWriter(rec, time.Now(),
		func(int, http.Header, int64) error { return nil },
		func(data []byte, _ int64, isLast bool) error {
			if len(data) > 0 {
				require.LessOrEqual(t, len(data), maxChunkSize, "no chunk may exceed maxChunkSize")
				dataChunks = append(dataChunks, bytes.Clone(data))
			}
			return nil
		})

	// a single large write must be split inside the recorder.
	n, err := rw.Write(src)
	require.NoError(t, err)
	require.Equal(t, size, n)
	require.NoError(t, rw.Finish())

	require.GreaterOrEqual(t, len(dataChunks), 3, "data larger than maxChunkSize must split")
	require.Equal(t, src, bytes.Join(dataChunks, nil))
	// the client still receives the full, unmodified payload.
	require.Equal(t, size, rec.Body.Len())
}

// TestResponseWriterHeaderEmittedOnce checks both explicit and implicit
// response heads.
func TestResponseWriterHeaderEmittedOnce(t *testing.T) {
	t.Parallel()
	t.Run("explicit WriteHeader", func(t *testing.T) {
		t.Parallel()
		var (
			calls   int
			status  int
			headers http.Header
		)
		rec := httptest.NewRecorder()
		rw := newResponseWriter(rec, time.Now(),
			func(s int, h http.Header, _ int64) error {
				calls++
				status = s
				headers = h
				return nil
			},
			func([]byte, int64, bool) error { return nil })

		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusBadGateway)
		_, err := rw.Write([]byte("a"))
		require.NoError(t, err)
		_, err = rw.Write([]byte("b"))
		require.NoError(t, err)
		require.NoError(t, rw.Finish())

		require.Equal(t, 1, calls, "header callback must fire exactly once")
		require.Equal(t, http.StatusBadGateway, status)
		require.Equal(t, "application/json", headers.Get("Content-Type"))
	})

	t.Run("implicit 200 on first Write", func(t *testing.T) {
		t.Parallel()
		var (
			calls  int
			status int
		)
		rec := httptest.NewRecorder()
		rw := newResponseWriter(rec, time.Now(),
			func(s int, _ http.Header, _ int64) error {
				calls++
				status = s
				return nil
			},
			func([]byte, int64, bool) error { return nil })

		_, err := rw.Write([]byte("body"))
		require.NoError(t, err)
		require.NoError(t, rw.Finish())

		require.Equal(t, 1, calls)
		require.Equal(t, http.StatusOK, status)
	})
}

// flushHijackRecorder lets tests check the optional streaming and upgrade
// interfaces.
type flushHijackRecorder struct {
	*httptest.ResponseRecorder
	flushed  bool
	hijacked bool
}

func (f *flushHijackRecorder) Flush() { f.flushed = true }

func (f *flushHijackRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	f.hijacked = true
	return nil, nil, nil
}

func TestResponseWriterForwardsFlushAndHijack(t *testing.T) {
	t.Parallel()
	inner := &flushHijackRecorder{ResponseRecorder: httptest.NewRecorder()}
	rw := newResponseWriter(inner, time.Now(), func(int, http.Header, int64) error { return nil }, func([]byte, int64, bool) error { return nil })

	// the wrapper keeps streaming and protocol upgrades available.
	flusher, ok := any(rw).(http.Flusher)
	require.True(t, ok, "ResponseWriter must implement http.Flusher")
	flusher.Flush()
	require.True(t, inner.flushed)

	hijacker, ok := any(rw).(http.Hijacker)
	require.True(t, ok, "ResponseWriter must implement http.Hijacker")
	_, _, err := hijacker.Hijack()
	require.NoError(t, err)
	require.True(t, inner.hijacked)
}

// TestResponseWriterFlushRecordsHead covers a handler (such as an SSE stream)
// that flushes before any Write or WriteHeader: the flush must record the
// implicit 200 head before committing it to the client.
func TestResponseWriterFlushRecordsHead(t *testing.T) {
	t.Parallel()
	var (
		calls  int
		status int
	)
	inner := &flushHijackRecorder{ResponseRecorder: httptest.NewRecorder()}
	rw := newResponseWriter(inner, time.Now(),
		func(s int, _ http.Header, _ int64) error {
			calls++
			status = s
			return nil
		},
		func([]byte, int64, bool) error { return nil })

	rw.Flush()

	require.Equal(t, 1, calls, "flush must record the implicit response head")
	require.Equal(t, http.StatusOK, status)
	require.True(t, inner.flushed)
}

// TestResponseWriterFlushFailsClosed covers a flush whose implicit-head
// recording fails: the flush must be suppressed and the failure surfaced
// through Finish so an unrecorded response is never committed to the client.
func TestResponseWriterFlushFailsClosed(t *testing.T) {
	t.Parallel()
	inner := &flushHijackRecorder{ResponseRecorder: httptest.NewRecorder()}
	rw := newResponseWriter(inner, time.Now(),
		func(int, http.Header, int64) error { return assert.AnError },
		func([]byte, int64, bool) error { return nil })

	rw.Flush()
	require.False(t, inner.flushed, "an unrecorded head must not be flushed to the client")

	require.ErrorIs(t, rw.Finish(), assert.AnError)
}

// TestResponseWriterConcurrentWriteFlush exercises the mutex that lets a
// goroutine flush on an interval (the SSE-style use case) run concurrently with
// the handler's writes. synctest's fake clock fires the ticker deterministically
// so the interleaving of Write and Flush is reproducible; run under the race
// detector (go test -race) to check the locking.
func TestResponseWriterConcurrentWriteFlush(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		inner := &flushHijackRecorder{ResponseRecorder: httptest.NewRecorder()}
		rw := newResponseWriter(inner, time.Now(),
			func(int, http.Header, int64) error { return nil },
			func([]byte, int64, bool) error { return nil })

		const (
			chunk       = "chunk"
			writes      = 5
			flushPeriod = 10 * time.Millisecond
		)

		// a goroutine that flushes on an interval, like a periodic flusher.
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(flushPeriod)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					rw.Flush()
				}
			}
		}()

		for range writes {
			_, err := rw.Write([]byte(chunk))
			require.NoError(t, err)

			// Advance the fake clock past the flush period so the ticker fires
			// once, then wait for that Flush to complete. This pins the ordering
			// of each Write/Flush pair.
			time.Sleep(flushPeriod)
			synctest.Wait()
		}

		close(done)
		synctest.Wait()

		require.NoError(t, rw.Finish())
		require.Equal(t, writes*len(chunk), inner.Body.Len(), "every write must reach the client")
		require.True(t, inner.flushed, "the periodic flusher must have run")
	})
}

// TestResponseWriterHijackUnsupported covers a writer that cannot be hijacked.
func TestResponseWriterHijackUnsupported(t *testing.T) {
	t.Parallel()
	rw := newResponseWriter(httptest.NewRecorder(), time.Now(), func(int, http.Header, int64) error { return nil }, func([]byte, int64, bool) error { return nil })
	_, _, err := rw.Hijack()
	require.ErrorIs(t, err, http.ErrNotSupported)
}

// deadlineRecorder exposes deadline controls only through
// http.ResponseController's Unwrap path.
type deadlineRecorder struct {
	*httptest.ResponseRecorder
	deadlineSet bool
}

func (d *deadlineRecorder) SetWriteDeadline(time.Time) error {
	d.deadlineSet = true
	return nil
}

// TestResponseWriterUnwrap verifies http.ResponseController can reach the
// underlying writer's controls through the wrapper via Unwrap.
func TestResponseWriterUnwrap(t *testing.T) {
	t.Parallel()
	inner := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	rw := newResponseWriter(inner, time.Now(), func(int, http.Header, int64) error { return nil }, func([]byte, int64, bool) error { return nil })

	require.Same(t, inner, rw.Unwrap())

	// http.ResponseController should reach SetWriteDeadline through Unwrap.
	rc := http.NewResponseController(rw)
	require.NoError(t, rc.SetWriteDeadline(time.Now().Add(time.Second)))
	require.True(t, inner.deadlineSet, "SetWriteDeadline must reach the underlying writer")
}

// TestFinishIdempotent makes sure Finish can be safely deferred or called more
// than once.
func TestFinishIdempotent(t *testing.T) {
	t.Parallel()
	type chunk struct {
		isLast bool
		data   []byte
	}
	var chunks []chunk
	rec := httptest.NewRecorder()
	rw := newResponseWriter(rec, time.Now(),
		func(int, http.Header, int64) error { return nil },
		func(data []byte, _ int64, isLast bool) error {
			chunks = append(chunks, chunk{isLast, bytes.Clone(data)})
			return nil
		})

	_, err := rw.Write([]byte("body"))
	require.NoError(t, err)
	require.NoError(t, rw.Finish())
	require.NoError(t, rw.Finish())
	// a stray write after Finish should not record after the terminator.
	_, err = rw.Write([]byte("late"))
	require.NoError(t, err)

	var lastCount int
	for i, c := range chunks {
		if c.isLast {
			lastCount++
			require.Equal(t, len(chunks)-1, i, "terminating chunk must be last")
		}
	}
	require.Equal(t, 1, lastCount, "exactly one terminating chunk")
}

// TestRecordingFailureFailsClosed covers the fail-closed path for response
// recording.
func TestRecordingFailureFailsClosed(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// errAfterFirst lets New succeed, then fails the next event.
	rec := &errAfterFirstRecorder{}
	underlying := httptest.NewRecorder()

	_, rw, err := New(Config{
		Request:        httptest.NewRequest(http.MethodGet, "/", nil),
		ResponseWriter: underlying,
		Recorder:       events.WithNoOpPreparer(rec),
		AppMetadata:    apievents.AppMetadata{AppName: "app"},
		UserMetadata:   apievents.UserMetadata{User: "user"},
		Logger:         logger,
	})
	require.NoError(t, err)

	// the response metadata event fails, so Write returns the error before
	// writing anything to the client.
	n, err := rw.Write([]byte("body"))
	require.Error(t, err)
	require.Zero(t, n)
	require.Empty(t, underlying.Body.String(), "un-recorded bytes must not reach the client")
	require.Contains(t, buf.String(), "Failed to record HTTP session event")
}

// TestBodyReadFailsClosed keeps unrecorded request bytes away from the
// downstream reader.
func TestBodyReadFailsClosed(t *testing.T) {
	t.Parallel()
	rb := newBody(io.NopCloser(strings.NewReader("payload")), func([]byte, int64, bool) error {
		return assert.AnError
	})

	buf := make([]byte, 8)
	n, err := rb.Read(buf)
	require.ErrorIs(t, err, assert.AnError)
	require.Zero(t, n, "no bytes must be handed downstream when recording fails")
}

// headerSpy is an http.ResponseWriter that records whether WriteHeader reached
// the underlying writer.
type headerSpy struct {
	http.ResponseWriter
	wroteHeader bool
	status      int
}

func (s *headerSpy) WriteHeader(status int) {
	s.wroteHeader = true
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

// TestResponseWriteHeaderFailureFailsClosed covers a failed response-head
// recording. WriteHeader cannot return an error, so it must not forward the
// un-recorded head and must surface the failure through the next Write, or
// through Finish for a header-only response (HEAD, 204/304, a bodyless
// redirect) that never calls Write.
func TestResponseWriteHeaderFailureFailsClosed(t *testing.T) {
	t.Parallel()

	newFailing := func(spy *headerSpy) *ResponseWriter {
		return newResponseWriter(spy, time.Now(),
			func(int, http.Header, int64) error { return assert.AnError },
			func([]byte, int64, bool) error { return nil })
	}

	t.Run("header-only response surfaces the error through Finish", func(t *testing.T) {
		t.Parallel()
		spy := &headerSpy{ResponseWriter: httptest.NewRecorder()}
		rw := newFailing(spy)

		rw.WriteHeader(http.StatusNoContent)
		require.False(t, spy.wroteHeader, "un-recorded response head must not reach the client")

		// with no Write to carry it, Finish reports the failure so the caller
		// can fail the exchange closed.
		require.ErrorIs(t, rw.Finish(), assert.AnError)
	})

	t.Run("following Write surfaces the error and writes nothing", func(t *testing.T) {
		t.Parallel()
		spy := &headerSpy{ResponseWriter: httptest.NewRecorder()}
		rw := newFailing(spy)

		rw.WriteHeader(http.StatusOK)
		require.False(t, spy.wroteHeader, "un-recorded response head must not reach the client")

		n, err := rw.Write([]byte("body"))
		require.ErrorIs(t, err, assert.AnError)
		require.Zero(t, n)
	})
}

// errAfterFirstRecorder records the first event and fails all subsequent ones.
type errAfterFirstRecorder struct {
	eventstest.MockRecorderEmitter
	seen int
}

func (e *errAfterFirstRecorder) RecordEvent(ctx context.Context, ev apievents.PreparedSessionEvent) error {
	e.seen++
	if e.seen > 1 {
		return assert.AnError
	}
	return e.MockRecorderEmitter.RecordEvent(ctx, ev)
}

// reassembleRequest rebuilds the request body and checks chunk ordering.
func reassembleRequest(t *testing.T, all []apievents.AuditEvent) string {
	t.Helper()
	var chunks []*apievents.AppSessionHTTPRequestBodyChunk
	for _, e := range all {
		if c, ok := e.(*apievents.AppSessionHTTPRequestBodyChunk); ok {
			chunks = append(chunks, c)
		}
	}
	require.NotEmpty(t, chunks, "expected at least one request body chunk")
	var buf []byte
	for i, c := range chunks {
		require.Equal(t, int64(i), c.ChunkIndex)
		require.Equal(t, i == len(chunks)-1, c.IsLast, "only the last chunk may set is_last")
		buf = append(buf, c.Data...)
	}
	return string(buf)
}

func reassembleResponse(t *testing.T, all []apievents.AuditEvent) string {
	t.Helper()
	var chunks []*apievents.AppSessionHTTPResponseBodyChunk
	for _, e := range all {
		if c, ok := e.(*apievents.AppSessionHTTPResponseBodyChunk); ok {
			chunks = append(chunks, c)
		}
	}
	require.NotEmpty(t, chunks, "expected at least one response body chunk")
	var buf []byte
	for i, c := range chunks {
		require.Equal(t, int64(i), c.ChunkIndex)
		require.Equal(t, i == len(chunks)-1, c.IsLast, "only the last chunk may set is_last")
		buf = append(buf, c.Data...)
	}
	return string(buf)
}

func firstEvent[T apievents.AuditEvent](t *testing.T, all []apievents.AuditEvent) T {
	t.Helper()
	for _, e := range all {
		if v, ok := e.(T); ok {
			return v
		}
	}
	var zero T
	require.Failf(t, "missing event", "no event of type %T was recorded", zero)
	return zero
}

func headerValue(headers []*apievents.HTTPHeader, name string) string {
	for _, h := range headers {
		if http.CanonicalHeaderKey(h.Name) == http.CanonicalHeaderKey(name) {
			return h.Value
		}
	}
	return ""
}

// TestRecordsFullExchange runs a real request through a wrapped handler and
// checks that each part of the exchange is recorded.
func TestRecordsFullExchange(t *testing.T) {
	t.Parallel()

	const requestBody = "ping-request-body"
	const responseBody = "pong-response-body"

	capture, recorder := newCaptureRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, rw, err := New(Config{
			Request:        r,
			ResponseWriter: w,
			Recorder:       recorder,
			AppMetadata:    apievents.AppMetadata{AppName: "test-app"},
			UserMetadata:   apievents.UserMetadata{User: "alice"},
			Logger:         slog.Default(),
		})
		if !assert.NoError(t, err) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer func() { assert.NoError(t, rw.Finish()) }()

		// draining the request body is what records request chunks.
		got, err := io.ReadAll(req.Body)
		assert.NoError(t, err)
		assert.Equal(t, requestBody, string(got))

		rw.Header().Set("Content-Type", "text/plain")
		rw.Header().Set("X-Custom", "custom-value")
		rw.WriteHeader(http.StatusCreated)
		_, err = io.WriteString(rw, responseBody)
		assert.NoError(t, err)
	})

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/path?q=1&q=2", strings.NewReader(requestBody))
	require.NoError(t, err)
	req.Header.Set("X-Request-Header", "req-value")

	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	clientBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, responseBody, string(clientBody))

	all := capture.Events()

	reqEvent := firstEvent[*apievents.AppSessionHTTPRequest](t, all)
	require.Equal(t, http.MethodPost, reqEvent.Method)
	require.Equal(t, "/path?q=1&q=2", reqEvent.Url)
	require.Equal(t, "q=1&q=2", reqEvent.RawQuery)
	require.Equal(t, "test-app", reqEvent.AppMetadata.AppName)
	require.Equal(t, "alice", reqEvent.UserMetadata.User)
	require.Equal(t, "req-value", headerValue(reqEvent.Headers, "X-Request-Header"))

	require.Equal(t, requestBody, reassembleRequest(t, all))

	respEvent := firstEvent[*apievents.AppSessionHTTPResponse](t, all)
	require.Equal(t, uint32(http.StatusCreated), respEvent.StatusCode)
	require.Equal(t, http.StatusText(http.StatusCreated), respEvent.StatusText)
	require.Equal(t, "test-app", respEvent.AppMetadata.AppName)
	require.Equal(t, "text/plain", headerValue(respEvent.Headers, "Content-Type"))
	require.Equal(t, "custom-value", headerValue(respEvent.Headers, "X-Custom"))

	require.Equal(t, responseBody, reassembleResponse(t, all))

	// every event uses the same request ID for correlation.
	requestID := reqEvent.RequestId
	require.NotEmpty(t, requestID)
	for _, e := range all {
		switch v := e.(type) {
		case *apievents.AppSessionHTTPRequest:
			require.Equal(t, requestID, v.RequestId)
		case *apievents.AppSessionHTTPRequestBodyChunk:
			require.Equal(t, requestID, v.RequestId)
		case *apievents.AppSessionHTTPResponse:
			require.Equal(t, requestID, v.RequestId)
		case *apievents.AppSessionHTTPResponseBodyChunk:
			require.Equal(t, requestID, v.RequestId)
		}
	}
}

// TestRecordsExchangeWithoutRequestBody covers a GET-style exchange with no
// request body.
func TestRecordsExchangeWithoutRequestBody(t *testing.T) {
	t.Parallel()

	capture, recorder := newCaptureRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, rw, err := New(Config{
			Request:        r,
			ResponseWriter: w,
			Recorder:       recorder,
			AppMetadata:    apievents.AppMetadata{AppName: "test-app"},
			UserMetadata:   apievents.UserMetadata{User: "alice"},
			Logger:         slog.Default(),
		})
		assert.NoError(t, err)
		defer func() { assert.NoError(t, rw.Finish()) }()
		// the handler writes nothing; Finish still records the response metadata
		// and final chunk.
	})

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	resp, err := srv.Client().Get(srv.URL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	all := capture.Events()

	reqEvent := firstEvent[*apievents.AppSessionHTTPRequest](t, all)
	require.Equal(t, http.MethodGet, reqEvent.Method)

	respEvent := firstEvent[*apievents.AppSessionHTTPResponse](t, all)
	require.Equal(t, uint32(http.StatusOK), respEvent.StatusCode)

	// a handler that never writes still gets a terminating response chunk.
	require.Empty(t, reassembleResponse(t, all))
}

// errRecorder is a SessionRecorder whose RecordEvent always fails.
type errRecorder struct{ eventstest.MockRecorderEmitter }

func (e *errRecorder) RecordEvent(context.Context, apievents.PreparedSessionEvent) error {
	return assert.AnError
}

// TestNewFailsClosedWhenRecorderFails keeps an exchange from starting when the
// initial audit event cannot be recorded.
func TestNewFailsClosedWhenRecorderFails(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	_, _, err := New(Config{
		Request:        httptest.NewRequest(http.MethodGet, "/", nil),
		ResponseWriter: httptest.NewRecorder(),
		Recorder:       events.WithNoOpPreparer(&errRecorder{}),
		AppMetadata:    apievents.AppMetadata{AppName: "app"},
		UserMetadata:   apievents.UserMetadata{User: "user"},
		Logger:         logger,
	})
	require.Error(t, err, "a recorder failure must fail New (fail-closed)")
	require.Contains(t, buf.String(), "Failed to record HTTP session event")
}

// TestConfigValidate covers required config fields.
func TestConfigValidate(t *testing.T) {
	t.Parallel()
	valid := func() Config {
		return Config{
			Request:        httptest.NewRequest(http.MethodGet, "/", nil),
			ResponseWriter: httptest.NewRecorder(),
			Recorder:       events.WithNoOpPreparer(&eventstest.MockRecorderEmitter{}),
			AppMetadata:    apievents.AppMetadata{AppName: "app"},
			UserMetadata:   apievents.UserMetadata{User: "user"},
			Logger:         slog.Default(),
		}
	}

	base := valid()
	require.NoError(t, base.Validate())

	tests := map[string]func(*Config){
		"missing request":         func(c *Config) { c.Request = nil },
		"missing response writer": func(c *Config) { c.ResponseWriter = nil },
		"missing recorder":        func(c *Config) { c.Recorder = nil },
		"missing app metadata":    func(c *Config) { c.AppMetadata = apievents.AppMetadata{} },
		"missing user metadata":   func(c *Config) { c.UserMetadata = apievents.UserMetadata{} },
		"missing logger":          func(c *Config) { c.Logger = nil },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfg := valid()
			mutate(&cfg)
			require.Error(t, cfg.Validate())
		})
	}
}

// TestFilterHeadersRedactsSensitive checks that known secret-bearing headers
// are redacted while ordinary headers stay intact.
func TestFilterHeadersRedactsSensitive(t *testing.T) {
	t.Parallel()
	h := http.Header{}
	h.Set("Authorization", "Bearer secret-token")
	h.Set("Proxy-Authorization", "Basic abc")
	h.Add("Cookie", "session=abc")
	h.Set("Set-Cookie", "session=def")
	h.Set("X-Api-Key", "key-123")
	h.Set("X-Auth-Token", "tok-123")
	h.Set("X-Amz-Security-Token", "amz-123")
	h.Set("X-Csrf-Token", "csrf-123")
	h.Set("Teleport-Jwt-Assertion", "jwt-123")
	h.Set("X-Teleport-Aws-Assumed-Role-Authorization", "AWS4-HMAC-SHA256 Credential=...")
	h.Set("X-Tenant-Secret", "extra-123")
	h.Set("Content-Type", "application/json")
	h.Add("X-Multi", "one")
	h.Add("X-Multi", "two")

	out := filterHeaders(h)

	// default-sensitive headers stay present, but their values are redacted.
	for _, name := range []string{
		"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie",
		"X-Api-Key", "X-Auth-Token", "X-Amz-Security-Token", "X-Csrf-Token",
		"Teleport-Jwt-Assertion", "X-Teleport-Aws-Assumed-Role-Authorization",
	} {
		require.Equal(t, redactedValue, headerValue(out, name), "%s must be redacted", name)
	}

	// non-sensitive headers keep their original values, including repeats.
	require.Equal(t, "application/json", headerValue(out, "Content-Type"))
	var multi []string
	for _, hdr := range out {
		if http.CanonicalHeaderKey(hdr.Name) == "X-Multi" {
			multi = append(multi, hdr.Value)
		}
	}
	require.ElementsMatch(t, []string{"one", "two"}, multi)

	for key := range sensitiveHeaders {
		require.Equal(t, http.CanonicalHeaderKey(key), key, "sensitiveHeaders keys must be canonicalized")
	}
}

// TestEmitHeaderSkipsInformational records the final status instead of an early
// 1xx response.
func TestEmitHeaderSkipsInformational(t *testing.T) {
	t.Parallel()
	var (
		calls  int
		status int
	)
	rec := httptest.NewRecorder()
	rw := newResponseWriter(rec, time.Now(),
		func(s int, _ http.Header, _ int64) error {
			calls++
			status = s
			return nil
		},
		func([]byte, int64, bool) error { return nil })

	// simulate informational responses before the final status.
	rw.WriteHeader(http.StatusContinue)
	rw.WriteHeader(http.StatusEarlyHints)
	rw.WriteHeader(http.StatusOK)
	require.NoError(t, rw.Finish())

	require.Equal(t, 1, calls, "only the final status must be recorded")
	require.Equal(t, http.StatusOK, status)
}

// ctxAwareRecorder behaves like SessionWriter.RecordEvent when given a
// canceled context.
type ctxAwareRecorder struct{ eventstest.MockRecorderEmitter }

func (c *ctxAwareRecorder) RecordEvent(ctx context.Context, e apievents.PreparedSessionEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.MockRecorderEmitter.RecordEvent(ctx, e)
}

// TestRecordsAfterRequestContextCancelled keeps recording after request
// cancellation, as happens on client disconnect.
func TestRecordsAfterRequestContextCancelled(t *testing.T) {
	t.Parallel()
	inner := &ctxAwareRecorder{}
	recorder := events.WithNoOpPreparer(inner)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("payload")).WithContext(ctx)
	rw := httptest.NewRecorder()

	wreq, wrw, err := New(Config{
		Request:        req,
		ResponseWriter: rw,
		Recorder:       recorder,
		AppMetadata:    apievents.AppMetadata{AppName: "app"},
		UserMetadata:   apievents.UserMetadata{User: "user"},
		Logger:         slog.Default(),
	})
	require.NoError(t, err)

	// cancel mid-exchange, as if the client disconnected, then keep proxying.
	cancel()

	_, err = io.ReadAll(wreq.Body)
	require.NoError(t, err)
	require.NoError(t, wreq.Body.Close())

	_, err = wrw.Write([]byte("response"))
	require.NoError(t, err)
	require.NoError(t, wrw.Finish())

	all := inner.Events()
	// cancellation should not drop the rest of the audit trail.
	require.Equal(t, "payload", reassembleRequest(t, all))
	require.Equal(t, "response", reassembleResponse(t, all))
	firstEvent[*apievents.AppSessionHTTPResponse](t, all)
}

// TestBodyConcurrentReadClose covers the Read/Close race net/http can trigger.
func TestBodyConcurrentReadClose(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	lastSeen := false
	rb := newBody(io.NopCloser(strings.NewReader(strings.Repeat("x", 4096))), func(_ []byte, _ int64, isLast bool) error {
		mu.Lock()
		defer mu.Unlock()
		if isLast {
			lastSeen = true
		}
		return nil
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 64)
		for {
			if _, err := rb.Read(buf); err != nil {
				return
			}
		}
	}()
	require.NoError(t, rb.Close())
	<-done

	mu.Lock()
	defer mu.Unlock()
	require.True(t, lastSeen, "a terminating chunk must be emitted")
}
