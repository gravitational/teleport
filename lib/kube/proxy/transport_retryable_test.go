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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

func TestRetryableTransport_WithGOAWAY(t *testing.T) {
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

					assert.Equal(t, http.StatusOK, res.StatusCode)
					assert.Equal(t, body, got)
				}
			}
		}()
	}
	wg.Wait()

	// just log it, actual count varies.
	t.Logf("connections used: %d", connCount.Load())
}

func TestRetryableTransport_GetBody(t *testing.T) {
	cases := []struct {
		name        string
		body        []byte
		wantGetBody bool
	}{
		{
			name:        "with body",
			body:        []byte("some data here"),
			wantGetBody: true,
		},
		{
			name:        "nil body",
			body:        nil,
			wantGetBody: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transport := &mockTransport{
				fn: func(req *http.Request) (*http.Response, error) {
					if tc.wantGetBody {
						require.NotNil(t, req.GetBody)
						// make sure it works.
						body, err := req.GetBody()
						require.NoError(t, err)
						data, err := io.ReadAll(body)
						require.NoError(t, err)
						require.Equal(t, tc.body, data)
					} else {
						require.Nil(t, req.GetBody)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader(nil)),
					}, nil
				},
			}

			wrapper := &retryableTransport{
				inner: transport,
			}

			var reqBody io.Reader
			if tc.body != nil {
				reqBody = io.NopCloser(bytes.NewReader(tc.body))
			}

			req, err := http.NewRequest(http.MethodPost, "http://test.local/api", reqBody)
			require.NoError(t, err)
			if tc.body != nil {
				req.ContentLength = int64(len(tc.body))
			}

			resp, err := wrapper.RoundTrip(req)
			require.NoError(t, err)
			require.NotNil(t, resp)
			resp.Body.Close()
		})
	}
}

func TestRetryableTransport_ReadOnlyStreams(t *testing.T) {
	// `watch` and `log follow` are buffered because they have
	// small or empty request bodies.
	paths := []string{
		"/api/v1/pods?watch=true",
		"/api/v1/namespaces/default/pods/mypod/log?follow=true",
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			transport := &mockTransport{
				fn: func(req *http.Request) (*http.Response, error) {
					require.NotNil(t, req.GetBody, "expect read-only streams buffered")
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader(nil)),
					}, nil
				},
			}

			wrapper := &retryableTransport{
				inner: transport,
			}

			req, err := http.NewRequest(http.MethodPost, "http://test.local"+p,
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

func TestRetryableTransport_IsStreamingProtocol(t *testing.T) {
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
			name: "Connection Upgrade without Upgrade header (malformed)",
			url:  "https://kube/api/v1/pods",
			headers: map[string]string{
				"Connection": "Upgrade",
			},
			expectStreaming: true,
		},

		// Kubernetes streaming protocol marker
		{
			name: "X-Stream-Protocol-Version present",
			url:  "https://kube/api/v1/pods",
			headers: map[string]string{
				"X-Stream-Protocol-Version": "v4.channel.k8s.io",
			},
			expectStreaming: true,
		},

		// Non-streaming operations
		{
			name:            "watch (read-only stream)",
			url:             "https://kube/api/v1/pods?watch=true",
			expectStreaming: false, // GET with no body, can be retried
		},
		{
			name:            "log follow (read-only stream)",
			url:             "https://kube/api/v1/namespaces/default/pods/mypod/log?follow=true",
			expectStreaming: false, // GET with no body, can be retried
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
			req, err := http.NewRequest("GET", tt.url, nil)
			require.NoError(t, err)

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := rt.isStreamingProtocol(req)
			assert.Equal(t, tt.expectStreaming, got)
		})
	}
}

func TestRetryableTransport_BufferLimits(t *testing.T) {
	tests := []struct {
		name          string
		description   string
		bodySize      int64
		contentLength int64
		expectGetBody bool
	}{
		{
			name:          "small body (1KB)",
			description:   "Small bodies are buffered",
			bodySize:      1024,
			contentLength: 1024,
			expectGetBody: true,
		},
		{
			name:          "exactly 5MB",
			description:   "Exact limit is buffered",
			bodySize:      5 * 1024 * 1024,
			contentLength: 5 * 1024 * 1024,
			expectGetBody: true,
		},
		{
			name:          "5MB + 1 byte",
			description:   "Over the limit is not buffered",
			bodySize:      5*1024*1024 + 1,
			contentLength: 5*1024*1024 + 1,
			expectGetBody: false,
		},
		{
			name:          "chunked encoding (ContentLength -1)",
			description:   "Chunked encoding, which have unknown size, are not buffered",
			bodySize:      1024,
			contentLength: -1,
			expectGetBody: false,
		},
		{
			name:          "zero length",
			description:   "Zero-length bodies have GetBody for consistency",
			bodySize:      0,
			contentLength: 0,
			expectGetBody: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.bodySize > 0 {
				body = bytes.Repeat([]byte("x"), int(tt.bodySize))
			}

			var gotGetBody bool
			transport := &mockTransport{
				fn: func(req *http.Request) (*http.Response, error) {
					gotGetBody = req.GetBody != nil
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader(nil)),
					}, nil
				},
			}

			wrapper := &retryableTransport{
				inner: transport,
			}

			req, err := http.NewRequest(http.MethodPost, "http://test.local/api",
				io.NopCloser(bytes.NewReader(body)))
			require.NoError(t, err)
			req.ContentLength = tt.contentLength

			resp, err := wrapper.RoundTrip(req)
			require.NoError(t, err)
			require.NotNil(t, resp)
			resp.Body.Close()

			assert.Equal(t, tt.expectGetBody, gotGetBody, tt.description)
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
