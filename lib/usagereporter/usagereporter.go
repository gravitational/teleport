/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package usagereporter

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

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
	// Entry is a log entry
	*logrus.Entry

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

	// maxBatchAge is the
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
		select {
		case <-ctx.Done():
			return
		case batch, ok := <-r.submissionQueue:
			if !ok {
				return
			}
			t0 := time.Now()

			if failed, err := r.submit(r, batch); err != nil {
				r.WithField("batch_size", len(batch)).Warnf("failed to submit batch of usage events: %v", err)
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
					r.WithField("dropped_count", droppedCount).Warnf("dropping events due to error: %+v", err)
					usageEventsDropped.Add(float64(droppedCount))
				}

				// Put the failed events back on the queue.
				r.resubmitEvents(resubmit)
			} else {
				usageBatchesSubmitted.Inc()

				r.WithField("batch_size", len(batch)).Debug("successfully submitted batch of usage events")
			}

			usageBatchSubmissionDuration.Observe(time.Since(t0).Seconds())
		}

		// Always sleep a bit to avoid spamming the server. We need a secondary
		// (possibly fake) clock here for testing to ensure
		// FakeClock.BlockUntil() doesn't include this sleep call.
		select {
		case <-ctx.Done():
			return
		case <-r.submitClock.After(r.submitDelay):
			continue
		}
	}
}

// enqueueBatch prepares a batch for submission, removing it from the buffer and
// adding it to the submission queue.
func (r *UsageReporter[T]) enqueueBatch() {
	if len(r.buf) == 0 {
		// Nothing to do.
		return
	}

	var events []*SubmittedEvent[T]
	var remaining []*SubmittedEvent[T]
	if len(r.buf) > r.maxBatchSize {
		// Split the request and send the first batch. Any remaining events will
		// sit in the buffer to send with the next batch.
		events = r.buf[:r.maxBatchSize]
		remaining = r.buf[r.maxBatchSize:]
	} else {
		// The event buf is small enough to send in one request. We'll replace
		// the buf to allow any excess memory from the last buf to be GC'd.
		events = r.buf
		remaining = make([]*SubmittedEvent[T], 0, r.minBatchSize)
	}

	select {
	case r.submissionQueue <- events:
		// Wrote to the queue successfully, so swap buf with the shortened one.
		r.buf = remaining

		usageBatchesTotal.Inc()

		r.WithField("batch_size", len(events)).Debug("enqueued batch of usage events")
	default:
		// The queue is full, we'll try again later. Leave the existing buf in
		// place.
		r.WithField("batch_size", len(r.buf)).Debug("waiting to submit batch due to full queue")
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
	timer := r.clock.NewTimer(r.maxBatchAge)

	// Also start the submission goroutine.
	r.wg.Add(1)
	go r.runSubmit(ctx)
	defer close(r.submissionQueue)

	r.Debug("usage reporter is ready")

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.eventsClosed:
			r.enqueueBatch()
			return
		case <-timer.Chan():
			// Once the timer triggers, send any non-empty batch.
			timer.Reset(r.maxBatchAge)
			r.enqueueBatch()
		case events := <-r.events:
			// If the buffer's already full, just warn and discard.
			if len(r.buf) >= r.maxBufferSize {
				r.WithField("discarded_count", len(events)).Warn("usage event buffer is full, events will be discarded")

				usageEventsDropped.Add(float64(len(events)))
				break
			}

			if len(r.buf)+len(events) > r.maxBufferSize {
				keep := r.maxBufferSize - len(r.buf)
				r.WithField("discarded_count", len(events)-keep).Warn("usage event buffer is full, events will be discarded")
				events = events[:keep]

				usageEventsDropped.Add(float64(len(events) - keep))
			}

			r.buf = append(r.buf, events...)

			// call the receiver if any
			if r.receiveFunc != nil {
				r.receiveFunc()
			}

			// If we've accumulated enough events to trigger an early send, do
			// so and reset the timer.
			if len(r.buf) >= r.minBatchSize {
				if !timer.Stop() {
					<-timer.Chan()
				}
				timer.Reset(r.maxBatchAge)
				r.enqueueBatch()
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
	r.submitEvents(submitted)
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
	Log logrus.FieldLogger
	//Submit is a func that submits a batch of usage events.
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
	if options.Log == nil {
		options.Log = logrus.StandardLogger()
	}
	if options.Clock == nil {
		options.Clock = clockwork.NewRealClock()
	}
	if options.SubmitClock == nil {
		options.SubmitClock = clockwork.NewRealClock()
	}

	reporter := &UsageReporter[T]{
		Entry: options.Log.WithField(
			trace.Component,
			teleport.Component(teleport.ComponentUsageReporting),
		),
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
