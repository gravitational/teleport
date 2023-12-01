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

const (
	errorProxyPeerTunnelNotFound     = "TUNNEL_NOT_FOUND"
	errorProxyPeerTunnelDial         = "TUNNEL_DIAL"
	errorProxyPeerTunnelDirectDial   = "TUNNEL_DIRECT_DIAL"
	errorProxyPeerTunnelRPC          = "TUNNEL_RPC"
	errorProxyPeerFetchProxies       = "FETCH_PROXIES"
	errorProxyPeerProxiesUnreachable = "PROXIES_UNREACHABLE"
)

// clientMetrics represents a collection of grpcMetrics for a proxy peer client
type clientMetrics struct {
	dialErrors      *prometheus.CounterVec
	connections     *prometheus.GaugeVec
	rpcs            *prometheus.GaugeVec
	rpcTotal        *prometheus.CounterVec
	rpcDuration     *prometheus.HistogramVec
	messageSent     *prometheus.HistogramVec
	messageReceived *prometheus.HistogramVec
}

// newClientMetrics inits and registers client grpcMetrics prometheus collectors.
func newClientMetrics() (*clientMetrics, error) {
	cm := &clientMetrics{
		dialErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "proxy_peer",
				Subsystem: "client",
				Name:      "dial_error_total",
				Help:      "Total number of errors encountered dialing peer proxies.",
			},
			[]string{"error_type"},
		),

		connections: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "proxy_peer",
				Subsystem: "client",
				Name:      "connections",
				Help:      "Number of currently opened connection to proxy peer servers.",
			},
			[]string{"local_id", "remote_id", "state"},
		),

		rpcs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "proxy_peer",
				Subsystem: "client",
				Name:      "rpc",
				Help:      "Number of current client RPC requests.",
			},
			[]string{"service", "method"},
		),

		rpcTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "proxy_peer",
				Subsystem: "client",
				Name:      "rpc_total",
				Help:      "Total number of client RPC requests.",
			},
			[]string{"service", "method", "code"},
		),

		rpcDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "proxy_peer",
				Subsystem: "client",
				Name:      "rpc_duration_seconds",
				Help:      "Duration in seconds of RPCs sent by the client.",
			},
			[]string{"service", "handler", "code"},
		),

		messageSent: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "proxy_peer",
				Subsystem: "client",
				Name:      "message_sent_size",
				Help:      "Size of messages sent by the client.",
				Buckets:   messageByteBuckets,
			},
			[]string{"service", "handler"},
		),

		messageReceived: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "proxy_peer",
				Subsystem: "client",
				Name:      "message_received_size",
				Help:      "Size of messages received by the client.",
				Buckets:   messageByteBuckets,
			},
			[]string{"service", "handler"},
		),
	}

	if err := metrics.RegisterPrometheusCollectors(
		cm.dialErrors,
		cm.connections,
		cm.rpcs,
		cm.rpcTotal,
		cm.rpcDuration,
		cm.messageSent,
		cm.messageReceived,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	return cm, nil
}

// reportTunnelError reports errors encountered dialing an existing peer tunnel.
func (c *clientMetrics) reportTunnelError(errorType string) {
	c.dialErrors.WithLabelValues(errorType).Inc()
}

// getConnectionGauge is a getter for the connections collector.
func (c *clientMetrics) getConnectionGauge() *prometheus.GaugeVec {
	return c.connections
}

// getRPCGauge is a getter for the rpcs collector.
func (c *clientMetrics) getRPCGauge() *prometheus.GaugeVec {
	return c.rpcs
}

// getRPCCounter is a getter for the rpcTotal collector.
func (c *clientMetrics) getRPCCounter() *prometheus.CounterVec {
	return c.rpcTotal
}

// getRPCDuration is a getter for the rpcDuration collector.
func (c *clientMetrics) getRPCDuration() *prometheus.HistogramVec {
	return c.rpcDuration
}

// getMessageSent is a getter for the messageSent collector.
func (c *clientMetrics) getMessageSent() *prometheus.HistogramVec {
	return c.messageSent
}

// getMessageReceived is a getter for the messageReceived collector.
func (c *clientMetrics) getMessageReceived() *prometheus.HistogramVec {
	return c.messageReceived
}
