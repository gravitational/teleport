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
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/sync/semaphore"
)

func TestRetryableTransport_GOAWAY(t *testing.T) {
	t.Parallel()
	// Verify that requests with buffered bodies are automatically retried
	// when the server closes a connection. This simulates HTTP/2 GOAWAY
	// behavior using "Connection: close" header. Identical to http2.Transport
	// retries when a GetBody is set.
	// See https://github.com/kubernetes/kubernetes/blob/ee1ff4866e30ac3685da3e007979b0e9ab7651a6/staging/src/k8s.io/apiserver/pkg/server/filters/goaway.go#L65-L73

	backend := httptest.NewUnstartedServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.RequestURI == "/goaway" {
				w.Header().Set("Connection", "close")
			}

			data, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to read request body: %s", err),
					http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write(data)
		}),
	)
	t.Cleanup(backend.Close)

	http2Options := &http2.Server{}
	http2.ConfigureServer(backend.Config, http2Options)
	backend.TLS = backend.Config.TLSConfig
	backend.EnableHTTP2 = true
	backend.StartTLS()

	var connCount atomic.Int32

	dialFn := func(network, addr string, cfg *tls.Config) (conn net.Conn, err error) {
		conn, err = tls.Dial(network, addr, cfg)
		if err != nil {
			t.Fatalf("dial failed: %s", err)
		}
		connCount.Add(1)
		return
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{http2.NextProtoTLS},
	}

	tr := &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
		MaxIdleConnsPerHost: -1,
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialFn(network, addr, tlsConfig)
		},
	}

	if err := http2.ConfigureTransport(tr); err != nil {
		t.Fatalf("failed to configure transport: %s", err)
	}

	rt := &retryableTransport{
		inner:     tr,
		log:       slog.New(slog.DiscardHandler),
		semaphore: semaphore.NewWeighted(1024),
	}

	client := &http.Client{
		Transport: rt,
	}

	var wg sync.WaitGroup
	wg.Add(10)
	for range 10 {
		go func() {
			defer wg.Done()
			for range 5 {
				for _, path := range []string{"/", "/goaway"} {
					body := []byte("test payload")
					req, err := http.NewRequestWithContext(context.Background(),
						http.MethodPost, backend.URL+path, bytes.NewReader(body))
					if err != nil {
						t.Errorf("failed to create request: %s", err)
						return
					}

					res, err := client.Do(req)
					if err != nil {
						t.Errorf("request failed: %s", err)
						return
					}

					got, err := io.ReadAll(res.Body)
					require.NoError(t, res.Body.Close())
					if err != nil {
						t.Errorf("failed to read response: %s", err)
						return
					}

					require.Equal(t, http.StatusOK, res.StatusCode)
					require.Equal(t, body, got)
				}
			}
		}()
	}
	wg.Wait()

	t.Logf("connections used: %d", connCount.Load())
}

func TestRetryableTransport_MemoryBuffer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		bodySize int64
	}{
		{name: "small (1 KiB)", bodySize: 1024},
		{name: "medium (1 MiB)", bodySize: 1024 * 1024},
		{name: "large (10 MiB)", bodySize: 10 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			originalBody := bytes.Repeat([]byte("x"), int(tt.bodySize))
			rt := &retryableTransport{
				inner:     &mockTransport{},
				log:       slog.New(slog.DiscardHandler),
				semaphore: semaphore.NewWeighted(11 * 1024 * 1024),
			}

			req, _ := http.NewRequest("POST", "http://test",
				io.NopCloser(bytes.NewReader(originalBody)))
			req.ContentLength = int64(len(originalBody))

			cleanup, err := rt.makeRetryable(req)
			require.NoError(t, err)
			require.NotNil(t, cleanup)
			defer cleanup()
			require.NotNil(t, req.GetBody)

			// Simulate http2.Transport reading the body
			// This populates the buffer via TeeReader
			sentData, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			require.Equal(t, originalBody, sentData, "original body should be readable")
			require.NoError(t, req.Body.Close())

			// Verify GetBody works for retry
			retryBody, err := req.GetBody()
			require.NoError(t, err)
			retryData, err := io.ReadAll(retryBody)
			require.NoError(t, err)
			require.Equal(t, originalBody, retryData, "GetBody should return complete data")
			require.NoError(t, retryBody.Close())
		})
	}
}

func TestRetryableTransport_SemaphoreBlocking(t *testing.T) {
	t.Parallel()

	rt := &retryableTransport{
		inner:     &mockTransport{},
		log:       slog.New(slog.DiscardHandler),
		semaphore: semaphore.NewWeighted(10 * 1024),
	}

	// Exhaust semaphore with multiple small requests.
	cleanups := make([]func(), 0)
	for range 10 {
		body := bytes.Repeat([]byte("x"), 1024)
		req, _ := http.NewRequest("POST", "http://test",
			io.NopCloser(bytes.NewReader(body)))
		req.ContentLength = 1024

		cleanup, err := rt.makeRetryable(req)
		require.NoError(t, err)
		require.NotNil(t, cleanup)
		cleanups = append(cleanups, cleanup)
	}

	// Semaphore exhausted, expect next request with timeout to fail.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	body := bytes.Repeat([]byte("y"), 1024)
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://test",
		io.NopCloser(bytes.NewReader(body)))
	req.ContentLength = 1024

	cleanup, err := rt.makeRetryable(req)
	require.NoError(t, err)
	require.Nil(t, cleanup, "expect semaphore to timeout")

	// Cleanup
	for _, c := range cleanups {
		c()
	}
}

func TestRetryableTransport_SemaphoreContextCanceled(t *testing.T) {
	t.Parallel()

	rt := &retryableTransport{
		inner:     &mockTransport{},
		log:       slog.New(slog.DiscardHandler),
		semaphore: semaphore.NewWeighted(5 * 1024),
	}

	// Exhaust semaphore.
	body1 := bytes.Repeat([]byte("x"), 5*1024)
	req1, _ := http.NewRequest("POST", "http://test", io.NopCloser(bytes.NewReader(body1)))
	req1.ContentLength = int64(len(body1))
	cleanup1, _ := rt.makeRetryable(req1)
	defer cleanup1()

	// Request with pre-canceled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	body2 := bytes.Repeat([]byte("y"), 5*1024)
	req2, _ := http.NewRequestWithContext(ctx, "POST", "http://test",
		io.NopCloser(bytes.NewReader(body2)))
	req2.ContentLength = int64(len(body2))

	cleanup2, err := rt.makeRetryable(req2)
	require.NoError(t, err)
	require.Nil(t, cleanup2)
	require.Nil(t, req2.GetBody)
}

func TestRetryableTransport_SemaphoreReleasedOnError(t *testing.T) {
	t.Parallel()

	rt := &retryableTransport{
		inner: &mockTransport{
			fn: func(req *http.Request) (*http.Response, error) {
				// Return error during request
				return nil, errors.New("simulated error")
			},
		},
		log:       slog.New(slog.DiscardHandler),
		semaphore: semaphore.NewWeighted(5 * 1024),
	}

	// Make a first request which acquires semaphore.
	body := bytes.Repeat([]byte("x"), 5*1024)
	req, _ := http.NewRequest("POST", "http://test",
		io.NopCloser(bytes.NewReader(body)))
	req.ContentLength = 5 * 1024

	// This will acquire semaphore, then return error.
	resp, err := rt.RoundTrip(req)
	require.Error(t, err)
	if resp != nil {
		require.NoError(t, resp.Body.Close())
	}

	// Expect semaphore to release despite error.
	// Verify by immediately acquiring again.
	err = rt.semaphore.Acquire(context.Background(), 5*1024)
	require.NoError(t, err, "expect semaphore to release after error")
}

func TestRetryableTransport_SkipConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		contentLength int64
		expectSkip    bool
		reason        string
	}{
		{
			name:          "chunked encoding",
			contentLength: -1,
			expectSkip:    true,
			reason:        "unknown size",
		},
		{
			name:          "too large",
			contentLength: 60 * 1024 * 1024,
			expectSkip:    true,
			reason:        "exceeds 50 MiB limit",
		},
		{
			name:          "exactly at limit",
			contentLength: 50 * 1024 * 1024,
			expectSkip:    false,
			reason:        "at 50 MiB limit should buffer",
		},
		{
			name:          "just under limit",
			contentLength: 50*1024*1024 - 1,
			expectSkip:    false,
			reason:        "under limit should buffer",
		},
		{
			name:          "normal size",
			contentLength: 1024,
			expectSkip:    false,
			reason:        "normal size should buffer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rt := &retryableTransport{
				inner:     &mockTransport{},
				log:       slog.New(slog.DiscardHandler),
				semaphore: semaphore.NewWeighted(61 * 1024 * 1024), // 61 MiB
			}

			body := bytes.Repeat([]byte("x"), 1024)
			req, _ := http.NewRequest("POST", "http://test",
				io.NopCloser(bytes.NewReader(body)))
			req.ContentLength = tt.contentLength

			cleanup, err := rt.makeRetryable(req)
			require.NoError(t, err)

			if tt.expectSkip {
				require.Nil(t, cleanup, tt.reason)
				require.Nil(t, req.GetBody, tt.reason)
			} else {
				require.NotNil(t, cleanup, tt.reason)
				require.NotNil(t, req.GetBody, tt.reason)
				if cleanup != nil {
					defer cleanup()
				}
			}
		})
	}
}

func TestRetryableTransport_IsUpgrade(t *testing.T) {
	t.Parallel()

	rt := &retryableTransport{}

	tests := []struct {
		name      string
		headers   map[string]string
		isUpgrade bool
	}{
		{
			name: "WebSocket upgrade",
			headers: map[string]string{
				"Upgrade":    "websocket",
				"Connection": "Upgrade",
			},
			isUpgrade: true,
		},
		{
			name: "SPDY upgrade",
			headers: map[string]string{
				"Upgrade":    "SPDY/3.1",
				"Connection": "Upgrade",
			},
			isUpgrade: true,
		},
		{
			name: "X-Stream-Protocol-Version",
			headers: map[string]string{
				"X-Stream-Protocol-Version": "v4.channel.k8s.io",
			},
			isUpgrade: true,
		},
		{
			name:      "no upgrade headers",
			headers:   map[string]string{},
			isUpgrade: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequest("GET", "http://test", nil)
			require.NoError(t, err)

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := rt.isUpgrade(req)
			require.Equal(t, tt.isUpgrade, got)
		})
	}
}

func TestRetryableTransport_WatchAndLogAreBuffered(t *testing.T) {
	t.Parallel()

	paths := []string{
		"/api/v1/pods?watch=true",
		"/api/v1/namespaces/default/pods/mypod/log?follow=true",
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			t.Parallel()
			transport := &mockTransport{
				fn: func(req *http.Request) (*http.Response, error) {
					require.NotNil(t, req.GetBody, "watch/log should be buffered")
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader(nil)),
					}, nil
				},
			}

			wrapper := &retryableTransport{
				inner:     transport,
				log:       slog.New(slog.DiscardHandler),
				semaphore: semaphore.NewWeighted(1024),
			}

			req, err := http.NewRequest(http.MethodPost, "http://test"+p,
				io.NopCloser(bytes.NewReader([]byte("body"))))
			require.NoError(t, err)
			req.ContentLength = 4

			resp, err := wrapper.RoundTrip(req)
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.NoError(t, resp.Body.Close())
		})
	}
}

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
