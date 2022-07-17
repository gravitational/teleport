/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

package utils

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func setUpServer(t *testing.T) *httptest.Server {
	// Set up an HTTP server which listens and responds to queries.
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		// GET /slow?delay=10ms sleeps for a given delay, then returns word "slow"
		case "/slow":
			delay, err := time.ParseDuration(r.URL.Query().Get("delay"))
			require.NoError(t, err)
			time.Sleep(delay)
			fmt.Fprintf(w, "slow")

		// GET /ping returns "pong"
		case "/ping":
			fmt.Fprintf(w, "pong")
		}
	}))
}

func TestSlowOperation(t *testing.T) {
	t.Parallel()

	server := setUpServer(t)
	defer server.Close()

	client := newClient(time.Millisecond * 5)
	resp, err := client.Get(server.URL + "/slow?delay=20ms")
	if err == nil {
		resp.Body.Close()
	}
	// must fail with I/O timeout
	require.NotNil(t, err)
	require.ErrorContains(t, err, "i/o timeout")
}

func TestNormalOperation(t *testing.T) {
	t.Parallel()

	server := setUpServer(t)
	defer server.Close()

	client := newClient(time.Millisecond * 100)
	resp, err := client.Get(server.URL + "/ping")
	require.NoError(t, err)
	require.Equal(t, bodyText(resp), "pong")
}

// newClient helper returns HTTP client configured to use a connection
// which drops itself after N idle time
func newClient(timeout time.Duration) *http.Client {
	var t http.Transport
	t.DialContext = func(ctx context.Context, network string, addr string) (net.Conn, error) {
		var d net.Dialer
		conn, err := d.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		return ObeyIdleTimeout(conn, timeout, "test"), nil
	}
	return &http.Client{Transport: &t}
}

// bodyText helper returns a body string from an http response
func bodyText(resp *http.Response) string {
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return string(bytes)
}
