// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package top

import (
	"cmp"
	"context"
	"fmt"
	"iter"
	"math"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// Report is a report rendered over the data
type Report struct {
	// Version is a report version
	Version string
	// Timestamp is the date when this report has been generated
	Timestamp time.Time
	// Hostname is the hostname of the report
	Hostname string
	// Process contains process stats
	Process ProcessStats
	// Go contains go runtime stats
	Go GoStats
	// Backend is a backend stats
	Backend *BackendStats
	// Cache is cache stats
	Cache *BackendStats
	// Cluster is cluster stats
	Cluster ClusterStats
	// Watcher is watcher stats
	Watcher *WatcherStats
	// Audit contains stats for audit event backends.
	Audit *AuditStats
}

// AuditStats contains metrics related to the audit log.
type AuditStats struct {
	// FailedEventsCounter tallies the frequency of failed events.
	FailedEventsCounter *Counter
	// FailedEventsBuffer contains the historical frequencies of
	// the FailedEventsCounter.
	FailedEventsBuffer *utils.CircularBuffer
	// EmittedEventsCounter tallies the frequency of all emitted events.
	EmittedEventsCounter *Counter
	// EmittedEventsBuffer contains the historical frequencies of
	// the EmittedEventsCounter.
	EmittedEventsBuffer *utils.CircularBuffer
	// EventSizeCounter tallies the frequency of all events.
	EventSizeCounter *Counter
	// EventSizeBuffer contains the historical sizes of
	// the EventSizeCounter.
	EventSizeBuffer *utils.CircularBuffer
	// EventsCounter tallies the frequency of trimmed events.
	TrimmedEventsCounter *Counter
	// TrimmedEventsBuffer contains the historical sizes of
	// the TrimmedEventsCounter.
	TrimmedEventsBuffer *utils.CircularBuffer
}

// WatcherStats contains watcher stats
type WatcherStats struct {
	// EventSize is an event size histogram
	EventSize Histogram
	// TopEvents is a collection of resources to their events
	TopEvents map[string]Event
	// EventsPerSecond is the events per sec buffer
	EventsPerSecond *utils.CircularBuffer
	// BytesPerSecond is the bytes per sec buffer
	BytesPerSecond *utils.CircularBuffer
}

// SortedTopEvents returns top events sorted either
// by frequency if frequency is present, or by count, if both
// frequency and count are identical then by name to preserve order
func (b *WatcherStats) SortedTopEvents() []Event {
	out := make([]Event, 0, len(b.TopEvents))
	for _, events := range b.TopEvents {
		out = append(out, events)
	}

	// Comparisons are inverted to ensure ordering is highest to lowest.
	slices.SortFunc(out, func(a, b Event) int {
		if a.GetFreq() != b.GetFreq() {
			return cmp.Compare(a.GetFreq(), b.GetFreq()) * -1
		}

		if a.Count != b.Count {
			return cmp.Compare(a.Count, b.Count) * -1
		}

		return cmp.Compare(a.Resource, b.Resource) * -1
	})
	return out
}

// Event is a watcher event stats
type Event struct {
	// Resource is the resource of the event
	Resource string
	// Size is the size of the serialized event
	Size float64
	// Counter maintains the count and the resource frequency
	Counter
}

// AverageSize returns the average size for the event
func (e Event) AverageSize() float64 {
	return e.Size / float64(e.Count)
}

// ProcessStats is a process statistics
type ProcessStats struct {
	// CPUSecondsTotal is a total user and system CPU time spent in seconds.
	CPUSecondsTotal float64
	// MaxFDs is the maximum number of open file descriptors.
	MaxFDs float64
	// OpenFDs is a number of open file descriptors.
	OpenFDs float64
	// ResidentMemoryBytes is a resident memory size in bytes.
	ResidentMemoryBytes float64
	// StartTime is a process start time
	StartTime time.Time
}

// GoStats is stats about go runtime
type GoStats struct {
	// Info is a runtime info (version, etc)
	Info string
	// Threads is a number of OS threads created.
	Threads float64
	// Goroutines is a number of goroutines that currently exist.
	Goroutines float64
	// Number of heap bytes allocated and still in use.
	HeapAllocBytes float64
	// Number of bytes allocated and still in use.
	AllocBytes float64
	// HeapObjects is a number of allocated objects.
	HeapObjects float64
}

// BackendStats contains backend stats
type BackendStats struct {
	// Read is a read latency histogram
	Read Histogram
	// BatchRead is a batch read latency histogram
	BatchRead Histogram
	// Write is a write latency histogram
	Write Histogram
	// BatchWrite is a batch write latency histogram
	BatchWrite Histogram
	// TopRequests is a collection of requests to
	// backend and their counts
	TopRequests map[RequestKey]Request
	// QueueSize is a queue size of the cache watcher
	QueueSize float64
}

// SortedTopRequests returns top requests sorted either
// by frequency if frequency is present, or by count, if both
// frequency and count are identical then by name to preserve order
func (b *BackendStats) SortedTopRequests() []Request {
	out := make([]Request, 0, len(b.TopRequests))
	for _, req := range b.TopRequests {
		out = append(out, req)
	}

	// Comparisons are inverted to ensure ordering is highest to lowest.
	slices.SortFunc(out, func(a, b Request) int {
		if a.GetFreq() != b.GetFreq() {
			return cmp.Compare(a.GetFreq(), b.GetFreq()) * -1
		}

		if a.Count != b.Count {
			return cmp.Compare(a.Count, b.Count) * -1
		}

		return cmp.Compare(a.Key.Key, b.Key.Key) * -1
	})
	return out
}

// ClusterStats contains some teleport specific stats
type ClusterStats struct {
	// InteractiveSessions is a number of active sessions.
	InteractiveSessions float64
	// RemoteClusters is a list of remote clusters and their status.
	RemoteClusters []RemoteCluster
	// GenerateRequests is a number of active generate requests
	GenerateRequests float64
	// GenerateRequestsCount is a total number of generate requests
	GenerateRequestsCount Counter
	// GenerateRequestThrottledCount is a total number of throttled generate
	// requests
	GenerateRequestsThrottledCount Counter
	// GenerateRequestsHistogram is a histogram of generate requests latencies
	GenerateRequestsHistogram Histogram
	// ActiveMigrations is a set of active migrations
	ActiveMigrations []string
	// Roles is the number of roles that exist in the cluster.
	Roles float64
}

// RemoteCluster is a remote cluster (or local cluster)
// connected to this cluster
type RemoteCluster struct {
	// Name is a cluster name
	Name string
	// Connected is true when cluster is connected
	Connected bool
}

// IsConnected returns user-friendly "connected"
// or "disconnected" cluster status
func (rc RemoteCluster) IsConnected() string {
	if rc.Connected {
		return "connected"
	}
	return "disconnected"
}

// RequestKey is a composite request Key
type RequestKey struct {
	// Range is set when it's a range request
	Range bool
	// Key is a backend key and operation
	Key string
}

// IsRange returns user-friendly "range" if
// request is a range request
func (r RequestKey) IsRange() string {
	if r.Range {
		return "range"
	}
	return ""
}

// Request is a backend request stats
type Request struct {
	// Key is a request key
	Key RequestKey
	// Counter maintains the count and the key access frequency
	Counter
}

// Counter contains count and frequency
type Counter struct {
	// Freq is a key access frequency in requests per second
	Freq *float64
	// Count is a last recorded count
	Count int64
}

// SetFreq sets counter frequency based on the previous value
// and the time period. SetFreq should be preffered over UpdateFreq
// when initializing a Counter from previous statistics.
func (c *Counter) SetFreq(prevCount Counter, period time.Duration) {
	if period == 0 {
		return
	}
	freq := float64(c.Count-prevCount.Count) / float64(period/time.Second)
	c.Freq = &freq
}

// UpdateFreq sets counter frequency based on the previous value
// and the time period. UpdateFreq should be preferred over SetFreq
// if the Counter is reused.
func (c *Counter) UpdateFreq(currentCount int64, period time.Duration) {
	if period == 0 {
		return
	}

	// Do not calculate the frequency until there are two data points.
	if c.Count == 0 && c.Freq == nil {
		c.Count = currentCount
		return
	}

	freq := float64(currentCount-c.Count) / float64(period/time.Second)
	c.Freq = &freq
	c.Count = currentCount
}

// GetFreq returns frequency of the request
func (c Counter) GetFreq() float64 {
	if c.Freq == nil {
		return 0
	}
	return *c.Freq
}

// Histogram is a histogram with buckets
type Histogram struct {
	// Count is a total number of elements counted
	Count int64
	// Sum is sum of all elements counted
	Sum float64
	// Buckets is a list of buckets
	Buckets []Bucket
}

// Percentile is a latency percentile
type Percentile struct {
	// Percentile is a percentile value
	Percentile float64
	// Value is a value of the percentile
	Value time.Duration
}

// Percentiles returns an iterator of the percentiles
// of the buckets within the historgram.
func (h Histogram) Percentiles() iter.Seq[Percentile] {
	return func(yield func(Percentile) bool) {
		if h.Count == 0 {
			return
		}

		for _, bucket := range h.Buckets {
			if bucket.Count == 0 {
				continue
			}
			if bucket.Count == h.Count || math.IsInf(bucket.UpperBound, 0) {
				yield(Percentile{
					Percentile: 100,
					Value:      time.Duration(bucket.UpperBound * float64(time.Second)),
				})
				return
			}

			if !yield(Percentile{
				Percentile: 100 * (float64(bucket.Count) / float64(h.Count)),
				Value:      time.Duration(bucket.UpperBound * float64(time.Second)),
			}) {
				return
			}
		}
	}
}

// Bucket is a histogram bucket
type Bucket struct {
	// Count is a count of elements in the bucket
	Count int64
	// UpperBound is an upper bound of the bucket
	UpperBound float64
}

func fetchAndGenerateReport(ctx context.Context, client *roundtrip.Client, prev *Report, period time.Duration) (*Report, error) {
	re, err := client.Get(ctx, client.Endpoint("metrics"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(trace.ConvertSystemError(err))
	}

	var parser expfmt.TextParser
	metrics, err := parser.TextToMetricFamilies(re.Reader())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return generateReport(metrics, prev, period)
}

func generateReport(metrics map[string]*dto.MetricFamily, prev *Report, period time.Duration) (*Report, error) {
	// format top backend requests
	hostname, _ := os.Hostname()
	re := Report{
		Version:   types.V1,
		Timestamp: time.Now().UTC(),
		Hostname:  hostname,
		Backend: &BackendStats{
			TopRequests: make(map[RequestKey]Request),
		},
		Cache: &BackendStats{
			TopRequests: make(map[RequestKey]Request),
		},
	}

	collectBackendStats := func(component string, stats *BackendStats, prevStats *BackendStats) {
		for _, req := range getRequests(component, metrics[teleport.MetricBackendRequests]) {
			if prev != nil {
				prevReq, ok := prevStats.TopRequests[req.Key]
				if ok {
					// if previous value is set, can calculate req / second
					req.SetFreq(prevReq.Counter, period)
				}
			}
			stats.TopRequests[req.Key] = req
		}
		stats.Read = getHistogram(metrics[teleport.MetricBackendReadHistogram], forLabel(component))
		stats.Write = getHistogram(metrics[teleport.MetricBackendWriteHistogram], forLabel(component))
		stats.BatchRead = getHistogram(metrics[teleport.MetricBackendBatchReadHistogram], forLabel(component))
		stats.BatchWrite = getHistogram(metrics[teleport.MetricBackendBatchWriteHistogram], forLabel(component))
	}

	var stats *BackendStats
	if prev != nil {
		stats = prev.Backend
	}
	collectBackendStats(teleport.ComponentBackend, re.Backend, stats)
	if prev != nil {
		stats = prev.Cache
	} else {
		stats = nil
	}
	collectBackendStats(teleport.ComponentCache, re.Cache, stats)
	re.Cache.QueueSize = getComponentGaugeValue(teleport.Component(teleport.ComponentAuth, teleport.ComponentCache),
		metrics[teleport.MetricBackendWatcherQueues])

	var watchStats *WatcherStats
	if prev != nil {
		watchStats = prev.Watcher
	}

	re.Watcher = getWatcherStats(metrics, watchStats, period)

	re.Process = ProcessStats{
		CPUSecondsTotal:     getGaugeValue(metrics[teleport.MetricProcessCPUSecondsTotal]),
		MaxFDs:              getGaugeValue(metrics[teleport.MetricProcessMaxFDs]),
		OpenFDs:             getGaugeValue(metrics[teleport.MetricProcessOpenFDs]),
		ResidentMemoryBytes: getGaugeValue(metrics[teleport.MetricProcessResidentMemoryBytes]),
		StartTime:           time.Unix(int64(getGaugeValue(metrics[teleport.MetricProcessStartTimeSeconds])), 0),
	}

	re.Go = GoStats{
		Info:           getLabels(metrics[teleport.MetricGoInfo]),
		Threads:        getGaugeValue(metrics[teleport.MetricGoThreads]),
		Goroutines:     getGaugeValue(metrics[teleport.MetricGoGoroutines]),
		AllocBytes:     getGaugeValue(metrics[teleport.MetricGoAllocBytes]),
		HeapAllocBytes: getGaugeValue(metrics[teleport.MetricGoHeapAllocBytes]),
		HeapObjects:    getGaugeValue(metrics[teleport.MetricGoHeapObjects]),
	}

	re.Cluster = ClusterStats{
		InteractiveSessions:            getGaugeValue(metrics[teleport.MetricServerInteractiveSessions]),
		RemoteClusters:                 getRemoteClusters(metrics[teleport.MetricRemoteClusters]),
		GenerateRequests:               getGaugeValue(metrics[teleport.MetricGenerateRequestsCurrent]),
		GenerateRequestsCount:          Counter{Count: getCounterValue(metrics[teleport.MetricGenerateRequests])},
		GenerateRequestsThrottledCount: Counter{Count: getCounterValue(metrics[teleport.MetricGenerateRequestsThrottled])},
		GenerateRequestsHistogram:      getHistogram(metrics[teleport.MetricGenerateRequestsHistogram], atIndex(0)),
		ActiveMigrations:               getActiveMigrations(metrics[prometheus.BuildFQName(teleport.MetricNamespace, "", teleport.MetricMigrations)]),
		Roles:                          getGaugeValue(metrics[prometheus.BuildFQName(teleport.MetricNamespace, "", "roles_total")]),
	}

	var auditStats *AuditStats
	if prev != nil {
		auditStats = prev.Audit
	}

	re.Audit = getAuditStats(metrics, auditStats, period)

	if prev != nil {
		re.Cluster.GenerateRequestsCount.SetFreq(prev.Cluster.GenerateRequestsCount, period)
		re.Cluster.GenerateRequestsThrottledCount.SetFreq(prev.Cluster.GenerateRequestsThrottledCount, period)
	}

	return &re, nil
}

// matchesLabelValue returns true if a list of label pairs
// matches required name/value pair, used to slice vectors by component
func matchesLabelValue(labels []*dto.LabelPair, name, value string) bool {
	for _, label := range labels {
		if label.GetName() == name {
			return label.GetValue() == value
		}
	}
	return false
}

func getRequests(component string, metric *dto.MetricFamily) []Request {
	if metric == nil || metric.GetType() != dto.MetricType_COUNTER || len(metric.Metric) == 0 {
		return nil
	}
	out := make([]Request, 0, len(metric.Metric))
	for _, counter := range metric.Metric {
		if !matchesLabelValue(counter.Label, teleport.ComponentLabel, component) {
			continue
		}
		req := Request{
			Counter: Counter{
				Count: int64(*counter.Counter.Value),
			},
		}
		for _, label := range counter.Label {
			if label.GetName() == teleport.TagReq {
				req.Key.Key = label.GetValue()
			}
			if label.GetName() == teleport.TagRange {
				req.Key.Range = label.GetValue() == teleport.TagTrue
			}
		}
		out = append(out, req)
	}
	return out
}

func getWatcherStats(metrics map[string]*dto.MetricFamily, prev *WatcherStats, period time.Duration) *WatcherStats {
	eventsEmitted := metrics[teleport.MetricWatcherEventsEmitted]
	if eventsEmitted == nil || eventsEmitted.GetType() != dto.MetricType_HISTOGRAM || len(eventsEmitted.Metric) == 0 {
		eventsEmitted = &dto.MetricFamily{}
	}

	events := make(map[string]Event)
	for i, metric := range eventsEmitted.Metric {
		histogram := getHistogram(eventsEmitted, atIndex(i))

		resource := ""
		for _, pair := range metric.GetLabel() {
			if pair.GetName() == teleport.TagResource {
				resource = pair.GetValue()
				break
			}
		}

		// only continue processing if we found the resource
		if resource == "" {
			continue
		}

		evt := Event{
			Resource: resource,
			Size:     histogram.Sum,
			Counter: Counter{
				Count: histogram.Count,
			},
		}

		if prev != nil {
			prevReq, ok := prev.TopEvents[evt.Resource]
			if ok {
				// if previous value is set, can calculate req / second
				evt.SetFreq(prevReq.Counter, period)
			}
		}

		events[evt.Resource] = evt
	}

	histogram := getHistogram(metrics[teleport.MetricWatcherEventSizes], atIndex(0))
	var (
		eventsPerSec *utils.CircularBuffer
		bytesPerSec  *utils.CircularBuffer
	)
	if prev == nil {
		eps, err := utils.NewCircularBuffer(150)
		if err != nil {
			return nil
		}

		bps, err := utils.NewCircularBuffer(150)
		if err != nil {
			return nil
		}

		eventsPerSec = eps
		bytesPerSec = bps
	} else {
		eventsPerSec = prev.EventsPerSecond
		bytesPerSec = prev.BytesPerSecond

		eventsPerSec.Add(float64(histogram.Count-prev.EventSize.Count) / float64(period/time.Second))
		bytesPerSec.Add(histogram.Sum - prev.EventSize.Sum/float64(period/time.Second))
	}

	stats := &WatcherStats{
		EventSize:       histogram,
		TopEvents:       events,
		EventsPerSecond: eventsPerSec,
		BytesPerSecond:  bytesPerSec,
	}

	return stats
}

func getAuditStats(metrics map[string]*dto.MetricFamily, prev *AuditStats, period time.Duration) *AuditStats {
	if prev == nil {
		failed, err := utils.NewCircularBuffer(150)
		if err != nil {
			return nil
		}

		events, err := utils.NewCircularBuffer(150)
		if err != nil {
			return nil
		}

		trimmed, err := utils.NewCircularBuffer(150)
		if err != nil {
			return nil
		}

		sizes, err := utils.NewCircularBuffer(150)
		if err != nil {
			return nil
		}

		prev = &AuditStats{
			FailedEventsBuffer:   failed,
			FailedEventsCounter:  &Counter{},
			EmittedEventsBuffer:  events,
			EmittedEventsCounter: &Counter{},
			TrimmedEventsBuffer:  trimmed,
			TrimmedEventsCounter: &Counter{},
			EventSizeBuffer:      sizes,
			EventSizeCounter:     &Counter{},
		}
	}

	updateCounter := func(metrics map[string]*dto.MetricFamily, metric string, counter *Counter, buf *utils.CircularBuffer) {
		current := getCounterValue(metrics[metric])
		counter.UpdateFreq(current, period)
		buf.Add(counter.GetFreq())
	}

	updateCounter(metrics, prometheus.BuildFQName("", "audit", "failed_emit_events"), prev.FailedEventsCounter, prev.FailedEventsBuffer)
	updateCounter(metrics, prometheus.BuildFQName(teleport.MetricNamespace, "audit", "stored_trimmed_events"), prev.TrimmedEventsCounter, prev.TrimmedEventsBuffer)

	histogram := getHistogram(metrics[prometheus.BuildFQName(teleport.MetricNamespace, "", "audit_emitted_event_sizes")], atIndex(0))

	prev.EmittedEventsCounter.UpdateFreq(histogram.Count, period)
	prev.EmittedEventsBuffer.Add(prev.EmittedEventsCounter.GetFreq())

	prev.EventSizeCounter.UpdateFreq(int64(histogram.Sum), period)
	prev.EventSizeBuffer.Add(prev.EventSizeCounter.GetFreq())

	return &AuditStats{
		FailedEventsBuffer:   prev.FailedEventsBuffer,
		FailedEventsCounter:  prev.FailedEventsCounter,
		EmittedEventsBuffer:  prev.EmittedEventsBuffer,
		EmittedEventsCounter: prev.EmittedEventsCounter,
		TrimmedEventsBuffer:  prev.TrimmedEventsBuffer,
		TrimmedEventsCounter: prev.TrimmedEventsCounter,
		EventSizeBuffer:      prev.EventSizeBuffer,
		EventSizeCounter:     prev.EventSizeCounter,
	}
}

func getRemoteClusters(metric *dto.MetricFamily) []RemoteCluster {
	if metric == nil || metric.GetType() != dto.MetricType_GAUGE || len(metric.Metric) == 0 {
		return nil
	}
	out := make([]RemoteCluster, len(metric.Metric))
	for i, counter := range metric.Metric {
		rc := RemoteCluster{
			Connected: counter.Gauge.GetValue() > 0,
		}
		for _, label := range counter.Label {
			if label.GetName() == teleport.TagCluster {
				rc.Name = label.GetValue()
			}
		}
		out[i] = rc
	}
	return out
}

func getActiveMigrations(metric *dto.MetricFamily) []string {
	if metric == nil || metric.GetType() != dto.MetricType_GAUGE || len(metric.Metric) == 0 {
		return nil
	}
	var out []string
	for _, counter := range metric.Metric {
		if counter.Gauge.GetValue() == 0 {
			continue
		}
		for _, label := range counter.Label {
			if label.GetName() == teleport.TagMigration {
				out = append(out, label.GetValue())
				break
			}
		}
	}
	return out
}

func getComponentGaugeValue(component string, metric *dto.MetricFamily) float64 {
	if metric == nil || metric.GetType() != dto.MetricType_GAUGE || len(metric.Metric) == 0 || metric.Metric[0].Gauge == nil || metric.Metric[0].Gauge.Value == nil {
		return 0
	}
	for i := range metric.Metric {
		if matchesLabelValue(metric.Metric[i].Label, teleport.ComponentLabel, component) {
			return *metric.Metric[i].Gauge.Value
		}
	}
	return 0
}

func getGaugeValue(metric *dto.MetricFamily) float64 {
	if metric == nil || metric.GetType() != dto.MetricType_GAUGE || len(metric.Metric) == 0 || metric.Metric[0].Gauge == nil || metric.Metric[0].Gauge.Value == nil {
		return 0
	}
	return *metric.Metric[0].Gauge.Value
}

func getCounterValue(metric *dto.MetricFamily) int64 {
	if metric == nil || metric.GetType() != dto.MetricType_COUNTER || len(metric.Metric) == 0 || metric.Metric[0].Counter == nil || metric.Metric[0].Counter.Value == nil {
		return 0
	}
	return int64(*metric.Metric[0].Counter.Value)
}

type histogramFilterFunc func(metrics []*dto.Metric) *dto.Histogram

func atIndex(index int) histogramFilterFunc {
	return func(metrics []*dto.Metric) *dto.Histogram {
		if index < 0 || index >= len(metrics) {
			return nil
		}

		return metrics[index].Histogram
	}
}

func forLabel(label string) histogramFilterFunc {
	return func(metrics []*dto.Metric) *dto.Histogram {
		var hist *dto.Histogram
		for i := range metrics {
			if matchesLabelValue(metrics[i].Label, teleport.ComponentLabel, label) {
				hist = metrics[i].Histogram
				break
			}
		}

		return hist
	}
}

func getHistogram(metric *dto.MetricFamily, filterFn histogramFilterFunc) Histogram {
	if metric == nil || metric.GetType() != dto.MetricType_HISTOGRAM || len(metric.Metric) == 0 || metric.Metric[0].Histogram == nil {
		return Histogram{}
	}

	hist := filterFn(metric.Metric)
	if hist == nil {
		return Histogram{}
	}

	out := Histogram{
		Count:   int64(hist.GetSampleCount()),
		Sum:     hist.GetSampleSum(),
		Buckets: make([]Bucket, len(hist.Bucket)),
	}

	for i, bucket := range hist.Bucket {
		out.Buckets[i] = Bucket{
			Count:      int64(bucket.GetCumulativeCount()),
			UpperBound: bucket.GetUpperBound(),
		}
	}
	return out
}

func getLabels(metric *dto.MetricFamily) string {
	if metric == nil {
		return ""
	}
	var out []string
	for _, metric := range metric.Metric {
		for _, label := range metric.Label {
			out = append(out, fmt.Sprintf("%v:%v", label.GetName(), label.GetValue()))
		}
	}
	return strings.Join(out, ", ")
}
