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
	"io"
	"log/slog"
	"net/http"

	"github.com/gravitational/trace"
	"golang.org/x/sync/semaphore"
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
// acquired from the semaphore.
//
// Requests that are not made retryable are:
// 1. HTTP/1.1 protocol upgrades (exec/attach/portforward), which won't
// receive an HTTP/2.0 GOAWAY.
// 2. Requests with body sizes greater than 50 MiB.
// 3. Requests with unknown body sizes used with chunked encoding
// cannot safely acquire the semaphore.
type retryableTransport struct {
	inner     http.RoundTripper
	log       *slog.Logger
	semaphore *semaphore.Weighted
}

const (
	// maxTotalBufferSize limits total memory used for buffering request bodies
	// across all concurrent requests to prevent OOM at scale.
	maxTotalBufferSize = 500 * 1024 * 1024 // 500 MiB

	// maxBufferPerRequest limits individual request body size to allow
	// balanced memory usage across requests. Kubernetes ConfigMaps and Secrets
	// have 1 MiB etcd limits, so 50 MiB is generous.
	maxBufferPerRequest = 50 * 1024 * 1024 // 50 MiB
)

var retryBufferSemaphore = semaphore.NewWeighted(maxTotalBufferSize)

// newRetryableTransport creates a retryableTransport that wraps the provided
// HTTP/2 transport to enable automatic retry on GOAWAY frames.
//
// Intended to wrap transports created via newH2Transport() or equivalent
// HTTP/2-configured transports. Wrapping non-HTTP/2 transports would add
// unnecessary buffering.
func newRetryableTransport(inner http.RoundTripper, log *slog.Logger) *retryableTransport {
	return &retryableTransport{
		inner:     inner,
		log:       log,
		semaphore: retryBufferSemaphore,
	}
}

// RoundTrip implements http.RoundTripper and makes requests retryable.
func (rt *retryableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Detect HTTP/1.1 protocol upgrades that won't receive HTTP/2 GOAWAY.
	if rt.isUpgrade(req) {
		// Perform roundtrip, no retry available.
		return rt.inner.RoundTrip(req)
	}

	if req.Body == nil || req.GetBody != nil {
		// Perform roundtrip, retry available.
		return rt.inner.RoundTrip(req)
	}

	// Buffer body to make retryable.
	releaseSemaphore, err := rt.makeRetryable(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if releaseSemaphore != nil {
		// Release semaphore after roundtrip completes.
		defer releaseSemaphore()
	}

	// Perform roundtrip, retry available.
	return rt.inner.RoundTrip(req)
}

// makeRetryable buffers the request body to enable retry on HTTP/2 GOAWAY.
func (rt *retryableTransport) makeRetryable(req *http.Request) (func(), error) {
	// Chunked request are not retried due to unknown size.
	if req.ContentLength < 0 {
		rt.log.DebugContext(req.Context(),
			"Skipping retry buffer for chunked encoding, request will not be retryable on GOAWAY",
			"method", req.Method,
			"path", req.URL.Path)
		return nil, nil
	}

	// Request body is too large to retry.
	// Allows balanced use of memory budget across requests.
	if req.ContentLength > maxBufferPerRequest {
		rt.log.InfoContext(req.Context(),
			"Request body too large for retry buffer, request will not be retryable on GOAWAY",
			"size", req.ContentLength,
			"limit", maxBufferPerRequest,
			"method", req.Method,
			"path", req.URL.Path)
		return nil, nil
	}

	// Acquire memory, blocks if unavailable.
	if err := rt.semaphore.Acquire(req.Context(), req.ContentLength); err != nil {
		rt.log.InfoContext(req.Context(),
			"Unable to acquire retry buffer semaphore, request will not be retryable on GOAWAY",
			"size", req.ContentLength,
			"method", req.Method,
			"path", req.URL.Path,
			"error", err)
		return nil, nil
	}
	releaseSemaphore := func() { rt.semaphore.Release(req.ContentLength) }

	buf := bytes.NewBuffer(make([]byte, 0, req.ContentLength))
	req.Body = newTeeReadCloser(req.Body, buf)
	req.GetBody = func() (io.ReadCloser, error) {
		// Return a fresh reader for retry (bytes.NewReader resets read position)
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	}

	return releaseSemaphore, nil
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

// teeReadCloser wraps an io.TeeReader to allow the
// Closer to be explicitly called.
type teeReadCloser struct {
	io.Reader
	closer io.Closer
}

func newTeeReadCloser(r io.ReadCloser, w io.Writer) io.ReadCloser {
	return &teeReadCloser{
		Reader: io.TeeReader(r, w),
		closer: r,
	}
}

func (t *teeReadCloser) Close() error {
	return t.closer.Close()
}
