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

package common

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gravitational/trace"

	tracehttp "github.com/gravitational/teleport/api/observability/tracing/http"
	"github.com/gravitational/teleport/api/utils"
)

type raceResult struct {
	addr string
	err  error
}

// maxPingBodySize represents the absolute maximum number of bytes that we will
// read from a ping response body before abandoning the read. A minimal ping
// response is about 256 bytes, so this should be more than enough for a valid
// response without being too onerous on the client side if we hit a non-
// teleport responder by mistake.
const maxPingBodySize = 16 * 1024

// raceRequest drives an HTTP request to completion and posts the results back
// to the supplied channel.
func raceRequest(ctx context.Context, cli *http.Client, addr string, waitgroup *sync.WaitGroup, results chan<- raceResult) {
	defer waitgroup.Done()

	target := fmt.Sprintf("https://%s/webapi/ping", addr)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		results <- raceResult{addr: addr, err: err}
		return
	}

	rsp, err := cli.Do(request)
	if err != nil {
		logger.DebugContext(ctx, "Proxy address test failed", "error", err)
		results <- raceResult{addr: addr, err: err}
		return
	}
	defer rsp.Body.Close()

	// NB: `ReadAll()` will time out (or be canceled) according to the
	//     context originally supplied to the request that initiated this
	//     response, so no need to have an independent reading timeout
	//     here.
	resBody, err := io.ReadAll(io.LimitReader(rsp.Body, maxPingBodySize))
	if err != nil {
		// Log but do not return. We could receive HTTP OK, and we should not fail on error here.
		logger.DebugContext(ctx, "Failed to read whole response body", "error", err)
	}

	// If the request returned a non-OK response then we're still going
	// to treat this as a failure and return an error to the race
	// aggregator.
	if rsp.StatusCode != http.StatusOK {
		logger.DebugContext(ctx, "Proxy address test received non-OK response",
			"status_code", rsp.StatusCode,
			"response_body", string(resBody),
		)
		err = trace.BadParameter("Proxy address test received non-OK response: %03d", rsp.StatusCode)
		results <- raceResult{addr: addr, err: err}
		return
	}

	// Post the results back to the caller, so they can be aggregated.
	results <- raceResult{addr: addr}
}

// startRacer starts the asynchronous execution of a single request, and keeps
// all the associated bookkeeping up to date.
func startRacer(ctx context.Context, cli *http.Client, host string, candidates []int, waitGroup *sync.WaitGroup, results chan<- raceResult) []int {
	port, tail := candidates[0], candidates[1:]
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	logger.DebugContext(ctx, "Trying request", "addr", addr)
	waitGroup.Add(1)
	go raceRequest(ctx, cli, addr, waitGroup, results)
	return tail
}

// pickDefaultAddr implements proxy selection via an RFC-8305 "Happy Eyeballs"
// -like algorithm. In practical terms, that means that it:
//  1. issues a GET request against multiple potential proxy ports,
//  2. races the requests against one another, and finally
//  3. selects the first to respond as the canonical proxy
func pickDefaultAddr(ctx context.Context, insecure bool, host string, ports []int) (string, error) {
	logger.DebugContext(ctx, "Resolving default proxy port", "insecure_mode", insecure)

	if len(ports) == 0 {
		return "", trace.BadParameter("port list may not be empty")
	}

	httpClient := &http.Client{
		Transport: tracehttp.NewTransport(
			&http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: insecure,
				},
				Proxy: func(req *http.Request) (*url.URL, error) {
					return utils.GetProxyURL(req.URL.String()), nil
				},
			},
		),
	}

	// NOTE: We rely on a specific order of deferred function execution in
	//       order not to deadlock as we exit this function.
	//       Please be careful when moving chunks around.

	// Make sure all of our live goroutines have quit before we return. This is
	// mainly for testing, so we can assert that the racers are all exiting
	// properly in error conditions.
	var racersInFlight sync.WaitGroup
	defer func() {
		logger.DebugContext(ctx, "Waiting for all in-flight proxy address tests to finish")
		racersInFlight.Wait()
	}()

	// Define an inner context that we'll give to the requests to cancel
	// them regardless of if we exit successfully, or we are killed from above.
	raceCtx, cancelRace := context.WithCancel(ctx)
	defer cancelRace()

	// Make the channel for the race results big enough, so we're guaranteed that a
	// channel write will never block. Once we have a hit we will stop reading the
	// channel, and we don't want to leak a bunch of goroutines while they're
	// blocked on writing to a full reply channel
	candidates := ports
	results := make(chan raceResult, len(candidates))

	// Start the first attempt racing
	unfinishedRacers := len(candidates)
	candidates = startRacer(raceCtx, httpClient, host, candidates, &racersInFlight, results)

	// Start a ticker that will kick off the subsequent racers after a small
	// interval. We don't want to start them all at once, as we may swamp the
	// network and give away any advantage we have from doing this concurrently in
	// the first place. RFC8305 recommends an interval of between 100ms and 2s,
	// with 250ms being a "sensible default"
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	var errors []error
	for {
		select {
		case <-ctx.Done():
			// We've timed out or been canceled. Bail out ASAP. Remember that returning
			// will implicitly cancel all the already-started racers.
			return "", ctx.Err()

		case <-ticker.C:
			// It's time to kick off a new racer
			if len(candidates) > 0 {
				candidates = startRacer(raceCtx, httpClient, host, candidates, &racersInFlight, results)
			}

		case r := <-results:
			unfinishedRacers--

			// if the request succeeded, it wins the race
			if r.err == nil {
				// Accept the winner as having the canonical web proxy address.
				//
				// Note that returning will implicitly cancel the inner context, telling
				// any outstanding racers that there is no point trying anymore, and they
				// should exit.
				logger.DebugContext(ctx, "Request to address succeeded, selected as canonical proxy address", "address", r.addr)
				return r.addr, nil
			}
			errors = append(errors, r.err)

			// the ping failed. This could be for any number of reasons. All we
			// really care about is whether _all_ of the ping attempts have
			// failed, and it's time to return with error
			if unfinishedRacers == 0 {
				// Context errors like cancellation or timeout take precedence over any
				// underlying HTTP errors, as the caller is expected to interrogate them
				// to decide what it should do next. This is not so much the case for other
				// types of error.
				overallError := ctx.Err()
				if overallError != nil {
					return "", overallError
				}

				return "", trace.NewAggregate(errors...)
			}
		}
	}
}
