/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package tbot

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/utils/retryutils"
)

type runOnIntervalConfig struct {
	name  string
	f     func(ctx context.Context) error
	clock clockwork.Clock
	// reloadCh allows the task to be triggered immediately, ideal for handling
	// CA rotations or a manual signal from a user.
	// reloadCh can be nil, in which case, the task will only run on the
	// interval.
	reloadCh             chan struct{}
	log                  *slog.Logger
	interval             time.Duration
	retryLimit           int
	exitOnRetryExhausted bool
	waitBeforeFirstRun   bool
}

// runOnInterval runs a function on a given interval, with retries and jitter.
//
// TODO(noah): Emit Prometheus metrics for:
// - Success/Failure of attempts
// - Time taken to execute attempt
// - Time of next attempt
func runOnInterval(ctx context.Context, cfg runOnIntervalConfig) error {
	switch {
	case cfg.interval <= 0:
		return trace.BadParameter("interval must be greater than 0")
	case cfg.retryLimit < 0:
		return trace.BadParameter("retryLimit must be greater than or equal to 0")
	case cfg.log == nil:
		return trace.BadParameter("log is required")
	case cfg.f == nil:
		return trace.BadParameter("f is required")
	case cfg.name == "":
		return trace.BadParameter("name is required")
	}

	log := cfg.log.With("task", cfg.name)

	if cfg.clock == nil {
		cfg.clock = clockwork.NewRealClock()
	}

	ticker := cfg.clock.NewTicker(cfg.interval)
	defer ticker.Stop()
	jitter := retryutils.DefaultJitter
	firstRun := true
	for {
		if !firstRun || (firstRun && cfg.waitBeforeFirstRun) {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.Chan():
			case <-cfg.reloadCh:
			}
		}
		firstRun = false

		var err error
		for attempt := 1; attempt <= cfg.retryLimit; attempt++ {
			log.InfoContext(
				ctx,
				"Attempting task",
				"attempt", attempt,
				"retry_limit", cfg.retryLimit,
			)
			err = cfg.f(ctx)
			if err == nil {
				break
			}

			if attempt != cfg.retryLimit {
				// exponentially back off with jitter, starting at 1 second.
				backoffTime := time.Second * time.Duration(math.Pow(2, float64(attempt-1)))
				backoffTime = jitter(backoffTime)
				cfg.log.WarnContext(
					ctx,
					"Task failed. Backing off and retrying",
					"attempt", attempt,
					"retry_limit", cfg.retryLimit,
					"backoff", backoffTime,
					"error", err,
				)
				select {
				case <-ctx.Done():
					return nil
				case <-cfg.clock.After(backoffTime):
				}
			}
		}
		if err != nil {
			if cfg.exitOnRetryExhausted {
				log.ErrorContext(
					ctx,
					"All retry attempts exhausted. Exiting",
					"error", err,
					"retry_limit", cfg.retryLimit,
				)
				return trace.Wrap(err)
			}
			log.WarnContext(
				ctx,
				"All retry attempts exhausted. Will wait for next interval",
				"retry_limit", cfg.retryLimit,
				"interval", cfg.interval,
			)
		} else {
			log.InfoContext(
				ctx,
				"Task succeeded. Waiting interval",
				"interval", cfg.interval,
			)
		}
	}
}
