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

// clientMetrics represents a collection of metrics for a proxy peer client
type clientMetrics struct {
	tunnelErrorCounter *prometheus.CounterVec

	connections     *prometheus.GaugeVec
	rpcs            *prometheus.GaugeVec
	rpcTotal        *prometheus.CounterVec
	rpcDuration     *prometheus.HistogramVec
	messageSent     *prometheus.HistogramVec
	messageReceived *prometheus.HistogramVec
}

// newClientMetrics inits and registers client metrics prometheus collectors.
func newClientMetrics() (*clientMetrics, error) {
	cm := &clientMetrics{
		tunnelErrorCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "proxy_peer",
				Subsystem: "client",
				Name:      "dial_error_total",
				Help:      "Total number of errors encountered dialling peer proxies.",
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
			[]string{"local_addr", "remote_addr"},
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
			},
			[]string{"service", "handler"},
		),

		messageReceived: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "proxy_peer",
				Subsystem: "client",
				Name:      "message_received_size",
				Help:      "Size of messages received by the client.",
			},
			[]string{"service", "handler"},
		),
	}

	if err := utils.RegisterPrometheusCollectors(
		cm.tunnelErrorCounter,

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
func (c *clientMetrics) reportTunnelError(status string) {
	c.tunnelErrorCounter.WithLabelValues(status).Inc()
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
