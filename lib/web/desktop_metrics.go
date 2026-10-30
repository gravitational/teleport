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

package web

import (
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

func init() {
	_ = metrics.RegisterPrometheusCollectors(
		windowsDesktopMetrics.loginAttempts,
		windowsDesktopMetrics.loginErrors,
		windowsDesktopMetrics.connectLatency,

		linuxDesktopMetrics.loginAttempts,
		linuxDesktopMetrics.loginErrors,
		linuxDesktopMetrics.connectLatency,
	)
}

// desktopMetrics is the set of login metrics this proxy records for one kind of desktop.
type desktopMetrics struct {
	loginAttempts  prometheus.Counter
	loginErrors    prometheus.Counter
	connectLatency prometheus.Histogram
}

// latencyBuckets covers 50ms up to ~102s.
var latencyBuckets = prometheus.ExponentialBuckets(0.05, 2, 12)

var (
	windowsDesktopMetrics = desktopMetrics{
		loginAttempts: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "proxy_windows_desktop",
			Name:      "login_attempts_total",
			Help:      "Number of Windows desktop logins attempted through this proxy.",
		}),
		loginErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "proxy_windows_desktop",
			Name:      "login_errors_total",
			Help:      "Number of Windows desktop logins that failed before this proxy started forwarding traffic to the agent.",
		}),
		connectLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "proxy_windows_desktop",
			Name:      "connect_latency_seconds",
			Help:      "Time from the user's certificates being issued until this proxy has connected to a windows_desktop_service.",
			Buckets:   latencyBuckets,
		}),
	}

	linuxDesktopMetrics = desktopMetrics{
		loginAttempts: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "proxy_linux_desktop",
			Name:      "login_attempts_total",
			Help:      "Number of Linux desktop logins attempted through this proxy.",
		}),
		loginErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "proxy_linux_desktop",
			Name:      "login_errors_total",
			Help:      "Number of Linux desktop logins that failed before this proxy started forwarding traffic to the agent.",
		}),
		connectLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: "proxy_linux_desktop",
			Name:      "connect_latency_seconds",
			Help:      "Time from the user's certificates being issued until this proxy has connected to a linux_desktop_service.",
			Buckets:   latencyBuckets,
		}),
	}
)

// desktopMetricsFor returns the metric set matching the kind of desktop that
// certUsage requests access to.
func desktopMetricsFor(certUsage proto.UserCertsRequest_CertUsage) desktopMetrics {
	if certUsage == proto.UserCertsRequest_LinuxDesktop {
		return linuxDesktopMetrics
	}
	return windowsDesktopMetrics
}

// desktopMetricRecorder records the Prometheus metrics for a single desktop
// login handled by this proxy.
type desktopMetricRecorder struct {
	clock   clockwork.Clock
	metrics desktopMetrics

	start    time.Time
	recorded bool
}

func newDesktopMetricRecorder(clock clockwork.Clock, metrics desktopMetrics) *desktopMetricRecorder {
	return &desktopMetricRecorder{
		clock:   clock,
		metrics: metrics,
		start:   clock.Now(),
	}
}

// startLatency starts measuring connect latency. It must be called once the
// user's certificates have been issued.
func (d *desktopMetricRecorder) startLatency() {
	d.start = d.clock.Now()
}

// agentDialed records the connect latency, and must be called once a connection
// to the agent has been established.
func (d *desktopMetricRecorder) agentDialed() {
	d.metrics.connectLatency.Observe(d.clock.Since(d.start).Seconds())
}

// success counts a login that made it all the way to the agent.
func (d *desktopMetricRecorder) success() {
	if d.recorded {
		return
	}
	d.recorded = true
	d.metrics.loginAttempts.Inc()
}

// failure counts a login that didn't make it to the agent.
func (d *desktopMetricRecorder) failure() {
	if d.recorded {
		return
	}
	d.recorded = true
	d.metrics.loginAttempts.Inc()
	d.metrics.loginErrors.Inc()
}
