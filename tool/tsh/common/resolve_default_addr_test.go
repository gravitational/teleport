/*
Copyright 2021 Gravitational, Inc.

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

package common

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integration/helpers"
)

var testLog = log.WithField(trace.Component, "test")

func newWaitForeverHandler() (http.Handler, chan struct{}) {
	doneChannel := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testLog.Debug("Waiting forever...")
		<-doneChannel
	})

	return handler, doneChannel
}

func newRespondingHandlerWithStatus(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testLog.Debug("Responding")
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		io.WriteString(w, "Hello, world")
	})
}

func newRespondingHandler() http.Handler {
	return newRespondingHandlerWithStatus(http.StatusOK)
}

func mustGetCandidatePorts(servers []*httptest.Server) []int {
	result := make([]int, len(servers))
	for i, svr := range servers {
		u, err := url.Parse(svr.URL)
		if err != nil {
			panic(err)
		}

		p, err := strconv.Atoi(u.Port())
		if err != nil {
			panic(err)
		}

		result[i] = p
	}
	return result
}

type testServerOption func(*httptest.Server)

func makeTestServer(t *testing.T, h http.Handler, opts ...testServerOption) *httptest.Server {
	svr := httptest.NewUnstartedServer(h)
	for _, opt := range opts {
		opt(svr)
	}
	svr.StartTLS()
	t.Cleanup(func() { svr.Close() })
	return svr
}

func TestResolveDefaultAddr(t *testing.T) {
	t.Parallel()

	// Given a set of candidate servers, with one "magic" server configured to
	// respond, and all the others configured to wait forever
	const magicServerIndex = 3

	blockingHandler, doneCh := newWaitForeverHandler()
	respondingHandler := newRespondingHandler()

	servers := make([]*httptest.Server, 5)
	for i := 0; i < 5; i++ {
		handler := blockingHandler
		if i == magicServerIndex {
			handler = respondingHandler
		}
		servers[i] = makeTestServer(t, handler)
	}

	// NB: We need to defer this channel close  such that it happens *before*
	// the httpstest server shutdowns, or the blocking requests will never
	// finish and we will deadlock.
	defer close(doneCh)

	ports := mustGetCandidatePorts(servers)
	expectedAddr := fmt.Sprintf("127.0.0.1:%d", ports[magicServerIndex])

	// When I attempt to resolve a default address
	addr, err := pickDefaultAddr(context.Background(), true, "127.0.0.1", ports)

	// Expect that the "magic" server is selected
	require.NoError(t, err)
	require.Equal(t, expectedAddr, addr)
}

func TestResolveDefaultAddrNoCandidates(t *testing.T) {
	t.Parallel()
	_, err := pickDefaultAddr(context.Background(), true, "127.0.0.1", []int{})
	require.Error(t, err)
}

// Test that the resolver doesn't crash on a single candidate. This situation
// should not arise in production; it would be better to just assume that the
// single candidate is correct, as you have no other choice.
func TestResolveDefaultAddrSingleCandidate(t *testing.T) {
	t.Parallel()
	// Given a single candidate
	respondingHandler := newRespondingHandler()

	servers := make([]*httptest.Server, 1)
	for i := 0; i < len(servers); i++ {
		servers[i] = makeTestServer(t, respondingHandler)
	}

	ports := mustGetCandidatePorts(servers)
	expectedAddr := fmt.Sprintf("127.0.0.1:%d", ports[0])

	// When I attempt to resolve a default address
	addr, err := pickDefaultAddr(context.Background(), true, "127.0.0.1", ports)

	// Expect that the only server is selected
	require.NoError(t, err)
	require.Equal(t, expectedAddr, addr)
}

func TestResolveDefaultAddrTimeout(t *testing.T) {
	t.Parallel()
	// Given a set of candidate servers that will all block forever...

	blockingHandler, doneCh := newWaitForeverHandler()

	servers := make([]*httptest.Server, 5)
	for i := 0; i < 5; i++ {
		servers[i] = makeTestServer(t, blockingHandler)
	}

	// NB: We need to defer this channel close  such that it happens *before*
	// the httpstest server shutdowns, or the blocking requests will never
	// finish and we will deadlock.
	defer close(doneCh)

	ports := mustGetCandidatePorts(servers)

	// When I attempt to resolve the default address with a finite timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
	defer cancel()
	_, err := pickDefaultAddr(ctx, true, "127.0.0.1", ports)

	// Expect that the resolution will fail with `Deadline Exceeded` due to
	// the call timing out.
	require.Equal(t, context.DeadlineExceeded, err)
}

func TestResolveNonOKResponseIsAnError(t *testing.T) {
	t.Parallel()

	// Given a single candidate server configured to respond with a non-OK status
	// code
	servers := []*httptest.Server{
		makeTestServer(t, newRespondingHandlerWithStatus(http.StatusTeapot)),
	}
	ports := mustGetCandidatePorts(servers)

	// When I attempt to resolve a default address
	_, err := pickDefaultAddr(context.Background(), true, "127.0.0.1", ports)

	// Expect that the resolution fails because the server responded with a non-OK
	// response
	require.Error(t, err)
}

func TestResolveUndeliveredBodyDoesNotBlockForever(t *testing.T) {
	t.Parallel()

	// Given a single candidate server configured to respond with a non-OK status
	// code and a looooong, streaming body that never arrives
	doneChannel := make(chan struct{})
	defer close(doneChannel)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			testLog.Error("ResponseWriter must also be a Flusher, or the test is invalid")
			t.Fatal()
		}

		testLog.Debugf("Writing response header to %T", w)
		w.Header().Set("Content-Length", "1048576")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusTeapot)

		w.Write([]byte("I'm a little teapot, short and stout."))
		f.Flush()

		testLog.Debug("Waiting forever instead of sending response body")
		<-doneChannel

		testLog.Debug("Exiting handler")
	})

	servers := []*httptest.Server{makeTestServer(t, handler)}
	ports := mustGetCandidatePorts(servers)

	// When I attempt to resolve a default address
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err := pickDefaultAddr(ctx, true, "127.0.0.1", ports)

	// Expect that the resolution fails with a context timeout
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestResolveDefaultAddrTimeoutBeforeAllRacersLaunched(t *testing.T) {
	// Given a large set of candidate servers that will all block forever...

	blockingHandler, doneCh := newWaitForeverHandler()

	servers := make([]*httptest.Server, 1000)
	for i := 0; i < len(servers); i++ {
		servers[i] = makeTestServer(t, blockingHandler)
	}

	// NB: We need to defer this channel close  such that it happens *before*
	// the httpstest server shutdowns, or the blocking requests will never
	// finish and we will deadlock.
	defer close(doneCh)

	ports := mustGetCandidatePorts(servers)

	// When I attempt to resolve the default address with a timeout *smaller* than
	// would allow for all of the racers to have been launched...
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err := pickDefaultAddr(ctx, true, "127.0.0.1", ports)

	// Expect that the resolution will fail with `Deadline Exceeded` due to
	// the call timing out.
	require.Equal(t, context.DeadlineExceeded, err)
}

func TestResolveDefaultAddrHTTPProxy(t *testing.T) {
	proxyHandler := &helpers.ProxyHandler{}
	proxyServer := httptest.NewServer(proxyHandler)
	t.Cleanup(proxyServer.Close)

	// Go won't proxy to localhost, so use this address instead.
	localIP, err := helpers.GetLocalIP()
	require.NoError(t, err)

	var serverAddr net.Addr
	respondingHandler := newRespondingHandler()
	server := makeTestServer(t, respondingHandler, func(srv *httptest.Server) {
		// Replace the test server's address.
		l, err := net.Listen("tcp", localIP+":0")
		require.NoError(t, err)
		require.NoError(t, srv.Listener.Close())
		srv.Listener = l
		serverAddr = l.Addr()
	})

	ports := mustGetCandidatePorts([]*httptest.Server{server})

	// Given an http proxy address...
	t.Setenv("HTTPS_PROXY", proxyServer.URL)
	// When I attempt to resove an address...
	addr, err := pickDefaultAddr(context.Background(), true, localIP, ports)
	// Expect that pickDefaultAddr uses the http proxy.
	require.NoError(t, err)
	require.Equal(t, serverAddr.String(), addr)
	require.Equal(t, 1, proxyHandler.Count())
}
