package directory

import (
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/services"
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

func newMetrics(reg *metrics.Registry) (*directoryMetrics, error) {
	accessListReconcilerMetrics, err := services.NewReconcilerMetrics(reg.Wrap("accesslist"))
	if err != nil {
		return nil, trace.Wrap(err, "creating access list reconciler metrics")
	}
	userReconcilerMetrics, err := services.NewReconcilerMetrics(reg.Wrap("user"))
	if err != nil {
		return nil, trace.Wrap(err, "creating user reconciler metrics")
	}

	return &directoryMetrics{
		reconciliationCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: reg.Namespace(),
			Subsystem: reg.Subsystem(),
			Name:      "reconciliation_count",
			Help:      "Number of times the entra directory reconciliation run.",
		}, []string{metricLabelResult}),
		reconciliationDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: reg.Namespace(),
			Subsystem: reg.Subsystem(),
			Name:      "reconciliation_duration_seconds",
			Help:      "The duration of each entra directory reconciliation run.",
		}, []string{metricLabelSection}),
		discoveredEntraGroups: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: reg.Namespace(),
			Subsystem: reg.Subsystem(),
			Name:      "discovered_groups",
			Help:      "Number of entra groups discovered.",
		}),
		discoveredEntraUsers: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: reg.Namespace(),
			Subsystem: reg.Subsystem(),
			Name:      "discovered_users",
			Help:      "Number of entra users discovered.",
		}),
		discoveredEntraMemberships: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: reg.Namespace(),
			Subsystem: reg.Subsystem(),
			Name:      "discovered_memberships",
			Help:      "Number of entra group memberships discovered.",
		}),
		reconciledNestedMemberTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: reg.Namespace(),
			Subsystem: reg.Subsystem(),
			Name:      "reconciled_nested_member_total",
			Help:      "The number of Teleport nested access lists reconciled.",
		}, []string{metricLabelResult}),
		reconciledNestedMemberDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: reg.Namespace(),
			Subsystem: reg.Subsystem(),
			Name:      "reconciled_nested_member_duration_seconds",
			Help:      "The duration of each Teleport nested access list reconciliation run.",
		}),

		// We create and keep the reconciler on our side, because the directory plugin
		// uses short-lived one-shot reconcilers. To get accurate rates, we'll want
		// the counters to not be reset after each reconciliation cycle.
		accessListReconcilerMetrics: accessListReconcilerMetrics,
		userReconcilerMetrics:       userReconcilerMetrics,
	}, nil
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
		metrics.accessListReconcilerMetrics.Register(r),
		metrics.userReconcilerMetrics.Register(r),
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

	userReconcilerMetrics       *services.ReconcilerMetrics
	accessListReconcilerMetrics *services.ReconcilerMetrics
}
