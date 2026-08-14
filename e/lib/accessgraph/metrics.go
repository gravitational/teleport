package accessgraph

import (
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
)

const (
	accessGraphMetricSubsystem = "access_graph"
	accessGraphMetricLabel     = "stream"

	accessGraphMetricStreamEvent    = "event_stream"
	accessGraphMetricStreamAuditLog = "audit_log"
)

var accessGraphConnected = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: accessGraphMetricSubsystem,
		Name:      "auth_connected",
		Help:      "Whether this Teleport Auth Service instance is connected to a healthy Access Graph service stream.",
	},
	[]string{accessGraphMetricLabel},
)

func setAccessGraphConnected(stream string, connected bool) {
	value := 0
	if connected {
		value = 1
	}
	accessGraphConnected.WithLabelValues(stream).Set(float64(value))
}

func registerAccessGraphMetrics(registry prometheus.Registerer, hasAuditLog bool) error {
	if err := registry.Register(accessGraphConnected); err != nil {
		return trace.Wrap(err)
	}

	// Initialize metrics with default values.
	setAccessGraphConnected(accessGraphMetricStreamEvent, false)
	if hasAuditLog {
		setAccessGraphConnected(accessGraphMetricStreamAuditLog, false)
	}

	return nil
}
