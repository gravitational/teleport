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
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tbot/cli"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

func waitFetch(ctx context.Context, client *http.Client, endpoint *url.URL) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return trace.Wrap(err, "could not build wait request")
	}

	resp, err := client.Do(req)
	if err != nil {
		return trace.Wrap(err, "making http request")
	}
	defer resp.Body.Close()

	bytes, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return trace.Wrap(err)
	}

	err = trace.ReadError(resp.StatusCode, bytes)
	if err != nil {
		return trace.Wrap(err, "wait api response")
	}

	// TODO: parse the status and ensure it is actually healthy

	return nil
}

func onWaitCommand(ctx context.Context, cmd *cli.WaitCommand) error {
	l := log.With("diag_addr", cmd.DiagAddr, "service", cmd.Service)

	if cmd.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cmd.Timeout)
		defer cancel()
	}

	var clock clockwork.Clock
	if cmd.Clock != nil {
		clock = cmd.Clock
	} else {
		clock = clockwork.NewRealClock()
	}

	diagAddr := cmd.DiagAddr
	if diagAddr == "" {
		return trace.BadParameter("--diag-addr is required")
	}

	// Allow plain host:port syntax; url.Parse will fail without a scheme, so if
	// none is specified, prepend http
	if !strings.Contains(diagAddr, "://") {
		diagAddr = "http://" + diagAddr
	}

	baseURL, err := url.Parse(diagAddr)
	if err != nil {
		return trace.Wrap(err, "parsing --diag-addr")
	}

	var endpoint *url.URL
	if cmd.Service == "" {
		endpoint = baseURL.JoinPath("wait")
	} else {
		endpoint = baseURL.JoinPath("wait", cmd.Service)
	}

	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver: retryutils.NewExponentialDriver(20 * time.Millisecond),
		Jitter: retryutils.HalfJitter,
		Max:    2 * time.Second,
		Clock:  clock,
	})
	if err != nil {
		return trace.Wrap(err, "creating retry helper")
	}

	client, err := defaults.HTTPClient()
	if err != nil {
		return trace.Wrap(err, "creating http client")
	}

	// Set a reasonably strict timeout. It could theoretically take a long time
	// for a service to become ready even if the endpoint is functioning and
	// exceed this timeout, but retrying again is mostly harmless and avoids our
	// retry loop getting stuck for any other reason.
	client.Timeout = 5 * time.Second

	l.InfoContext(ctx, "waiting for bot to become available")

	now := clock.Now()

LOOP:
	for {
		err = waitFetch(ctx, client, endpoint)
		if err == nil {
			break LOOP
		} else {
			l.DebugContext(ctx, "wait failed, retrying", "error", err)
		}

		retry.Inc()
		select {
		case <-ctx.Done():
			l.WarnContext(ctx, "bot did not become ready in time", "timeout", cmd.Timeout)
			return ctx.Err()
		case <-retry.After():
		}
	}

	if err == nil {
		l.InfoContext(ctx, "bot reported healthy", "after", clock.Since(now))
	}

	return trace.Wrap(err, "waiting for bot to become ready")
}
