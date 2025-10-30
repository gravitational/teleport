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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

func TestRetryableTransport_WithGOAWAY(t *testing.T) {
	t.Parallel()

	backend := httptest.NewUnstartedServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate connection close on /goaway.
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

	// setup http2.
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
		MaxIdleConnsPerHost: -1, // Disable pooling for test.
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialFn(network, addr, tlsConfig)
		},
	}

	if err := http2.ConfigureTransport(tr); err != nil {
		t.Fatalf("failed to configure transport: %s", err)
	}

	wrapper := &retryableTransport{
		inner: tr,
	}

	client := &http.Client{
		Transport: wrapper,
	}

	// Hammer it with requests.
	var wg sync.WaitGroup
	wg.Add(10)
	for range 10 {
		go func() {
			defer wg.Done()
			for range 5 {
				// hit both normal and goaway endpoints.
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
					res.Body.Close()
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

	// just log it, actual count varies.
	// multiple connections show retry working
	t.Logf("connections used: %d", connCount.Load())
}

func TestRetryableTransport_MemoryBuffer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		bodySize int64
	}{
		{
			name:     "zero",
			bodySize: 0,
		},
		{
			name:     "small (1KB)",
			bodySize: 1024,
		},
		{
			name:     "medium (1MB)",
			bodySize: 1024 * 1024,
		},
		{
			name:     "exact limit (5MB)",
			bodySize: 5 * 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := bytes.Repeat([]byte("x"), int(tt.bodySize))

			transport := &mockTransport{
				fn: func(req *http.Request) (*http.Response, error) {
					readData, err := io.ReadAll(req.Body)
					require.NoError(t, err)
					require.Equal(t, body, readData)

					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader(nil)),
					}, nil
				},
			}

			wrapper := &retryableTransport{
				inner: transport,
			}

			req, err := http.NewRequest(http.MethodPost, "http://test/api",
				io.NopCloser(bytes.NewReader(body)))
			require.NoError(t, err)
			req.ContentLength = int64(len(body))

			cleanup, err := wrapper.makeRetryable(req)
			require.NoError(t, err)
			require.Nil(t, cleanup, " expect memory buffer not to return cleanup function")
			require.NotNil(t, req.GetBody, "expect GetBody to be set for memory buffer")

			// Test request flow
			resp, err := wrapper.inner.RoundTrip(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())

			// Verify GetBody returns correct data
			retryBody, err := req.GetBody()
			require.NoError(t, err)
			retryData, err := io.ReadAll(retryBody)
			require.NoError(t, err)
			require.NoError(t, retryBody.Close())
			require.Equal(t, body, retryData, "expect GetBody to return complete body")
		})
	}
}

func TestRetryableTransport_DiskBuffer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		bodySize      int64
		contentLength int64
		description   string
	}{
		{
			name:          "5MB + 1",
			bodySize:      5*1024*1024 + 1,
			contentLength: 5*1024*1024 + 1,
			description:   "just over limit",
		},
		{
			name:          "6MB",
			bodySize:      6 * 1024 * 1024,
			contentLength: 6 * 1024 * 1024,
			description:   "over limit",
		},
		{
			name:          "10MB",
			bodySize:      10 * 1024 * 1024,
			contentLength: 10 * 1024 * 1024,
			description:   "twice limit",
		},
		{
			name:          "unknown size",
			bodySize:      1024,
			contentLength: -1,
			description:   "chunked encoding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := bytes.Repeat([]byte("x"), int(tt.bodySize))
			rt := &retryableTransport{inner: &mockTransport{}}

			req, _ := http.NewRequest("POST", "http://test", io.NopCloser(bytes.NewReader(body)))
			req.ContentLength = tt.contentLength

			cleanup, err := rt.makeRetryable(req)
			require.NoError(t, err)
			require.NotNil(t, cleanup, "expect disk buffer to return cleanup")
			defer cleanup()
			require.NotNil(t, req.GetBody)

			// Verify original body can be read
			readData, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			require.Equal(t, body, readData)
			require.NoError(t, req.Body.Close())

			// Verify retry body works
			retryBody, err := req.GetBody()
			require.NoError(t, err)
			retryData, err := io.ReadAll(retryBody)
			require.NoError(t, err)
			require.Equal(t, body, retryData, "expect retry body to match original")
			require.NoError(t, retryBody.Close())
		})
	}
}

func TestRetryableTransport_DiskCleanup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		scenario func(t *testing.T, req *http.Request)
	}{
		{
			name: "cleanup after successful request",
			scenario: func(t *testing.T, req *http.Request) {
				io.Copy(io.Discard, req.Body)
				require.NoError(t, req.Body.Close())
			},
		},
		{
			name: "cleanup after retry",
			scenario: func(t *testing.T, req *http.Request) {
				io.Copy(io.Discard, req.Body)
				require.NoError(t, req.Body.Close())

				retryBody, err := req.GetBody()
				require.NoError(t, err)
				io.Copy(io.Discard, retryBody)
				require.NoError(t, retryBody.Close())
			},
		},
		{
			name: "cleanup after partial read",
			scenario: func(t *testing.T, req *http.Request) {
				buf := make([]byte, 1024)
				req.Body.Read(buf)
				require.NoError(t, req.Body.Close())
			},
		},
		{
			name: "cleanup without reading body",
			scenario: func(t *testing.T, req *http.Request) {
				// Don't read or close body. Simulates an early return/error
				// before the body is used. Expect cleanup to work.
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rt := &retryableTransport{inner: &mockTransport{}}
			body := bytes.Repeat([]byte("x"), 6*1024*1024)
			req, _ := http.NewRequest("POST", "http://test", io.NopCloser(bytes.NewReader(body)))
			req.ContentLength = int64(len(body))

			cleanup, err := rt.makeRetryable(req)
			require.NoError(t, err)
			require.NotNil(t, cleanup, "expect cleanup function for disk buffer")

			// Run scenario
			tt.scenario(t, req)

			// Verify cleanup succeeds
			err = cleanup()
			if err != nil {
				require.True(t, os.IsNotExist(err), "expect cleanup to succeed")
			}
		})
	}
}

func TestRetryableTransport_IsStreamingProtocol(t *testing.T) {
	t.Parallel()
	rt := &retryableTransport{}

	tests := []struct {
		name            string
		url             string
		headers         map[string]string
		expectStreaming bool
	}{
		// Protocol upgrades
		{
			name: "WebSocket upgrade",
			url:  "https://kube/api/v1/pods",
			headers: map[string]string{
				"Upgrade":    "websocket",
				"Connection": "Upgrade",
			},
			expectStreaming: true,
		},
		{
			name: "SPDY upgrade",
			url:  "https://kube/api/v1/pods",
			headers: map[string]string{
				"Upgrade":    "SPDY/3.1",
				"Connection": "Upgrade",
			},
			expectStreaming: true,
		},
		{
			name: "Connection Upgrade only (malformed but defensive)",
			url:  "https://kube/api/v1/pods",
			headers: map[string]string{
				"Connection": "Upgrade",
			},
			expectStreaming: true,
		},
		{
			name: "X-Stream-Protocol-Version",
			url:  "https://kube/api/v1/pods",
			headers: map[string]string{
				"X-Stream-Protocol-Version": "v4.channel.k8s.io",
			},
			expectStreaming: true,
		},

		// Non-streaming
		{
			name:            "watch (read-only stream)",
			url:             "https://kube/api/v1/pods?watch=true",
			expectStreaming: false,
		},
		{
			name:            "log follow (read-only stream)",
			url:             "https://kube/api/v1/namespaces/default/pods/mypod/log?follow=true",
			expectStreaming: false,
		},
		{
			name:            "regular GET",
			url:             "https://kube/api/v1/pods",
			expectStreaming: false,
		},
		{
			name:            "regular POST",
			url:             "https://kube/api/v1/namespaces/default/pods",
			expectStreaming: false,
		},
		{
			name:            "proxy request",
			url:             "https://kube/api/v1/namespaces/default/pods/mypod/proxy/healthz",
			expectStreaming: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequest("GET", tt.url, nil)
			require.NoError(t, err)

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := rt.isStreamingProtocol(req)
			require.Equal(t, tt.expectStreaming, got)
		})
	}
}

func TestRetryableTransport_WatchAndLogAreBuffered(t *testing.T) {
	// `watch` and `log?follow` are buffered because they have
	// small or empty request bodies that can be retried,
	// despite having streaming responses.
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
				inner: transport,
			}

			req, err := http.NewRequest(http.MethodPost, "http://test"+p,
				io.NopCloser(bytes.NewReader([]byte("body"))))
			require.NoError(t, err)
			req.ContentLength = 4

			resp, err := wrapper.RoundTrip(req)
			require.NoError(t, err)
			require.NotNil(t, resp)
			resp.Body.Close()
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
