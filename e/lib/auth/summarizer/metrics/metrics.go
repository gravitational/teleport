package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	libmetrics "github.com/gravitational/teleport/lib/observability/metrics"
)

const (
	SummarizerSubsystem     = "summarizer"
	LabelInferenceModelName = "inference_model_name"
)

var (
	SummarizationsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: SummarizerSubsystem,
		Name:      "summarizations_total",
		Help:      "Total number of summarization jobs started",
	}, []string{LabelInferenceModelName})

	SummarizationErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: SummarizerSubsystem,
		Name:      "summarization_errors",
		Help:      "Number of summarization errors",
	}, []string{LabelInferenceModelName})

	SummarizationsPending = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: SummarizerSubsystem,
		Name:      "summarization_jobs_pending",
		Help:      "Number of summarization jobs currently awaiting execution",
	}, []string{LabelInferenceModelName})

	SummarizationsRunning = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: SummarizerSubsystem,
		Name:      "summarization_jobs_running",
		Help:      "Number of summarization jobs currently being executed",
	}, []string{LabelInferenceModelName})
)

func init() {
	libmetrics.RegisterPrometheusCollectors(
		SummarizationsTotal, SummarizationErrors, SummarizationsPending, SummarizationsRunning,
	)
}
