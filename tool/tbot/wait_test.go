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

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/cli"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

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

func TestWaitTimeoutExceeded(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		reg := readyz.NewRegistry()
		_ = reg.AddService("svc", "a")

		handler := readyz.HTTPWaitHandler(reg)
		requests := make(chan requestNotification, 20)
		client := createInMemoryWaitClient(handler, requests)

		ch := make(chan error)
		go func() {
			ch <- onWaitCommand(t.Context(), &cli.WaitCommand{
				DiagAddr: "http://fake",
				Service:  "a",
				Timeout:  time.Millisecond * 250,
				Client:   client,
			})
		}()

		// Wait for an initial request. This request will delay as no status has
		// been reported.
		req := <-requests
		require.False(t, req.isComplete())

		select {
		case res := <-ch:
			require.ErrorContains(t, res, "context deadline exceeded")
		case <-time.After(500 * time.Millisecond):
			require.Fail(t, "wait failed to honor timeout")
		}
	})
}

func TestWaitSuccess(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		reg := readyz.NewRegistry()
		b := reg.AddService("svc", "b")

		handler := readyz.HTTPWaitHandler(reg)
		requests := make(chan requestNotification, 20)
		client := createInMemoryWaitClient(handler, requests)

		ch := make(chan error)
		go func() {
			ch <- onWaitCommand(t.Context(), &cli.WaitCommand{
				DiagAddr: "http://fake",
				Service:  "b",
				Timeout:  time.Second * 2,
				Client:   client,
			})
		}()

		// Wait for an initial request. This request will delay as no status has
		// been reported.
		req := <-requests
		require.False(t, req.isComplete())

		// Once the service reports healthy, the request should complete
		b.Report(readyz.Healthy)
		<-req.done

		select {
		case res := <-ch:
			require.NoError(t, res, "must report ready")
		case <-time.After(3 * time.Second):
			require.Fail(t, "test timed out and failed to honor configured timeout")
		}
	})
}

func TestWaitEventualSuccess(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		reg := readyz.NewRegistry()
		c := reg.AddService("svc", "c")

		handler := readyz.HTTPWaitHandler(reg)

		// We don't want to strictly predict the precise number of requests
		// `onWaitCommand` makes, so use an overly large buffer to ensure it can
		// never plausibly block. The retry driver increases exponentially from
		// 250ms so 5000ms/250ms=20 maximum possible retries - not that it
		// should ever reach that in a synctest bubble, especially in go1.25+.
		requests := make(chan requestNotification, 20)

		client := createInMemoryWaitClient(handler, requests)

		ch := make(chan error)
		go func() {
			ch <- onWaitCommand(t.Context(), &cli.WaitCommand{
				DiagAddr: "http://fake",
				Service:  "c",
				Client:   client,

				// More generous timeout since this will depend on exponential
				// backoff (with configured worst case of 2 seconds)
				Timeout: time.Second * 5,
			})
		}()

		// Wait for an initial request. This request will delay as no status has
		// been reported.
		req := <-requests
		require.False(t, req.isComplete())

		// Initially report unhealthy. This internally triggers a response and
		// the HTTP endpoint will return the unhealthy status instead of
		// waiting. The CLI should retry until the endpoint reports healthy.
		c.ReportReason(readyz.Unhealthy, "oops")

		// Allow that request to return with the unhealthy status.
		select {
		case <-req.done:
		case <-time.After(1 * time.Second):
			require.Fail(t, "initial wait request did not return as expected")
		}

		// The nonblocking select below is probably safe, but yield here out of
		// caution.
		synctest.Wait()

		// Make sure `onWaitCommand` still hasn't returned.
		select {
		case res := <-ch:
			require.Fail(t, "received unexpected ready status: %+v", res)
		default:
			// Expected, should still be waiting due to unhealthy status.
		}

		// Wait for at least one additional request to begin. synctest will
		// advance the synthetic clock automatically to ensure the internal
		// timers trigger.
		<-requests
		c.Report(readyz.Healthy)

		// Wait for a final response.
		select {
		case res := <-ch:
			require.NoError(t, res, "must report ready")
		case <-time.After(6 * time.Second):
			// Modest wait beyond the maximum timeout (still synthetic)
			require.Fail(t, "test timed out and failed to honor configured timeout")
		}
	})
}
