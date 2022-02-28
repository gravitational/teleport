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

// clientMetrics represents a collection of metrics for a proxy peer client
type clientMetrics struct {
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
		tunnelErrorCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerClientTunnelError,
				Help: "counts the number of errors encountered dialing peer proxies.",
			},
			[]string{"error_type"},
		),
		requestCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerClientRequest,
				Help: "Counts the number of client requests.",
			},
			[]string{"service", "method"},
		),
		handledCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerClientRequestHandled,
				Help: "Counts the number of handled client requests.",
			},
			[]string{"service", "method", "code"},
		),
		streamMsgReceivedCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerClientStreamReceived,
				Help: "Counts the number of received stream messages on the client.",
			},
			[]string{"service", "method", "code", "size"},
		),
		streamMsgSentCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerClientStreamSent,
				Help: "Counts the number of sent stream messages on the client.",
			},
			[]string{"service", "method", "code", "size"},
		),
		handledHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: teleport.MetricProxyPeerClientRequestLatency,
				Help: "Measures the latency of handled grpc client requests.",
				// lowest bucket start at upper bound 0.001 sec (1 ms) with factor 2
				// highest bucket start at 0.001 sec * 2^15 == 32.768 sec
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
			},
			[]string{"service", "method"},
		),
		streamReceivedHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: teleport.MetricProxyPeerClientStreamReceivedLatency,
				Help: "Measures the latency of received stream messages on the client.",
				// lowest bucket start at upper bound 0.001 sec (1 ms) with factor 2
				// highest bucket start at 0.001 sec * 2^15 == 32.768 sec
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
			},
			[]string{"service", "method"},
		),
		streamSentHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: teleport.MetricProxyPeerClientStreamSentLatency,
				Help: "Measures the latency of sent stream messages on the client.",
				// lowest bucket start at upper bound 0.001 sec (1 ms) with factor 2
				// highest bucket start at 0.001 sec * 2^15 == 32.768 sec
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
			},
			[]string{"service", "method"},
		),
	}

	if err := utils.RegisterPrometheusCollectors(
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

// reportTunnelError reports errors encountered dialing an existing peer tunnel.
func (c *clientMetrics) reportTunnelError(status string) {
	c.tunnelErrorCounter.WithLabelValues(status).Inc()
}

// getRequestCounter is a getter for the requestCounter collector.
func (c *clientMetrics) getRequestCounter() *prometheus.CounterVec {
	return c.requestCounter
}

// getHandledCounter is a getter for the handledCounter collector.
func (c *clientMetrics) getHandledCounter() *prometheus.CounterVec {
	return c.handledCounter
}

// getStreamMsgReceivedCounter is a getter for the streamMsgReceivedCounter collector.
func (c *clientMetrics) getStreamMsgReceivedCounter() *prometheus.CounterVec {
	return c.streamMsgReceivedCounter
}

// getStreamMsgSentCounter is a getter for the streamMsgSentCounter collector.
func (c *clientMetrics) getStreamMsgSentCounter() *prometheus.CounterVec {
	return c.streamMsgSentCounter
}

// getHandledHistogram is a getter for the handledHistogram collector.
func (c *clientMetrics) getHandledHistogram() *prometheus.HistogramVec {
	return c.handledHistogram
}

// getStreamReceivedHistogram is a getter for the streamReceivedHistogram collector.
func (c *clientMetrics) getStreamReceivedHistogram() *prometheus.HistogramVec {
	return c.streamReceivedHistogram
}

// getStreamSentHistogram is a getter for the streamSentHistogram collector.
func (c *clientMetrics) getStreamSentHistogram() *prometheus.HistogramVec {
	return c.streamSentHistogram
}
