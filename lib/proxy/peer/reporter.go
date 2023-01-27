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
