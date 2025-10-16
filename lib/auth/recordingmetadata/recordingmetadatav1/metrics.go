package recordingmetadatav1

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

const subsystem = "recording_metadata"

var sessionsProcessedMetric = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: teleport.MetricNamespace,
	Subsystem: subsystem,
	Name:      "sessions_processed_total",
	Help:      "Total number of session recordings processed, with success or failure",
}, []string{"success"})

var sessionsProcessingMetric = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: teleport.MetricNamespace,
	Subsystem: subsystem,
	Name:      "sessions_processing_total",
	Help:      "Number of session recordings being processed",
})

var sessionsPendingMetric = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: teleport.MetricNamespace,
	Subsystem: subsystem,
	Name:      "sessions_pending_total",
	Help:      "Number of sessions waiting to be processed",
})

func init() {
	metrics.RegisterPrometheusCollectors(
		sessionsProcessedMetric,
		sessionsProcessingMetric,
	)
}
