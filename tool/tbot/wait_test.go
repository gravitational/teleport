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
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tbot/cli"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

func testDefaultClient(t *testing.T) *http.Client {
	t.Helper()

	client, err := defaults.HTTPClient()
	require.NoError(t, err)

	client.Timeout = time.Second

	return client
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

func testWaitFetch(t *testing.T, client *http.Client, service string, endpoint *url.URL) error {
	t.Helper()

	if service != "" {
		endpoint = endpoint.JoinPath(service)
	}

	return waitFetch(t.Context(), slog.Default(), client, service, endpoint)
}

func TestWaitTimeoutExceeded(t *testing.T) {
	t.Parallel()

	reg := readyz.NewRegistry()
	_ = reg.AddService("svc", "a")

	srv := httptest.NewServer(readyz.HTTPWaitHandler(reg))
	baseURL := srv.URL
	srv.URL = baseURL + "/wait"
	t.Cleanup(srv.Close)

	ch := make(chan error)
	go func() {
		ch <- onWaitCommand(t.Context(), &cli.WaitCommand{
			DiagAddr: baseURL,
			Service:  "a",
			Timeout:  time.Millisecond * 250,
		})
	}()

	// Wait with a decent buffer.
	time.Sleep(time.Millisecond * 500)

	select {
	case res := <-ch:
		require.ErrorContains(t, res, "context deadline exceeded")
	case <-time.After(250 * time.Millisecond):
		require.Fail(t, "wait failed to honor timeout")
	}
}

func TestWaitSuccess(t *testing.T) {
	t.Parallel()

	reg := readyz.NewRegistry()
	a := reg.AddService("svc", "a")

	srv := httptest.NewServer(readyz.HTTPWaitHandler(reg))
	baseURL := srv.URL
	srv.URL = baseURL + "/wait"
	t.Cleanup(srv.Close)

	ch := make(chan error)
	go func() {
		ch <- onWaitCommand(t.Context(), &cli.WaitCommand{
			DiagAddr: baseURL,
			Service:  "a",
			Timeout:  time.Second * 2,
		})
	}()

	a.Report(readyz.Healthy)

	select {
	case res := <-ch:
		require.NoError(t, res, "must report ready")
	case <-time.After(3 * time.Second):
		require.Fail(t, "test timed out and failed to honor configured timeout")
	}
}

func TestWaitEventualSuccess(t *testing.T) {
	t.Parallel()

	reg := readyz.NewRegistry()
	a := reg.AddService("svc", "a")

	srv := httptest.NewServer(readyz.HTTPWaitHandler(reg))
	baseURL := srv.URL
	srv.URL = baseURL + "/wait"
	t.Cleanup(srv.Close)

	ch := make(chan error)
	go func() {
		ch <- onWaitCommand(t.Context(), &cli.WaitCommand{
			DiagAddr: baseURL,
			Service:  "a",

			// More generous timeout since this will depend on exponential
			// backoff (with configured worst case of 2 seconds)
			Timeout:  time.Second * 5,
		})
	}()

	// Initially report unhealthy. This internally triggers a response and the
	// HTTP endpoint will return the unhealthy status instead of waiting. The
	// CLI should retry until the endpoint reports healthy.
	a.ReportReason(readyz.Unhealthy, "oops")
	time.Sleep(time.Millisecond * 200)
	a.Report(readyz.Healthy)

	select {
	case res := <-ch:
		require.NoError(t, res, "must report ready")
	case <-time.After(6 * time.Second):
		require.Fail(t, "test timed out and failed to honor configured timeout")
	}
}
