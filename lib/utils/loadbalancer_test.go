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

package utils

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

var randomLocalAddr = *MustParseAddr("127.0.0.1:0")

func TestSingleBackendLB(t *testing.T) {
	t.Parallel()

	backends := startBackends(t, 1)

	lb, err := NewLoadBalancer(t.Context(), randomLocalAddr, backends[0])
	require.NoError(t, err)
	err = lb.Listen()
	require.NoError(t, err)
	go lb.Serve()
	defer lb.Close()

	out, err := httpGet(lb.Addr().String())
	require.NoError(t, err)
	require.Equal(t, "backend 1", out)
}

func TestTwoBackendsLB(t *testing.T) {
	t.Parallel()

	backends := startBackends(t, 2)

	lb, err := NewLoadBalancer(t.Context(), randomLocalAddr, backends...)
	require.NoError(t, err)
	err = lb.Listen()
	require.NoError(t, err)
	go lb.Serve()
	defer lb.Close()

	lb.AddBackend(backends[0])
	out, err := httpGet(lb.Addr().String())
	require.NoError(t, err)
	require.Equal(t, "backend 1", out)

	lb.AddBackend(backends[1])
	out, err = httpGet(lb.Addr().String())
	require.NoError(t, err)
	require.Equal(t, "backend 2", out)
}

func TestDropConnections(t *testing.T) {
	t.Parallel()

	backends := startBackends(t, 1)

	lb, err := NewLoadBalancer(t.Context(), randomLocalAddr, backends[0])
	require.NoError(t, err)
	err = lb.Listen()
	require.NoError(t, err)
	go lb.Serve()
	defer lb.Close()

	lbURL := "http://" + lb.Addr().String()
	client := new(http.Client)

	// Make sure we can make multiple requests.
	// We reuse the same client here to exercise net/http's connection reuse.
	resp, err := client.Get(lbURL)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "backend 1", string(body))

	resp, err = client.Get(lbURL)
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "backend 1", string(body))

	// removing backend results in error
	err = lb.RemoveBackend(backends[0])
	require.NoError(t, err)
	resp, err = client.Get(lbURL)
	if err == nil {
		resp.Body.Close()
	}
	require.Error(t, err)
}

func startBackends(t *testing.T, count int) []NetAddr {
	addrs := make([]NetAddr, 0, count)
	for i := range count {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "backend %d", i+1)
		}))
		t.Cleanup(srv.Close)

		addrs = append(addrs, urlToNetAddr(srv.URL))
	}
	return addrs
}

func urlToNetAddr(u string) NetAddr {
	parsed, err := url.Parse(u)
	if err != nil {
		panic(err)
	}
	return *MustParseAddr(parsed.Host)
}

func httpGet(addr string) (string, error) {
	// Use a dedicated client instead of http.DeafultClient so that
	// we don't share state across test cases
	client := new(http.Client)
	defer client.CloseIdleConnections()

	resp, err := client.Get("http://" + addr)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(out), nil
}
