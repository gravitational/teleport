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
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

var randomLocalAddr = *MustParseAddr("127.0.0.1:0")

func TestSingleBackendLB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "backend 1")
	}))
	defer backend1.Close()

	lb, err := NewLoadBalancer(ctx, randomLocalAddr, urlToNetAddr(backend1.URL))
	require.NoError(t, err)
	err = lb.Listen()
	require.NoError(t, err)
	go lb.Serve()
	defer lb.Close()

	out, err := Roundtrip(lb.Addr().String())
	require.NoError(t, err)
	require.Equal(t, "backend 1", out)
}

func TestTwoBackendsLB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "backend 1")
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "backend 2")
	}))
	defer backend2.Close()

	backend1Addr, backend2Addr := urlToNetAddr(backend1.URL), urlToNetAddr(backend2.URL)

	lb, err := NewLoadBalancer(ctx, randomLocalAddr)
	require.NoError(t, err)
	err = lb.Listen()
	require.NoError(t, err)
	go lb.Serve()
	defer lb.Close()

	// no endpoints
	_, err = Roundtrip(lb.Addr().String())
	require.Error(t, err)

	lb.AddBackend(backend1Addr)
	out, err := Roundtrip(lb.Addr().String())
	require.NoError(t, err)
	require.Equal(t, "backend 1", out)

	lb.AddBackend(backend2Addr)
	out, err = Roundtrip(lb.Addr().String())
	require.NoError(t, err)
	require.Equal(t, "backend 2", out)
}

func TestOneFailingBackend(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "backend 1")
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "backend 2")
	}))
	backend2.Close()

	backend1Addr, backend2Addr := urlToNetAddr(backend1.URL), urlToNetAddr(backend2.URL)

	lb, err := NewLoadBalancer(ctx, randomLocalAddr)
	require.NoError(t, err)
	err = lb.Listen()
	require.NoError(t, err)
	go lb.Serve()
	defer lb.Close()

	lb.AddBackend(backend1Addr)
	lb.AddBackend(backend2Addr)

	out, err := Roundtrip(lb.Addr().String())
	require.NoError(t, err)
	require.Equal(t, "backend 1", out)

	_, err = Roundtrip(lb.Addr().String())
	require.Error(t, err)

	out, err = Roundtrip(lb.Addr().String())
	require.NoError(t, err)
	require.Equal(t, "backend 1", out)
}

func TestClose(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "backend 1")
	}))
	defer backend1.Close()

	lb, err := NewLoadBalancer(ctx, randomLocalAddr, urlToNetAddr(backend1.URL))
	require.NoError(t, err)
	err = lb.Listen()
	require.NoError(t, err)
	go lb.Serve()
	defer lb.Close()

	out, err := Roundtrip(lb.Addr().String())
	require.NoError(t, err)
	require.Equal(t, "backend 1", out)

	lb.Close()
	// second close works
	lb.Close()

	lb.Wait()

	// requests are failing
	out, err = Roundtrip(lb.Addr().String())
	require.Error(t, err, "output: %s, err: %v", out, err)
}

func TestDropConnections(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "backend 1")
	}))
	defer backend1.Close()

	backendAddr := urlToNetAddr(backend1.URL)
	lb, err := NewLoadBalancer(ctx, randomLocalAddr, backendAddr)
	require.NoError(t, err)
	err = lb.Listen()
	require.NoError(t, err)
	go lb.Serve()
	defer lb.Close()

	conn, err := net.Dial("tcp", lb.Addr().String())
	require.NoError(t, err)
	defer conn.Close()

	out, err := RoundtripWithConn(conn)
	require.NoError(t, err)
	require.Equal(t, "backend 1", out)

	// to make sure multiple requests work on the same wire
	out, err = RoundtripWithConn(conn)
	require.NoError(t, err)
	require.Equal(t, "backend 1", out)

	// removing backend results in dropped connection to this backend
	err = lb.RemoveBackend(backendAddr)
	require.NoError(t, err)
	_, err = RoundtripWithConn(conn)
	require.Error(t, err)
}

func urlToNetAddr(u string) NetAddr {
	parsed, err := url.Parse(u)
	if err != nil {
		panic(err)
	}
	return *MustParseAddr(parsed.Host)
}
