/*
Copyright 2015 Gravitational, Inc.

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

package state

import (
	"context"
	"io"
	"io/ioutil"
	"time"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const (
	// DefaultQueueLen determines how many logging events to queue in-memory
	// before start dropping them (probably because logging server is down)
	DefaultQueueLen = 300
	// DefaultFlushTimeout is a period to flush after no other events have been received
	DefaultFlushTimeout = time.Second
	// DefaultFlushChunks is a max chunks accumulated over period to flush
	DefaultFlushChunks = 250
	// DefaultFlushBytes is a max bytes of the chunks before the flush will be triggered
	DefaultFlushBytes = 100000
	// DefaultThrottleTimeout is a latency after we will
	DefaultThrottleTimeout = 500 * time.Millisecond
	// DefaultThrottleDuration is a period that we will throttle the slow network for
	// before trying to send again
	DefaultThrottleDuration = 10 * time.Second
	// DefaultBackoffInitialInterval is initial interval for backoff
	DefaultBackoffInitialInterval = 100 * time.Millisecond
	// DefaultBackoffMaxInterval is maximum interval for backoff
	DefaultBackoffMaxInterval = DefaultThrottleDuration
)

var (
	errNotSupported = trace.BadParameter("method not supported")
)

var (
	auditLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "audit_latency_microseconds",
			Help: "Latency for audit log submission",
			// Buckets in microsecnd latencies
			Buckets: prometheus.ExponentialBuckets(5000, 1.5, 15),
		},
	)
	auditChunks = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "audit_chunks_total",
			Help:    "Chunks per slice submitted",
			Buckets: prometheus.LinearBuckets(10, 20, 10),
		},
	)
	auditBytesPerChunk = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "audit_chunk_bytes",
			Help:    "Bytes per submitted per chunk",
			Buckets: prometheus.ExponentialBuckets(10, 3.0, 10),
		},
	)
	auditBytesPerSlice = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "audit_slice_bytes",
			Help:    "Bytes per submitted slice of chunks",
			Buckets: prometheus.ExponentialBuckets(100, 3.0, 10),
		},
	)
	auditRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "audit_requests_total",
			Help: "Number of audit requests",
		},
		[]string{"result"},
	)
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(auditLatencies)
	prometheus.MustRegister(auditChunks)
	prometheus.MustRegister(auditBytesPerChunk)
	prometheus.MustRegister(auditBytesPerSlice)
	prometheus.MustRegister(auditRequests)
}

// CachingAuditLogConifig sets configuration for caching audit log
type CachingAuditLogConfig struct {
	// Namespace is session namespace
	Namespace string
	// SessionID is session ID this log forwards for
	SessionID string
	// Server is the server receiving audit events
	Server events.IAuditLog
	// QueueLen is length of the caching queue
	QueueLen int
	// FlushChunks controls how many chunks to aggregate before submit
	FlushChunks int
	// Context is an optional context
	Context context.Context
	// ThrottleTimeout is a timeout that triggers throttling
	ThrottleTimeout time.Duration
	// ThrottleDuration is a duration for throttling
	ThrottleDuration time.Duration
	// FlushTimeout is a period to flush buffered chunks if the queue
	// has not filled up yet
	FlushTimeout time.Duration
	// FlushBytes sets amount of bytes per slice that triggers
	// the flush to the server
	FlushBytes int64
	// BackoffInitialInterval is initial interval for backoff
	BackoffInitialInterval time.Duration
	// BackoffMaxInterval is maximum interval for backoff
	BackoffMaxInterval time.Duration
}

// CheckAndSetDefaults checks and sets defaults
func (c *CachingAuditLogConfig) CheckAndSetDefaults() error {
	if c.Namespace == "" {
		return trace.BadParameter("missing parameter Namespace")
	}
	if c.SessionID == "" {
		return trace.BadParameter("missing parameter SessionID")
	}
	if c.Server == nil {
		return trace.BadParameter("missing parameter Server")
	}
	if c.QueueLen == 0 {
		c.QueueLen = DefaultQueueLen
	}
	if c.FlushChunks == 0 {
		c.FlushChunks = DefaultFlushChunks
	}
	if c.Context == nil {
		c.Context = context.TODO()
	}
	if c.ThrottleTimeout == 0 {
		c.ThrottleTimeout = DefaultThrottleTimeout
	}
	if c.ThrottleTimeout == 0 {
		c.ThrottleTimeout = DefaultThrottleTimeout
	}
	if c.ThrottleDuration == 0 {
		c.ThrottleDuration = DefaultThrottleDuration
	}
	if c.FlushTimeout == 0 {
		c.FlushTimeout = DefaultFlushTimeout
	}
	if c.FlushBytes == 0 {
		c.FlushBytes = DefaultFlushBytes
	}
	if c.BackoffInitialInterval == 0 {
		c.BackoffInitialInterval = DefaultBackoffInitialInterval
	}
	if c.BackoffMaxInterval == 0 {
		c.BackoffMaxInterval = DefaultBackoffMaxInterval
	}
	return nil
}

// CachingAuditLog implements events.IAuditLog on the recording machine (SSH server)
// It captures the local recording and forwards it to the AuditLog network server
// Some important properties of this implementation:
//
// * Without back pressure on posting session chunks, audit log was loosing events
//   because produce was much faster than consume and buffer was oveflowing
//
// * Throttle is important to continue the session in case if audit log
//   slowness, as the session output will block and timeout on every request
//
// * It is important to pack chunnks, because ls -laR / would otherwise
//   generate about 10K requests per second. With this packing approach
//   we reduced this number to about 40-50 requests per second,
//   we can now tweak this parameter now by setting queue size and flush buffers.
//
// * Current implementation attaches audit log forwarder per session
type CachingAuditLog struct {
	CachingAuditLogConfig
	queue         chan []*events.SessionChunk
	cancel        context.CancelFunc
	ctx           context.Context
	chunks        []*events.SessionChunk
	bytes         int64
	throttleStart time.Time
}

func (ll *CachingAuditLog) add(chunks []*events.SessionChunk) {
	ll.chunks = append(ll.chunks, chunks...)
	for i := range chunks {
		ll.bytes += int64(len(chunks[i].Data))
	}
}

func (ll *CachingAuditLog) reset() []*events.SessionChunk {
	out := ll.chunks
	ll.chunks = make([]*events.SessionChunk, 0, ll.FlushChunks)
	ll.bytes = 0
	return out
}

// NewCachingAuditLog creaets a new & fully initialized instance of the alog
func NewCachingAuditLog(cfg CachingAuditLogConfig) (*CachingAuditLog, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(context.TODO())
	ll := &CachingAuditLog{
		CachingAuditLogConfig: cfg,
		cancel:                cancel,
		ctx:                   ctx,
	}
	ll.queue = make(chan []*events.SessionChunk, ll.QueueLen)
	go ll.run()
	return ll, nil
}

func (ll *CachingAuditLog) collectRemainingChunks() {
	for {
		select {
		case chunks := <-ll.queue:
			ll.add(chunks)
		default:
			return
		}
	}
}

// run thread is picking up logging events and tries to forward them
// to the logging server
func (ll *CachingAuditLog) run() {
	ticker := time.NewTicker(ll.FlushTimeout)
	defer ticker.Stop()
	var tickerC <-chan time.Time
	for {
		select {
		case <-ll.ctx.Done():
			ll.collectRemainingChunks()
			ll.flush(flushOpts{force: true, noRetry: true})
			return
		case <-tickerC:
			// tick received to force flush after time passed
			tickerC = nil
			ll.flush(flushOpts{force: true})
		case chunks := <-ll.queue:
			ll.add(chunks)
			// we have received, set the timer
			// if no other chunks will not arrive to flush it
			tickerC = ticker.C
			ll.flush(flushOpts{force: false})
		}
	}
}

func (ll *CachingAuditLog) newExponentialBackoff() *backoff.ExponentialBackOff {
	b := &backoff.ExponentialBackOff{
		InitialInterval:     ll.BackoffInitialInterval,
		RandomizationFactor: backoff.DefaultRandomizationFactor,
		Multiplier:          backoff.DefaultMultiplier,
		MaxInterval:         ll.BackoffMaxInterval,
		Clock:               backoff.SystemClock,
	}
	b.Reset()
	return b
}

type flushOpts struct {
	force   bool
	noRetry bool
}

func (ll *CachingAuditLog) flush(opts flushOpts) {
	if len(ll.chunks) == 0 {
		return
	}
	if !opts.force {
		if len(ll.chunks) < ll.FlushChunks && ll.bytes < ll.FlushBytes {
			return
		}
	}
	chunks := ll.reset()
	slice := events.SessionSlice{
		Namespace: ll.Namespace,
		SessionID: ll.SessionID,
		Chunks:    chunks,
	}
	err := ll.postSlice(slice)
	if err == nil {
		return
	}
	log.Warningf("lost connection: %v", err)
	if opts.noRetry {
		return
	}
	ticker := backoff.NewTicker(ll.newExponentialBackoff())
	defer ticker.Stop()
	for {
		select {
		case <-ll.ctx.Done():
			return
		case <-ticker.C:
			err := ll.postSlice(slice)
			if err == nil {
				return
			} else {
				log.Warningf("lost connection, retried with error: %v", err)
			}
		}
	}
}

func (ll *CachingAuditLog) postSlice(slice events.SessionSlice) error {
	start := time.Now()
	err := ll.Server.PostSessionSlice(slice)
	auditRequests.WithLabelValues("total").Inc()
	if err != nil {
		auditRequests.WithLabelValues("fail").Inc()
	} else {
		auditRequests.WithLabelValues("ok").Inc()
	}
	auditLatencies.Observe(float64(time.Now().Sub(start) / time.Microsecond))
	auditChunks.Observe(float64(len(slice.Chunks)))

	var bytes int64
	for _, c := range slice.Chunks {
		bytes += int64(len(c.Data))
		auditBytesPerChunk.Observe(float64(len(c.Data)))
	}
	auditBytesPerSlice.Observe(float64(bytes))
	return err
}

func (ll *CachingAuditLog) post(chunks []*events.SessionChunk) error {
	if time.Now().Before(ll.throttleStart) {
		return nil
	}

	select {
	case <-ll.ctx.Done():
		return nil
	case ll.queue <- chunks:
		return nil
	default:
		// the queue is blocked, now we will create a timer
		// to detect the timeout
	}
	timer := time.NewTimer(ll.ThrottleTimeout)
	defer timer.Stop()
	select {
	case ll.queue <- chunks:
	case <-ll.ctx.Done():
		return nil
	case <-timer.C:
		ll.throttleStart = time.Now().Add(ll.ThrottleDuration)
		log.Warningf("latency spiked over %v, will throttle audit log forward until %v", ll.ThrottleTimeout, ll.throttleStart)
	}
	return nil
}

func (ll *CachingAuditLog) Close() error {
	ll.cancel()
	return nil
}

func (ll *CachingAuditLog) EmitAuditEvent(eventType string, fields events.EventFields) error {
	return ll.Server.EmitAuditEvent(eventType, fields)
}

func (ll *CachingAuditLog) PostSessionChunk(namespace string, sid session.ID, reader io.Reader) error {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return trace.Wrap(err)
	}
	chunks := []*events.SessionChunk{
		{
			Data: data,
			Time: time.Now().UTC().UnixNano(),
		},
	}
	return ll.post(chunks)
}

func (ll *CachingAuditLog) PostSessionSlice(slice events.SessionSlice) error {
	return ll.post(slice.Chunks)
}

func (ll *CachingAuditLog) GetSessionChunk(string, session.ID, int, int) ([]byte, error) {
	return nil, errNotSupported
}
func (ll *CachingAuditLog) GetSessionEvents(string, session.ID, int) ([]events.EventFields, error) {
	return nil, errNotSupported
}
func (ll *CachingAuditLog) SearchEvents(time.Time, time.Time, string) ([]events.EventFields, error) {
	return nil, errNotSupported
}
func (ll *CachingAuditLog) SearchSessionEvents(time.Time, time.Time) ([]events.EventFields, error) {
	return nil, errNotSupported
}
