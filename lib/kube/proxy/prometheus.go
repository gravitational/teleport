/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package proxy

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/gravitational/teleport"
	tracehttp "github.com/gravitational/teleport/api/observability/tracing/http"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

const (
	// kubernetesSubsystem is used to prefix Prometheus metrics for this
	// subsystem.
	// See https://prometheus.io/docs/practices/naming/#subsystem-name
	kubernetesSubsystem = "kubernetes"
)

func init() {
	metrics.RegisterPrometheusCollectors(
		clientRequestCounter,
		clientTLSLatencyVec,
		clientRequestDurationHistVec,
		clientInFlightGauge,
		clietGotConnLatencyVec,
		clientFirstByteLatencyVec,
		serverInFlightGauge,
		serverRequestCounter,
		serverRequestDurationHist,
		serverResponseSizeHist,
		execSessionsInFlightGauge,
		execSessionsRequestCounter,
		portforwardSessionsInFlightGauge,
		portforwardRequestCounter,
	)
}

// The following section defines Prometheus metrics for the clients used by
// Teleport proxy to connect to the Teleport Kubernetes service and by the
// Teleport Kubernetes service to connect to the Kubernetes cluster.
var (
	clientInFlightGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: kubernetesSubsystem,
			Name:      "client_in_flight_requests",
			Help:      "In-flight requests waiting for the upstream response.",
		},
		[]string{"component"},
	)

	clientRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: kubernetesSubsystem,
			Name:      "client_requests_total",
			Help:      "Total number of requests sent to the upstream teleport proxy, kube_service or Kubernetes Cluster servers.",
		},
		[]string{"component", "code", "method"},
	)

	clientTLSLatencyVec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: kubernetesSubsystem,
			Name:      "client_tls_duration_seconds",
			Help:      "Latency distribution of TLS handshakes.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"component", "event"},
	)

	clietGotConnLatencyVec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: kubernetesSubsystem,
			Name:      "client_got_conn_duration_seconds",
			Help:      "A histogram of latency to dial to the upstream server.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"component"},
	)

	clientFirstByteLatencyVec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: kubernetesSubsystem,
			Name:      "client_first_byte_response_duration_seconds",
			Help:      "Teleport Kubernetes Service | Latency distribution of time to receive the first response byte from the upstream server.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"component"},
	)

	clientRequestDurationHistVec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: kubernetesSubsystem,
			Name:      "client_request_duration_seconds",
			Help:      "Latency distribution of the upstream request time.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"component"},
	)
)

// instrumentedRoundtripper instruments the provided RoundTripper with
// Prometheus metrics and OpenTelemetry tracing.
func instrumentedRoundtripper(component string, tr http.RoundTripper) http.RoundTripper {
	// Define functions for the available httptrace.ClientTrace hook
	// functions that we want to instrument.
	httpTrace := &promhttp.InstrumentTrace{
		GotConn: func(t float64) {
			clietGotConnLatencyVec.WithLabelValues(component).Observe(t)
		},
		GotFirstResponseByte: func(t float64) {
			clientFirstByteLatencyVec.WithLabelValues(component).Observe(t)
		},
		TLSHandshakeStart: func(t float64) {
			clientTLSLatencyVec.WithLabelValues(component, "tls_handshake_start").Observe(t)
		},
		TLSHandshakeDone: func(t float64) {
			clientTLSLatencyVec.WithLabelValues(component, "tls_handshake_done").Observe(t)
		},
	}
	curryWith := prometheus.Labels{"component": component}
	return tracehttp.NewTransportWithInner(
		promhttp.InstrumentRoundTripperInFlight(
			clientInFlightGauge.WithLabelValues(component),
			promhttp.InstrumentRoundTripperCounter(
				clientRequestCounter.MustCurryWith(curryWith),
				promhttp.InstrumentRoundTripperTrace(
					httpTrace,
					promhttp.InstrumentRoundTripperDuration(clientRequestDurationHistVec.MustCurryWith(curryWith), tr),
				),
			),
		),
		// Pass the original RoundTripper to the inner transport so that it can
		// be used to close idle connections because promhttp roundtrippers don't
		// implement CloseIdleConnections.
		tr,
	)
}

// The following section defines Prometheus metrics for the HTTP server used by
// the Teleport Kubernetes Proxy and the Teleport Kubernetes service.
var (
	serverInFlightGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: kubernetesSubsystem,
			Name:      "server_in_flight_requests",
			Help:      "In-flight requests currently handled by the server.",
		},
		[]string{"component"},
	)

	serverRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: kubernetesSubsystem,
			Name:      "server_api_requests_total",
			Help:      "Total number of requests handled by the server.",
		},
		[]string{"component", "code", "method"},
	)

	serverRequestDurationHist = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: kubernetesSubsystem,
			Name:      "server_request_duration_seconds",
			Help:      "Latency distribution of the total request time.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"component", "method"},
	)

	serverResponseSizeHist = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: kubernetesSubsystem,
			Name:      "server_response_size_bytes",
			Help:      "Distribution of the response size.",
			// The following exponential buckets are equivalent to the following:
			// [50B 150B 450B 1.32KB 3.96KB 11.87KB 35.6KB 106.79KB 320.36KB 961.08KB 2.82MB 8.45MB 25.34MB]
			Buckets: prometheus.ExponentialBuckets(50, 3, 13),
		},
		[]string{"component"},
	)

	execSessionsInFlightGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: kubernetesSubsystem,
			Name:      "server_exec_in_flight_sessions",
			Help:      "Number of active kubectl exec sessions.",
		},
		[]string{"component"},
	)

	execSessionsRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: kubernetesSubsystem,
			Name:      "server_exec_sessions_total",
			Help:      "Total number of kubectl exec sessions. ",
		},
		[]string{"component"},
	)

	portforwardSessionsInFlightGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: kubernetesSubsystem,
			Name:      "server_portforward_in_flight_sessions",
			Help:      " Number of active kubectl portforward sessions.",
		},
		[]string{"component"},
	)

	portforwardRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: kubernetesSubsystem,
			Name:      "server_portforward_sessions_total",
			Help:      "Number of active kubectl portforward sessions.",
		},
		[]string{"component"},
	)

	joinSessionsInFlightGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: kubernetesSubsystem,
			Name:      "server_join_in_flight_sessions",
			Help:      "Number of active joining sessions,",
		},
		[]string{"component"},
	)

	joinSessionsRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: kubernetesSubsystem,
			Name:      "server_join_sessions_total",
			Help:      "Total number of joining sessions.",
		},
		[]string{"component"},
	)
)

// instrumentHTTPHandler instruments the HTTP handler with OpenTelemetry and
// Prometheus metrics.
func instrumentHTTPHandler(component string, handler http.Handler) http.Handler {
	return otelhttp.NewHandler(
		instrumentHTTPHandlerWithPrometheus(component, handler),
		component,
		otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
	)
}

// instrumentHTTPHandlerWithPrometheus instruments the HTTP handler with
// Prometheus metrics.
func instrumentHTTPHandlerWithPrometheus(component string, handler http.Handler) http.Handler {
	curryWith := prometheus.Labels{"component": component}
	return promhttp.InstrumentHandlerInFlight(
		serverInFlightGauge.WithLabelValues(component),
		promhttp.InstrumentHandlerDuration(
			serverRequestDurationHist.MustCurryWith(
				curryWith,
			),
			promhttp.InstrumentHandlerCounter(
				serverRequestCounter.MustCurryWith(
					curryWith,
				),
				promhttp.InstrumentHandlerResponseSize(
					serverResponseSizeHist.MustCurryWith(
						curryWith,
					),
					handler,
				),
			),
		),
	)
}
