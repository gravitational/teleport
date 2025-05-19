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
package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/tbot/service/status"
)

// NewPeriodicHandler wraps a OneShotHandler to run it on the given interval.
func NewPeriodicHandler[HandlerT OneShotHandler](
	handler HandlerT,
	opts PeriodicHandlerOptions,
) (*PeriodicHandler[HandlerT], error) {
	if err := opts.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &PeriodicHandler[HandlerT]{
		PeriodicHandlerOptions: opts,
		handler:                handler,
	}, nil
}

// PeriodicHandler wraps a OneShotHandler so that it can be used as a long-running
// service by calling it on an interval. It manages the service's status.
type PeriodicHandler[HandlerT OneShotHandler] struct {
	PeriodicHandlerOptions

	handler HandlerT
}

// OneShot satisfies the OneShotHandler interface.
func (p *PeriodicHandler[HandlerT]) OneShot(ctx context.Context) error {
	return p.handler.OneShot(ctx)
}

// Run the OneShot handler on an interval until, updating the service status
// whenever the handler succeeds or fails, until the given context is canceled
// or reaches its deadline.
func (p *PeriodicHandler[HandlerT]) Run(ctx context.Context, runtime *Runtime) error {
	timer := p.Clock.NewTimer(p.Interval)
	defer timer.Stop()

	for {
		retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
			First:  1 * time.Second,
			Driver: retryutils.NewExponentialDriver(1 * time.Second),
			Max:    1 * time.Minute,
			Jitter: retryutils.DefaultJitter,
			Clock:  p.Clock,
		})
		if err != nil {
			return trace.Wrap(err, "creating retrier")
		}

		start := time.Now()
		for attempt := 1; ; attempt++ {
			p.Logger.InfoContext(
				ctx,
				"Attempting task",
				"attempt", attempt,
				"max_attempts", p.MaxAttempts,
			)

			err := p.handler.OneShot(ctx)

			if err == nil {
				p.Logger.InfoContext(
					ctx,
					"Task succeeded. Waiting interval",
					"interval", p.Interval,
				)
				runtime.SetStatus(status.Ready)
				p.Metrics.Success(attempt, time.Since(start))
				break
			}

			if attempt == p.MaxAttempts {
				p.Logger.WarnContext(
					ctx,
					"All retry attempts exhausted. Will wait for next interval",
					"max_attempts", p.MaxAttempts,
					"interval", p.Interval,
				)
				runtime.SetStatus(status.Failed)
				p.Metrics.Failure(time.Since(start))
				break
			}

			retry.Inc()

			p.Logger.WarnContext(
				ctx,
				"Task failed. Backing off and retrying",
				"attempt", attempt,
				"max_attempts", p.MaxAttempts,
				"backoff", retry.Duration(),
				"error", err,
			)

			select {
			case <-retry.After():
				// Time for another attempt.
			case <-p.ReloadCh:
				// Immediate run triggered (e.g. by config reload).
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		timer.Reset(p.Interval)

		select {
		case <-timer.Chan():
			// Time to run again.
		case <-p.ReloadCh:
			// Immediate run triggered (e.g. by config reload).
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Get the original OneShotHandler.
func (p *PeriodicHandler[HandlerT]) Get() HandlerT { return p.handler }

// PeriodicHandlerOptions control the behavior of a PeriodicHandler.
type PeriodicHandlerOptions struct {
	// Interval at which the handler will run.
	Interval time.Duration

	// Logger to which messages will be written.
	Logger *slog.Logger

	// Clock used as the source of timers, overridable for tests.
	Clock clockwork.Clock

	// MaxAttempts is the maximum number of times we will try to run the
	// handler's OneShot method before giving up and waiting until the next
	// interval.
	MaxAttempts int

	// Metrics is used to record metrics about the handler.
	Metrics PeriodicHandlerMetrics

	// ReloadCh triggers the handler immediately.
	ReloadCh chan struct{}
}

func (o *PeriodicHandlerOptions) checkAndSetDefaults() error {
	if o.Interval <= 0 {
		return trace.BadParameter("Interval must greater than zero")
	}
	if o.MaxAttempts < 1 {
		return trace.BadParameter("MaxAttempts must be grater than zero")
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Clock == nil {
		o.Clock = clockwork.NewRealClock()
	}
	if o.Metrics == nil {
		o.Metrics = noopPeriodicHandlerMetrics{}
	}
	return nil
}

// PeriodicHandlerMetrics is used to record metrics about a PeriodicHandler.
type PeriodicHandlerMetrics interface {
	// Success records when a handle succeeds, including how many attempts it
	// took and the overall duration including retries.
	Success(attempts int, duration time.Duration)

	// Failure records when a handler fails, and the overall duration including
	// retries.
	Failure(duration time.Duration)
}

type noopPeriodicHandlerMetrics struct{}

func (noopPeriodicHandlerMetrics) Success(int, time.Duration) {}
func (noopPeriodicHandlerMetrics) Failure(time.Duration)      {}
