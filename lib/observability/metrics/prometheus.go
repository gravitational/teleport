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
	"runtime"

	"github.com/gravitational/trace"
	om "github.com/grpc-ecosystem/go-grpc-middleware/providers/openmetrics/v2"
	"github.com/prometheus/client_golang/prometheus"

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
