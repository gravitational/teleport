// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
)

// serverMetrics represents a collection of metrics for a proxy peer server
type serverMetrics struct {
	requestCounter           *prometheus.CounterVec
	handledCounter           *prometheus.CounterVec
	streamMsgReceivedCounter *prometheus.CounterVec
	streamMsgSentCounter     *prometheus.CounterVec
	handledHistogram         *prometheus.HistogramVec
}

// newServerMetrics inits and registers client metrics prometheus collectors.
func newServerMetrics() (*serverMetrics, error) {
	sm := &serverMetrics{
		requestCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerServerRequest,
				Help: "Counts the number of server requests.",
			},
			[]string{"grpc_service", "grpc_method"},
		),
		handledCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerServerRequestHandled,
				Help: "Counts the number of handled server requests.",
			},
			[]string{"grpc_service", "grpc_method", "grpc_code"},
		),
		streamMsgReceivedCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerServerStreamReceived,
				Help: "Counts the number of received stream messages on the server.",
			},
			[]string{"grpc_service", "grpc_method"},
		),
		streamMsgSentCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerServerStreamSent,
				Help: "Counts the number of sent stream messages on the server.",
			},
			[]string{"grpc_service", "grpc_method"},
		),
		handledHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: teleport.MetricProxyPeerServerRequestLatency,
				Help: "Measures the latency of handled grpc server requests.",
				// lowest bucket start at upper bound 0.001 sec (1 ms) with factor 2
				// highest bucket start at 0.001 sec * 2^15 == 32.768 sec
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
			},
			[]string{"grpc_service", "grpc_method"},
		),
	}

	if err := utils.RegisterPrometheusCollectors(
		sm.requestCounter,
		sm.handledCounter,
		sm.streamMsgReceivedCounter,
		sm.streamMsgSentCounter,
		sm.handledHistogram,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	return sm, nil
}

// clientMetrics represents a collection of metrics for a proxy peer client
type clientMetrics struct {
	dialErrorCounter   prometheus.Counter
	tunnelErrorCounter *prometheus.CounterVec

	requestCounter           *prometheus.CounterVec
	handledCounter           *prometheus.CounterVec
	streamMsgReceivedCounter *prometheus.CounterVec
	streamMsgSentCounter     *prometheus.CounterVec
	handledHistogram         *prometheus.HistogramVec
	streamReceivedHistogram  *prometheus.HistogramVec
	streamSentHistogram      *prometheus.HistogramVec
}

// newClientMetrics inits and registers client metrics prometheus collectors.
func newClientMetrics() (*clientMetrics, error) {
	cm := &clientMetrics{
		dialErrorCounter: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerClientDialError,
				Help: "Counts the number of failed dial attempts.",
			},
		),
		tunnelErrorCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerClientTunnelError,
				Help: "counts the number of errors encountered fetching existing grpc connections.",
			},
			[]string{"error_type"},
		),
		requestCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerClientRequest,
				Help: "Counts the number of client requests.",
			},
			[]string{"grpc_service", "grpc_method"},
		),
		handledCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerClientRequestHandled,
				Help: "Counts the number of handled client requests.",
			},
			[]string{"grpc_service", "grpc_method", "grpc_code"},
		),
		streamMsgReceivedCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerClientStreamReceived,
				Help: "Counts the number of received stream messages on the client.",
			},
			[]string{"grpc_service", "grpc_method"},
		),
		streamMsgSentCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerClientStreamSent,
				Help: "Counts the number of sent stream messages on the client.",
			},
			[]string{"grpc_service", "grpc_method"},
		),
		handledHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: teleport.MetricProxyPeerClientRequestLatency,
				Help: "Measures the latency of handled grpc client requests.",
				// lowest bucket start at upper bound 0.001 sec (1 ms) with factor 2
				// highest bucket start at 0.001 sec * 2^15 == 32.768 sec
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
			},
			[]string{"grpc_service", "grpc_method"},
		),
		streamReceivedHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: teleport.MetricProxyPeerClientStreamReceivedLatency,
				Help: "Measures the latency of received stream messages on the client.",
				// lowest bucket start at upper bound 0.001 sec (1 ms) with factor 2
				// highest bucket start at 0.001 sec * 2^15 == 32.768 sec
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
			},
			[]string{"grpc_service", "grpc_method"},
		),
		streamSentHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: teleport.MetricProxyPeerClientStreamSentLatency,
				Help: "Measures the latency of sent stream messages on the client.",
				// lowest bucket start at upper bound 0.001 sec (1 ms) with factor 2
				// highest bucket start at 0.001 sec * 2^15 == 32.768 sec
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
			},
			[]string{"grpc_service", "grpc_method"},
		),
	}

	if err := utils.RegisterPrometheusCollectors(
		cm.dialErrorCounter,
		cm.tunnelErrorCounter,
		cm.requestCounter,
		cm.handledCounter,
		cm.streamMsgReceivedCounter,
		cm.streamMsgSentCounter,
		cm.handledHistogram,
		cm.streamReceivedHistogram,
		cm.streamSentHistogram,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	return cm, nil
}
