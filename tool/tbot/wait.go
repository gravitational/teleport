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
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tbot/cli"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/utils"
)

// waitFetch fetches a status report from the given endpoint using the provided
// client. It only returns without an error if the endpoint returns a valid
// and healthy status report.
func waitFetch(ctx context.Context, l *slog.Logger, client *http.Client, service string, endpoint *url.URL) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return trace.Wrap(err, "building wait request")
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

	// If the status doesn't appear to be OK, try to parse out service status if
	// possible. 404 is ignored because the JSON structure is different.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		if service == "" {
			// No service was specified, so produce logs/errors specific to
			// overall status
			var status readyz.OverallStatus
			if err := json.Unmarshal(bytes, &status); err == nil {
				l.WarnContext(ctx, "bot is not yet ready", "status", status.Status)
				return trace.ConnectionProblem(nil, "bot is not yet ready: %s", status.Status)
			}
		}

		var status readyz.ServiceStatus
		if err := json.Unmarshal(bytes, &status); err == nil {
			l.WarnContext(ctx, "service is not yet ready", "status", status.Status, "reason", status.Reason)
			return trace.ConnectionProblem(nil, "service is not yet ready: %s", status.Reason)
		}

		// Note: `trace.ReadError()` doesn't provide any useful info for 5xx
		// errors, so return something sane if it failed to parse above.
		return trace.ConnectionProblem(nil, "unexpected response from server: %s", string(bytes))
	}

	// Given the above check, `trace.ReadError` just handles 404s.
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

	diagAddr := cmd.DiagAddr
	if diagAddr == "" {
		return trace.BadParameter("--diag-addr is required")
	}

	// Allow plain host:port syntax; url.Parse will fail without a scheme, so if
	// none is specified, prepend http://
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
	})
	if err != nil {
		return trace.Wrap(err, "creating retry helper")
	}

	var client *http.Client
	if cmd.Client != nil {
		client = cmd.Client
	} else {
		client, err = defaults.HTTPClient()
		if err != nil {
			return trace.Wrap(err, "creating http client")
		}
	}

	// Set a reasonably strict timeout. It could theoretically take a long time
	// for a service to become ready even if the endpoint is functioning and
	// exceed this timeout, but retrying again is mostly harmless and avoids our
	// retry loop getting stuck for any other reason.
	client.Timeout = 5 * time.Second

	l.InfoContext(ctx, "waiting for bot to become available")

	now := time.Now()

	i := 0
	for {
		i += 1
		l := l.With("attempt", i)

		err = waitFetch(ctx, l, client, cmd.Service, endpoint)
		if err == nil {
			break
		} else {
			l.DebugContext(ctx, "wait failed, retrying", "error", err)
		}

		retry.Inc()
		select {
		case <-ctx.Done():
			l.WarnContext(ctx, "bot did not become ready in time", "timeout", cmd.Timeout, "last_error", err)
			return ctx.Err()
		case <-retry.After():
		}
	}

	l.InfoContext(ctx, "bot reported healthy", "after", time.Since(now))
	return nil
}
