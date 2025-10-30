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
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/gravitational/trace"
)

// retryableTransport wraps an http.RoundTripper to enable automatic retry
// on HTTP/2 GOAWAY frames.
//
// Kubernetes API servers send GOAWAY on HTTP/2 connections to redistribute
// load across replicas. Go's http2.Transport automatically retries if
// req.GetBody is set. Request bodies are buffered in memory for up to 5MB,
// and buffered on disk beyond 5MB.
type retryableTransport struct {
	inner http.RoundTripper
	log   *slog.Logger
	ctx   context.Context
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
	cleanup, err := t.makeRetryable(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cleanup != nil {
		defer func() {
			// Cleanup may delete a temp file after roundtrip.
			if err := cleanup(); err != nil && errors.Is(err, fs.ErrNotExist) {
				if t.log != nil {
					t.log.WarnContext(t.ctx, "Unable to cleanup temp file", "error", err)
				}
			}
		}()
	}

	return t.inner.RoundTrip(req)
}

// makeRetryable buffers the request body to enable retry on HTTP/2 GOAWAY.
func (t *retryableTransport) makeRetryable(req *http.Request) (func() error, error) {
	// Skip if the request is already retryable
	if req.Body == nil || req.GetBody != nil {
		return nil, nil
	}

	// Limit the body buffer size to a large 5X for most cases.
	// ConfigMaps and Secrets have 1MB limits with kube etcd.
	// Large bodies might come from a custom CRD or batch operation,
	// and is likely rare.
	const maxBufferSize = 5 * 1024 * 1024 // 5MB

	// Buffer on disk when unknown size or body > 5MB.
	if req.ContentLength < 0 || req.ContentLength > maxBufferSize {
		return t.makeRetryableWithDisk(req)
	}

	// Buffer body in memory up to 5MB.
	return nil, t.makeRetryableWithMemory(req)
}

func (t *retryableTransport) makeRetryableWithMemory(req *http.Request) error {
	buf := bytes.NewBuffer(make([]byte, 0, req.ContentLength))
	req.Body = newTeeReadCloser(req.Body, buf)
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(buf), nil
	}
	return nil
}

const tmpFilePattern = "teleport-kube-proxy-*.tmp"

func (t *retryableTransport) makeRetryableWithDisk(req *http.Request) (func() error, error) {
	tmpFile, err := os.CreateTemp("", tmpFilePattern)
	if err != nil {
		return nil, nil
	}
	filePath := tmpFile.Name()
	tee := newTeeReadCloser(req.Body, tmpFile)
	req.Body = &fileCloser{
		Reader: tee,
		onClose: func() {
			if err := tee.Close(); err != nil {
				t.log.WarnContext(t.ctx, "Unable to close original request body", "error", err)
			}
			if err := tmpFile.Close(); err != nil {
				t.log.WarnContext(t.ctx, "Unable to close temp file", "error", err)
			}
		},
	}
	req.GetBody = func() (io.ReadCloser, error) {
		f, err := os.Open(filePath)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return f, nil
	}

	return func() error {
		return os.Remove(filePath)
	}, nil
}

// isStreamingProtocol detects HTTP/1.1 protocol upgrades (WebSocket, SPDY)
// that cannot be retried because of unbounded or bidirectional request bodies.
// This includes Kubernetes exec, attach, and portforward operations.
func (*retryableTransport) isStreamingProtocol(req *http.Request) bool {
	// Detect HTTP/1.1 protocol upgrades
	return req.Header.Get("Connection") == "Upgrade" ||
		req.Header.Get("Upgrade") != "" ||
		req.Header.Get("X-Stream-Protocol-Version") != ""
}

// CloseIdleConnections closes idle connections in the inner transport.
func (t *retryableTransport) CloseIdleConnections() {
	if ci, ok := t.inner.(interface{ CloseIdleConnections() }); ok {
		ci.CloseIdleConnections()
	}
}

// teeReadCloser wraps an io.TeeReader to allow the
// Closer to be be explicitly called.
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

type fileCloser struct {
	io.Reader
	onClose func()
}

func (f *fileCloser) Close() error {
	if f.onClose != nil {
		f.onClose()
	}
	return nil
}
