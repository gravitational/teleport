/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package peer

import (
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport/lib/observability/metrics"
)

// serverMetrics represents a collection of grpcMetrics for a proxy peer server
type serverMetrics struct {
	connections     *prometheus.GaugeVec
	rpcs            *prometheus.GaugeVec
	rpcTotal        *prometheus.CounterVec
	rpcDuration     *prometheus.HistogramVec
	messageSent     *prometheus.HistogramVec
	messageReceived *prometheus.HistogramVec
}

// newServerMetrics inits and registers client grpcMetrics prometheus collectors.
func newServerMetrics() (*serverMetrics, error) {
	sm := &serverMetrics{
		connections: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "proxy_peer",
				Subsystem: "server",
				Name:      "connections",
				Help:      "Number of currently opened connection to proxy peer clients.",
			},
			[]string{"local_id", "remote_id", "state"},
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
				Buckets:   messageByteBuckets,
			},
			[]string{"service", "handler"},
		),

		messageReceived: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "proxy_peer",
				Subsystem: "server",
				Name:      "message_received_size",
				Help:      "Size of messages received by the server.",
				Buckets:   messageByteBuckets,
			},
			[]string{"service", "handler"},
		),
	}

	if err := metrics.RegisterPrometheusCollectors(
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
