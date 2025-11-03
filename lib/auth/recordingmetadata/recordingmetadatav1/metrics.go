/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package recordingmetadatav1

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

const subsystem = "recording_metadata"

var sessionsProcessedMetric = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: teleport.MetricNamespace,
	Subsystem: subsystem,
	Name:      "sessions_processed_total",
	Help:      "Total number of session recordings processed, with success or failure",
}, []string{"success"})

var sessionsProcessingMetric = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: teleport.MetricNamespace,
	Subsystem: subsystem,
	Name:      "sessions_processing_total",
	Help:      "Number of session recordings being processed",
})

var sessionsPendingMetric = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: teleport.MetricNamespace,
	Subsystem: subsystem,
	Name:      "sessions_pending_total",
	Help:      "Number of sessions waiting to be processed",
})

func init() {
	metrics.RegisterPrometheusCollectors(
		sessionsProcessedMetric,
		sessionsProcessingMetric,
	)
}
