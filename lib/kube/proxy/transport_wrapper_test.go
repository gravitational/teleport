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

	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

func TestTransportWrapperWithGOAWAY(t *testing.T) {
	backend := httptest.NewUnstartedServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// trigger connection close on /goaway.
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
		MaxIdleConnsPerHost: -1, // disable pooling for test.
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialFn(network, addr, tlsConfig)
		},
	}
	
	if err := http2.ConfigureTransport(tr); err != nil {
		t.Fatalf("failed to configure transport: %s", err)
	}

	wrapper := &transportWrapper{
		RoundTripper: tr,
	}

	client := &http.Client{
		Transport: wrapper,
	}

	// hammer it with requests.
	wg := &sync.WaitGroup{}
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
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
	t.Logf("connections used: %d", connCount.Load())
}

func TestTransportWrapperGetBody(t *testing.T) {
	cases := []struct {
		name      string
		body      []byte
		wantGetBody bool
	}{
		{
			name:      "with body",
			body:      []byte("some data here"),
			wantGetBody: true,
		},
		{
			name:      "nil body",
			body:      nil,
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

			wrapper := &transportWrapper{
				RoundTripper: transport,
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

func TestTransportWrapperStreamingSkipped(t *testing.T) {
	// these shouldn't get retryable treatment.
	paths := []string{
		"/api/v1/namespaces/default/pods/mypod/exec",
		"/api/v1/namespaces/default/pods/mypod/attach",
		"/api/v1/namespaces/default/pods/mypod/portforward",
		"/api/v1/pods?watch=true",
		"/api/v1/namespaces/default/pods/mypod/log?follow=true",
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			transport := &mockTransport{
				fn: func(req *http.Request) (*http.Response, error) {
					// streaming requests shouldn't have GetBody set.
					require.Nil(t, req.GetBody)
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader(nil)),
					}, nil
				},
			}

			wrapper := &transportWrapper{
				RoundTripper: transport,
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