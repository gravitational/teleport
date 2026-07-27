/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package tunnel

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

const initializationMaxBackoff = 10 * time.Second

// RetryInitialization retries remote tunnel initialization until it
// succeeds or ctx is canceled. It reports failures as unhealthy.
// On recovery (or success) it does not report Success, it's up to the
// caller to do so once it finishes creating the proxy.
func RetryInitialization(
	ctx context.Context,
	log *slog.Logger,
	statusReporter readyz.Reporter,
	initialize func(context.Context) error,
) error {
	return retryInitialization(ctx, log, statusReporter, retryutils.HalfJitter, initialize)
}

// retryInitialization exposes the jitter setting to allow predictable testing.
func retryInitialization(
	ctx context.Context,
	log *slog.Logger,
	statusReporter readyz.Reporter,
	jitter retryutils.Jitter,
	initialize func(context.Context) error,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver: retryutils.NewExponentialDriver(time.Second),
		Max:    initializationMaxBackoff,
		Jitter: jitter,
	})
	if err != nil {
		return trace.Wrap(err, "creating tunnel initialization retry")
	}

	failures := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := initialize(ctx)
		if err == nil {
			if failures > 0 {
				log.InfoContext(ctx, "Tunnel initialization recovered", "failed_attempts", failures)
			}
			return nil
		}

		// Cancelation can race with the operation returning an error. Do not
		// report shutdown as another unhealthy initialization attempt.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		failures++
		statusReporter.ReportReason(readyz.Unhealthy, err.Error())

		retry.Inc()

		retryAfter := retry.Duration()

		log.WarnContext(ctx, "Tunnel initialization failed, will retry",
			"error", err,
			"failed_attempts", failures,
			"retry_after", retryAfter,
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryAfter):
		}
	}
}
