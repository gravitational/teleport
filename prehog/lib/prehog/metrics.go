package prehog

import "github.com/prometheus/client_golang/prometheus"

var ApiRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "prehog",
	Subsystem: "api",
	Name:      "requests_total",
}, []string{"code", "method"})

var ApiRequestsDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "prehog",
	Subsystem: "api",
	Name:      "request_duration_seconds",
}, []string{"code"})

var ApiRequestsInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "prehog",
	Subsystem: "api",
	Name:      "requests_in_flight",
})

func init() {
	prometheus.MustRegister(
		ApiRequestsTotal,
		ApiRequestsDuration,
		ApiRequestsInFlight,
	)
}
