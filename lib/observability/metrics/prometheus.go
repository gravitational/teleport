/*
Copyright 2021 Gravitational, Inc.

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

package metrics

import (
	"context"
	"net/http"
	"runtime"

	"github.com/gravitational/trace"
	om "github.com/grpc-ecosystem/go-grpc-middleware/providers/openmetrics/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/gravitational/teleport"
)

// RegisterPrometheusCollectors is a wrapper around prometheus.Register that
//   - ignores equal or re-registered collectors
//   - returns an error if a collector does not fulfill the consistency and
//     uniqueness criteria
func RegisterPrometheusCollectors(collectors ...prometheus.Collector) error {
	var errs []error
	for _, c := range collectors {
		if err := prometheus.Register(c); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); ok {
				continue
			}
			errs = append(errs, err)
		}
	}
	return trace.NewAggregate(errs...)
}

// BuildCollector provides a Collector that contains build information gauge
func BuildCollector() prometheus.Collector {
	return prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBuildInfo,
			Help:      "Provides build information of Teleport including gitref (git describe --long --tags), Go version, and Teleport version. The value of this gauge will always be 1.",
			ConstLabels: prometheus.Labels{
				teleport.TagVersion:   teleport.Version,
				teleport.TagGitref:    teleport.Gitref,
				teleport.TagGoVersion: runtime.Version(),
			},
		},
		func() float64 { return 1 },
	)
}

// CreateGRPCServerMetrics creates server grpc metrics configuration that is to be registered and used by the caller
// in an openmetrics unary and/or stream interceptor
func CreateGRPCServerMetrics(latencyEnabled bool, labels prometheus.Labels) *om.ServerMetrics {
	serverOpts := []om.ServerMetricsOption{om.WithServerCounterOptions(om.WithConstLabels(labels))}
	if latencyEnabled {
		histOpts := grpcHistogramOpts(labels)
		serverOpts = append(serverOpts, om.WithServerHandlingTimeHistogram(histOpts...))
	}
	return om.NewServerMetrics(serverOpts...)
}

// CreateGRPCClientMetrics creates client grpc metrics configuration that is to be registered and used by the caller
// in an openmetrics unary and/or stream interceptor
func CreateGRPCClientMetrics(latencyEnabled bool, labels prometheus.Labels) *om.ClientMetrics {
	clientOpts := []om.ClientMetricsOption{om.WithClientCounterOptions(om.WithConstLabels(labels))}
	if latencyEnabled {
		histOpts := grpcHistogramOpts(labels)
		clientOpts = append(clientOpts, om.WithClientHandlingTimeHistogram(histOpts...))
	}
	return om.NewClientMetrics(clientOpts...)
}

func grpcHistogramOpts(labels prometheus.Labels) []om.HistogramOption {
	return []om.HistogramOption{
		om.WithHistogramBuckets(prometheus.ExponentialBuckets(0.001, 2, 16)),
		om.WithHistogramConstLabels(labels),
	}
}

// DynamicPromLabelsHandler is a middleware that adds Prometheus labels map to
// the request context. This map is used to add labels to Prometheus metrics
// which are unknown at the time of metric registration.
// For example, the cluster name is not known at the time [promhttp.Handler]
// execution and depends on the request certificate. This middleware allows
// adding the cluster name to registered [prometheus.Labels] map and then
// using it in the [promhttp.Handler] to fill the labels in the metrics.
func DynamicPromLabelsHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(
		// This is a middleware that adds Prometheus labels map to the request
		// context. This map is used to add labels to Prometheus metrics which
		// are then used to fill the labels in the metrics.
		func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), promLabelsKey{}, prometheus.Labels{}))
			handler.ServeHTTP(w, r)
		},
	)
}

// WithLabelFromCtx is a wrapper around [promhttp.WithLabelFromCtx] that
// collects the label value from the context and returns it.
// This function is used to fill the labels in the metrics that are unknown at
// the time of metric registration.
// If the label is not found in the context or is empty, it returns "unknown"
// as value.
func WithLabelFromCtx(label string) promhttp.Option {
	return promhttp.WithLabelFromCtx(
		label,
		labelGetter(label),
	)
}

func labelGetter(label string) func(context.Context) string {
	return func(ctx context.Context) string {
		value := "unknown"
		v, ok := ctx.Value(promLabelsKey{}).(prometheus.Labels)
		if ok && v[label] != "" {
			value = v[label]
		}
		return value
	}
}

// LabelsFromCtx returns Prometheus labels map from the request context.
// This function is used to retrieve the labels map from the request context
// and fill the labels values.
func LabelsFromCtx(ctx context.Context) (prometheus.Labels, bool) {
	v, ok := ctx.Value(promLabelsKey{}).(prometheus.Labels)
	return v, ok
}

// promLabelsKey is the context key used to store Prometheus labels map in the
// request context.
// This map is filled by the middleware upstream and should be made available
// to the Prometheus instrumentation.
type promLabelsKey struct{}
