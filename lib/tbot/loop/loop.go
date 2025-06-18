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

package loop

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport/api/utils/retryutils"
)

var (
	// IterationsCounter counts the number of loop iterations, not including
	// retries.
	IterationsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tbot_task_iterations_total",
			Help: "Number of task iteration attempts, not counting retries",
		}, []string{"service", "name"},
	)

	// IterationsSuccessCounter counts the number of successful iterations,
	// bucketed by number of retries.
	IterationsSuccessCounter = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tbot_task_iterations_successful",
			Help:    "Histogram of task iterations that ultimately succeeded, bucketed by number of retries before success",
			Buckets: []float64{0, 1, 2, 3, 4, 5},
		}, []string{"service", "name"},
	)

	// IterationsFailureCounter counts the numver of failed iterations.
	IterationsFailureCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tbot_task_iterations_failed",
			Help: "Number of task iterations that ultimately failed, not counting retries",
		}, []string{"service", "name"},
	)

	// IterationTime measures the time each iteration of task takes, including
	// all retries.
	IterationTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tbot_task_iteration_duration_seconds",
			Help:    "Time between beginning and ultimate end of one task iteration regardless of outcome, including all retries",
			Buckets: prometheus.ExponentialBuckets(0.1, 1.75, 6),
		}, []string{"service", "name"},
	)
)

// Config for the loop.
type Config struct {
	// Service this task belongs to.
	Service string

	// Name of the specific task this loop is performing.
	Name string

	// Fn is the function that will be called on each iteration of the loop.
	Fn func(ctx context.Context) error

	// Clock allows you to override the clock in tests.
	Clock clockwork.Clock

	// Log is the logger to which messages will be written.
	Log *slog.Logger

	// Interval controls how often the loop will run.
	Interval time.Duration

	// RetryLimit is the maximum number of attempts that will be made to run the
	// function on each iteration of the loop.
	RetryLimit int

	// ExitOnRetryExhausted controls whether the loop will terminate if RetryLimit
	// is reached.
	ExitOnRetryExhausted bool

	// WaitBeforeFirstRun delays the first iteration of the loop until Interval
	// has passed.
	WaitBeforeFirstRun bool

	// ReloadCh allows the task to be triggered immediately, ideal for handling
	// CA rotations or a manual signal from a user. ReloadCh can be nil, in
	// which case, the task will only run on the interval.
	ReloadCh chan struct{}

	// IdentityReadyCh allows the loop to wait until the internal bot identity
	// renewal has completed before running, to avoid spamming the logs if the
	// service doesn't support gracefully degrading when there is no API client
	// available.
	IdentityReadyCh <-chan struct{}
}

// CheckAndSetDefaults validates the loop configuration and sets any default
// values.
func (cfg *Config) CheckAndSetDefaults() error {
	switch {
	case cfg.Interval <= 0:
		return trace.BadParameter("Interval must be greater than 0")
	case cfg.RetryLimit < 0:
		return trace.BadParameter("RetryLimit must be greater than or equal to 0")
	case cfg.Log == nil:
		return trace.BadParameter("Log is required")
	case cfg.Fn == nil:
		return trace.BadParameter("Fn is required")
	case cfg.Name == "":
		return trace.BadParameter("Name is required")
	}

	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return nil
}

// Run a function on a given interval, with retries and jitter.
func Run(ctx context.Context, cfg Config) error {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return err
	}

	log := cfg.Log.With("task", cfg.Name)

	if cfg.IdentityReadyCh != nil {
		select {
		case <-cfg.IdentityReadyCh:
		default:
			log.InfoContext(ctx, "Waiting for internal bot identity to be renewed before running")
			select {
			case <-cfg.IdentityReadyCh:
			case <-ctx.Done():
				return nil
			}
		}
	}

	ticker := cfg.Clock.NewTicker(cfg.Interval)
	defer ticker.Stop()
	jitter := retryutils.DefaultJitter
	firstRun := true
	for {
		if !firstRun || (firstRun && cfg.WaitBeforeFirstRun) {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.Chan():
			case <-cfg.ReloadCh:
			}
		}
		firstRun = false

		IterationsCounter.WithLabelValues(cfg.Service, cfg.Name).Inc()
		startTime := time.Now()

		var err error
		for attempt := 1; attempt <= cfg.RetryLimit; attempt++ {
			log.InfoContext(
				ctx,
				"Attempting task",
				"attempt", attempt,
				"retry_limit", cfg.RetryLimit,
			)
			err = cfg.Fn(ctx)
			if err == nil {
				IterationsSuccessCounter.WithLabelValues(cfg.Service, cfg.Name).Observe(float64(attempt - 1))
				break
			}

			if attempt != cfg.RetryLimit {
				// exponentially back off with jitter, starting at 1 second.
				backoffTime := time.Second * time.Duration(math.Pow(2, float64(attempt-1)))
				backoffTime = jitter(backoffTime)
				cfg.Log.WarnContext(
					ctx,
					"Task failed. Backing off and retrying",
					"attempt", attempt,
					"retry_limit", cfg.RetryLimit,
					"backoff", backoffTime,
					"error", err,
				)
				select {
				case <-ctx.Done():
					// Note: will discard metric update for this loop. It
					// probably won't be collected if we're shutting down,
					// anyway.
					return nil
				case <-cfg.Clock.After(backoffTime):
				}
			}
		}

		IterationTime.WithLabelValues(cfg.Service, cfg.Name).Observe(time.Since(startTime).Seconds())

		if err != nil {
			IterationsFailureCounter.WithLabelValues(cfg.Service, cfg.Name).Inc()

			if cfg.ExitOnRetryExhausted {
				log.ErrorContext(
					ctx,
					"All retry attempts exhausted. Exiting",
					"error", err,
					"retry_limit", cfg.RetryLimit,
				)
				return trace.Wrap(err)
			}
			log.WarnContext(
				ctx,
				"All retry attempts exhausted. Will wait for next interval",
				"retry_limit", cfg.RetryLimit,
				"interval", cfg.Interval,
			)
		} else {
			log.InfoContext(
				ctx,
				"Task succeeded. Waiting interval",
				"interval", cfg.Interval,
			)
		}
	}
}
