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
)

// transportWrapper wraps an http.RoundTripper and ensures requests with bodies
// are retryable by setting GetBody when possible. This allows the underlying
// transport (typically http2.Transport) to automatically retry on GOAWAY.
type transportWrapper struct {
	http.RoundTripper
}

// RoundTrip implements http.RoundTripper, ensuring requests are retryable when possible.
func (t *transportWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Skip streaming operations that shouldn't be made retryable
	if t.isStreamingRequest(req) {
		return t.RoundTripper.RoundTrip(req)
	}

	// If we already have GetBody or no body, just pass through
	if req.Body == nil || req.GetBody != nil {
		return t.RoundTripper.RoundTrip(req)
	}

	// Try to make the request retryable by setting GetBody
	if err := t.makeRetryable(req); err != nil {
		return nil, trace.Wrap(err)
	}

	return t.RoundTripper.RoundTrip(req)
}

// makeRetryable attempts to make a request retryable by buffering small bodies
// and setting GetBody. This allows the underlying HTTP/2 transport to automatically
// retry on GOAWAY.
func (t *transportWrapper) makeRetryable(req *http.Request) error {
	// Skip if no body or already has GetBody
	if req.Body == nil || req.GetBody != nil {
		return nil
	}

	// Don't buffer large or unknown size bodies
	const maxBufferSize = 5 * 1024 * 1024 // 5MB
	if req.ContentLength < 0 || req.ContentLength > maxBufferSize {
		return nil
	}

	// Buffer the body
	bodyBytes, err := io.ReadAll(io.LimitReader(req.Body, maxBufferSize+1))
	req.Body.Close()
	if err != nil {
		return trace.Wrap(err)
	}

	// Check size limit
	if len(bodyBytes) > maxBufferSize {
		// Restore body without GetBody
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return nil
	}

	// Set GetBody for retry capability
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}

	// Set initial body
	req.Body, err = req.GetBody()
	if err != nil {
		return trace.Wrap(err)
	}
	req.ContentLength = int64(len(bodyBytes))

	return nil
}

// isStreamingRequest checks if this is a streaming request that shouldn't be made retryable
func (*transportWrapper) isStreamingRequest(req *http.Request) bool {
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

	// Check for watch operations
	if req.URL.Query().Get("watch") == "true" {
		return true
	}

	// Check for SPDY upgrade
	if req.Header.Get("X-Stream-Protocol-Version") != "" {
		return true
	}

	// Check for WebSocket upgrade
	if req.Header.Get("Upgrade") == "websocket" || req.Header.Get("Connection") == "Upgrade" {
		return true
	}

	return false
}

// CloseIdleConnections closes idle connections in the underlying transport.
func (t *transportWrapper) CloseIdleConnections() {
	if ci, ok := t.RoundTripper.(interface{ CloseIdleConnections() }); ok {
		ci.CloseIdleConnections()
	}
}