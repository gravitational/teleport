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
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// messageByteBuckets creates buckets ranging from 32-65536 bytes.
var messageByteBuckets = prometheus.ExponentialBuckets(32, 2, 12)

type grpcMetrics interface {
	getConnectionGauge() *prometheus.GaugeVec
	getRPCGauge() *prometheus.GaugeVec
	getRPCCounter() *prometheus.CounterVec
	getRPCDuration() *prometheus.HistogramVec
	getMessageSent() *prometheus.HistogramVec
	getMessageReceived() *prometheus.HistogramVec
}

// reporter is grpc request specific grpcMetrics reporter.
type reporter struct {
	grpcMetrics
}

// newReporter returns a new reporter object that is used to
// report grpcMetrics relative to proxy peer client or server.
func newReporter(m grpcMetrics) *reporter {
	return &reporter{
		grpcMetrics: m,
	}
}

// resetConnections resets the current number of connections.
func (r *reporter) resetConnections() {
	r.getConnectionGauge().Reset()
}

// incConnection increases the current number of connections.
func (r *reporter) incConnection(localID, remoteID, state string) {
	r.getConnectionGauge().WithLabelValues(localID, remoteID, state).Inc()
}

// decConnection decreases the current number of connections.
func (r *reporter) decConnection(localAddr, remoteAddr, state string) {
	r.getConnectionGauge().WithLabelValues(localAddr, remoteAddr, state).Dec()
}

// incRPC increases the current number of rpcs.
func (r *reporter) incRPC(service, method string) {
	r.getRPCGauge().WithLabelValues(service, method).Inc()
}

// decRPC decreases the current number of rpcs.
func (r *reporter) decRPC(service, method string) {
	r.getRPCGauge().WithLabelValues(service, method).Dec()
}

// countRPC reports the total number of handled rpcs.
func (r *reporter) countRPC(service, method, code string) {
	r.getRPCCounter().WithLabelValues(service, method, code).Inc()
}

// timeRPC reports the duration of handled rpcs.
func (r *reporter) timeRPC(service, method, code string, duration time.Duration) {
	r.getRPCDuration().WithLabelValues(service, method, code).Observe(duration.Seconds())
}

// measureMessageSent reports the size of sent messages.
func (r *reporter) measureMessageSent(service, method string, size float64) {
	r.getMessageSent().WithLabelValues(service, method).Observe(size)
}

// measureMessageReceived reports the size of received messages.
func (r *reporter) measureMessageReceived(service, method string, size float64) {
	r.getMessageReceived().WithLabelValues(service, method).Observe(size)
}
