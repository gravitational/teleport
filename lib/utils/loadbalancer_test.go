/*
Copyright 2017 Gravitational, Inc.

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
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/utilsaddr"
)

var randomLocalAddr = *utilsaddr.MustParseAddr("127.0.0.1:0")

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
	require.Equal(t, out, "backend 1")
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
	require.NotNil(t, err)

	lb.AddBackend(backend1Addr)
	out, err := Roundtrip(lb.Addr().String())
	require.NoError(t, err)
	require.Equal(t, out, "backend 1")

	lb.AddBackend(backend2Addr)
	out, err = Roundtrip(lb.Addr().String())
	require.NoError(t, err)
	require.Equal(t, out, "backend 2")
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
	require.Equal(t, out, "backend 1")

	_, err = Roundtrip(lb.Addr().String())
	require.NotNil(t, err)

	out, err = Roundtrip(lb.Addr().String())
	require.NoError(t, err)
	require.Equal(t, out, "backend 1")
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
	require.Equal(t, out, "backend 1")

	lb.Close()
	// second close works
	lb.Close()

	lb.Wait()

	// requests are failing
	out, err = Roundtrip(lb.Addr().String())
	require.NotNilf(t, err, fmt.Sprintf("output: %s, err: %v", out, err))
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
	require.Equal(t, out, "backend 1")

	// to make sure multiple requests work on the same wire
	out, err = RoundtripWithConn(conn)
	require.NoError(t, err)
	require.Equal(t, out, "backend 1")

	// removing backend results in dropped connection to this backend
	err = lb.RemoveBackend(backendAddr)
	require.NoError(t, err)
	_, err = RoundtripWithConn(conn)
	require.NotNil(t, err)
}

func urlToNetAddr(u string) utilsaddr.NetAddr {
	parsed, err := url.Parse(u)
	if err != nil {
		panic(err)
	}
	return *utilsaddr.MustParseAddr(parsed.Host)
}
