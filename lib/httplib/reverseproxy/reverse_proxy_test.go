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
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
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

	backendURL, err := url.Parse(backend.URL)
	require.NoError(t, err)
	proxyHandler := newSingleHostReverseProxy(backendURL)

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

func newSingleHostReverseProxy(target *url.URL) *Forwarder {
	return &Forwarder{
		ReverseProxy: httputil.NewSingleHostReverseProxy(target),
		logger:       slog.Default(),
	}

}
