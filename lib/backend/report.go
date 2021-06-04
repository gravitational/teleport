/*
Copyright 2019 Gravitational, Inc.

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

package backend

import (
	"bytes"
	"context"
	"math"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const reporterDefaultCacheSize = 1000

// ReporterConfig configures reporter wrapper
type ReporterConfig struct {
	// Backend is a backend to wrap
	Backend Backend
	// Component is a component name to report
	Component string
	// Number of the most recent backend requests to preserve for top requests
	// metric. Higher value means higher memory usage but fewer infrequent
	// requests forgotten.
	TopRequestsCount int
}

// CheckAndSetDefaults checks and sets
func (r *ReporterConfig) CheckAndSetDefaults() error {
	if r.Backend == nil {
		return trace.BadParameter("missing parameter Backend")
	}
	if r.Component == "" {
		r.Component = teleport.ComponentBackend
	}
	if r.TopRequestsCount == 0 {
		r.TopRequestsCount = reporterDefaultCacheSize
	}
	return nil
}

// Reporter wraps a Backend implementation and reports
// statistics about the backend operations
type Reporter struct {
	// ReporterConfig contains reporter wrapper configuration
	ReporterConfig

	// topRequestsCache is an LRU cache to track the most frequent recent
	// backend keys. All keys in this cache map to existing labels in the
	// requests metric. Any evicted keys are also deleted from the metric.
	//
	// This will keep an upper limit on our memory usage while still always
	// reporting the most active keys.
	topRequestsCache *lru.Cache
}

// NewReporter returns a new Reporter.
func NewReporter(cfg ReporterConfig) (*Reporter, error) {
	err := utils.RegisterPrometheusCollectors(prometheusCollectors...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cache, err := lru.NewWithEvict(cfg.TopRequestsCount, func(key interface{}, value interface{}) {
		labels, ok := key.(topRequestsCacheKey)
		if !ok {
			log.Errorf("BUG: invalid cache key type: %T", key)
			return
		}
		// Evict the key from requests metric.
		requests.DeleteLabelValues(labels.component, labels.key, labels.isRange)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	r := &Reporter{
		ReporterConfig:   cfg,
		topRequestsCache: cache,
	}
	return r, nil
}

// GetRange returns query range
func (s *Reporter) GetRange(ctx context.Context, startKey []byte, endKey []byte, limit int) (*GetResult, error) {
	start := s.Clock().Now()
	res, err := s.Backend.GetRange(ctx, startKey, endKey, limit)
	batchReadLatencies.WithLabelValues(s.Component).Observe(time.Since(start).Seconds())
	batchReadRequests.WithLabelValues(s.Component).Inc()
	if err != nil {
		batchReadRequestsFailed.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpGet, startKey, endKey)
	return res, err
}

// Create creates item if it does not exist
func (s *Reporter) Create(ctx context.Context, i Item) (*Lease, error) {
	start := s.Clock().Now()
	lease, err := s.Backend.Create(ctx, i)
	writeLatencies.WithLabelValues(s.Component).Observe(time.Since(start).Seconds())
	writeRequests.WithLabelValues(s.Component).Inc()
	if err != nil {
		writeRequestsFailed.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpPut, i.Key, nil)
	return lease, err
}

// Put puts value into backend (creates if it does not
// exists, updates it otherwise)
func (s *Reporter) Put(ctx context.Context, i Item) (*Lease, error) {
	start := s.Clock().Now()
	lease, err := s.Backend.Put(ctx, i)
	writeLatencies.WithLabelValues(s.Component).Observe(time.Since(start).Seconds())
	writeRequests.WithLabelValues(s.Component).Inc()
	if err != nil {
		writeRequestsFailed.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpPut, i.Key, nil)
	return lease, err
}

// Update updates value in the backend
func (s *Reporter) Update(ctx context.Context, i Item) (*Lease, error) {
	start := s.Clock().Now()
	lease, err := s.Backend.Update(ctx, i)
	writeLatencies.WithLabelValues(s.Component).Observe(time.Since(start).Seconds())
	writeRequests.WithLabelValues(s.Component).Inc()
	if err != nil {
		writeRequestsFailed.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpPut, i.Key, nil)
	return lease, err
}

// Get returns a single item or not found error
func (s *Reporter) Get(ctx context.Context, key []byte) (*Item, error) {
	start := s.Clock().Now()
	readLatencies.WithLabelValues(s.Component).Observe(time.Since(start).Seconds())
	readRequests.WithLabelValues(s.Component).Inc()
	item, err := s.Backend.Get(ctx, key)
	if err != nil && !trace.IsNotFound(err) {
		readRequestsFailed.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpGet, key, nil)
	return item, err
}

// CompareAndSwap compares item with existing item
// and replaces is with replaceWith item
func (s *Reporter) CompareAndSwap(ctx context.Context, expected Item, replaceWith Item) (*Lease, error) {
	start := s.Clock().Now()
	lease, err := s.Backend.CompareAndSwap(ctx, expected, replaceWith)
	writeLatencies.WithLabelValues(s.Component).Observe(time.Since(start).Seconds())
	writeRequests.WithLabelValues(s.Component).Inc()
	if err != nil && !trace.IsNotFound(err) && !trace.IsCompareFailed(err) {
		writeRequestsFailed.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpPut, expected.Key, nil)
	return lease, err
}

// Delete deletes item by key
func (s *Reporter) Delete(ctx context.Context, key []byte) error {
	start := s.Clock().Now()
	err := s.Backend.Delete(ctx, key)
	writeLatencies.WithLabelValues(s.Component).Observe(time.Since(start).Seconds())
	writeRequests.WithLabelValues(s.Component).Inc()
	if err != nil && !trace.IsNotFound(err) {
		writeRequestsFailed.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpDelete, key, nil)
	return err
}

// DeleteRange deletes range of items
func (s *Reporter) DeleteRange(ctx context.Context, startKey []byte, endKey []byte) error {
	start := s.Clock().Now()
	err := s.Backend.DeleteRange(ctx, startKey, endKey)
	batchWriteLatencies.WithLabelValues(s.Component).Observe(time.Since(start).Seconds())
	batchWriteRequests.WithLabelValues(s.Component).Inc()
	if err != nil && !trace.IsNotFound(err) {
		batchWriteRequestsFailed.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpDelete, startKey, endKey)
	return err
}

// KeepAlive keeps object from expiring, updates lease on the existing object,
// expires contains the new expiry to set on the lease,
// some backends may ignore expires based on the implementation
// in case if the lease managed server side
func (s *Reporter) KeepAlive(ctx context.Context, lease Lease, expires time.Time) error {
	start := s.Clock().Now()
	err := s.Backend.KeepAlive(ctx, lease, expires)
	writeLatencies.WithLabelValues(s.Component).Observe(time.Since(start).Seconds())
	writeRequests.WithLabelValues(s.Component).Inc()
	if err != nil && !trace.IsNotFound(err) {
		writeRequestsFailed.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpPut, lease.Key, nil)
	return err
}

// NewWatcher returns a new event watcher
func (s *Reporter) NewWatcher(ctx context.Context, watch Watch) (Watcher, error) {
	w, err := s.Backend.NewWatcher(ctx, watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewReporterWatcher(ctx, s.Component, w), nil
}

// Close releases the resources taken up by this backend
func (s *Reporter) Close() error {
	return s.Backend.Close()
}

// CloseWatchers closes all the watchers
// without closing the backend
func (s *Reporter) CloseWatchers() {
	s.Backend.CloseWatchers()
}

// Clock returns clock used by this backend
func (s *Reporter) Clock() clockwork.Clock {
	return s.Backend.Clock()
}

// Migrate runs the necessary data migrations for this backend.
func (s *Reporter) Migrate(ctx context.Context) error { return s.Backend.Migrate(ctx) }

type topRequestsCacheKey struct {
	component string
	key       string
	isRange   string
}

// trackRequests tracks top requests, endKey is supplied for ranges
func (s *Reporter) trackRequest(opType types.OpType, key []byte, endKey []byte) {
	if len(key) == 0 {
		return
	}
	keyLabel := buildKeyLabel(key, sensitiveBackendPrefixes)
	rangeSuffix := teleport.TagFalse
	if len(endKey) != 0 {
		// Range denotes range queries in stat entry
		rangeSuffix = teleport.TagTrue
	}

	s.topRequestsCache.Add(topRequestsCacheKey{
		component: s.Component,
		key:       keyLabel,
		isRange:   rangeSuffix,
	}, struct{}{})
	counter, err := requests.GetMetricWithLabelValues(s.Component, keyLabel, rangeSuffix)
	if err != nil {
		log.Warningf("Failed to get counter: %v", err)
		return
	}
	counter.Inc()
}

// buildKeyLabel builds the key label for storing to the backend. The last
// portion of the key is scrambled if it is determined to be sensitive based
// on sensitivePrefixes.
func buildKeyLabel(key []byte, sensitivePrefixes []string) string {
	// Take just the first two parts, otherwise too many distinct requests
	// can end up in the map.
	parts := bytes.Split(key, []byte{Separator})
	if len(parts) > 3 {
		parts = parts[:3]
	}
	if len(parts) < 3 || len(parts[0]) != 0 {
		return string(bytes.Join(parts, []byte{Separator}))
	}

	if utils.SliceContainsStr(sensitivePrefixes, string(parts[1])) {
		hiddenBefore := int(math.Floor(0.75 * float64(len(parts[2]))))
		asterisks := bytes.Repeat([]byte("*"), hiddenBefore)
		parts[2] = append(asterisks, parts[2][hiddenBefore:]...)
	}
	return string(bytes.Join(parts, []byte{Separator}))
}

// sensitiveBackendPrefixes is a list of backend request prefixes preceding
// sensitive values.
var sensitiveBackendPrefixes = []string{
	"tokens",
	"resetpasswordtokens",
	"adduseru2fchallenges",
	"access_requests",
}

// ReporterWatcher is a wrapper around backend
// watcher that reports events
type ReporterWatcher struct {
	Watcher
	Component string
}

// NewReporterWatcher creates new reporter watcher instance
func NewReporterWatcher(ctx context.Context, component string, w Watcher) *ReporterWatcher {
	rw := &ReporterWatcher{
		Watcher:   w,
		Component: component,
	}
	go rw.watch(ctx)
	return rw
}

func (r *ReporterWatcher) watch(ctx context.Context) {
	watchers.WithLabelValues(r.Component).Inc()
	defer watchers.WithLabelValues(r.Component).Dec()
	select {
	case <-r.Done():
		return
	case <-ctx.Done():
		return
	}
}

var (
	requests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendRequests,
			Help: "Number of write requests to the backend",
		},
		[]string{teleport.ComponentLabel, teleport.TagReq, teleport.TagRange},
	)
	watchers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: teleport.MetricBackendWatchers,
			Help: "Number of active backend watchers",
		},
		[]string{teleport.ComponentLabel},
	)
	watcherQueues = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: teleport.MetricBackendWatcherQueues,
			Help: "Watcher queue sizes",
		},
		[]string{teleport.ComponentLabel},
	)
	writeRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendWriteRequests,
			Help: "Number of write requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	writeRequestsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendWriteFailedRequests,
			Help: "Number of failed write requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	batchWriteRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendBatchWriteRequests,
			Help: "Number of batch write requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	batchWriteRequestsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendBatchFailedWriteRequests,
			Help: "Number of failed write requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	readRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendReadRequests,
			Help: "Number of read requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	readRequestsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendFailedReadRequests,
			Help: "Number of failed read requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	batchReadRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendBatchReadRequests,
			Help: "Number of read requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	batchReadRequestsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: teleport.MetricBackendBatchFailedReadRequests,
			Help: "Number of failed read requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	writeLatencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: teleport.MetricBackendWriteHistogram,
			Help: "Latency for backend write operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
		[]string{teleport.ComponentLabel},
	)
	batchWriteLatencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: teleport.MetricBackendBatchWriteHistogram,
			Help: "Latency for backend batch write operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
		[]string{teleport.ComponentLabel},
	)
	batchReadLatencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: teleport.MetricBackendBatchReadHistogram,
			Help: "Latency for batch read operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
		[]string{teleport.ComponentLabel},
	)
	readLatencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: teleport.MetricBackendReadHistogram,
			Help: "Latency for read operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
		[]string{teleport.ComponentLabel},
	)

	prometheusCollectors = []prometheus.Collector{
		watchers, watcherQueues, requests, writeRequests,
		writeRequestsFailed, batchWriteRequests, batchWriteRequestsFailed, readRequests,
		readRequestsFailed, batchReadRequests, batchReadRequestsFailed, writeLatencies,
		batchWriteLatencies, batchReadLatencies, readLatencies,
	}
)
