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

type inMemoryTransport struct {
	handler http.Handler
}

func (c *inMemoryTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	done := make(chan struct{})
	rr := httptest.NewRecorder()

	// We can't just naively return the result of ServeHTTP since it doesn't
	// return errors when the context is cancelled. To fix that, wrap it in
	// another goroutine and manually return an error if the context is
	// cancelled.
	go func() {
		c.handler.ServeHTTP(rr, r)
		close(done)
	}()

	select {
	case <-r.Context().Done():
		<-done
		return nil, r.Context().Err()
	case <-done:
		return rr.Result(), nil
	}
}

func createInMemoryWaitClient(handler http.Handler) *http.Client {
	return &http.Client{
		Transport: &inMemoryTransport{
			handler: handler,
		},
	}
}

func TestWaitTimeoutExceeded(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		reg := readyz.NewRegistry()
		_ = reg.AddService("svc", "a")

		handler := readyz.HTTPWaitHandler(reg)

		ch := make(chan error)
		go func() {
			ch <- onWaitCommand(t.Context(), &cli.WaitCommand{
				DiagAddr: "http://fake",
				Service:  "a",
				Timeout:  time.Millisecond * 250,
				Client:   createInMemoryWaitClient(handler),
			})
		}()

		synctest.Wait()

		time.Sleep(time.Millisecond * 500)

		synctest.Wait()

		select {
		case res := <-ch:
			require.ErrorContains(t, res, "context deadline exceeded")
		case <-time.After(250 * time.Millisecond):
			require.Fail(t, "wait failed to honor timeout")
		}
	})
}

func TestWaitSuccess(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		reg := readyz.NewRegistry()
		a := reg.AddService("svc", "a")

		handler := readyz.HTTPWaitHandler(reg)

		ch := make(chan error)
		go func() {
			ch <- onWaitCommand(t.Context(), &cli.WaitCommand{
				DiagAddr: "http://fake",
				Service:  "a",
				Timeout:  time.Second * 2,
				Client:   createInMemoryWaitClient(handler),
			})
		}()

		synctest.Wait()

		a.Report(readyz.Healthy)

		synctest.Wait()

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
		a := reg.AddService("svc", "a")

		handler := readyz.HTTPWaitHandler(reg)

		ch := make(chan error)
		go func() {
			ch <- onWaitCommand(t.Context(), &cli.WaitCommand{
				DiagAddr: "http://fake",
				Service:  "a",
				Client:   createInMemoryWaitClient(handler),

				// More generous timeout since this will depend on exponential
				// backoff (with configured worst case of 2 seconds)
				Timeout: time.Second * 5,
			})
		}()

		synctest.Wait()

		// Initially report unhealthy. This internally triggers a response and the
		// HTTP endpoint will return the unhealthy status instead of waiting. The
		// CLI should retry until the endpoint reports healthy.
		a.ReportReason(readyz.Unhealthy, "oops")
		time.Sleep(time.Millisecond * 200)

		synctest.Wait()

		a.Report(readyz.Healthy)

		synctest.Wait()

		select {
		case res := <-ch:
			require.NoError(t, res, "must report ready")
		case <-time.After(6 * time.Second):
			require.Fail(t, "test timed out and failed to honor configured timeout")
		}
	})
}
