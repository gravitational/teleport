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
	"net/http"

	"github.com/gravitational/trace"
)

// retryableTransport wraps an http.RoundTripper and enables
// automatic retry of HTTP/2 GOAWAY frames.
//
// Kubernetes API servers (EKS 1.27+) send GOAWAY frames to
// redistribute load across replicas. Go's http2.Transport automatically
// retries requests when req.GetBody is set. retryableTransport
// buffers request bodies up to 5MB for GetBody.
//
// Some requests aren't retried:
// 1. Streaming protocols (/exec, /attach, /portforward) have unbounded body sizes
// 2. HTTP/1.1 protocol upgrades (WebSocket, SPDY) switch protocols
// 3. Requests with body sizes larger than 5MB
type retryableTransport struct {
	inner http.RoundTripper
}

// RoundTrip implements http.RoundTripper and makes requests retryable.
func (t *retryableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Skip streaming protocols which aren't retryable.
	if t.isStreamingProtocol(req) {
		return t.inner.RoundTrip(req)
	}

	// Retry is ok with no body, or buffered in call to GetBody.
	if req.Body == nil || req.GetBody != nil {
		return t.inner.RoundTrip(req)
	}

	// Make retryable by buffering and setting GetBody.
	if err := t.makeRetryable(req); err != nil {
		return nil, trace.Wrap(err)
	}

	return t.inner.RoundTrip(req)
}

// makeRetryable attempts to make a request retryable by buffering bodies < 5MB
// and setting GetBody. This allows the underlying transport to automatically
// retry on GOAWAY.
func (t *retryableTransport) makeRetryable(req *http.Request) error {
	// Skip if the request is already retryable
	if req.Body == nil || req.GetBody != nil {
		return nil
	}

	// Limit the body buffer size to a large 5X for most cases.
	// ConfigMaps and Secrets have 1MB limits with kube etcd.
	// Large bodies might come from a custom CRD or batch operation,
	// and is likely rare.
	const maxBufferSize = 5 * 1024 * 1024 // 5MB

	// Avoid buffering a chunked body with an unknown size,
	// or excessively large bodies.
	if req.ContentLength < 0 || req.ContentLength > maxBufferSize {
		return nil
	}

	// Buffer the body
	bodyBytes, err := io.ReadAll(io.LimitReader(req.Body, maxBufferSize+1))
	req.Body.Close()
	if err != nil {
		return trace.Wrap(err)
	}

	// Check actual body size
	if len(bodyBytes) > maxBufferSize {
		// Too large, restore for initial attempt and don't set GetBody
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return nil
	}

	// Set GetBody to allow retries
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}
	req.Body, err = req.GetBody()
	if err != nil {
		return trace.Wrap(err)
	}
	req.ContentLength = int64(len(bodyBytes))

	return nil
}

// isStreamingProtocol detects HTTP/1.1 protocol upgrades (WebSocket, SPDY)
// that cannot be retried because of unbounded or bidirectional request bodies.
// This includes Kubernetes exec, attach, and portforward operations.
func (*retryableTransport) isStreamingProtocol(req *http.Request) bool {
	// Detect HTTP/1.1 protocol upgrades
	if req.Header.Get("Connection") == "Upgrade" ||
		req.Header.Get("Upgrade") != "" ||
		req.Header.Get("X-Stream-Protocol-Version") != "" {
		return true
	}

	// Note: We allow requests for watch, log streaming, and proxy.
	// Watch and log stream have GET with no body, and can be retried.
	// Proxy requests can have the body buffered when not too large.

	return false
}

// CloseIdleConnections closes idle connections in the inner transport.
func (t *retryableTransport) CloseIdleConnections() {
	if ci, ok := t.inner.(interface{ CloseIdleConnections() }); ok {
		ci.CloseIdleConnections()
	}
}
