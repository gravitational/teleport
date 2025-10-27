package entraid

import (
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
)

const (
	metricSubsystem          = "entraid_sync"
	metricLabelResult        = "result"
	metricLabelSection       = "section"
	metricLabelResultSuccess = "success"
	metricLabelResultError   = "error"
)

func metricLabelResultFromError(err error) string {
	if err != nil {
		return metricLabelResultError
	}
	return metricLabelResultSuccess
}

func newMetrics() *directoryMetrics {
	return &directoryMetrics{
		reconciliationCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricSubsystem,
			Name:      "reconciliation_count",
			Help:      "Number of times the entra directory reconciliation run.",
		}, []string{metricLabelResult}),
		reconciliationDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricSubsystem,
			Name:      "reconciliation_duration_seconds",
			Help:      "The duration of each entra directory reconciliation run.",
		}, []string{metricLabelSection}),
		discoveredEntraGroups: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricSubsystem,
			Name:      "discovered_entra_groups",
			Help:      "Number of entra groups discovered.",
		}),
		discoveredEntraUsers: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricSubsystem,
			Name:      "discovered_entra_users",
			Help:      "Number of entra users discovered.",
		}),
		discoveredEntraMemberships: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricSubsystem,
			Name:      "discovered_entra_memberships",
			Help:      "Number of entra group memberships discovered.",
		}),
		reconciledNestedMemberTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricSubsystem,
			Name:      "reconciled_nested_member_total",
			Help:      "The number of Teleport nested access lists reconciled.",
		}, []string{metricLabelResult}),
		reconciledNestedMemberDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricSubsystem,
			Name:      "reconciled_nested_member_duration_seconds",
			Help:      "The duration of each Teleport nested access list reconciliation run.",
		}),
	}
}

func (metrics *directoryMetrics) register(r prometheus.Registerer) error {
	return trace.NewAggregate(
		r.Register(metrics.reconciliationCount),
		r.Register(metrics.reconciliationDuration),
		r.Register(metrics.discoveredEntraGroups),
		r.Register(metrics.discoveredEntraUsers),
		r.Register(metrics.discoveredEntraMemberships),
		r.Register(metrics.reconciledNestedMemberTotal),
		r.Register(metrics.reconciledNestedMemberDuration),
	)
}

type directoryMetrics struct {
	reconciliationCount    *prometheus.CounterVec
	reconciliationDuration *prometheus.HistogramVec

	discoveredEntraGroups      prometheus.Gauge
	discoveredEntraUsers       prometheus.Gauge
	discoveredEntraMemberships prometheus.Gauge

	reconciledNestedMemberTotal    *prometheus.CounterVec
	reconciledNestedMemberDuration prometheus.Histogram
}
