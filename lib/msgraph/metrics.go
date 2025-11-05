/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package msgraph

import (
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
)

type clientMetrics struct {
	// requestsTotal keeps track of the number of requests done by the client
	// This metric is labeled by status code.
	requestTotal *prometheus.CounterVec
	// requestDuration keeps track of the request duration, in seconds.
	requestDuration *prometheus.HistogramVec
}

const (
	metricSubsystem     = "msgraph"
	metricsLabelStatus  = "status"
	metricsLabelsMethod = "method"
)

func newMetrics() *clientMetrics {
	return &clientMetrics{
		requestTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricSubsystem,
			Name:      "request_total",
			Help:      "Total number of requests made to MS Graph",
		}, []string{metricsLabelsMethod, metricsLabelStatus}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricSubsystem,
			Name:      "request_duration_seconds",
			Help:      "Request to MS Graph duration in seconds.",
		}, []string{metricsLabelsMethod}),
	}
}

func (metrics *clientMetrics) register(r prometheus.Registerer) error {
	return trace.NewAggregate(
		r.Register(metrics.requestTotal),
		r.Register(metrics.requestDuration),
	)
}
