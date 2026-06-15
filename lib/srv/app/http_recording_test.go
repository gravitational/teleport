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
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecordingBody_NormalRead(t *testing.T) {
	t.Parallel()
	type chunk struct {
		index  int64
		isLast bool
		data   []byte
	}
	var chunks []chunk
	body := io.NopCloser(strings.NewReader("hello world"))
	rb := newRecordingBody(body, func(data []byte, index int64, isLast bool) {
		cp := make([]byte, len(data))
		copy(cp, data)
		chunks = append(chunks, chunk{index, isLast, cp})
	})

	// Read in 5-byte chunks, then drain and close.
	buf := make([]byte, 5)
	n, err := rb.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 5, n)

	n, err = rb.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 5, n)

	n, err = rb.Read(buf)
	require.Equal(t, 1, n)
	// err may be nil or io.EOF depending on the reader implementation.

	// Drain any trailing zero-byte EOF reads, then close.
	_, _ = io.ReadAll(rb)
	require.NoError(t, rb.Close())

	// All data chunks must reconstruct the full body.
	var got []byte
	for _, c := range chunks {
		got = append(got, c.data...)
	}
	require.Equal(t, "hello world", string(got))

	// Exactly one chunk must carry isLast=true, and it must be the last one.
	require.NotEmpty(t, chunks)
	for i, c := range chunks[:len(chunks)-1] {
		require.False(t, c.isLast, "chunk %d must not be last", i)
	}
	require.True(t, chunks[len(chunks)-1].isLast, "last chunk must have isLast=true")

	// Indices must be sequential starting at 0.
	for i, c := range chunks {
		require.Equal(t, int64(i), c.index)
	}
}

func TestRecordingBody_EmptyBody(t *testing.T) {
	t.Parallel()
	type chunk struct {
		index  int64
		isLast bool
		data   []byte
	}
	var chunks []chunk
	body := io.NopCloser(bytes.NewReader(nil))
	rb := newRecordingBody(body, func(data []byte, index int64, isLast bool) {
		cp := make([]byte, len(data))
		copy(cp, data)
		chunks = append(chunks, chunk{index, isLast, cp})
	})

	buf := make([]byte, 8)
	n, err := rb.Read(buf)
	require.Equal(t, 0, n)
	require.ErrorIs(t, err, io.EOF)

	require.NoError(t, rb.Close())

	// An empty body emits one termination chunk from Close (no data, isLast=true).
	require.Len(t, chunks, 1)
	require.True(t, chunks[0].isLast)
	require.Empty(t, chunks[0].data)
}

func TestRecordingBody_CloseBeforeEOF(t *testing.T) {
	t.Parallel()
	type chunk struct {
		index  int64
		isLast bool
		data   []byte
	}
	var chunks []chunk
	body := io.NopCloser(strings.NewReader("not fully read"))
	rb := newRecordingBody(body, func(data []byte, index int64, isLast bool) {
		cp := make([]byte, len(data))
		copy(cp, data)
		chunks = append(chunks, chunk{index, isLast, cp})
	})

	buf := make([]byte, 3)
	_, err := rb.Read(buf)
	require.NoError(t, err)

	require.NoError(t, rb.Close())

	// The last chunk recorded must have isLast=true.
	require.NotEmpty(t, chunks)
	last := chunks[len(chunks)-1]
	require.True(t, last.isLast)
}

func TestRecordingResponseWriter(t *testing.T) {
	t.Parallel()
	type chunk struct {
		index  int64
		isLast bool
		data   []byte
	}
	var chunks []chunk
	rec := httptest.NewRecorder()
	rw := newRecordingResponseWriter(rec, func(data []byte, index int64, isLast bool) {
		cp := make([]byte, len(data))
		copy(cp, data)
		chunks = append(chunks, chunk{index, isLast, cp})
	})

	rw.WriteHeader(http.StatusCreated)
	_, err := rw.Write([]byte("hello "))
	require.NoError(t, err)
	_, err = rw.Write([]byte("world"))
	require.NoError(t, err)
	rw.finish()

	// The underlying writer must receive the status and full body unchanged.
	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "hello world", rec.Body.String())

	// Data chunks must reconstruct the full body.
	var got []byte
	for _, c := range chunks {
		got = append(got, c.data...)
	}
	require.Equal(t, "hello world", string(got))

	// Indices must be sequential starting at 0.
	for i, c := range chunks {
		require.Equal(t, int64(i), c.index)
	}

	// Exactly one terminating chunk: the last one, empty and isLast=true.
	for i, c := range chunks[:len(chunks)-1] {
		require.False(t, c.isLast, "chunk %d must not be last", i)
	}
	last := chunks[len(chunks)-1]
	require.True(t, last.isLast)
	require.Empty(t, last.data)
}

// flushHijackRecorder is an http.ResponseWriter that also implements
// http.Flusher and http.Hijacker, used to verify the recording writer
// forwards those optional interfaces.
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

func TestRecordingResponseWriter_ForwardsFlushAndHijack(t *testing.T) {
	t.Parallel()
	inner := &flushHijackRecorder{ResponseRecorder: httptest.NewRecorder()}
	rw := newRecordingResponseWriter(inner, func([]byte, int64, bool) {})

	// The wrapper must expose Flusher and Hijacker so the reverse proxy's
	// streaming and protocol upgrades keep working.
	flusher, ok := any(rw).(http.Flusher)
	require.True(t, ok, "recordingResponseWriter must implement http.Flusher")
	flusher.Flush()
	require.True(t, inner.flushed)

	hijacker, ok := any(rw).(http.Hijacker)
	require.True(t, ok, "recordingResponseWriter must implement http.Hijacker")
	_, _, err := hijacker.Hijack()
	require.NoError(t, err)
	require.True(t, inner.hijacked)
}

// TestRecordingResponseWriter_HijackUnsupported verifies the wrapper reports
// ErrNotSupported when the underlying writer is not a Hijacker.
func TestRecordingResponseWriter_HijackUnsupported(t *testing.T) {
	t.Parallel()
	rw := newRecordingResponseWriter(httptest.NewRecorder(), func([]byte, int64, bool) {})
	_, _, err := rw.Hijack()
	require.ErrorIs(t, err, http.ErrNotSupported)
}
