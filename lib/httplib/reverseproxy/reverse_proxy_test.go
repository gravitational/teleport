/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package reverseproxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

// TestRequestCancelWithoutPanic tests that canceling a request does not
// cause a panic in the reverse proxy handler. This is important to ensure
// that the reverse proxy can handle client disconnects gracefully without
// crashing the server.
// It simulates a long-running request and then cancels it, ensuring that
// frontend doesn't panic, the backend handler receives the cancelation,
// and all resources are cleaned up properly.
func TestRequestCancelWithoutPanic(t *testing.T) {
	var numberOfActiveRequests atomic.Int64

	wg := &sync.WaitGroup{}
	wg.Add(1)

	backend := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer wg.Done()

			numberOfActiveRequests.Add(1)
			defer numberOfActiveRequests.Add(-1)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Hello, world!"))
			// Ensure the response is flushed to the client immediately.
			w.(http.Flusher).Flush()

			// Simulate a long-running request.
			select {
			case <-r.Context().Done():
				// Request was canceled, do nothing.
				return
			case <-t.Context().Done():
				// Test context was canceled. At this point, the test failed
				panic("test context canceled before request completed")
			}
		},
		))

	t.Cleanup(backend.Close)

	proxyHandler := newSingleHostReverseProxy(t, backend)

	wg.Add(1)
	frontend := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			numberOfActiveRequests.Add(1)
			proxyHandler.ServeHTTP(w, r)
			// Place the wg.Done() call here to ensure that
			// if the panic occurs, it will never be called.
			numberOfActiveRequests.Add(-1)
			wg.Done()
		}),
	)

	ctx, cancel := context.WithCancel(t.Context())
	getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, frontend.URL, nil)

	frontendClient := frontend.Client()
	res, err := frontendClient.Do(getReq)
	require.NoError(t, err)
	t.Cleanup(func() {
		io.Copy(io.Discard, res.Body) // Drain the body to avoid resource leaks.
		_ = res.Body.Close()          // Ensure we close the response body to avoid resource leaks.
	})

	require.Equal(t, http.StatusOK, res.StatusCode)

	data := make([]byte, 20)
	n, err := res.Body.Read(data)
	require.NoError(t, err)
	// Ensure we read the expected response.
	require.Equal(t, "Hello, world!", string(data[:n]))

	require.Equal(t, int64(2), numberOfActiveRequests.Load(), "There should two active handlers at this point.")

	cancel()  // Cancel the request to simulate client disconnect.
	wg.Wait() // Wait for the backend handler to finish.

	require.Equal(t, int64(0), numberOfActiveRequests.Load(), "There should be no active handlers after the request is canceled.")

}

func newSingleHostReverseProxy(t *testing.T, target *httptest.Server) *Forwarder {
	targetURL, err := url.Parse(target.URL)
	require.NoError(t, err)
	transport := target.Client().Transport.(*http.Transport)
	transport.MaxIdleConnsPerHost = -1
	fwd := &Forwarder{
		ReverseProxy:   httputil.NewSingleHostReverseProxy(targetURL),
		logger:         slog.Default(),
		withBodyRewind: true,
		transport:      transport,
	}
	fwd.ReverseProxy.Transport = transport
	return fwd
}

// TestClientReceivedGOAWAY tests the in-flight watch requests will not be affected and new requests use a new
// connection after client received GOAWAY.
func TestClientReceivedGOAWAY(t *testing.T) {
	backend := httptest.NewUnstartedServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println(r.Proto)
			if r.RequestURI == "/goaway" {
				w.Header().Set("Connection", "close")
			}

			data, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to read request body: %v", err),
					http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write(data)
		},
		))

	t.Cleanup(backend.Close)

	http2Options := &http2.Server{}

	http2.ConfigureServer(backend.Config, http2Options)

	backend.TLS = backend.Config.TLSConfig
	backend.EnableHTTP2 = true
	backend.StartTLS()

	var newConn atomic.Int32

	// create the http client
	dialFn := func(network, addr string, cfg *tls.Config) (conn net.Conn, err error) {
		conn, err = tls.Dial(network, addr, cfg)
		if err != nil {
			t.Fatalf("unexpect connection err: %v", err)
		}

		newConn.Add(1)
		return
	}
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{http2.NextProtoTLS},
	}
	tr := &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
		// Disable connection pooling to avoid additional connections
		// that cause the test to flake
		MaxIdleConnsPerHost: -1,
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialFn(network, addr, tlsConfig)
		},
	}
	if err := http2.ConfigureTransport(tr); err != nil {
		t.Fatalf("failed to configure http transport, err: %v", err)
	}

	proxyHandler := newSingleHostReverseProxy(t, backend)
	proxyHandler.Transport = tr
	proxyHandler.ReverseProxy.Transport = tr
	frontend := httptest.NewUnstartedServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			proxyHandler.ServeHTTP(w, r)
		}),
	)

	frontend.EnableHTTP2 = true
	frontend.StartTLS()

	frontendClient := frontend.Client()

	wg := &sync.WaitGroup{}
	wg.Add(50)
	for range 50 {
		go func() {
			defer wg.Done()
			for range 5 {
				for _, url := range []string{"/", "/goaway"} {
					t.Logf("Sending request to %s", url)
					payload := bytes.NewReader([]byte("Hello, world!"))
					getReq, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, frontend.URL+url, payload)

					res, err := frontendClient.Do(getReq)
					require.NoError(t, err)
					t.Cleanup(func() {
						io.Copy(io.Discard, res.Body) // Drain the body to avoid resource leaks.
						_ = res.Body.Close()          // Ensure we close the response body to avoid resource leaks.
					})

					if url == "/goaway" {
						require.Equal(t, http.StatusOK, res.StatusCode)
					}

					require.Equal(t, http.StatusOK, res.StatusCode)

					receivedPayload, err := io.ReadAll(res.Body)
					require.NoError(t, err)
					// Ensure we read the expected response.
					require.Equal(t, "Hello, world!", string(receivedPayload))

				}
			}
		}()
	}
	wg.Wait()
}
