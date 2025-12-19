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
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tbot/cli"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

var errServiceNotHealthy = errors.New("service is not yet healthy")

// waitFetch fetches a status report from the given endpoint using the provided
// client. It only returns without an error if the endpoint returns a valid
// and healthy status report.
func waitFetch(ctx context.Context, l *slog.Logger, client *http.Client, endpoint *url.URL) error {
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

	// If the status doesn't appear to be OK, log some additional info if
	// possible. 404 is ignored because the JSON structure is different.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		var status readyz.ServiceStatus
		if err := json.Unmarshal(bytes, &status); err == nil {
			l.WarnContext(ctx, "service is not yet ready", "status", status.Status, "reason", status.Reason)
		}
	}

	return trace.Wrap(trace.ReadError(resp.StatusCode, bytes), "response from wait API")
}

// onWaitCommand handles `tbot wait ...`
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
		Driver: retryutils.NewExponentialDriver(100 * time.Millisecond),
		Jitter: retryutils.HalfJitter,
		First:  250 * time.Millisecond,
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

	i := 0
	for {
		i += 1
		l := l.With("attempt", i)

		err = waitFetch(ctx, l, client, endpoint)
		if err == nil {
			break
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
