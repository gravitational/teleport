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
	streamReceivedHistogram  *prometheus.HistogramVec
	streamSentHistogram      *prometheus.HistogramVec
}

// newServerMetrics inits and registers client metrics prometheus collectors.
func newServerMetrics() (*serverMetrics, error) {
	sm := &serverMetrics{
		requestCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerServerRequest,
				Help: "Counts the number of server requests.",
			},
			[]string{"service", "method"},
		),
		handledCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerServerRequestHandled,
				Help: "Counts the number of handled server requests.",
			},
			[]string{"service", "method", "code"},
		),
		streamMsgReceivedCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerServerStreamReceived,
				Help: "Counts the number of received stream messages on the server.",
			},
			[]string{"service", "method", "code", "size"},
		),
		streamMsgSentCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: teleport.MetricProxyPeerServerStreamSent,
				Help: "Counts the number of sent stream messages on the server.",
			},
			[]string{"service", "method", "code", "size"},
		),
		handledHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: teleport.MetricProxyPeerServerRequestLatency,
				Help: "Measures the latency of handled grpc server requests.",
				// lowest bucket start at upper bound 0.001 sec (1 ms) with factor 2
				// highest bucket start at 0.001 sec * 2^15 == 32.768 sec
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
			},
			[]string{"service", "method"},
		),
		streamReceivedHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: teleport.MetricProxyPeerServerStreamReceivedLatency,
				Help: "Measures the latency of received stream messages on the server.",
				// lowest bucket start at upper bound 0.001 sec (1 ms) with factor 2
				// highest bucket start at 0.001 sec * 2^15 == 32.768 sec
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
			},
			[]string{"service", "method"},
		),
		streamSentHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: teleport.MetricProxyPeerServerStreamSentLatency,
				Help: "Measures the latency of sent stream messages on the server.",
				// lowest bucket start at upper bound 0.001 sec (1 ms) with factor 2
				// highest bucket start at 0.001 sec * 2^15 == 32.768 sec
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
			},
			[]string{"service", "method"},
		),
	}

	if err := utils.RegisterPrometheusCollectors(
		sm.requestCounter,
		sm.handledCounter,
		sm.streamMsgReceivedCounter,
		sm.streamMsgSentCounter,
		sm.handledHistogram,
		sm.streamReceivedHistogram,
		sm.streamSentHistogram,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	return sm, nil
}

// getRequestCounter is a getter for the requestCounter collector.
func (s *serverMetrics) getRequestCounter() *prometheus.CounterVec {
	return s.requestCounter
}

// getHandledCounter is a getter for the handledCounter collector.
func (s *serverMetrics) getHandledCounter() *prometheus.CounterVec {
	return s.handledCounter
}

// getStreamMsgReceivedCounter is a getter for the streamMsgReceivedCounter collector.
func (s *serverMetrics) getStreamMsgReceivedCounter() *prometheus.CounterVec {
	return s.streamMsgReceivedCounter
}

// getStreamMsgSentCounter is a getter for the streamMsgSentCounter collector.
func (s *serverMetrics) getStreamMsgSentCounter() *prometheus.CounterVec {
	return s.streamMsgSentCounter
}

// getHandledHistogram is a getter for the handledHistogram collector.
func (s *serverMetrics) getHandledHistogram() *prometheus.HistogramVec {
	return s.handledHistogram
}

// getStreamReceivedHistogram is a getter for the streamReceivedHistogram collector.
func (s *serverMetrics) getStreamReceivedHistogram() *prometheus.HistogramVec {
	return s.streamReceivedHistogram
}

// getStreamSentHistogram is a getter for the streamSentHistogram collector.
func (s *serverMetrics) getStreamSentHistogram() *prometheus.HistogramVec {
	return s.streamSentHistogram
}
