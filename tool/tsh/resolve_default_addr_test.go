/*
Copyright 2015-2021 Gravitational, Inc.

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

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

var testLog = log.WithField(trace.Component, "test")

func newWaitForeverHandler() (http.Handler, chan interface{}) {
	doneChannel := make(chan interface{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testLog.Debug("Waiting forever...")
		<-doneChannel
	})

	return handler, doneChannel
}

func newRespondingHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testLog.Debug("Responding")
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "Hello, world")
	})
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

func Test_ResolveDefaultAddr(t *testing.T) {
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
		svr := httptest.NewTLSServer(handler)
		defer svr.Close()
		servers[i] = svr
	}

	// NB: We need to defer this channel close  such that it happens *before*
	// the httpstest server shutdowns, or the blocking requests will never
	// finish and we will deadlock.
	defer close(doneCh)

	ports := mustGetCandidatePorts(servers)
	expectedAddr := fmt.Sprintf("127.0.0.1:%d", ports[magicServerIndex])

	// When I attempt to resolve a default address
	addr, err := pickDefaultAddr(context.Background(), true, "127.0.0.1", ports, nil)

	// Expect that the "magic" server is selected
	require.NoError(t, err)
	require.Equal(t, expectedAddr, addr)
}

func Test_ResolveDefaultAddr_NoCandidates(t *testing.T) {
	_, err := pickDefaultAddr(context.Background(), true, "127.0.0.1", []int{}, nil)
	require.Error(t, err)
}

// Test that the resolver doesn't crash on a single candidate. This situation
// should not arise in production; it would be better to just assume that the
// single candidate is correct, as you have no other choice.
func Test_ResolveDefaultAddr_SingleCandidate(t *testing.T) {
	// Given a single candidate
	respondingHandler := newRespondingHandler()

	servers := make([]*httptest.Server, 1)
	for i := 0; i < len(servers); i++ {
		svr := httptest.NewTLSServer(respondingHandler)
		defer svr.Close()
		servers[i] = svr
	}

	ports := mustGetCandidatePorts(servers)
	expectedAddr := fmt.Sprintf("127.0.0.1:%d", ports[0])

	// When I attempt to resolve a default address
	addr, err := pickDefaultAddr(context.Background(), true, "127.0.0.1", ports, nil)

	// Expect that the only server is selected
	require.NoError(t, err)
	require.Equal(t, expectedAddr, addr)
}

func Test_ResolveDefaultAddr_Timeout(t *testing.T) {
	// Given a set of candidate servers that will all block forever...

	blockingHandler, doneCh := newWaitForeverHandler()

	servers := make([]*httptest.Server, 5)
	for i := 0; i < 5; i++ {
		svr := httptest.NewTLSServer(blockingHandler)
		defer svr.Close()
		servers[i] = svr
	}

	// NB: We need to defer this channel close  such that it happens *before*
	// the httpstest server shutdowns, or the blocking requests will never
	// finish and we will deadlock.
	defer close(doneCh)

	ports := mustGetCandidatePorts(servers)

	// When I attempt to resolve the default address with a finite timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
	defer cancel()
	_, err := pickDefaultAddr(ctx, true, "127.0.0.1", ports, nil)

	// Expect that the resolution will fail with `Deadline Exceeded` due to
	// the call timing out.
	require.Equal(t, context.DeadlineExceeded, err)
}

func Test_ResolveDefaultAddr_TimeoutBeforeAllRacersLaunched(t *testing.T) {
	// Given a large set of candidate servers that will all block forever...

	blockingHandler, doneCh := newWaitForeverHandler()

	servers := make([]*httptest.Server, 1000)
	for i := 0; i < len(servers); i++ {
		svr := httptest.NewTLSServer(blockingHandler)
		defer svr.Close()
		servers[i] = svr
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
	_, err := pickDefaultAddr(ctx, true, "127.0.0.1", ports, nil)

	// Expect that the resolution will fail with `Deadline Exceeded` due to
	// the call timing out.
	require.Equal(t, context.DeadlineExceeded, err)
}
