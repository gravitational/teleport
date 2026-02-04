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

package readyz_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/testutils/synctest"
)

func TestReadyz(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	clock := clockwork.NewFakeClockAt(now)

	reg := readyz.NewRegistry(readyz.WithClock(clock))

	a := reg.AddService("svc", "a")
	b := reg.AddService("svc", "b")

	srv := httptest.NewServer(readyz.HTTPHandler(reg))
	srv.URL = srv.URL + "/readyz"
	t.Cleanup(srv.Close)

	t.Run("initial state - overall", func(t *testing.T) {
		rsp, err := http.Get(srv.URL)
		require.NoError(t, err)
		defer rsp.Body.Close()

		require.Equal(t, http.StatusServiceUnavailable, rsp.StatusCode)

		var response readyz.OverallStatus
		err = json.NewDecoder(rsp.Body).Decode(&response)
		require.NoError(t, err)

		require.Equal(t,
			readyz.OverallStatus{
				Status: readyz.Unhealthy,
				Services: map[string]*readyz.ServiceStatus{
					"a": {Status: readyz.Initializing},
					"b": {Status: readyz.Initializing},
				},
				PID: os.Getpid(),
			},
			response,
		)
	})

	t.Run("individual service", func(t *testing.T) {
		a.ReportReason(readyz.Unhealthy, "database is down")

		rsp, err := http.Get(srv.URL + "/a")
		require.NoError(t, err)
		defer rsp.Body.Close()

		require.Equal(t, http.StatusServiceUnavailable, rsp.StatusCode)

		var response readyz.ServiceStatus
		err = json.NewDecoder(rsp.Body).Decode(&response)
		require.NoError(t, err)

		require.Equal(t,
			readyz.ServiceStatus{
				Status:    readyz.Unhealthy,
				Reason:    "database is down",
				UpdatedAt: &now,
			},
			response,
		)
	})

	t.Run("mixed state", func(t *testing.T) {
		a.Report(readyz.Healthy)
		b.ReportReason(readyz.Unhealthy, "database is down")

		rsp, err := http.Get(srv.URL)
		require.NoError(t, err)
		defer rsp.Body.Close()

		require.Equal(t, http.StatusServiceUnavailable, rsp.StatusCode)

		var response readyz.OverallStatus
		err = json.NewDecoder(rsp.Body).Decode(&response)
		require.NoError(t, err)

		require.Equal(t,
			readyz.OverallStatus{
				Status: readyz.Unhealthy,
				Services: map[string]*readyz.ServiceStatus{
					"a": {Status: readyz.Healthy, UpdatedAt: &now},
					"b": {Status: readyz.Unhealthy, Reason: "database is down", UpdatedAt: &now},
				},
				PID: os.Getpid(),
			},
			response,
		)
	})

	t.Run("all healthy", func(t *testing.T) {
		a.Report(readyz.Healthy)
		b.Report(readyz.Healthy)

		rsp, err := http.Get(srv.URL)
		require.NoError(t, err)
		defer rsp.Body.Close()

		require.Equal(t, http.StatusOK, rsp.StatusCode)

		var response readyz.OverallStatus
		err = json.NewDecoder(rsp.Body).Decode(&response)
		require.NoError(t, err)

		require.Equal(t,
			readyz.OverallStatus{
				Status: readyz.Healthy,
				Services: map[string]*readyz.ServiceStatus{
					"a": {Status: readyz.Healthy, UpdatedAt: &now},
					"b": {Status: readyz.Healthy, UpdatedAt: &now},
				},
				PID: os.Getpid(),
			},
			response,
		)
	})

	t.Run("unknown service", func(t *testing.T) {
		rsp, err := http.Get(srv.URL + "/foo")
		require.NoError(t, err)
		defer rsp.Body.Close()

		require.Equal(t, http.StatusNotFound, rsp.StatusCode)
	})
}

func TestAllServicesReported(t *testing.T) {
	reg := readyz.NewRegistry()

	a := reg.AddService("svc", "a")
	b := reg.AddService("svc", "b")

	select {
	case <-reg.AllServicesReported():
		t.Fatal("AllServicesReported should be blocked")
	default:
	}

	a.Report(readyz.Healthy)

	select {
	case <-reg.AllServicesReported():
		t.Fatal("AllServicesReported should be blocked")
	default:
	}

	b.Report(readyz.Unhealthy)

	select {
	case <-reg.AllServicesReported():
	default:
		t.Fatal("AllServicesReported should not be blocked")
	}
}

// testDefaultClient returns a usable http.Client with a relatively short
// timeout
func testDefaultClient(t *testing.T) *http.Client {
	t.Helper()

	client, err := defaults.HTTPClient()
	require.NoError(t, err)

	client.Timeout = time.Second

	return client
}

// requestNotification is minimal helper that is sent when a request starts, and
// provides a done channel to notify when the request ends.
type requestNotification struct {
	done <-chan struct{}
}

// isComplete checks if the request is still in flight (false) or complete
// (true).
func (r *requestNotification) isComplete() bool {
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}

type inMemoryTransport struct {
	handler  http.Handler
	requests chan<- requestNotification
}

func (c *inMemoryTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	done := make(chan struct{})
	rr := httptest.NewRecorder()

	// We can't just naively return the result of ServeHTTP since it doesn't
	// return errors when the context is canceled. To fix that, wrap it in
	// another goroutine and manually return an error if the context is
	// canceled.
	go func() {
		// Notify test that the request is in flight.
		reqDone := make(chan struct{})

		// Attempt to notify the test about the request. Drop any notifications
		// to a full channel - we never want to block, and the tests in practice
		// only care about the first request and the existence of future
		// requests.
		c.requests <- requestNotification{done: reqDone}

		c.handler.ServeHTTP(rr, r)

		close(done)
		close(reqDone)
	}()

	select {
	case <-r.Context().Done():
		<-done
		return nil, r.Context().Err()
	case <-done:
		return rr.Result(), nil
	}
}

func createInMemoryWaitClient(handler http.Handler, requests chan<- requestNotification) *http.Client {
	return &http.Client{
		Transport: &inMemoryTransport{
			handler:  handler,
			requests: requests,
		},
	}
}

// errorTransport is a mock roundtripper that just returns an error
type errorTransport struct{}

func (c *errorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, errors.New("error")
}

func testDummyClient() *http.Client {
	return &http.Client{
		Transport: &errorTransport{},
		Timeout:   time.Second * 1,
	}
}

func testWaitFetch(t *testing.T, client *http.Client, service string, endpoint *url.URL) (int, error) {
	t.Helper()

	if service != "" {
		endpoint = endpoint.JoinPath(service)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, endpoint.String(), nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	if err != nil {
		// Let the test examine errors here
		return 0, trace.Wrap(err)
	}
	defer resp.Body.Close()

	bytes, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	require.NoError(t, err)

	// Note: trace.ReadError does not categorize 5xx errors, so we'll just add
	// some details to the status.
	return resp.StatusCode, trace.Wrap(trace.ReadError(resp.StatusCode, bytes), resp.Status)
}

func TestWaitAPI(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	clock := clockwork.NewFakeClockAt(now)

	tests := []struct {
		name string
		exec func(t *testing.T, reg *readyz.Registry, endpoint *url.URL)
	}{
		{
			name: "simple - not ready",
			exec: func(t *testing.T, reg *readyz.Registry, endpoint *url.URL) {
				reg.AddService("svc", "a")

				_, err := testWaitFetch(t, testDefaultClient(t), "", endpoint)
				require.ErrorContains(t, err, "Client.Timeout exceeded")
			},
		},
		{
			name: "simple - ready",
			exec: func(t *testing.T, reg *readyz.Registry, endpoint *url.URL) {
				a := reg.AddService("svc", "a")
				a.Report(readyz.Healthy)

				code, err := testWaitFetch(t, testDefaultClient(t), "", endpoint)
				require.NoError(t, err)
				require.Equal(t, 200, code)
			},
		},
		{
			name: "simple - error",
			exec: func(t *testing.T, reg *readyz.Registry, endpoint *url.URL) {
				a := reg.AddService("svc", "a")
				a.ReportReason(readyz.Unhealthy, "testing error")

				code, err := testWaitFetch(t, testDefaultClient(t), "", endpoint)
				require.Error(t, err)
				require.Equal(t, 503, code)
			},
		},
		{
			name: "simple - no server",
			exec: func(t *testing.T, reg *readyz.Registry, endpoint *url.URL) {
				_, err := testWaitFetch(t, testDummyClient(), "", endpoint)
				require.ErrorContains(t, err, "error")
			},
		},
		{
			name: "eventually becomes healthy",
			exec: func(t *testing.T, reg *readyz.Registry, endpoint *url.URL) {
				a := reg.AddService("svc", "a")

				// First try times out (waiting for initial status)
				_, err := testWaitFetch(t, testDefaultClient(t), "", endpoint)
				require.ErrorContains(t, err, "Client.Timeout exceeded")

				a.Report(readyz.Unhealthy)

				// Second try returns, but reports unhealthy
				code, err := testWaitFetch(t, testDefaultClient(t), "", endpoint)
				require.Error(t, err) // error has no details to examine
				require.Equal(t, 503, code)

				// Third try reports healthy
				a.Report(readyz.Healthy)

				code, err = testWaitFetch(t, testDefaultClient(t), "", endpoint)
				require.NoError(t, err)
				require.Equal(t, 200, code)
			},
		},
		{
			name: "specific service - not found",
			exec: func(t *testing.T, reg *readyz.Registry, endpoint *url.URL) {
				_ = reg.AddService("svc", "a")

				_, err := testWaitFetch(t, testDefaultClient(t), "invalid", endpoint)
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name: "specific service - error",
			exec: func(t *testing.T, reg *readyz.Registry, endpoint *url.URL) {
				a := reg.AddService("svc", "a")
				a.ReportReason(readyz.Unhealthy, "testing error")

				code, err := testWaitFetch(t,
					testDefaultClient(t),
					"a",
					endpoint,
				)
				require.Error(t, err) // no message to examine
				require.Equal(t, 503, code)
			},
		},
		{
			name: "multiple services",
			exec: func(t *testing.T, reg *readyz.Registry, endpoint *url.URL) {
				a := reg.AddService("svc", "a")
				b := reg.AddService("svc", "b")
				a.Report(readyz.Healthy)

				_, err := testWaitFetch(t, testDefaultClient(t), "a", endpoint)
				require.NoError(t, err)

				// "b" should still be waiting
				_, err = testWaitFetch(t, testDefaultClient(t), "b", endpoint)
				require.ErrorContains(t, err, "Client.Timeout")

				// Overall should also still wait
				_, err = testWaitFetch(t, testDefaultClient(t), "", endpoint)
				require.ErrorContains(t, err, "Client.Timeout")

				b.ReportReason(readyz.Unhealthy, "invalid")

				// It should now show an error
				code, err := testWaitFetch(t, testDefaultClient(t), "", endpoint)
				require.Error(t, err) // error has no message
				require.Equal(t, 503, code)

				// Once "b" is heathy, both specific and overall status should report healthy.
				b.Report(readyz.Healthy)

				code, err = testWaitFetch(t, testDefaultClient(t), "b", endpoint)
				require.NoError(t, err)
				require.Equal(t, 200, code)

				code, err = testWaitFetch(t, testDefaultClient(t), "", endpoint)
				require.NoError(t, err)
				require.Equal(t, 200, code)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reg := readyz.NewRegistry(readyz.WithClock(clock))

			srv := httptest.NewServer(readyz.HTTPWaitHandler(reg))
			srv.URL = srv.URL + "/wait"
			t.Cleanup(srv.Close)

			u, err := url.Parse(srv.URL)
			require.NoError(t, err)

			tt.exec(t, reg, u)
		})
	}
}

func TestWaitAPIOngoingWaiter(t *testing.T) {
	// TODO: Evaluate reenabling parallel execution if this branch is updated to
	// go1.25. Otherwise, parallel execution triggers race conditions in go1.24
	// synctest bubbles and causes spurious failures.
	// t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		reg := readyz.NewRegistry()

		requests := make(chan requestNotification)
		handler := readyz.HTTPWaitHandler(reg)

		u, err := url.Parse("http://invalid/wait")
		require.NoError(t, err)

		a := reg.AddService("svc", "a")

		// Start waiting
		ch := make(chan error)
		go func() {
			code, err := testWaitFetch(t, createInMemoryWaitClient(handler, requests), "", u)
			if err != nil && code != 200 {
				err = fmt.Errorf("error: %d", code)
			}

			ch <- err
		}()

		req := <-requests
		require.False(t, req.isComplete())

		a.Report(readyz.Healthy)
		<-req.done

		select {
		case res := <-ch:
			require.NoError(t, res, "overall status should report healthy")
		case <-time.After(100 * time.Millisecond):
			require.Fail(t, "timed out waiting for waiter to receive ready status")
		}
	})
}
