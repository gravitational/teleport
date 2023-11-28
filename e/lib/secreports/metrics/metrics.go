package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

func init() {
	metrics.RegisterPrometheusCollectors(
		QueryExecutionTimeHist,
		QueryScannedBytes,
		LimitUsage,
		LimitRefillTimestamp,
	)
}

// accessMonitoring is the subsystem used to prefix Prometheus access monitoring metrics.
const accessMonitoring = "access_monitoring"

const (
	// DaysTag is used to tag days range used in for athena query.
	DaysTag = "days"
	// StatusTag is used to tag status of athena query.
	StatusTag = "status"
)

var (
	// QueryExecutionTimeHist is a histogram of query execution time.
	QueryExecutionTimeHist = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: accessMonitoring,
		Name:      "query_execution_time_seconds",
		Help:      "Athena Query execution time in seconds.",
		Buckets:   prometheus.ExponentialBuckets(0.1, 2, 20),
	}, []string{DaysTag, StatusTag})

	// QueryScannedBytes is a counter of bytes scanned by athena query.
	QueryScannedBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: accessMonitoring,
		Name:      "query_scanned_bytes_total",
		Help:      "Total number of bytes scanned by athena query.",
	}, []string{DaysTag})

	// LimitUsage is a gauge of current athena query limit usage.
	LimitUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: accessMonitoring,
		Name:      "limit_usage_percentage",
		Help:      "Limit usage percentage for Athena query.",
	})

	// LimitRefillTimestamp is a gauge of refill timestamp for athena query limit.
	LimitRefillTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: accessMonitoring,
		Name:      "limit_refill_timestamp",
		Help:      "Limit refill timestamp for Athena query limit.",
	})
)
