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
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// MaxQueueSize determines how many logging events to queue in-memory
	// before start dropping them (probably because logging server is down)
	MaxQueueSize = 200
	// FlushAfterPeriod is a period to flush after no other events have been received
	FlushAfterPeriod = time.Second
	// FlushMaxChunks is a max chunks accumulated over period to flush
	FlushMaxChunks = 150
	// FlushMaxBytes is a max bytes of the chunks before the flush will be triggered
	FlushMaxBytes = 100000
	// ThrottleLatency is a latency after we will
	ThrottleLatency = 500 * time.Millisecond
	// ThrottleDuration is a period that we will throttle the slow network for
	// before trying to send again
	ThrottleDuration = 10 * time.Second
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

// CachingAuditLog implements events.IAuditLog on the recording machine (SSH server)
// It captures the local recording and forwards it to the AuditLog network server
type CachingAuditLog struct {
	sync.Mutex
	server           events.IAuditLog
	queue            chan []events.SessionChunk
	cancel           context.CancelFunc
	ctx              context.Context
	chunks           []events.SessionChunk
	bytes            int64
	throttleStart    time.Time
	withBackPressure bool
}

func (ll *CachingAuditLog) add(chunks []events.SessionChunk) {
	ll.chunks = append(ll.chunks, chunks...)
	for i := range chunks {
		ll.bytes += int64(len(chunks[i].Data))
	}
}

func (ll *CachingAuditLog) reset() []events.SessionChunk {
	out := ll.chunks
	ll.chunks = make([]events.SessionChunk, 0, FlushMaxChunks)
	ll.bytes = 0
	return out
}

// NewCachingAuditLog creaets a new & fully initialized instance of the alog
func NewCachingAuditLog(logServer events.IAuditLog) *CachingAuditLog {
	ctx, cancel := context.WithCancel(context.TODO())
	ll := &CachingAuditLog{
		server: logServer,
		cancel: cancel,
		ctx:    ctx,
	}
	// start the queue:
	if logServer != nil {
		ll.queue = make(chan []events.SessionChunk, MaxQueueSize+1)
		go ll.run()
	}
	return ll
}

// run thread is picking up logging events and tries to forward them
// to the logging server
func (ll *CachingAuditLog) run() {
	ticker := time.NewTicker(FlushAfterPeriod)
	defer ticker.Stop()
	var tickerC <-chan time.Time
	for {
		select {
		case <-ll.ctx.Done():
			return
		case <-tickerC:
			// tick received to force flush after time passed
			tickerC = nil
			ll.flush(true)
		case chunks := <-ll.queue:
			ll.add(chunks)
			// we have received, set the timer
			// if no other chunks will not arrive to flush it
			tickerC = ticker.C
			ll.flush(false)
		}
	}
}

func (ll *CachingAuditLog) flush(force bool) {
	if len(ll.chunks) == 0 {
		return
	}
	if !force {
		if len(ll.chunks) < FlushMaxChunks && ll.bytes < FlushMaxBytes {
			return
		}
	}
	chunks := ll.reset()
	start := time.Now()
	err := ll.server.PostSessionChunks(chunks)
	auditRequests.WithLabelValues("total").Inc()
	if err != nil {
		auditRequests.WithLabelValues("failure").Inc()
	}
	auditLatencies.Observe(float64(time.Now().Sub(start) / time.Microsecond))
	auditChunks.Observe(float64(len(chunks)))

	var bytes int64
	for _, c := range chunks {
		bytes += int64(len(c.Data))
		auditBytesPerChunk.Observe(float64(len(c.Data)))
	}
	auditBytesPerSlice.Observe(float64(bytes))
}

func (ll *CachingAuditLog) post(chunks []events.SessionChunk) error {
	if time.Now().Before(ll.throttleStart) {
		return nil
	}
	select {
	case ll.queue <- chunks:
		return nil
	default:
		// the queue is blocked, now we will create a timer
		// to detect the timeout
	}
	timer := time.NewTimer(ThrottleLatency)
	defer timer.Stop()
	select {
	case ll.queue <- chunks:
	case <-timer.C:
		ll.throttleStart = time.Now().Add(ThrottleDuration)
		log.Warningf("will throttle audit log forward until %v", ll.throttleStart)
	}
	return nil
}

func (ll *CachingAuditLog) Close() error {
	ll.cancel()
	return nil
}

func (ll *CachingAuditLog) EmitAuditEvent(eventType string, fields events.EventFields) error {
	return ll.server.EmitAuditEvent(eventType, fields)
}

func (ll *CachingAuditLog) PostSessionChunk(namespace string, sid session.ID, reader io.Reader) error {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return trace.Wrap(err)
	}
	chunks := []events.SessionChunk{
		{
			Namespace: namespace,
			SessionID: string(sid),
			Data:      data,
			Time:      time.Now().UTC(),
		},
	}
	return ll.post(chunks)
}

func (ll *CachingAuditLog) PostSessionChunks(chunks []events.SessionChunk) error {
	return ll.post(chunks)
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
