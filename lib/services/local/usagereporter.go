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

package local

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/observability/metrics"
	prehogapi "github.com/gravitational/teleport/lib/prehog/gen/prehog/v1alpha"
	prehogclient "github.com/gravitational/teleport/lib/prehog/gen/prehog/v1alpha/prehogv1alphaconnect"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// usageReporterMinBatchSize determines the size at which a batch is sent
	// regardless of elapsed time
	usageReporterMinBatchSize = 20

	// usageReporterMaxBatchSize is the largest batch size that will be sent to
	// the server; batches larger than this will be split into multiple
	// requests.
	usageReporterMaxBatchSize = 100

	// usageReporterMaxBatchAge is the maximum age a batch may reach before
	// being flushed, regardless of the batch size.
	usageReporterMaxBatchAge = time.Second * 5

	// usageReporterMaxBufferSize is the maximum size to which the event buffer
	// may grow. Events submitted once this limit is reached will be discarded.
	// Events that were in the submission queue that fail to submit may also be
	// discarded when requeued.
	usageReporterMaxBufferSize = 500

	// usageReporterSubmitDelay is a mandatory delay added to each batch submission
	// to avoid spamming the prehog instance.
	usageReporterSubmitDelay = time.Second * 1

	// usageReporterRetryAttempts is the max number of attempts that
	// should be made to submit a particular event before it's dropped
	usageReporterRetryAttempts = 5
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

	usagePrometheusCollectors = []prometheus.Collector{
		usageEventsSubmitted, usageBatchesTotal, usageEventsRequeuedTotal,
		usageBatchSubmissionDuration, usageBatchesSubmitted, usageBatchesFailed,
		usageEventsDropped,
	}
)

// DiscardUsageReporter is a dummy usage reporter that drops all events.
type DiscardUsageReporter struct{}

func (d *DiscardUsageReporter) SubmitAnonymizedUsageEvents(event ...services.UsageAnonymizable) error {
	// do nothing
	return nil
}

// NewDiscardUsageReporter creates a new usage reporter that drops all events.
func NewDiscardUsageReporter() *DiscardUsageReporter {
	return &DiscardUsageReporter{}
}

// submitFunc is a func that submits a batch of usage events.
type UsageSubmitFunc func(reporter *UsageReporter, batch []*SubmittedEvent) ([]*SubmittedEvent, error)

// SubmittedEvent is an event that has been submitted
type SubmittedEvent struct {
	// Event is the Event to attempt to send
	Event *prehogapi.SubmitEventRequest

	// retriesRemaining is the number of attempts to make submitting this event
	// before it's discarded
	retriesRemaining int
}

type UsageReporter struct {
	// Entry is a log entry
	*logrus.Entry

	// clock is the clock used for the main batching goroutine
	clock clockwork.Clock

	// submitClock is the clock used for the submission goroutine
	submitClock clockwork.Clock

	// anonymizer is the anonymizer used for filtered audit events.
	anonymizer utils.Anonymizer

	// events receives batches of incoming events from various Teleport components
	events chan []*SubmittedEvent

	// buf stores events for batching
	buf []*SubmittedEvent

	// submissionQueue queues events for submission
	submissionQueue chan []*SubmittedEvent

	// submit is the func that submits batches of events to a backend
	submit UsageSubmitFunc

	// clusterName is the cluster's name, used for anonymization and as an event
	// field.
	clusterName types.ClusterName

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
}

// runSubmit starts the submission thread. It should be run as a background
// goroutine to ensure SubmitAnonymizedUsageEvents() never blocks.
func (r *UsageReporter) runSubmit(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-r.submissionQueue:
			t0 := time.Now()

			if failed, err := r.submit(r, batch); err != nil {
				r.WithField("batch_size", len(batch)).Warnf("failed to submit batch of usage events: %v", err)
				usageBatchesFailed.Inc()

				var resubmit []*SubmittedEvent
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
func (r *UsageReporter) enqueueBatch() {
	if len(r.buf) == 0 {
		// Nothing to do.
		return
	}

	var events []*SubmittedEvent
	var remaining []*SubmittedEvent
	if len(r.buf) > r.maxBatchSize {
		// Split the request and send the first batch. Any remaining events will
		// sit in the buffer to send with the next batch.
		events = r.buf[:r.maxBatchSize]
		remaining = r.buf[r.maxBatchSize:]
	} else {
		// The event buf is small enough to send in one request. We'll replace
		// the buf to allow any excess memory from the last buf to be GC'd.
		events = r.buf
		remaining = make([]*SubmittedEvent, 0, r.minBatchSize)
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

// Run begins processing incoming usage events. It should be run in a goroutine.
func (r *UsageReporter) Run(ctx context.Context) {
	timer := r.clock.NewTimer(r.maxBatchAge)

	// Also start the submission goroutine.
	go r.runSubmit(ctx)

	r.Debug("usage reporter is ready")

	for {
		select {
		case <-ctx.Done():
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
				timer.Reset(r.maxBatchAge)
				r.enqueueBatch()
			}
		}
	}
}

func (r *UsageReporter) SubmitAnonymizedUsageEvents(events ...services.UsageAnonymizable) error {
	var submitted []*SubmittedEvent

	for _, e := range events {
		req := e.Anonymize(r.anonymizer)
		req.ClusterName = r.anonymizer.AnonymizeString(r.clusterName.GetClusterName())
		req.Timestamp = timestamppb.New(r.clock.Now())
		submitted = append(submitted, &SubmittedEvent{
			Event:            &req,
			retriesRemaining: r.retryAttempts,
		})

		usageEventsSubmitted.Inc()
	}

	r.events <- submitted

	return nil
}

// resubmitEvents resubmits events that have already been processed (in case of
// some error during submission).
func (r *UsageReporter) resubmitEvents(events []*SubmittedEvent) {
	usageEventsRequeuedTotal.Add(float64(len(events)))

	r.events <- events
}

func NewPrehogSubmitter(ctx context.Context, prehogEndpoint string, clientCert *tls.Certificate, caCertPEM []byte) (UsageSubmitFunc, error) {
	tlsConfig := &tls.Config{
		// Self-signed test licenses may not have a proper issuer and won't be
		// used if just passed in via Certificates, so we'll use this to
		// explicitly set the client cert we want to use.
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return clientCert, nil
		},
	}

	if len(caCertPEM) > 0 {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caCertPEM)

		tlsConfig.RootCAs = pool
	}

	httpClient, err := defaults.HTTPClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	transport, ok := httpClient.Transport.(*http.Transport)
	if !ok {
		return nil, trace.BadParameter("invalid transport type %T", httpClient.Transport)
	}

	transport.Proxy = http.ProxyFromEnvironment
	transport.ForceAttemptHTTP2 = true
	transport.TLSClientConfig = tlsConfig

	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	httpClient.Timeout = 5 * time.Second

	client := prehogclient.NewTeleportReportingServiceClient(httpClient, prehogEndpoint)

	return func(reporter *UsageReporter, events []*SubmittedEvent) ([]*SubmittedEvent, error) {
		var failed []*SubmittedEvent
		var errors []error

		// Note: the backend doesn't support batching at the moment.
		for _, event := range events {
			// Note: this results in retrying the entire batch, which probably
			// isn't ideal.
			req := connect.NewRequest(event.Event)
			if _, err := client.SubmitEvent(ctx, req); err != nil {
				errors = append(errors, err)
			}
		}

		return failed, trace.NewAggregate(errors...)
	}, nil
}

// NewUsageReporter creates a new usage reporter. `Run()` must be executed to
// process incoming events.
func NewUsageReporter(ctx context.Context, log logrus.FieldLogger, clusterName types.ClusterName, submitter UsageSubmitFunc) (*UsageReporter, error) {
	if log == nil {
		log = logrus.StandardLogger()
	}

	anonymizer, err := utils.NewHMACAnonymizer(clusterName.GetClusterID())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = metrics.RegisterPrometheusCollectors(usagePrometheusCollectors...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &UsageReporter{
		Entry: log.WithField(
			trace.Component,
			teleport.Component(teleport.ComponentUsageReporting),
		),
		anonymizer:      anonymizer,
		events:          make(chan []*SubmittedEvent, 1),
		submissionQueue: make(chan []*SubmittedEvent, 1),
		submit:          submitter,
		clock:           clockwork.NewRealClock(),
		submitClock:     clockwork.NewRealClock(),
		clusterName:     clusterName,
		minBatchSize:    usageReporterMinBatchSize,
		maxBatchSize:    usageReporterMaxBatchSize,
		maxBatchAge:     usageReporterMaxBatchAge,
		maxBufferSize:   usageReporterMaxBufferSize,
		submitDelay:     usageReporterSubmitDelay,
		retryAttempts:   usageReporterRetryAttempts,
	}, nil
}
