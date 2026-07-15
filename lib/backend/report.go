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
	"iter"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/backendmetrics"
	"github.com/gravitational/teleport/lib/observability/tracing"
	logutils "github.com/gravitational/teleport/lib/utils/log"
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
	// Registerer is used to register prometheus metrics.
	Registerer prometheus.Registerer
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
	if r.Registerer == nil {
		r.Registerer = prometheus.DefaultRegisterer
	}
	return nil
}

var _ Backend = (*Reporter)(nil)
var _ BatchDeleter = (*Reporter)(nil)
var _ BatchPutter = (*Reporter)(nil)

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

	slowRangeLogLimiter *rate.Limiter

	// Component-scoped metrics, initialized to zero at construction.
	requests                        *prometheus.CounterVec
	readRequests                    prometheus.Counter
	readRequestsFailed              prometheus.Counter
	reads                           prometheus.Counter
	readLatencies                   prometheus.Observer
	batchReadRequests               prometheus.Counter
	batchReadRequestsFailed         prometheus.Counter
	batchReadLatencies              prometheus.Observer
	writeRequests                   prometheus.Counter
	writeRequestsFailed             prometheus.Counter
	writeRequestsFailedPrecondition prometheus.Counter
	writes                          prometheus.Counter
	writeLatencies                  prometheus.Observer
	batchWriteRequests              prometheus.Counter
	batchWriteRequestsFailed        prometheus.Counter
	batchWriteLatencies             prometheus.Observer
	atomicWriteRequests             prometheus.Counter
	atomicWriteRequestsFailed       prometheus.Counter
	atomicWriteConditionFailed      prometheus.Counter
	atomicWriteLatencies            prometheus.Observer
	atomicWriteSize                 prometheus.Observer
	streamingRequests               prometheus.Counter
	streamingRequestsFailed         prometheus.Counter
}

// NewReporter returns a new Reporter.
func NewReporter(cfg ReporterConfig) (*Reporter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := backendmetrics.RegisterCollectors(cfg.Registerer); err != nil {
		return nil, trace.Wrap(err)
	}

	cache, err := lru.NewWithEvict(cfg.TopRequestsCount, func(labels topRequestsCacheKey, value struct{}) {
		// Evict the key from requests metric.
		backendmetrics.Requests.DeleteLabelValues(labels.component, labels.key, labels.isRange)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Initialize component-scoped metrics to zero so they exist in Prometheus
	// from startup, before the first increment.
	r := &Reporter{
		ReporterConfig:      cfg,
		topRequestsCache:    cache,
		slowRangeLogLimiter: rate.NewLimiter(rate.Every(time.Minute), 12),

		requests:                        backendmetrics.Requests.MustCurryWith(prometheus.Labels{teleport.ComponentLabel: cfg.Component}),
		readRequests:                    backendmetrics.ReadRequests.WithLabelValues(cfg.Component),
		readRequestsFailed:              backendmetrics.ReadRequestsFailed.WithLabelValues(cfg.Component),
		reads:                           backendmetrics.Reads.WithLabelValues(cfg.Component),
		readLatencies:                   backendmetrics.ReadLatencies.WithLabelValues(cfg.Component),
		batchReadRequests:               backendmetrics.BatchReadRequests.WithLabelValues(cfg.Component),
		batchReadRequestsFailed:         backendmetrics.BatchReadRequestsFailed.WithLabelValues(cfg.Component),
		batchReadLatencies:              backendmetrics.BatchReadLatencies.WithLabelValues(cfg.Component),
		writeRequests:                   backendmetrics.WriteRequests.WithLabelValues(cfg.Component),
		writeRequestsFailed:             backendmetrics.WriteRequestsFailed.WithLabelValues(cfg.Component),
		writeRequestsFailedPrecondition: backendmetrics.WriteRequestsFailedPrecondition.WithLabelValues(cfg.Component),
		writes:                          backendmetrics.Writes.WithLabelValues(cfg.Component),
		writeLatencies:                  backendmetrics.WriteLatencies.WithLabelValues(cfg.Component),
		batchWriteRequests:              backendmetrics.BatchWriteRequests.WithLabelValues(cfg.Component),
		batchWriteRequestsFailed:        backendmetrics.BatchWriteRequestsFailed.WithLabelValues(cfg.Component),
		batchWriteLatencies:             backendmetrics.BatchWriteLatencies.WithLabelValues(cfg.Component),
		atomicWriteRequests:             backendmetrics.AtomicWriteRequests.WithLabelValues(cfg.Component),
		atomicWriteRequestsFailed:       backendmetrics.AtomicWriteRequestsFailed.WithLabelValues(cfg.Component),
		atomicWriteConditionFailed:      backendmetrics.AtomicWriteConditionFailed.WithLabelValues(cfg.Component),
		atomicWriteLatencies:            backendmetrics.AtomicWriteLatencies.WithLabelValues(cfg.Component),
		atomicWriteSize:                 backendmetrics.AtomicWriteSize.WithLabelValues(cfg.Component),
		streamingRequests:               backendmetrics.StreamingRequests.WithLabelValues(cfg.Component),
		streamingRequestsFailed:         backendmetrics.StreamingRequestsFailed.WithLabelValues(cfg.Component),
	}

	// Also initialize metrics used outside Reporter methods.
	backendmetrics.Watchers.WithLabelValues(cfg.Component)
	backendmetrics.WatcherQueues.WithLabelValues(cfg.Component)
	backendmetrics.AtomicWriteContention.WithLabelValues(cfg.Component)

	return r, nil
}

func (s *Reporter) GetName() string {
	return s.Backend.GetName()
}

// GetRange returns query range
func (s *Reporter) GetRange(ctx context.Context, startKey, endKey Key, limit int) (*GetResult, error) {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/GetRange",
		oteltrace.WithAttributes(
			attribute.Int("limit", limit),
			attribute.String("start_key", startKey.String()),
			attribute.String("end_key", endKey.String()),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	res, err := s.Backend.GetRange(ctx, startKey, endKey, limit)
	s.batchReadLatencies.Observe(s.Clock().Since(start).Seconds())
	s.batchReadRequests.Inc()
	if err != nil {
		s.batchReadRequestsFailed.Inc()
	} else {
		s.reads.Add(float64(len(res.Items)))
	}
	s.trackRequest(ctx, types.OpGet, startKey, endKey)
	end := s.Clock().Now()
	if d := end.Sub(start); d > time.Second*3 {
		if s.slowRangeLogLimiter.AllowN(end, 1) {
			slog.WarnContext(ctx, "slow GetRange request", "start_key", startKey.String(), "end_key", endKey.String(), "limit", limit, "duration", logutils.StringerAttr(d))
		}
	}
	return res, err
}

func (s *Reporter) Items(ctx context.Context, params ItemsParams) iter.Seq2[Item, error] {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/Items",
		oteltrace.WithAttributes(
			attribute.Int("limit", params.Limit),
			attribute.String("start_key", params.StartKey.String()),
			attribute.String("end_key", params.EndKey.String()),
		),
	)
	defer span.End()

	return func(yield func(Item, error) bool) {
		var count int
		defer func() {
			s.trackRequest(ctx, types.OpGet, params.StartKey, params.EndKey)
			s.streamingRequests.Inc()
			s.reads.Add(float64(count))

		}()
		for item, err := range s.Backend.Items(ctx, params) {
			if err != nil {
				s.streamingRequestsFailed.Inc()
			}

			count++
			if !yield(item, err) || err != nil {
				return
			}
		}
	}
}

// Create creates item if it does not exist
func (s *Reporter) Create(ctx context.Context, i Item) (*Lease, error) {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/Create",
		oteltrace.WithAttributes(
			attribute.String("revision", i.Revision),
			attribute.String("key", i.Key.String()),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	lease, err := s.Backend.Create(ctx, i)
	s.writeLatencies.Observe(s.Clock().Since(start).Seconds())
	s.writeRequests.Inc()
	if err != nil {
		s.writeRequestsFailed.Inc()
		if trace.IsAlreadyExists(err) {
			s.writeRequestsFailedPrecondition.Inc()
		}
	} else {
		s.writes.Inc()
	}
	s.trackRequest(ctx, types.OpPut, i.Key, Key{})
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
			attribute.String("key", i.Key.String()),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	lease, err := s.Backend.Put(ctx, i)
	s.writeLatencies.Observe(s.Clock().Since(start).Seconds())
	s.writeRequests.Inc()
	if err != nil {
		s.writeRequestsFailed.Inc()
	} else {
		s.writes.Inc()
	}
	s.trackRequest(ctx, types.OpPut, i.Key, Key{})
	return lease, err
}

// PutBatch puts multiple values into backend.
func (s *Reporter) PutBatch(ctx context.Context, items []Item) ([]string, error) {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/PutBatch",
		oteltrace.WithAttributes(
			attribute.Int("batch_size", len(items)),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	revisions, err := PutBatch(ctx, s.Backend, items)
	s.batchWriteLatencies.Observe(s.Clock().Since(start).Seconds())
	s.batchWriteRequests.Inc()
	if err != nil {
		s.batchWriteRequestsFailed.Inc()
	} else {
		s.writes.Add(float64(len(items)))
	}
	for _, item := range items {
		s.trackRequest(ctx, types.OpPut, item.Key, Key{})
	}
	return revisions, err
}

// Update updates value in the backend
func (s *Reporter) Update(ctx context.Context, i Item) (*Lease, error) {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/Update",
		oteltrace.WithAttributes(
			attribute.String("revision", i.Revision),
			attribute.String("key", i.Key.String()),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	lease, err := s.Backend.Update(ctx, i)
	s.writeLatencies.Observe(s.Clock().Since(start).Seconds())
	s.writeRequests.Inc()
	if err != nil {
		s.writeRequestsFailed.Inc()
		if trace.IsNotFound(err) {
			s.writeRequestsFailedPrecondition.Inc()
		}
	} else {
		s.writes.Inc()
	}
	s.trackRequest(ctx, types.OpPut, i.Key, Key{})
	return lease, err
}

// ConditionalUpdate updates value in the backend if revisions match.
func (s *Reporter) ConditionalUpdate(ctx context.Context, i Item) (*Lease, error) {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/ConditionalUpdate",
		oteltrace.WithAttributes(
			attribute.String("revision", i.Revision),
			attribute.String("key", i.Key.String()),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	lease, err := s.Backend.ConditionalUpdate(ctx, i)
	s.writeLatencies.Observe(s.Clock().Since(start).Seconds())
	s.writeRequests.Inc()
	if err != nil {
		s.writeRequestsFailed.Inc()
		if errors.Is(err, ErrIncorrectRevision) {
			s.writeRequestsFailedPrecondition.Inc()
		}
	} else {
		s.writes.Inc()
	}
	s.trackRequest(ctx, types.OpPut, i.Key, Key{})
	return lease, err
}

// Get returns a single item or not found error
func (s *Reporter) Get(ctx context.Context, key Key) (*Item, error) {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/Get",
		oteltrace.WithAttributes(
			attribute.String("key", key.String()),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	item, err := s.Backend.Get(ctx, key)
	s.readLatencies.Observe(s.Clock().Since(start).Seconds())
	s.readRequests.Inc()
	s.reads.Inc()
	if err != nil && !trace.IsNotFound(err) {
		s.readRequestsFailed.Inc()
	}
	s.trackRequest(ctx, types.OpGet, key, Key{})
	return item, err
}

// CompareAndSwap compares item with existing item
// and replaces is with replaceWith item
func (s *Reporter) CompareAndSwap(ctx context.Context, expected Item, replaceWith Item) (*Lease, error) {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/CompareAndSwap",
		oteltrace.WithAttributes(
			attribute.String("key", expected.Key.String()),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	lease, err := s.Backend.CompareAndSwap(ctx, expected, replaceWith)
	s.writeLatencies.Observe(s.Clock().Since(start).Seconds())
	s.writeRequests.Inc()
	if err != nil {
		s.writeRequestsFailed.Inc()
		if trace.IsNotFound(err) || trace.IsCompareFailed(err) {
			s.writeRequestsFailedPrecondition.Inc()
		}
	} else {
		s.writes.Inc()
	}
	s.trackRequest(ctx, types.OpPut, expected.Key, Key{})
	return lease, err
}

// Delete deletes item by key
func (s *Reporter) Delete(ctx context.Context, key Key) error {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/Delete",
		oteltrace.WithAttributes(
			attribute.String("key", key.String()),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	err := s.Backend.Delete(ctx, key)
	s.writeLatencies.Observe(s.Clock().Since(start).Seconds())
	s.writeRequests.Inc()
	if err != nil {
		s.writeRequestsFailed.Inc()
		if trace.IsNotFound(err) {
			s.writeRequestsFailedPrecondition.Inc()
		}
	} else {
		s.writes.Inc()
	}
	s.trackRequest(ctx, types.OpDelete, key, Key{})
	return err
}

// ConditionalDelete deletes the item by key if the revision matches the stored revision.
func (s *Reporter) ConditionalDelete(ctx context.Context, key Key, revision string) error {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/ConditionalDelete",
		oteltrace.WithAttributes(
			attribute.String("revision", revision),
			attribute.String("key", key.String()),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	err := s.Backend.ConditionalDelete(ctx, key, revision)
	s.writeLatencies.Observe(s.Clock().Since(start).Seconds())
	s.writeRequests.Inc()
	if err != nil {
		s.writeRequestsFailed.Inc()
		if trace.IsNotFound(err) {
			s.writeRequestsFailedPrecondition.Inc()
		}
	} else {
		s.writes.Inc()
	}
	s.trackRequest(ctx, types.OpDelete, key, Key{})
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
	s.writeLatencies.Observe(elapsed)
	s.atomicWriteLatencies.Observe(elapsed)

	s.writeRequests.Inc()
	s.atomicWriteRequests.Inc()
	s.atomicWriteSize.Observe(float64(len(condacts)))
	if err != nil {
		s.writeRequestsFailed.Inc()
		s.atomicWriteRequestsFailed.Inc()
		if errors.Is(err, ErrConditionFailed) {
			s.writeRequestsFailedPrecondition.Inc()
			s.atomicWriteConditionFailed.Inc()
		}
	}

	var writeTotal int
	for _, ca := range condacts {
		switch ca.Action.Kind {
		case KindPut:
			writeTotal++
			s.trackRequest(ctx, types.OpPut, ca.Key, Key{})
		case KindDelete:
			writeTotal++
			s.trackRequest(ctx, types.OpDelete, ca.Key, Key{})
		default:
			// ignore other variants
		}
	}

	if err == nil {
		s.writes.Add(float64(writeTotal))
	}
	return
}

// DeleteBatch deletes multiple keys from the backend.
func (s *Reporter) DeleteBatch(ctx context.Context, keys []Key) error {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/DeleteBatch",
		oteltrace.WithAttributes(
			attribute.Int("batch_size", len(keys)),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	err := DeleteBatch(ctx, s.Backend, keys)
	s.batchWriteLatencies.Observe(s.Clock().Since(start).Seconds())
	s.batchWriteRequests.Inc()
	if err != nil {
		s.batchWriteRequestsFailed.Inc()
	} else {
		s.writes.Add(float64(len(keys)))
	}
	for _, key := range keys {
		s.trackRequest(ctx, types.OpDelete, key, Key{})
	}
	return err
}

// DeleteRange deletes range of items
func (s *Reporter) DeleteRange(ctx context.Context, startKey, endKey Key) error {
	ctx, span := s.Tracer.Start(
		ctx,
		"backend/DeleteRange",
		oteltrace.WithAttributes(
			attribute.String("start_key", startKey.String()),
			attribute.String("end_key", endKey.String()),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	err := s.Backend.DeleteRange(ctx, startKey, endKey)
	s.batchWriteLatencies.Observe(s.Clock().Since(start).Seconds())
	s.batchWriteRequests.Inc()
	if err != nil && !trace.IsNotFound(err) {
		s.batchWriteRequestsFailed.Inc()
	}
	s.trackRequest(ctx, types.OpDelete, startKey, endKey)
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
			attribute.String("key", lease.Key.String()),
		),
	)
	defer span.End()

	start := s.Clock().Now()
	err := s.Backend.KeepAlive(ctx, lease, expires)
	s.writeLatencies.Observe(s.Clock().Since(start).Seconds())
	s.writeRequests.Inc()
	if err != nil {
		s.writeRequestsFailed.Inc()
		if trace.IsNotFound(err) {
			s.writeRequestsFailedPrecondition.Inc()
		}
	} else {
		s.writes.Inc()
	}
	s.trackRequest(ctx, types.OpPut, lease.Key, Key{})
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
func (s *Reporter) trackRequest(ctx context.Context, opType types.OpType, key Key, endKey Key) {
	if len(key.s) == 0 {
		return
	}
	keyLabel := buildKeyLabel(key.String(), sensitiveBackendPrefixes, singletonBackendPrefixes, len(endKey.s) != 0)
	rangeSuffix := teleport.TagFalse
	if len(endKey.s) != 0 {
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

	counter, err := s.requests.GetMetricWithLabelValues(keyLabel, rangeSuffix)
	if err != nil {
		slog.WarnContext(ctx, "Failed to get prometheus counter", "error", err)
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
		parts[realStart+1] = MaskKeyName(parts[realStart+1])
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
	backendmetrics.Watchers.WithLabelValues(r.Component).Inc()
	defer backendmetrics.Watchers.WithLabelValues(r.Component).Dec()
	select {
	case <-r.Done():
		return
	case <-ctx.Done():
		return
	}
}
