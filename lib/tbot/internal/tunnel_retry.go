package internal

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

const tunnelInitializationMaxBackoff = 10 * time.Second

// RetryTunnelInitialization retries remote tunnel initialization until it
// succeeds or ctx is canceled. It reports failures as unhealthy.
// On recovery (or success) it does not report Success, it's up to the
// caller to do so once it finishes creating the proxy.
func RetryTunnelInitialization(
	ctx context.Context,
	log *slog.Logger,
	statusReporter readyz.Reporter,
	initialize func(context.Context) error,
) error {
	return retryTunnelInitialization(ctx, log, statusReporter, retryutils.HalfJitter, initialize)
}

// retryTunnelInitialization exposes the jitter setting to allow predictable testing.
func retryTunnelInitialization(
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
		Max:    tunnelInitializationMaxBackoff,
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

		// Cancellation can race with the operation returning an error. Do not
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
