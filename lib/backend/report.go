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

package backend

import (
	"context"
	"errors"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/observability/tracing"
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
	// Tracer is used to create spans
	Tracer oteltrace.Tracer
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
	if r.Tracer == nil {
		r.Tracer = tracing.NoopTracer(teleport.ComponentBackend)
	}
	return nil
}

var _ Backend = (*Reporter)(nil)

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
	topRequestsCache *lru.Cache[topRequestsCacheKey, struct{}]
}

// NewReporter returns a new Reporter.
func NewReporter(cfg ReporterConfig) (*Reporter, error) {
	err := metrics.RegisterPrometheusCollectors(prometheusCollectors...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cache, err := lru.NewWithEvict(cfg.TopRequestsCount, func(labels topRequestsCacheKey, value struct{}) {
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

func (s *Reporter) GetName() string {
	return s.Backend.GetName()
}

// GetRange returns query range
func (s *Reporter) GetRange(ctx context.Context, startKey []byte, endKey []byte, limit int) (*GetResult, error) {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/GetRange",
		oteltrace.WithAttributes(
			attribute.Int("limit", limit),
			attribute.String("start_key", string(startKey)),
			attribute.String("end_key", string(endKey)),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	res, err := s.Backend.GetRange(ctx, startKey, endKey, limit)
	batchReadLatencies.WithLabelValues(s.Component).Observe(s.Clock().Since(start).Seconds())
	batchReadRequests.WithLabelValues(s.Component).Inc()
	if err != nil {
		batchReadRequestsFailed.WithLabelValues(s.Component).Inc()
	} else {
		reads.WithLabelValues(s.Component).Add(float64(len(res.Items)))
	}
	s.trackRequest(types.OpGet, startKey, endKey)
	return res, err
}

// Create creates item if it does not exist
func (s *Reporter) Create(ctx context.Context, i Item) (*Lease, error) {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/Create",
		oteltrace.WithAttributes(
			attribute.String("revision", i.Revision),
			attribute.String("key", string(i.Key)),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	lease, err := s.Backend.Create(ctx, i)
	writeLatencies.WithLabelValues(s.Component).Observe(s.Clock().Since(start).Seconds())
	writeRequests.WithLabelValues(s.Component).Inc()
	if err != nil {
		writeRequestsFailed.WithLabelValues(s.Component).Inc()
		if trace.IsAlreadyExists(err) {
			writeRequestsFailedPrecondition.WithLabelValues(s.Component).Inc()
		}
	} else {
		writes.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpPut, i.Key, nil)
	return lease, err
}

// Put puts value into backend (creates if it does not
// exists, updates it otherwise)
func (s *Reporter) Put(ctx context.Context, i Item) (*Lease, error) {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/Put",
		oteltrace.WithAttributes(
			attribute.String("revision", i.Revision),
			attribute.String("key", string(i.Key)),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	lease, err := s.Backend.Put(ctx, i)
	writeLatencies.WithLabelValues(s.Component).Observe(s.Clock().Since(start).Seconds())
	writeRequests.WithLabelValues(s.Component).Inc()
	if err != nil {
		writeRequestsFailed.WithLabelValues(s.Component).Inc()
	} else {
		writes.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpPut, i.Key, nil)
	return lease, err
}

// Update updates value in the backend
func (s *Reporter) Update(ctx context.Context, i Item) (*Lease, error) {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/Update",
		oteltrace.WithAttributes(
			attribute.String("revision", i.Revision),
			attribute.String("key", string(i.Key)),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	lease, err := s.Backend.Update(ctx, i)
	writeLatencies.WithLabelValues(s.Component).Observe(s.Clock().Since(start).Seconds())
	writeRequests.WithLabelValues(s.Component).Inc()
	if err != nil {
		writeRequestsFailed.WithLabelValues(s.Component).Inc()
		if trace.IsNotFound(err) {
			writeRequestsFailedPrecondition.WithLabelValues(s.Component).Inc()
		}
	} else {
		writes.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpPut, i.Key, nil)
	return lease, err
}

// ConditionalUpdate updates value in the backend if revisions match.
func (s *Reporter) ConditionalUpdate(ctx context.Context, i Item) (*Lease, error) {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/ConditionalUpdate",
		oteltrace.WithAttributes(
			attribute.String("revision", i.Revision),
			attribute.String("key", string(i.Key)),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	lease, err := s.Backend.ConditionalUpdate(ctx, i)
	writeLatencies.WithLabelValues(s.Component).Observe(s.Clock().Since(start).Seconds())
	writeRequests.WithLabelValues(s.Component).Inc()
	if err != nil {
		writeRequestsFailed.WithLabelValues(s.Component).Inc()
		if errors.Is(err, ErrIncorrectRevision) {
			writeRequestsFailedPrecondition.WithLabelValues(s.Component).Inc()
		}
	} else {
		writes.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpPut, i.Key, nil)
	return lease, err
}

// Get returns a single item or not found error
func (s *Reporter) Get(ctx context.Context, key []byte) (*Item, error) {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/Get",
		oteltrace.WithAttributes(
			attribute.String("key", string(key)),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	item, err := s.Backend.Get(ctx, key)
	readLatencies.WithLabelValues(s.Component).Observe(s.Clock().Since(start).Seconds())
	readRequests.WithLabelValues(s.Component).Inc()
	reads.WithLabelValues(s.Component).Inc()
	if err != nil && !trace.IsNotFound(err) {
		readRequestsFailed.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpGet, key, nil)
	return item, err
}

// CompareAndSwap compares item with existing item
// and replaces is with replaceWith item
func (s *Reporter) CompareAndSwap(ctx context.Context, expected Item, replaceWith Item) (*Lease, error) {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/CompareAndSwap",
		oteltrace.WithAttributes(
			attribute.String("key", string(expected.Key)),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	lease, err := s.Backend.CompareAndSwap(ctx, expected, replaceWith)
	writeLatencies.WithLabelValues(s.Component).Observe(s.Clock().Since(start).Seconds())
	writeRequests.WithLabelValues(s.Component).Inc()
	if err != nil {
		writeRequestsFailed.WithLabelValues(s.Component).Inc()
		if trace.IsNotFound(err) || trace.IsCompareFailed(err) {
			writeRequestsFailedPrecondition.WithLabelValues(s.Component).Inc()
		}
	} else {
		writes.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpPut, expected.Key, nil)
	return lease, err
}

// Delete deletes item by key
func (s *Reporter) Delete(ctx context.Context, key []byte) error {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/Delete",
		oteltrace.WithAttributes(
			attribute.String("key", string(key)),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	err := s.Backend.Delete(ctx, key)
	writeLatencies.WithLabelValues(s.Component).Observe(s.Clock().Since(start).Seconds())
	writeRequests.WithLabelValues(s.Component).Inc()
	if err != nil {
		writeRequestsFailed.WithLabelValues(s.Component).Inc()
		if trace.IsNotFound(err) {
			writeRequestsFailedPrecondition.WithLabelValues(s.Component).Inc()
		}
	} else {
		writes.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpDelete, key, nil)
	return err
}

// ConditionalDelete deletes the item by key if the revision matches the stored revision.
func (s *Reporter) ConditionalDelete(ctx context.Context, key []byte, revision string) error {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/ConditionalDelete",
		oteltrace.WithAttributes(
			attribute.String("revision", revision),
			attribute.String("key", string(key)),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	err := s.Backend.ConditionalDelete(ctx, key, revision)
	writeLatencies.WithLabelValues(s.Component).Observe(s.Clock().Since(start).Seconds())
	writeRequests.WithLabelValues(s.Component).Inc()
	if err != nil {
		writeRequestsFailed.WithLabelValues(s.Component).Inc()
		if trace.IsNotFound(err) {
			writeRequestsFailedPrecondition.WithLabelValues(s.Component).Inc()
		}
	} else {
		writes.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpDelete, key, nil)
	return err
}

// AtomicWrite implements batch conditional updates s.t. no writes occur unless all
// conditions hold.
func (s *Reporter) AtomicWrite(ctx context.Context, condacts []ConditionalAction) (revision string, err error) {
	// note: the atomic write method's metrics are counted toward both the general 'write'
	// metrics as well as equivalent metrics specific to atomic write.
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/AtomicWrite",
		oteltrace.WithAttributes(
			attribute.Int("condacts", len(condacts)),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	revision, err = s.Backend.AtomicWrite(ctx, condacts)

	elapsed := s.Clock().Since(start).Seconds()
	writeLatencies.WithLabelValues(s.Component).Observe(elapsed)
	atomicWriteLatencies.WithLabelValues(s.Component).Observe(elapsed)

	writeRequests.WithLabelValues(s.Component).Inc()
	atomicWriteRequests.WithLabelValues(s.Component).Inc()
	atomicWriteSize.WithLabelValues(s.Component).Observe(float64(len(condacts)))
	if err != nil {
		writeRequestsFailed.WithLabelValues(s.Component).Inc()
		atomicWriteRequestsFailed.WithLabelValues(s.Component).Inc()
		if errors.Is(err, ErrConditionFailed) {
			writeRequestsFailedPrecondition.WithLabelValues(s.Component).Inc()
			atomicWriteConditionFailed.WithLabelValues(s.Component).Inc()
		}
	}

	var writeTotal int
	for _, ca := range condacts {
		switch ca.Action.Kind {
		case KindPut:
			writeTotal++
			s.trackRequest(types.OpPut, ca.Key, nil)
		case KindDelete:
			writeTotal++
			s.trackRequest(types.OpDelete, ca.Key, nil)
		default:
			// ignore other variants
		}
	}

	if err == nil {
		writes.WithLabelValues(s.Component).Add(float64(writeTotal))
	}
	return
}

// DeleteRange deletes range of items
func (s *Reporter) DeleteRange(ctx context.Context, startKey []byte, endKey []byte) error {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/DeleteRange",
		oteltrace.WithAttributes(
			attribute.String("start_key", string(startKey)),
			attribute.String("end_key", string(endKey)),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	err := s.Backend.DeleteRange(ctx, startKey, endKey)
	batchWriteLatencies.WithLabelValues(s.Component).Observe(s.Clock().Since(start).Seconds())
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
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/KeepAlive",
		oteltrace.WithAttributes(
			attribute.String("revision", lease.Revision),
			attribute.Int64("lease", lease.ID),
			attribute.String("key", string(lease.Key)),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	err := s.Backend.KeepAlive(ctx, lease, expires)
	writeLatencies.WithLabelValues(s.Component).Observe(s.Clock().Since(start).Seconds())
	writeRequests.WithLabelValues(s.Component).Inc()
	if err != nil {
		writeRequestsFailed.WithLabelValues(s.Component).Inc()
		if trace.IsNotFound(err) {
			writeRequestsFailedPrecondition.WithLabelValues(s.Component).Inc()
		}
	} else {
		writes.WithLabelValues(s.Component).Inc()
	}
	s.trackRequest(types.OpPut, lease.Key, nil)
	return err
}

// NewWatcher returns a new event watcher
func (s *Reporter) NewWatcher(ctx context.Context, watch Watch) (Watcher, error) {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/NewWatcher",
		oteltrace.WithAttributes(
			attribute.String("name", watch.Name),
		),
	)
	defer span.End()

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
	keyLabel := buildKeyLabel(string(key), sensitiveBackendPrefixes, singletonBackendPrefixes, len(endKey) != 0)
	rangeSuffix := teleport.TagFalse
	if len(endKey) != 0 {
		// Range denotes range queries in stat entry
		rangeSuffix = teleport.TagTrue
	}

	cacheKey := topRequestsCacheKey{
		component: s.Component,
		key:       keyLabel,
		isRange:   rangeSuffix,
	}
	// We need to do ContainsOrAdd and then Get because if we do Add we hit
	// https://github.com/hashicorp/golang-lru/issues/141 which can cause a
	// memory leak in certain workloads (where we keep overwriting the same
	// key); it's not clear if Add to overwrite would be the correct thing to do
	// here anyway, as we use LRU eviction to delete unused metrics, but
	// overwriting might cause an eviction of the same metric we are about to
	// bump up in freshness, which is obviously wrong
	if ok, _ := s.topRequestsCache.ContainsOrAdd(cacheKey, struct{}{}); ok {
		// Refresh the key's position in the LRU cache, if it was already in it.
		s.topRequestsCache.Get(cacheKey)
	}

	counter, err := requests.GetMetricWithLabelValues(s.Component, keyLabel, rangeSuffix)
	if err != nil {
		log.Warningf("Failed to get counter: %v", err)
		return
	}
	counter.Inc()
}

// buildKeyLabel builds the key label for storing to the backend. The key's name
// is masked if it is determined to be sensitive based on sensitivePrefixes.
func buildKeyLabel(key string, sensitivePrefixes, singletonPrefixes []string, isRange bool) string {
	parts := strings.Split(key, string(Separator))

	finalLen := len(parts)
	var realStart int

	// skip leading space if one exists so that we can consistently access path segments by
	// index regardless of whether or not the specific path has a leading separator.
	if finalLen-realStart > 1 && parts[realStart] == "" {
		realStart = 1
	}

	// trim trailing space for consistency
	if finalLen-realStart > 1 && parts[finalLen-1] == "" {
		finalLen -= 1
	}

	// we typically always want to trim the final element from any multipart path to avoid tracking individual
	// resources. the two exceptions are if the path originates from a range request, or if the first element
	// in the path is a known singleton range.
	if finalLen-realStart > 1 && !isRange && !slices.Contains(singletonPrefixes, parts[realStart]) {
		finalLen -= 1
	}

	// paths may contain at most two segments excluding leading blank
	if finalLen-realStart > 2 {
		finalLen = realStart + 2
	}

	// if the first non-empty segment is a secret range and there are at least two non-empty
	// segments, then the second non-empty segment should be masked.
	if finalLen-realStart > 1 && slices.Contains(sensitivePrefixes, parts[realStart]) {
		parts[realStart+1] = string(MaskKeyName(parts[realStart+1]))
	}

	return strings.Join(parts[:finalLen], string(Separator))
}

// sensitiveBackendPrefixes is a list of backend request prefixes preceding
// sensitive values.
var sensitiveBackendPrefixes = []string{
	"tokens",
	"usertoken",
	// Global passwordless challenges, keyed by challenge, as per
	// https://github.com/gravitational/teleport/blob/01775b73f138ff124ff0351209d629bb01836869/lib/services/local/users.go#L1510.
	"sessionData",
	"access_requests",
}

// singletonBackendPrefixes is a list of prefixes where its not necessary to trim the trailing
// path component automatically since the range only contains singleton values.
var singletonBackendPrefixes = []string{
	"cluster_configuration",
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
			Help: "Number of requests to the backend (reads, writes, and keepalives)",
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
	writes = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendWrites,
			Help:      "Number of individual items written to the backend",
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
	writeRequestsFailedPrecondition = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendWriteFailedPreconditionRequests,
			Help:      "Number of write requests that failed due to a precondition (existence, revision, value, etc)",
		},
		[]string{teleport.ComponentLabel},
	)
	atomicWriteRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendAtomicWriteRequests,
			Help:      "Number of atomic write requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	atomicWriteRequestsFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendAtomicWriteFailedRequests,
			Help:      "Number of failed atomic write requests to the backend",
		},
		[]string{teleport.ComponentLabel},
	)
	atomicWriteConditionFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendAtomicWriteConditionFailed,
			Help:      "Number of times an atomic write request results in condition failure",
		},
		[]string{teleport.ComponentLabel},
	)
	atomicWriteLatencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendAtomicWriteHistogram,
			Help:      "Latency for backend atomic write operations",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
		[]string{teleport.ComponentLabel},
	)
	atomicWriteSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendAtomicWriteSize,
			Help:      "Atomic write batch size",
			// buckets of the form 1, 2, 4, 8, 16, etc...
			Buckets: prometheus.ExponentialBuckets(1, 2, int(math.Sqrt(MaxAtomicWriteSize))),
		},
		[]string{teleport.ComponentLabel},
	)
	AtomicWriteContention = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendAtomicWriteContention,
			Help:      "Number of times atomic write requests experience contention",
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
	reads = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBackendReads,
			Help:      "Number of individual items read from the backend",
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
		writeRequestsFailedPrecondition,
		atomicWriteRequests, atomicWriteRequestsFailed, atomicWriteConditionFailed, atomicWriteLatencies,
		AtomicWriteContention, atomicWriteSize, reads, writes,
		batchWriteLatencies, batchReadLatencies, readLatencies,
	}
)
