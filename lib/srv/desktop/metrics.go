// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package desktop

import (
	"sync/atomic"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
)

func init() {
	_ = metrics.RegisterPrometheusCollectors(
		windowsMetrics.loginAttempts,
		windowsMetrics.loginErrors,
		windowsMetrics.connectLatency,

		linuxMetrics.loginAttempts,
		linuxMetrics.loginErrors,
		linuxMetrics.connectLatency,
	)
}

// desktopMetrics is the set of login metrics recorded by
// a Teleport desktop agent (both Windows and Linux).
type desktopMetrics struct {
	loginAttempts  *prometheus.CounterVec
	loginErrors    *prometheus.CounterVec
	connectLatency *prometheus.HistogramVec
}

// latencyBuckets covers 50ms up to ~102s.
var latencyBuckets = prometheus.ExponentialBuckets(0.05, 2, 12)

var (
	windowsMetrics = desktopMetrics{
		loginAttempts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "windows_desktop",
			Name:      "login_attempts_total",
			Help:      "Number of Windows desktop logins attempted through this windows_desktop_service.",
		}, []string{"windows_desktop_service"}),
		loginErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "windows_desktop",
			Name:      "login_errors_total",
			Help:      "Number of Windows desktop logins that failed before the RDP session was established.",
		}, []string{"windows_desktop_service"}),
		connectLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "windows_desktop",
			Name:      "connect_latency_seconds",
			Help:      "Time from a proxy connection arriving at this windows_desktop_service until the first screen fragment of the desktop is sent back to the proxy.",
			Buckets:   latencyBuckets,
		}, []string{"windows_desktop_service"}),
	}

	linuxMetrics = desktopMetrics{
		loginAttempts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "linux_desktop",
			Name:      "login_attempts_total",
			Help:      "Number of Linux desktop logins attempted through this linux_desktop_service.",
		}, []string{"linux_desktop_service"}),
		loginErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "linux_desktop",
			Name:      "login_errors_total",
			Help:      "Number of Linux desktop logins that failed before the desktop session was started.",
		}, []string{"linux_desktop_service"}),
		connectLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "linux_desktop",
			Name:      "connect_latency_seconds",
			Help:      "Time from the user selecting an X session until the first screen fragment of the desktop is sent back to the proxy.",
			Buckets:   latencyBuckets,
		}, []string{"linux_desktop_service"}),
	}
)

// metricRecorder records the Prometheus metrics for a single desktop login.
type metricRecorder struct {
	clock   clockwork.Clock
	metrics desktopMetrics
	hostID  string

	recorded     atomic.Bool
	latencyStart atomic.Pointer[time.Time]
}

func newWindowsMetricRecorder(clock clockwork.Clock, hostID string) *metricRecorder {
	return newMetricRecorder(clock, hostID, windowsMetrics)
}

func newLinuxMetricRecorder(clock clockwork.Clock, hostID string) *metricRecorder {
	return newMetricRecorder(clock, hostID, linuxMetrics)
}

func newMetricRecorder(clock clockwork.Clock, hostID string, metrics desktopMetrics) *metricRecorder {
	return &metricRecorder{
		clock:   clock,
		metrics: metrics,
		hostID:  hostID,
	}
}

// startLatency starts measuring connect latency. It must be called once
// everything the login was waiting on the user for is done, so that user
// interactivity isn't counted as connect latency.
func (r *metricRecorder) startLatency() {
	now := r.clock.Now()
	r.latencyStart.Store(&now)
}

// success counts a login that established a desktop session.
func (r *metricRecorder) success() {
	if !r.recorded.CompareAndSwap(false, true) {
		return
	}
	r.metrics.loginAttempts.WithLabelValues(r.hostID).Inc()
}

// failure counts a login that didn't establish a desktop session.
func (r *metricRecorder) failure() {
	if !r.recorded.CompareAndSwap(false, true) {
		return
	}
	r.metrics.loginAttempts.WithLabelValues(r.hostID).Inc()
	r.metrics.loginErrors.WithLabelValues(r.hostID).Inc()
}

// intercept records the connect latency the first time it sees a screen
// fragment after the measurement has been armed. It's a [tdp.Interceptor] meant
// for the write path of the connection to the proxy.
func (r *metricRecorder) intercept(msg tdp.Message) ([]tdp.Message, error) {
	if start := r.latencyStart.Load(); start != nil && tdpb.IsScreenFragment(msg) {
		if r.latencyStart.CompareAndSwap(start, nil) {
			r.metrics.connectLatency.WithLabelValues(r.hostID).Observe(r.clock.Since(*start).Seconds())
		}
	}
	return []tdp.Message{msg}, nil
}
