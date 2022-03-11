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
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
)

// serverMetrics represents a collection of metrics for a proxy peer server
type serverMetrics struct {
	connections     *prometheus.GaugeVec
	rpcs            *prometheus.GaugeVec
	rpcTotal        *prometheus.CounterVec
	rpcDuration     *prometheus.HistogramVec
	messageSent     *prometheus.HistogramVec
	messageReceived *prometheus.HistogramVec
}

// newServerMetrics inits and registers client metrics prometheus collectors.
func newServerMetrics() (*serverMetrics, error) {
	sm := &serverMetrics{
		connections: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "proxy_peer",
				Subsystem: "server",
				Name:      "connections",
				Help:      "Number of currently opened connection to proxy peer clients.",
			},
			[]string{"local_addr", "remote_addr"},
		),

		rpcs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "proxy_peer",
				Subsystem: "server",
				Name:      "rpc",
				Help:      "Number of current server RPC requests.",
			},
			[]string{"service", "method"},
		),

		rpcTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "proxy_peer",
				Subsystem: "server",
				Name:      "rpc_total",
				Help:      "Total number of server RPC requests.",
			},
			[]string{"service", "method", "code"},
		),

		rpcDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "proxy_peer",
				Subsystem: "server",
				Name:      "rpc_duration_seconds",
				Help:      "Duration in seconds of RPCs sent by the server.",
			},
			[]string{"service", "handler", "code"},
		),

		messageSent: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "proxy_peer",
				Subsystem: "server",
				Name:      "message_sent_size",
				Help:      "Size of messages sent by the server.",
			},
			[]string{"service", "handler"},
		),

		messageReceived: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "proxy_peer",
				Subsystem: "server",
				Name:      "message_received_size",
				Help:      "Size of messages received by the server.",
			},
			[]string{"service", "handler"},
		),
	}

	if err := utils.RegisterPrometheusCollectors(
		sm.connections,
		sm.rpcs,
		sm.rpcTotal,
		sm.rpcDuration,
		sm.messageSent,
		sm.messageReceived,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	return sm, nil
}

// getConnectionGauge is a getter for the connectionCounter collector.
func (s *serverMetrics) getConnectionGauge() *prometheus.GaugeVec {
	return s.connections
}

// getRPCGauge is a getter for the rpcs collector.
func (s *serverMetrics) getRPCGauge() *prometheus.GaugeVec {
	return s.rpcs
}

// getRPCCounter is a getter for the rpcTotal collector.
func (s *serverMetrics) getRPCCounter() *prometheus.CounterVec {
	return s.rpcTotal
}

// getRPCDuration is a getter for the rpcDuration collector.
func (s *serverMetrics) getRPCDuration() *prometheus.HistogramVec {
	return s.rpcDuration
}

// getMessageSent is a getter for the messageSent collector.
func (s *serverMetrics) getMessageSent() *prometheus.HistogramVec {
	return s.messageSent
}

// getMessageReceived is a getter for the messageReceived collector.
func (s *serverMetrics) getMessageReceived() *prometheus.HistogramVec {
	return s.messageReceived
}
