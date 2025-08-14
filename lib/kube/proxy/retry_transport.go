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
	"strings"

	"github.com/gravitational/trace"
	"k8s.io/client-go/util/retry"
)

// retryTransport wraps an http.RoundTripper and automatically retries
// requests that fail with GOAWAY.
// It only retries requests that can be safely retried.
type retryTransport struct {
	http.RoundTripper
}

// RoundTrip implements http.RoundTripper with automatic retry for GOAWAY errors.
func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// For requests that can't be retried, just pass through.
	if !t.canRetry(req) {
		return t.RoundTripper.RoundTrip(req)
	}

	// If we don't have a body, or if we have a GetBody already, we can retry directly.
	if req.Body == nil || req.GetBody != nil {
		return t.retry(req)
	}

	// For requests with bodies, we need GetBody to retry.
	if req.Body != nil {
		// Try to make the request retryable by buffering small bodies.
		ok, err := t.makeRetryable(req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if ok {
			return t.retry(req)
		}
		// Can't make retryable, just pass through
	}

	// Default: pass through.
	return t.RoundTripper.RoundTrip(req)
}

// retry the roundtrip in case of retryable error.
func (t *retryTransport) retry(req *http.Request) (*http.Response, error) {
	var out *http.Response

	attempt := 0
	if err := retry.OnError(retry.DefaultRetry, t.isRetryableError, func() error {
		// Clone the request for each attempt.
		reqClone := req.Clone(req.Context())

		// Reset the body if needed.
		if req.GetBody != nil && attempt > 0 {
			body, err := req.GetBody()
			if err != nil {
				return trace.Wrap(err)
			}
			reqClone.Body = body
		}

		//nolint:bodyclose // The body is expected to be consumed and closed by the caller.
		resp, err := t.RoundTripper.RoundTrip(reqClone)
		if err != nil {
			attempt++
			return trace.Wrap(err)
		}
		out = resp
		return nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// makeRetryable attempts to make a request retryable by buffering small bodies
func (t *retryTransport) makeRetryable(req *http.Request) (bool, error) {
	// Skip if no body or already has GetBody.
	if req.Body == nil || req.GetBody != nil {
		return false, nil
	}

	// Don't buffer large or unknown size bodies.
	const maxBufferSize = 5 * 1024 * 1024 // 5MB.
	if req.ContentLength < 0 || req.ContentLength > maxBufferSize {
		return false, nil
	}

	// Don't buffer streaming operations.
	if t.isStreamingRequest(req) {
		return false, nil
	}

	// Buffer the body.
	bodyBytes, err := io.ReadAll(io.LimitReader(req.Body, maxBufferSize+1))
	req.Body.Close()
	if err != nil {
		return false, trace.Wrap(err)
	}

	// Check size limit.
	if len(bodyBytes) > maxBufferSize {
		// Restore body without GetBody.
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return false, nil
	}

	// Define GetBody for retry.
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}

	// Set initial body.
	req.Body, err = req.GetBody()
	if err != nil {
		return false, trace.Wrap(err)
	}
	req.ContentLength = int64(len(bodyBytes))

	return true, nil
}

// canRetry determines if a request can be retried
func (t *retryTransport) canRetry(req *http.Request) bool {
	// Don't retry streaming operations.
	if t.isStreamingRequest(req) {
		return false
	}

	// Don't retry WebSocket upgrades.
	if req.Header.Get("Upgrade") != "" {
		return false
	}

	return true
}

// isStreamingRequest checks if this is a streaming request
func (*retryTransport) isStreamingRequest(req *http.Request) bool {
	// Check for Kubernetes exec/attach/port-forward
	if strings.Contains(req.URL.Path, "/exec") ||
		strings.Contains(req.URL.Path, "/attach") ||
		strings.Contains(req.URL.Path, "/portforward") {
		return true
	}

	// Check for log streaming
	if strings.Contains(req.URL.Path, "/log") && req.URL.Query().Get("follow") == "true" {
		return true
	}

	// Check for watch operations.
	if req.URL.Query().Get("watch") == "true" {
		return true
	}

	// Check for SPDY upgrade.
	if req.Header.Get("X-Stream-Protocol-Version") != "" {
		return true
	}

	// Check for WebSocket upgrade
	if req.Header.Get("Upgrade") == "websocket" || req.Header.Get("Connection") == "Upgrade" {
		return true
	}

	return false
}

// isRetryableError determines if an error is retryable
func (*retryTransport) isRetryableError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "graceful shutdown GOAWAY")
}

// CloseIdleConnections closes idle connections in the underlying transport.
func (t *retryTransport) CloseIdleConnections() {
	if ci, ok := t.RoundTripper.(interface{ CloseIdleConnections() }); ok {
		ci.CloseIdleConnections()
	}
}
