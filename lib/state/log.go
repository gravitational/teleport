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
	"fmt"
	"io"
	"io/ioutil"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/codahale/hdrhistogram"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"
)

var (
	errNotSupported = trace.BadParameter("method not supported")
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

// CachingAuditLog implements events.IAuditLog on the recording machine (SSH server)
// It captures the local recording and forwards it to the AuditLog network server
type CachingAuditLog struct {
	sync.Mutex
	server            events.IAuditLog
	queue             chan []events.SessionChunk
	cancel            context.CancelFunc
	ctx               context.Context
	chunks            []events.SessionChunk
	bytes             int64
	latencyHist       *hdrhistogram.Histogram
	chunksHist        *hdrhistogram.Histogram
	bytesHist         *hdrhistogram.Histogram
	bytesPerChunkHist *hdrhistogram.Histogram
	throttleStart     time.Time
	lastReport        time.Time
	requests          int
	withBackPressure  bool
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
		server:            logServer,
		cancel:            cancel,
		ctx:               ctx,
		latencyHist:       hdrhistogram.New(1, 60000, 3),
		chunksHist:        hdrhistogram.New(1, 60000, 3),
		bytesHist:         hdrhistogram.New(1, 600000, 3),
		bytesPerChunkHist: hdrhistogram.New(1, 600000, 3),
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
			log.Warningf("flushing after timeout")
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
	if err != nil {
		log.Warningf("failed to post chunk: %v", err)
	}
	ll.requests += 1
	ll.latencyHist.RecordValue(int64(time.Now().Sub(start) / time.Microsecond))
	ll.chunksHist.RecordValue(int64(len(chunks)))
	var bytes int64
	for _, c := range chunks {
		bytes += int64(len(c.Data))
		ll.bytesPerChunkHist.RecordValue(int64(len(c.Data)))
	}
	ll.bytesHist.RecordValue(bytes)

	if time.Now().Sub(ll.lastReport) > 10*time.Second {
		diff := time.Now().Sub(ll.lastReport) / time.Second
		requests := ll.requests
		ll.requests = 0
		ll.lastReport = time.Now()

		fmt.Printf("Latency histogram\n")
		for _, quantile := range []float64{25, 50, 75, 90, 95, 99, 100} {
			fmt.Printf("%v\t%v microseconds\n", quantile, ll.latencyHist.ValueAtQuantile(quantile))
		}
		fmt.Printf("%v requests/sec\n", requests/int(diff))

		fmt.Printf("Chunk count histogram\n")
		for _, quantile := range []float64{25, 50, 75, 90, 95, 99, 100} {
			fmt.Printf("%v\t%v chunks\n", quantile, ll.chunksHist.ValueAtQuantile(quantile))
		}

		fmt.Printf("Bytes per slice of chunks  histogram\n")
		for _, quantile := range []float64{25, 50, 75, 90, 95, 99, 100} {
			fmt.Printf("%v\t%v bytes\n", quantile, ll.bytesHist.ValueAtQuantile(quantile))
		}

		fmt.Printf("Bytes per chunk histogram\n")
		for _, quantile := range []float64{25, 50, 75, 90, 95, 99, 100} {
			fmt.Printf("%v\t%v bytes\n", quantile, ll.bytesPerChunkHist.ValueAtQuantile(quantile))
		}
	}
}

func (ll *CachingAuditLog) post(chunks []events.SessionChunk) error {
	if time.Now().After(ll.throttleStart) {
		return nil
	}
	timer := time.NewTimer(ThrottleLatency)
	defer timer.Stop()
	select {
	case ll.queue <- chunks:
	case <-timer.C:
		ll.throttleStart = time.Now().Add(ThrottleDuration)
		log.Warningf("will throttle connection until %v", ll.throttleStart)
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
