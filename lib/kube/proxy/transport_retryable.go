/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package proxy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/sync/semaphore"
)

const (
	// envRetryBufferTotal is the environment variable for RetryBufferTotal.
	envRetryBufferTotal = "TELEPORT_UNSTABLE_KUBE_RETRY_BUFFER_TOTAL"
	// envRetryBufferPerRequest is the environment variable for RetryBufferPerRequest.
	envRetryBufferPerRequest = "TELEPORT_UNSTABLE_KUBE_RETRY_BUFFER_PER_REQ"
	// defaultRetryBufferTotal is the 500 MiB default value for RetryBufferTotal.
	defaultRetryBufferTotal = int64(500 * 1024 * 1024)
	// defaultRetryBufferPerRequest is the 50 MiB default value for RetryBufferPerRequest.
	defaultRetryBufferPerRequest = int64(50 * 1024 * 1024)
)

// retryableTransport wraps an http.RoundTripper to enable automatic retry
// on HTTP/2 GOAWAY frames by buffering request bodies in memory.
//
// Kubernetes API servers send GOAWAY on HTTP/2 connections to redistribute
// load across replicas. Go's http2.Transport automatically retries if
// req.GetBody is set.
//
// A global semaphore limits total buffered memory to 500 MiB across all
// concurrent requests, preventing OOM. Individual requests have a maximum
// body size of 50 MiB. Requests are blocked if required memory cannot be
// acquired from the semaphore. These limits are tunable with environment
// variables.
//
// Requests that are not retried are:
// 1. HTTP/1.1 protocol upgrades (exec/attach/portforward), which won't
// receive an HTTP/2.0 GOAWAY.
// 2. Requests with unknown body sizes, used with chunked encoding,
// cannot safely acquire the semaphore.
type retryableTransport struct {
	inner               http.RoundTripper
	log                 *slog.Logger
	semaphore           *semaphore.Weighted
	maxBufferPerRequest int64
}

// newRetryableTransport enables automatic retry during HTTP/2 GOAWAY
// load balancing.
//
// Intended to wrap transports created via newH2Transport() or equivalent
// HTTP/2-configured transports. Wrapping non-HTTP/2 transports would add
// unnecessary buffering.
func newRetryableTransport(inner http.RoundTripper, log *slog.Logger, semaphore *semaphore.Weighted, maxBufferPerRequest int64) *retryableTransport {
	return &retryableTransport{
		inner:               inner,
		log:                 log,
		semaphore:           semaphore,
		maxBufferPerRequest: maxBufferPerRequest,
	}
}

// RoundTrip implements http.RoundTripper and makes requests retryable.
func (rt *retryableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Detect HTTP/1.1 protocol upgrades that won't receive HTTP/2 GOAWAY.
	if rt.isUpgrade(req) {
		// Perform roundtrip, no retry necessary.
		return rt.inner.RoundTrip(req)
	}

	if req.Body == nil || req.Body == http.NoBody || req.GetBody != nil {
		// Perform roundtrip, retry available.
		return rt.inner.RoundTrip(req)
	}

	// Buffer body to make retryable.
	releaseSemaphore := rt.makeRetryable(req)

	// Release semaphore after roundtrip completes.
	defer releaseSemaphore()

	// Perform roundtrip, retry available.
	return rt.inner.RoundTrip(req)
}

// makeRetryable buffers the request body to enable retry on HTTP/2 GOAWAY.
func (rt *retryableTransport) makeRetryable(req *http.Request) func() {
	// Chunked request are not retried due to unknown size.
	if req.ContentLength < 0 {
		rt.log.DebugContext(req.Context(),
			"Skipping retry buffer due to unknown content length, request will not be retryable on GOAWAY",
			"method", req.Method,
			"path", req.URL.Path,
		)
		return func() {}
	}

	// Skip zero-length bodies.
	if req.ContentLength == 0 {
		return func() {}
	}

	// Request body is too large to retry.
	// Allows balanced use of memory budget across requests.
	if req.ContentLength > rt.maxBufferPerRequest {
		rt.log.InfoContext(req.Context(),
			"Request body too large for retry buffer, request will not be retryable on GOAWAY",
			"size", req.ContentLength,
			"limit", rt.maxBufferPerRequest,
			"method", req.Method,
			"path", req.URL.Path,
		)
		return func() {}
	}

	// Acquire memory, blocks if unavailable.
	if err := rt.semaphore.Acquire(req.Context(), req.ContentLength); err != nil {
		if !errors.Is(err, context.Canceled) {
			rt.log.DebugContext(req.Context(),
				"Unable to acquire retry buffer, request will not be retryable on GOAWAY",
				"size", req.ContentLength,
				"error", err,
			)
		}
		return func() {}
	}

	rb := newRetryBuffer(
		req.Body,
		rt.semaphore,
		req.ContentLength,
	)
	req.Body = rb
	req.GetBody = func() (io.ReadCloser, error) {
		return rb.getBody()
	}

	releaseSemaphore := func() {
		if err := rb.Close(); err != nil {
			rt.log.DebugContext(req.Context(),
				"Unable to close request body",
				"size", req.ContentLength,
				"method", req.Method,
				"path", req.URL.Path,
				"error", err,
			)
		}
		// Note: Semaphore is not released here. It will be released by the
		// finalizer when rb is garbage collected, ensuring accurate memory
		// accounting. The buffer remains allocated until garbage is collected.
	}
	return releaseSemaphore
}

// isUpgrade detects HTTP/1.1 protocol upgrades (WebSocket, SPDY) that won't
// receive HTTP/2 GOAWAY frames.
//
// We cannot check req.Proto to detect HTTP/2 because modifyRequest() sets it
// to "HTTP/1.1" for all outgoing requests. Instead, we rely on the fact that
// all transports wrapped by retryableTransport are created via newH2Transport(),
// which configures HTTP/2. Requests without Upgrade headers will use HTTP/2 and
// may receive GOAWAY.
func (rt *retryableTransport) isUpgrade(req *http.Request) bool {
	return req.Header.Get("Connection") == "Upgrade" ||
		req.Header.Get("Upgrade") != "" ||
		req.Header.Get("X-Stream-Protocol-Version") != ""
}

// CloseIdleConnections closes idle connections in the inner transport.
func (rt *retryableTransport) CloseIdleConnections() {
	if ci, ok := rt.inner.(interface{ CloseIdleConnections() }); ok {
		ci.CloseIdleConnections()
	}
}

// retryBuffer incrementally reads from a source, incrementally writes
// to a buffer, and gracefully completes reading during any early call to Close.
type retryBuffer struct {
	src           io.ReadCloser
	buf           *bytes.Buffer
	mu            sync.Mutex
	cond          sync.Cond
	readErr       error
	semaphore     *semaphore.Weighted
	semaphoreSize int64
}

func newRetryBuffer(source io.ReadCloser, semaphore *semaphore.Weighted, memSize int64) *retryBuffer {
	type limitedReadCloser struct {
		*io.LimitedReader
		io.Closer
	}
	rb := &retryBuffer{
		// Limit reading in case actual body size is
		// greater than presented ContentLength.
		src: &limitedReadCloser{
			LimitedReader: &io.LimitedReader{R: source, N: memSize},
			Closer:        source,
		},
		buf:           bytes.NewBuffer(make([]byte, 0, memSize)),
		semaphore:     semaphore,
		semaphoreSize: memSize,
	}
	rb.cond.L = &rb.mu

	// Release semaphore when buffer is garbage collected.
	if semaphore != nil {
		runtime.SetFinalizer(rb, func(rb *retryBuffer) {
			rb.releaseSemaphore()
		})
	}

	return rb
}

// releaseSemaphore is release only once.
func (rb *retryBuffer) releaseSemaphore() {
	if rb.semaphore != nil {
		rb.semaphore.Release(rb.semaphoreSize)
	}
}

func (rb *retryBuffer) Read(p []byte) (int, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.src == nil {
		return 0, io.EOF
	}

	n, err := rb.src.Read(p)
	if n > 0 {
		rb.buf.Write(p[:n])
	}
	if err != nil && !errors.Is(err, io.EOF) && rb.readErr == nil {
		rb.readErr = err
		rb.cond.Broadcast()
	}
	return n, err
}

// Close completes buffering by reading any remaining data,
// ensuring getBody() always has the complete buffer.
func (rb *retryBuffer) Close() error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Close is idempotent.
	if rb.src == nil {
		return nil
	}

	// If the connection closed in the middle of sending
	// due to a GOAWAY, read remaining data for a retry attempt.
	remaining, readErr := io.ReadAll(rb.src)
	if readErr != nil && !errors.Is(readErr, io.EOF) && rb.readErr == nil {
		rb.readErr = readErr
	}
	if len(remaining) > 0 {
		rb.buf.Write(remaining)
	}

	// Close source and mark as done.
	err := rb.src.Close()
	rb.src = nil
	rb.cond.Broadcast()

	return err
}

// getBody waits for any reading to complete
// before returning the buffered body.
func (rb *retryBuffer) getBody() (io.ReadCloser, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Wait for any remaining reading in Close to complete.
	for rb.src != nil {
		rb.cond.Wait()
	}

	// Return if there was any read error.
	if rb.readErr != nil {
		return nil, rb.readErr
	}

	return io.NopCloser(bytes.NewReader(rb.buf.Bytes())), nil
}

// GetRetryBufferTotal returns RetryBufferTotal and RetryBufferPerRequest
// from an environment variable or default.
func GetRetryBufferValues() (int64, int64, error) {
	// Parse RetryBufferTotal.
	retryBufferTotal := defaultRetryBufferTotal
	if env := os.Getenv(envRetryBufferTotal); env != "" {
		if val, err := strconv.ParseInt(env, 10, 64); err != nil {
			return 0, 0, trace.WrapWithMessage(err, "unable to parse env var %s, got %q", defaultRetryBufferTotal, env)
		} else {
			// Allow RetryBufferTotal to be zero or negative.
			// Zero or negative indicates RetryBuffer is disabled.
			retryBufferTotal = val
		}
	}

	// Parse RetryBufferPerRequest.
	retryBufferPerRequest := defaultRetryBufferPerRequest
	if env := os.Getenv(envRetryBufferPerRequest); env != "" {
		if val, err := strconv.ParseInt(env, 10, 64); err != nil {
			return 0, 0, trace.WrapWithMessage(err, "unable to parse env var %s, got %q", envRetryBufferPerRequest, env)
		} else if val > 0 {
			retryBufferPerRequest = val
		}
	}

	// Check bounds.
	if retryBufferTotal > 0 && retryBufferPerRequest > retryBufferTotal {
		return 0, 0, trace.BadParameter(
			"retry buffer per-request size (%d bytes) cannot exceed total buffer size (%d bytes)",
			retryBufferPerRequest,
			retryBufferTotal,
		)
	}

	return retryBufferTotal, retryBufferPerRequest, nil
}
