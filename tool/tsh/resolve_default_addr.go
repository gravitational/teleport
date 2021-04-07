/*
Copyright 2016-2022 Gravitational, Inc.

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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gravitational/trace"
)

type raceResult struct {
	addr string
	err  error
}

// raceRequest drives an HHTP request to completion and posts the results back
// to the supplied channel.
func raceRequest(ctx context.Context, cli *http.Client, addr string, results chan<- raceResult) {
	target := fmt.Sprintf("https://%s/", addr)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)

	if err == nil {
		var rsp *http.Response
		rsp, err = cli.Do(request)
		if err == nil {
			rsp.Body.Close()
		}
	}

	results <- raceResult{addr: addr, err: err}
}

// startRacer starts the asynchronous execution of a single request, and keeps
// all the associated bookeeping up to date.
func startRacer(ctx context.Context, cli *http.Client, host string, candidates []int, results chan<- raceResult) []int {
	port, tail := candidates[0], candidates[1:]
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	log.Debugf("Trying %s...", addr)
	go raceRequest(ctx, cli, addr, results)
	return tail
}

// pickDefaultAddr implements proxy selection via an RFC-8305 "Happy Eyeballs"
// -like algorithm. In practical terms, that means that it:
//  1. issues a GET request against multiple potential proxy ports,
//  2. races the requests against one another, and finally
//  3. selects the first to respond as the canonical proxy
func pickDefaultAddr(ctx context.Context, insecure bool, host string, ports []int, rootCAs *x509.CertPool) (string, error) {
	log.Debugf("Resolving default proxy port (insecure: %v)", insecure)

	if len(ports) == 0 {
		return "", trace.BadParameter("port list may not be empty")
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:            rootCAs,
				InsecureSkipVerify: insecure,
			},
		},
	}

	// Define an inner context that we'll give to the requests to cancel
	// them regardless of if we exit successfully or we are killed from above.
	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Make the channel for the race results big enough so we're guaranteed that a
	// channel write will never block. Once we have a hit we will stop reading the
	// channel, and we don't want to leak a bunch of goroutines while they're
	// blocked on writing to a full reply channel
	candidates := ports
	results := make(chan raceResult, len(candidates))

	// Start the first attempt racing
	outstandingRacers := len(candidates)
	candidates = startRacer(raceCtx, httpClient, host, candidates, results)

	// Start a ticker that will kick off the subsequent racers after a small
	// interval. We don't want to start them all at once, as we may swamp the
	// network and give away advantage we have from doing this concurrently in
	// the first place. RFC8305 recommends an interval of between 100ms and 2s,
	// with 250ms being a "sensible default"
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// We've timed out or been cancelled. Bail out ASAP. Remember that returning
			// will implicitly cancel all of the already-started racers.
			return "", ctx.Err()

		case <-ticker.C:
			// It's time to kick off a new racer
			if len(candidates) > 0 {
				candidates = startRacer(raceCtx, httpClient, host, candidates, results)
			}

		case r := <-results:
			outstandingRacers--

			// if the request succeeded, it wins the race
			if r.err == nil {
				// Accept the winner as having the canonical web proxy address.
				//
				// Note that returning will implicitly cancel the inner context, telling
				// any outstanding racers that there is no point trying any more and they
				// should exit.
				log.Debugf("Address %s succeeded. Selected as canonical proxy address", r.addr)
				return r.addr, nil
			}

			// the ping failed. This could be for any number of reasons. All we
			// really care about is whether _all_ of the ping attempts have
			// failed and it's time to return with error
			if outstandingRacers == 0 {
				// Context errors like cancellation or timeout take precedence over any
				// underlying HTTP errors, as the caller is expected to interrogate them
				// to decide what it should do next. This is not so much the case for other
				// types of error.
				overallError := ctx.Err()
				if overallError == nil {
					overallError = r.err
				}
				return "", overallError
			}
		}
	}
}
