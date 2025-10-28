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
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/semaphore"
)

const (
	testTotalLimit      = 50 * 1024
	testPerRequestLimit = 10 * 1024
	testSmallBody       = 16
	testMediumBody      = 64
	testLargeBody       = 8 * 1024
)

// TestRetryableTransport_GetBody validates retry mechanism buffer
// completeness after partial reads, which simulates a GOAWAY mid-send.
func TestRetryableTransport_GetBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		bodySize        int
		partialRead     bool
		expectRetryable bool
	}{
		{
			name:            "full_read_medium",
			bodySize:        testMediumBody,
			partialRead:     false,
			expectRetryable: true,
		},
		{
			name:            "partial_read_medium",
			bodySize:        testMediumBody,
			partialRead:     true,
			expectRetryable: true,
		},
		{
			name:            "full_read_large",
			bodySize:        testLargeBody,
			partialRead:     false,
			expectRetryable: true,
		},
		{
			name:            "partial_read_large",
			bodySize:        testLargeBody,
			partialRead:     true,
			expectRetryable: true,
		},
		{
			name:            "exactly_at_limit",
			bodySize:        testPerRequestLimit,
			partialRead:     false,
			expectRetryable: true,
		},
		{
			name:            "over_limit_not_retryable",
			bodySize:        testPerRequestLimit + 1,
			partialRead:     false,
			expectRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			originalBody := bytes.Repeat([]byte("x"), tt.bodySize)
			rt := newRetryableTransport(
				&mockTransport{},
				slog.New(slog.DiscardHandler),
				semaphore.NewWeighted(testTotalLimit),
				testPerRequestLimit,
			)

			req, err := http.NewRequest("POST", "http://test",
				io.NopCloser(bytes.NewReader(originalBody)))
			require.NoError(t, err)
			req.ContentLength = int64(tt.bodySize)

			cleanup := rt.makeRetryable(req)
			defer cleanup()

			if tt.expectRetryable {
				require.NotNil(t, req.GetBody, "GetBody must be set for retryable request")

				if tt.partialRead {
					// Simulate a GOAWAY mid-send: read half, then close it.
					partial := make([]byte, tt.bodySize/2)
					n, err := req.Body.Read(partial)
					require.NoError(t, err)
					require.Equal(t, tt.bodySize/2, n)
				} else {
					// Normal case reads everything.
					_, err := io.ReadAll(req.Body)
					require.NoError(t, err)
				}

				// Close completes buffering and reads any remaining data.
				require.NoError(t, req.Body.Close())

				// GetBody returns a complete buffer for retry.
				retryBody, err := req.GetBody()
				require.NoError(t, err)
				retryData, err := io.ReadAll(retryBody)
				require.NoError(t, err)
				require.Equal(t, originalBody, retryData, "GetBody must return complete buffer")
				require.NoError(t, retryBody.Close())

				// Verify idempotent.
				retryBody2, err := req.GetBody()
				require.NoError(t, err)
				retryData2, err := io.ReadAll(retryBody2)
				require.NoError(t, err)
				require.Equal(t, originalBody, retryData2, "GetBody must be idempotent")
				require.NoError(t, retryBody2.Close())
			} else {
				// When the body is too large buffering is skipped.
				require.Nil(t, req.GetBody, "GetBody should be nil for oversized body")
			}
		})
	}
}

// TestRetryableTransport_SemaphoreAcquiredNotReleasedEarly verifies that
// the semaphore is acquired during buffering and NOT released by cleanup().
// The semaphore will be released by a finalizer when the buffer is GC'd,
// but we cannot reliably test finalizer timing in unit tests.
func TestRetryableTransport_SemaphoreAcquiredNotReleasedEarly(t *testing.T) {
	t.Parallel()

	sem := semaphore.NewWeighted(testMediumBody)
	rt := newRetryableTransport(
		&mockTransport{},
		slog.New(slog.DiscardHandler),
		sem,
		testPerRequestLimit,
	)

	body := bytes.Repeat([]byte("x"), testMediumBody)
	req, err := http.NewRequest("POST", "http://test",
		io.NopCloser(bytes.NewReader(body)))
	require.NoError(t, err)
	req.ContentLength = testMediumBody

	// Acquire semaphore.
	cleanup := rt.makeRetryable(req)
	require.False(t, sem.TryAcquire(1), "semaphore should be acquired")

	// Read.
	io.ReadAll(req.Body)
	require.NoError(t, req.Body.Close())
	require.False(t, sem.TryAcquire(1), "semaphore still held during use")

	// Cleanup is not releasing memory..
	cleanup()
	require.False(t, sem.TryAcquire(1),
		"semaphore is not released during cleanup")
}

// TestRetryableTransport_SemaphoreIsolation validates test isolation
func TestRetryableTransport_SemaphoreIsolation(t *testing.T) {
	t.Parallel()

	sem1 := semaphore.NewWeighted(testMediumBody)
	sem2 := semaphore.NewWeighted(testMediumBody)

	rt1 := newRetryableTransport(&mockTransport{}, slog.New(slog.DiscardHandler), sem1, testPerRequestLimit)
	rt2 := newRetryableTransport(&mockTransport{}, slog.New(slog.DiscardHandler), sem2, testPerRequestLimit)

	// Exhaust rt1
	body1 := bytes.Repeat([]byte("x"), testMediumBody)
	req1, err := http.NewRequest("POST", "http://test", io.NopCloser(bytes.NewReader(body1)))
	require.NoError(t, err)
	req1.ContentLength = testMediumBody
	cleanup1 := rt1.makeRetryable(req1)
	defer cleanup1()

	require.False(t, sem1.TryAcquire(1), "rt1 semaphore exhausted")
	require.True(t, sem2.TryAcquire(testMediumBody), "rt2 semaphore available")
	sem2.Release(testMediumBody)

	// rt2 can still buffer
	body2 := bytes.Repeat([]byte("y"), testMediumBody)
	req2, err := http.NewRequest("POST", "http://test", io.NopCloser(bytes.NewReader(body2)))
	require.NoError(t, err)
	req2.ContentLength = testMediumBody
	cleanup2 := rt2.makeRetryable(req2)
	defer cleanup2()

	require.NotNil(t, req2.GetBody, "rt2 should buffer despite rt1 exhaustion")
}

// TestRetryableTransport_SemaphoreContextCanceled validates context cancellation
// during semaphore acquire.
func TestRetryableTransport_SemaphoreContextCanceled(t *testing.T) {
	t.Parallel()

	sem := semaphore.NewWeighted(testMediumBody)
	rt := newRetryableTransport(
		&mockTransport{},
		slog.New(slog.DiscardHandler),
		sem,
		testPerRequestLimit,
	)

	// Exhaust semaphore
	require.True(t, sem.TryAcquire(testMediumBody))
	defer sem.Release(testMediumBody)

	// Create request with pre-canceled context
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	body := bytes.Repeat([]byte("x"), testMediumBody)
	req, err := http.NewRequestWithContext(ctx, "POST", "http://test",
		io.NopCloser(bytes.NewReader(body)))
	require.NoError(t, err)
	req.ContentLength = testMediumBody

	cleanup := rt.makeRetryable(req)
	defer cleanup()

	require.Nil(t, req.GetBody, "should skip buffering on context cancel")
}

// TestRetryableTransport_ZeroCopyPaths validates that requests which don't
// require retry skip buffering and avoid unnecessary memory use.
func TestRetryableTransport_ZeroCopyPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupReq       func() *http.Request
		expectBuffered bool
	}{
		{
			name: "nil_body",
			setupReq: func() *http.Request {
				req, err := http.NewRequest("GET", "http://test", nil)
				require.NoError(t, err)
				return req
			},
			expectBuffered: false,
		},
		{
			name: "getbody_already_set",
			setupReq: func() *http.Request {
				body := bytes.NewReader([]byte("test"))
				req, err := http.NewRequest("POST", "http://test", body)
				require.NoError(t, err)
				req.ContentLength = 4
				req.GetBody = func() (io.ReadCloser, error) {
					return io.NopCloser(bytes.NewReader([]byte("test"))), nil
				}
				return req
			},
			expectBuffered: false,
		},
		{
			name: "upgrade_request",
			setupReq: func() *http.Request {
				body := bytes.NewReader([]byte("test"))
				req, err := http.NewRequest("POST", "http://test", body)
				require.NoError(t, err)
				req.Header.Set("Connection", "Upgrade")
				req.ContentLength = 4
				return req
			},
			expectBuffered: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rt := newRetryableTransport(
				&mockTransport{},
				slog.New(slog.DiscardHandler),
				semaphore.NewWeighted(1024),
				1024,
			)

			req := tt.setupReq()
			originalBody := req.Body

			resp, err := rt.RoundTrip(req)
			require.NoError(t, err)
			if resp != nil {
				require.NoError(t, resp.Body.Close())
			}

			if tt.expectBuffered {
				require.NotEqual(t, originalBody, req.Body, "body should be wrapped")
			} else {
				require.Equal(t, originalBody, req.Body, "body should not be wrapped")
			}
		})
	}
}

// TestRetryableTransport_SkipConditions validates requests that skip buffering.
func TestRetryableTransport_SkipConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		contentLength  int64
		expectBuffered bool
	}{
		{
			"chunked_encoding",
			-1,
			false,
		},
		{
			"zero_length",
			0,
			false,
		},
		{
			"too_large",
			testPerRequestLimit + 1,
			false,
		},
		{
			"exactly_at_limit",
			testPerRequestLimit,
			true,
		},
		{
			"under_limit",
			testPerRequestLimit - 1,
			true,
		},
		{
			"normal_size",
			testSmallBody,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rt := newRetryableTransport(
				&mockTransport{},
				slog.New(slog.DiscardHandler),
				semaphore.NewWeighted(testTotalLimit),
				testPerRequestLimit,
			)

			body := bytes.Repeat([]byte("x"), testSmallBody)
			req, err := http.NewRequest("POST", "http://test",
				io.NopCloser(bytes.NewReader(body)))
			require.NoError(t, err)
			req.ContentLength = tt.contentLength

			cleanup := rt.makeRetryable(req)
			defer cleanup()

			if tt.expectBuffered {
				require.NotNil(t, req.GetBody, tt.name)
			} else {
				require.Nil(t, req.GetBody, tt.name)
			}
		})
	}
}

// TestRetryableTransport_IsUpgrade validates protocol upgrade detection.
func TestRetryableTransport_IsUpgrade(t *testing.T) {
	t.Parallel()

	rt := &retryableTransport{}

	tests := []struct {
		name      string
		path      string
		headers   map[string]string
		isUpgrade bool
	}{
		{
			name: "websocket",
			headers: map[string]string{
				"Upgrade":    "websocket",
				"Connection": "Upgrade",
			},
			isUpgrade: true,
		},
		{
			name: "spdy",
			headers: map[string]string{
				"Upgrade":    "SPDY/3.1",
				"Connection": "Upgrade",
			},
			isUpgrade: true,
		},
		{
			name: "x_stream_protocol",
			headers: map[string]string{
				"X-Stream-Protocol-Version": "v4.channel.k8s.io",
			},
			isUpgrade: true,
		},
		{
			name:      "normal_request",
			path:      "/api/v1/pods",
			headers:   map[string]string{},
			isUpgrade: false,
		},
		{
			name:      "watch_not_upgrade",
			path:      "/api/v1/pods?watch=true",
			headers:   map[string]string{},
			isUpgrade: false,
		},
		{
			name:      "log_streaming_not_upgrade",
			path:      "/api/v1/namespaces/default/pods/mypod/log?follow=true",
			headers:   map[string]string{},
			isUpgrade: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequest("GET", "http://test"+tt.path, nil)
			require.NoError(t, err)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			require.Equal(t, tt.isUpgrade, rt.isUpgrade(req))
		})
	}
}

// TestRetryableTransport_ErrorPropagation validates error handling.
func TestRetryableTransport_ErrorPropagation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupBody func() io.ReadCloser
		expectErr string
	}{
		{
			name: "read_error",
			setupBody: func() io.ReadCloser {
				return io.NopCloser(&failingReader{
					data:    bytes.Repeat([]byte("x"), testSmallBody),
					failAt:  testSmallBody / 2,
					failErr: errors.New("read error"),
				})
			},
			expectErr: "read error",
		},
		{
			name: "close_error_does_not_fail_getbody",
			setupBody: func() io.ReadCloser {
				return &errorCloser{
					Reader:   bytes.NewReader(bytes.Repeat([]byte("x"), testSmallBody)),
					closeErr: errors.New("close error"),
				}
			},
			expectErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rt := newRetryableTransport(
				&mockTransport{},
				slog.New(slog.DiscardHandler),
				semaphore.NewWeighted(testTotalLimit),
				testPerRequestLimit,
			)

			req, err := http.NewRequest("POST", "http://test", tt.setupBody())
			require.NoError(t, err)
			req.ContentLength = testSmallBody

			cleanup := rt.makeRetryable(req)
			defer cleanup()

			io.Copy(io.Discard, req.Body)
			req.Body.Close()

			body, err := req.GetBody()
			if tt.expectErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectErr)
			} else {
				require.NoError(t, err)
				_, err := io.ReadAll(body)
				require.NoError(t, err)
			}
		})
	}
}

// TestRetryBuffer_ConcurrentGetBody validates thread safety.
func TestRetryBuffer_ConcurrentGetBody(t *testing.T) {
	t.Parallel()

	rb := newRetryBuffer(
		io.NopCloser(bytes.NewReader([]byte("test"))),
		nil,
		4,
	)

	// Prime buffer
	io.ReadAll(rb)
	rb.Close()

	// Multiple goroutines call GetBody concurrently
	const N = 10
	var wg sync.WaitGroup
	wg.Add(N)
	for range N {
		go func() {
			defer wg.Done()
			body, err := rb.getBody()
			require.NoError(t, err)
			data, err := io.ReadAll(body)
			require.NoError(t, err)
			require.Equal(t, []byte("test"), data)
		}()
	}
	wg.Wait()
}

// TestRetryBuffer_EnforcesLimit validates that the retryBuffer
// doesn't read beyond the limit if the ContentLength is inaccurate.
func TestRetryBuffer_EnforcesLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		sourceSize    int
		contentLength int64
		expectLen     int
	}{
		{
			name:          "source_larger_than_content_length",
			sourceSize:    200,
			contentLength: 100,
			expectLen:     100,
		},
		{
			name:          "source_equals_content_length",
			sourceSize:    100,
			contentLength: 100,
			expectLen:     100,
		},
		{
			name:          "source_smaller_than_content_length",
			sourceSize:    50,
			contentLength: 100,
			expectLen:     50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			source := io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("x"), tt.sourceSize)))
			rb := newRetryBuffer(source, nil, tt.contentLength)

			// Read all
			data, err := io.ReadAll(rb)
			require.NoError(t, err)
			require.Len(t, data, tt.expectLen)

			// Close and get body
			rb.Close()
			body, err := rb.getBody()
			require.NoError(t, err)

			replay, err := io.ReadAll(body)
			require.NoError(t, err)
			require.Len(t, replay, tt.expectLen)
		})
	}
}

// TestRetryBuffer_CloseIdempotent validates Close can be called multiple times.
func TestRetryBuffer_CloseIdempotent(t *testing.T) {
	t.Parallel()

	rb := newRetryBuffer(
		io.NopCloser(bytes.NewReader([]byte("test"))),
		nil,
		4,
	)

	require.NoError(t, rb.Close())
	require.NoError(t, rb.Close())
	require.NoError(t, rb.Close())

	body, err := rb.getBody()
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, []byte("test"), data)
}

func TestRetryBuffer_CompletesBufferingInClose(t *testing.T) {
	t.Parallel()

	data := []byte("ABCDEFGHIJ")
	rb := newRetryBuffer(io.NopCloser(bytes.NewReader(data)), nil, 10)

	// Read partial
	buf := make([]byte, 5)
	n, err := rb.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.Equal(t, []byte("ABCDE"), buf)

	// Close completes buffering
	require.NoError(t, rb.Close())

	// GetBody returns complete buffer
	body, err := rb.getBody()
	require.NoError(t, err)
	all, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, data, all)
}

func TestRetryBuffer_GetValues(t *testing.T) {
	tests := []struct {
		name              string
		envTotal          string
		envPerRequest     string
		expectTotal       int64
		expectPerRequest  int64
		expectErrContains string
	}{
		{
			name:             "defaults_when_no_env",
			envTotal:         "",
			envPerRequest:    "",
			expectTotal:      defaultRetryBufferTotal,
			expectPerRequest: defaultRetryBufferPerRequest,
		},
		{
			name:             "custom_values_valid",
			envTotal:         "1048576",
			envPerRequest:    "524288",
			expectTotal:      1048576,
			expectPerRequest: 524288,
		},
		{
			name:             "zero_total_disables_retry",
			envTotal:         "0",
			envPerRequest:    "1024",
			expectTotal:      0,
			expectPerRequest: 1024,
		},
		{
			name:             "negative_total_disables_retry",
			envTotal:         "-1",
			envPerRequest:    "1024",
			expectTotal:      -1,
			expectPerRequest: 1024,
		},
		{
			name:              "small_total_with_default_per_request_exceeds",
			envTotal:          "1048576", // 1 MiB
			envPerRequest:     "",        // Defaults to 50 MiB
			expectErrContains: "per-request size (52428800 bytes) cannot exceed total buffer size (1048576 bytes)",
		},
		{
			name:              "small_total_explicit_zero_per_request_exceeds",
			envTotal:          "1048576", // 1 MiB
			envPerRequest:     "0",       // Defaults to 50 MiB
			expectErrContains: "per-request size (52428800 bytes) cannot exceed total buffer size (1048576 bytes)",
		},
		{
			name:              "small_total_explicit_negative_per_request_exceeds",
			envTotal:          "1048576", // 1 MiB
			envPerRequest:     "-1",      // Defaults to 50 MiB
			expectErrContains: "per-request size (52428800 bytes) cannot exceed total buffer size (1048576 bytes)",
		},
		{
			name:              "invalid_total_returns_error",
			envTotal:          "not-a-number",
			envPerRequest:     "1024",
			expectErrContains: "unable to parse env var",
		},
		{
			name:              "invalid_per_request_returns_error",
			envTotal:          "1048576",
			envPerRequest:     "not-a-number",
			expectErrContains: "unable to parse env var",
		},
		{
			name:              "per_request_exceeds_total",
			envTotal:          "1024",
			envPerRequest:     "2048",
			expectErrContains: "per-request size (2048 bytes) cannot exceed total buffer size (1024 bytes)",
		},
		{
			name:             "per_request_equals_total",
			envTotal:         "1024",
			envPerRequest:    "1024",
			expectTotal:      1024,
			expectPerRequest: 1024,
		},
		{
			name:             "very_large_values",
			envTotal:         "10737418240", // 10 GiB
			envPerRequest:    "1073741824",  // 1 GiB
			expectTotal:      10737418240,
			expectPerRequest: 1073741824,
		},
		{
			name:              "only_small_total_set_errors",
			envTotal:          "2097152", // 2 MiB < 50 MiB default
			envPerRequest:     "",
			expectErrContains: "per-request size (52428800 bytes) cannot exceed total buffer size (2097152 bytes)",
		},
		{
			name:             "only_large_total_set_succeeds",
			envTotal:         "524288000", // 500 MiB > 50 MiB default
			envPerRequest:    "",
			expectTotal:      524288000,
			expectPerRequest: defaultRetryBufferPerRequest,
		},
		{
			name:             "only_per_request_set",
			envTotal:         "",
			envPerRequest:    "2097152",
			expectTotal:      defaultRetryBufferTotal,
			expectPerRequest: 2097152,
		},
		{
			name:             "both_unset_uses_defaults",
			envTotal:         "",
			envPerRequest:    "",
			expectTotal:      defaultRetryBufferTotal,
			expectPerRequest: defaultRetryBufferPerRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envTotal != "" {
				t.Setenv(envRetryBufferTotal, tt.envTotal)
			}
			if tt.envPerRequest != "" {
				t.Setenv(envRetryBufferPerRequest, tt.envPerRequest)
			}

			total, perRequest, err := GetRetryBufferValues()
			if tt.expectErrContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectErrContains)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectTotal, total, "total mismatch")
			require.Equal(t, tt.expectPerRequest, perRequest, "per-request mismatch")
		})
	}
}

// Test helpers

type mockTransport struct {
	fn func(*http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.fn != nil {
		return m.fn(req)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}, nil
}

type failingReader struct {
	data    []byte
	pos     int
	failAt  int
	failErr error
}

func (f *failingReader) Read(p []byte) (int, error) {
	if f.pos >= f.failAt {
		return 0, f.failErr
	}
	n := copy(p, f.data[f.pos:])
	f.pos += n
	if f.pos >= f.failAt {
		return n, f.failErr
	}
	return n, nil
}

type errorCloser struct {
	io.Reader
	closeErr error
}

func (e *errorCloser) Close() error {
	return e.closeErr
}
