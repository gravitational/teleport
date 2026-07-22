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

package usagereporter

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
)

var (
	usageEventsSubmitted = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Name:      teleport.MetricUsageEventsSubmitted,
		Help:      "a count of usage events that have been generated",
	})

	usageBatchesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Name:      teleport.MetricUsageBatches,
		Help:      "a count of batches enqueued for submission",
	})

	usageEventsRequeuedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Name:      teleport.MetricUsageEventsRequeued,
		Help:      "a count of events that were requeued after a submission failed",
	})

	usageBatchSubmissionDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: teleport.MetricNamespace,
		Name:      teleport.MetricUsageBatchSubmissionDuration,
		Help:      "a histogram of durations it took to submit a batch",
	})

	usageBatchesSubmitted = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Name:      teleport.MetricUsageBatchesSubmitted,
		Help:      "a count of event batches successfully submitted",
	})

	usageBatchesFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Name:      teleport.MetricUsageBatchesFailed,
		Help:      "a count of event batches that had at least one event that failed to submit",
	})

	usageEventsDropped = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Name:      teleport.MetricUsageEventsDropped,
		Help:      "a count of events dropped due to repeated errors or submission buffer overflow",
	})

	UsagePrometheusCollectors = []prometheus.Collector{
		usageEventsSubmitted, usageBatchesTotal, usageEventsRequeuedTotal,
		usageBatchSubmissionDuration, usageBatchesSubmitted, usageBatchesFailed,
		usageEventsDropped,
	}
)

// SubmitFunc is a func that submits a batch of usage events.
type SubmitFunc[T any] func(reporter *UsageReporter[T], batch []*SubmittedEvent[T]) ([]*SubmittedEvent[T], error)

// SubmittedEvent is an event that has been submitted
type SubmittedEvent[T any] struct {
	// Event is the Event to attempt to send
	Event *T

	// retriesRemaining is the number of attempts to make submitting this event
	// before it's discarded
	retriesRemaining int
}

type UsageReporter[T any] struct {
	// logger  writes log messages
	logger *slog.Logger

	// clock is the clock used for the main batching goroutine
	clock clockwork.Clock

	// submitClock is the clock used for the submission goroutine
	submitClock clockwork.Clock

	// events receives batches of incoming events from various Teleport components
	events chan []*SubmittedEvent[T]

	// buf stores events for batching
	buf []*SubmittedEvent[T]

	// submissionQueue queues events for submission
	submissionQueue chan []*SubmittedEvent[T]

	// submit is the func that submits batches of events to a backend
	submit SubmitFunc[T]

	// minBatchSize is the minimum batch size before a submit is triggered due
	// to size.
	minBatchSize int

	// maxBatchSize is the maximum size of a batch that may be sent at once.
	maxBatchSize int

	// maxBatchAge is the maximum time we're going to wait before we send a
	// batch, no matter how small it is.
	maxBatchAge time.Duration

	// maxBufferSize is the maximum number of events that can be queued in the
	// buffer.
	maxBufferSize int

	// submitDelay is the amount of delay added between all batch submission
	// attempts.
	submitDelay time.Duration

	// retryAttempts is the number of attempts that should be made to
	// submit a single event.
	retryAttempts int

	// receiveFunc is a callback for testing that's called when a batch has been
	// received, but before it's been potentially enqueued, used to ensure sane
	// sequencing in tests.
	receiveFunc func()

	eventsClosedOnce sync.Once
	eventsClosed     chan struct{}
	// wg is used to wait all goroutines to close
	wg sync.WaitGroup
}

// runSubmit starts the submission thread. It should be run as a background
// goroutine to ensure AddEventsToQueue() never blocks.
func (r *UsageReporter[T]) runSubmit(ctx context.Context) {
	defer r.wg.Done()

	for {
		var batch []*SubmittedEvent[T]
		var ok bool
		select {
		case <-ctx.Done():
			return
		case batch, ok = <-r.submissionQueue:
		}
		if !ok {
			return
		}

		t0 := time.Now()

		if failed, err := r.submit(r, batch); err != nil {
			r.logger.WarnContext(ctx, "failed to submit batch of usage events", "batch_size", len(batch), "error", err)
			usageBatchesFailed.Inc()

			var resubmit []*SubmittedEvent[T]
			for _, e := range failed {
				e.retriesRemaining--

				if e.retriesRemaining > 0 {
					resubmit = append(resubmit, e)
				}
			}

			droppedCount := len(failed) - len(resubmit)
			if droppedCount > 0 {
				r.logger.WarnContext(ctx, "dropping events due to error", "dropped_count", droppedCount, "error", err)
				usageEventsDropped.Add(float64(droppedCount))
			}

			// Put the failed events back on the queue.
			r.resubmitEvents(resubmit)
		} else {
			usageBatchesSubmitted.Inc()

			r.logger.DebugContext(ctx, "successfully submitted batch of usage events", "batch_size", len(batch))
		}

		usageBatchSubmissionDuration.Observe(time.Since(t0).Seconds())

		// Always sleep a bit to avoid spamming the server. We need a secondary
		// (possibly fake) clock here for testing to ensure
		// FakeClock.BlockUntil() doesn't include this sleep call.
		select {
		case <-ctx.Done():
			return
		case <-r.submitClock.After(r.submitDelay):
		}
	}
}

// GracefulStop stops receiving new events and schedules the
// final batch for submission. It blocks until the final batch
// has been sent, or until the provided context is canceled.
// Run must be called before GracefulStop is called.
func (r *UsageReporter[T]) GracefulStop(ctx context.Context) error {
	wait := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(wait)
	}()

	r.eventsClosedOnce.Do(func() { close(r.eventsClosed) })
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-wait:
		return nil
	}
}

// Run begins processing incoming usage events. It should be run in a goroutine.
func (r *UsageReporter[T]) Run(ctx context.Context) {
	defer r.wg.Done()

	splitBuffer := func() (batch []*SubmittedEvent[T], rest []*SubmittedEvent[T]) {
		if len(r.buf) > r.maxBatchSize {
			return r.buf[:r.maxBatchSize], r.buf[r.maxBatchSize:]
		}
		return r.buf, nil
	}

	// minBatchSize is the current minimum batch size, set to either 1 or to
	// r.minBatchSize depending on the batch age timer
	minBatchSize := r.minBatchSize

	// timer is the "batch age" timer, which triggers early batch submissions by
	// setting the minBatchSize to 1 temporarily. It should only be running if
	// r.buf is nonempty.
	timer := r.clock.NewTimer(r.maxBatchAge)
	defer timer.Stop()
	if len(r.buf) == 0 {
		if !timer.Stop() {
			<-timer.Chan()
		}
	}

	// Also start the submission goroutine.
	r.wg.Add(1)
	go r.runSubmit(ctx)
	defer close(r.submissionQueue)

	r.logger.DebugContext(ctx, "usage reporter is ready")

	for {
		var subQueue chan []*SubmittedEvent[T]
		var subBatch, subRest []*SubmittedEvent[T]
		if len(r.buf) >= minBatchSize {
			subQueue = r.submissionQueue
			subBatch, subRest = splitBuffer()
		}

		select {
		case <-ctx.Done():
			if len(r.buf) > 0 {
				r.logger.WarnContext(ctx, "dropped events due to context close", "discarded_count", len(r.buf))
			}
			return

		case <-r.eventsClosed:
			for len(r.buf) > 0 {
				subBatch, subRest := splitBuffer()
				select {
				case <-ctx.Done():
					r.logger.WarnContext(ctx, "dropped events due to context close during graceful stop", "discarded_count", len(r.buf))
					return
				case r.submissionQueue <- subBatch:
					usageBatchesTotal.Inc()
					r.logger.DebugContext(ctx, "enqueued batch of usage events during graceful stop", "batch_size", len(subBatch))
					r.buf = subRest
				}
			}
			return

		case <-timer.Chan():
			minBatchSize = 1

		case subQueue <- subBatch:
			usageBatchesTotal.Inc()
			r.logger.DebugContext(ctx, "enqueued batch of usage events", "batch_size", len(subBatch))
			r.buf = subRest
			minBatchSize = r.minBatchSize

			if !timer.Stop() {
				select {
				case <-timer.Chan():
				default:
				}
			}
			if len(r.buf) > 0 {
				timer.Reset(r.maxBatchAge)
			}

		case events := <-r.events:
			if len(r.buf)+len(events) > r.maxBufferSize {
				keep := max(r.maxBufferSize-len(r.buf), 0)

				r.logger.WarnContext(ctx, "usage event buffer is full, events will be discarded", "discarded_count", len(events)-keep)
				events = events[:keep]

				usageEventsDropped.Add(float64(len(events) - keep))
			}

			if len(events) == 0 {
				break
			}

			// about to become nonempty
			if len(r.buf) == 0 {
				timer.Reset(r.maxBatchAge)
			}

			r.buf = append(r.buf, events...)

			// call the receiver if any
			if r.receiveFunc != nil {
				r.receiveFunc()
			}
		}
	}
}

func (r *UsageReporter[T]) AddEventsToQueue(events ...*T) {
	submitted := make([]*SubmittedEvent[T], 0, len(events))
	for _, e := range events {
		submitted = append(submitted, &SubmittedEvent[T]{
			Event:            e,
			retriesRemaining: r.retryAttempts,
		})
	}

	usageEventsSubmitted.Add(float64(len(events)))
	// this should not be a separate goroutine, but under high load it's
	// possible to get noticeable contention (up to the hundreds of
	// milliseconds) when sending to the events channel, so for the user-facing
	// method we just spawn a short-lived goroutine for this
	//
	// TODO(espadolini): fix the usagereporter logic so that this is not needed
	go r.submitEvents(submitted)
}

// resubmitEvents resubmits events that have already been processed (in case of
// some error during submission).
func (r *UsageReporter[T]) resubmitEvents(events []*SubmittedEvent[T]) {
	usageEventsRequeuedTotal.Add(float64(len(events)))
	r.submitEvents(events)
}

func (r *UsageReporter[T]) submitEvents(events []*SubmittedEvent[T]) {
	select {
	case r.events <- events:
	case <-r.eventsClosed: // unblock submitEvent when there is no receiver (because reporter closes)
	}
}

type Options[T any] struct {
	Logger *slog.Logger
	// Submit is a func that submits a batch of usage events.
	Submit SubmitFunc[T]
	// MinBatchSize determines the size at which a batch is sent
	// regardless of elapsed time.
	MinBatchSize int
	// MaxBatchSize is the largest batch size that will be sent to
	// the server; batches larger than this will be split into multiple
	// requests.
	MaxBatchSize int
	// MaxBatchAge is the maximum age a batch may reach before
	// being flushed, regardless of the batch size
	MaxBatchAge time.Duration
	// MaxBufferSize is the maximum size to which the event buffer
	// may grow. Events submitted once this limit is reached will be discarded.
	// Events that were in the submission queue that fail to submit may also be
	// discarded when requeued.
	MaxBufferSize int
	// SubmitDelay is a mandatory delay added to each batch submission
	// to avoid spamming the prehog instance.
	SubmitDelay time.Duration
	// RetryAttempts is the number of attempts that should be made to
	// submit a single event.
	RetryAttempts int
	// Clock is the clock used for the main batching goroutine
	Clock clockwork.Clock
	// SubmitClock is the clock used for the submission goroutine
	SubmitClock clockwork.Clock
}

// NewUsageReporter creates a new usage reporter. `Run()` must be executed to
// process incoming events.
func NewUsageReporter[T any](options *Options[T]) *UsageReporter[T] {
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if options.Clock == nil {
		options.Clock = clockwork.NewRealClock()
	}
	if options.SubmitClock == nil {
		options.SubmitClock = clockwork.NewRealClock()
	}

	reporter := &UsageReporter[T]{
		logger:          options.Logger.With(teleport.ComponentKey, teleport.ComponentUsageReporting),
		events:          make(chan []*SubmittedEvent[T], 1),
		submissionQueue: make(chan []*SubmittedEvent[T], 1),
		eventsClosed:    make(chan struct{}),
		submit:          options.Submit,
		clock:           options.Clock,
		submitClock:     options.SubmitClock,
		minBatchSize:    options.MinBatchSize,
		maxBatchSize:    options.MaxBatchSize,
		maxBatchAge:     options.MaxBatchAge,
		maxBufferSize:   options.MaxBufferSize,
		submitDelay:     options.SubmitDelay,
		retryAttempts:   options.RetryAttempts,
	}

	// lowered when Run returns
	reporter.wg.Add(1)

	return reporter
}
