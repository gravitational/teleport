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
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type metrics interface {
	getConnectionGauge() *prometheus.GaugeVec
	getRPCGauge() *prometheus.GaugeVec
	getRPCCounter() *prometheus.CounterVec
	getRPCDuration() *prometheus.HistogramVec
	getMessageSent() *prometheus.HistogramVec
	getMessageReceived() *prometheus.HistogramVec
}

// reporter is grpc request specific metrics reporter.
type reporter struct {
	metrics
}

// newReporter returns a new reporter object that is used to
// report metrics relative to proxy peer client or server.
func newReporter(m metrics) *reporter {
	return &reporter{
		metrics: m,
	}
}

// incConnection increases the current number of connections.
func (r *reporter) incConnection(localAddr, remoteAddr string) {
	r.getConnectionGauge().WithLabelValues(localAddr, remoteAddr).Inc()
}

// decConnection decreases the current number of connections.
func (r *reporter) decConnection(localAddr, remoteAddr string) {
	r.getConnectionGauge().WithLabelValues(localAddr, remoteAddr).Dec()
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
