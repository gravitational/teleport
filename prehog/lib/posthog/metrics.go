package posthog

import "github.com/prometheus/client_golang/prometheus"

var EmitTotal = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: "prehog",
	Subsystem: "client",
	Name:      "emit_total",
})

var EmitErrorTotal = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: "prehog",
	Subsystem: "client",
	Name:      "emit_error_total",
})

var EmitDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
	Namespace: "prehog",
	Subsystem: "client",
	Name:      "emit_duration_seconds",
})

var BatchedTotal = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: "prehog",
	Subsystem: "client",
	Name:      "batched_total",
})

var BatchedErrorTotal = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: "prehog",
	Subsystem: "client",
	Name:      "batched_error_total",
})

var BatchedDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
	Namespace: "prehog",
	Subsystem: "client",
	Name:      "batched_duration_seconds",
})

func init() {
	prometheus.MustRegister(
		EmitTotal,
		EmitErrorTotal,
		EmitDuration,
		BatchedTotal,
		BatchedErrorTotal,
		BatchedDuration,
	)
}
