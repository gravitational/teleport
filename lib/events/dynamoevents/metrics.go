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

package dynamoevents

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
)

var (
	writeRequestsDeduped = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "dynamo_events_backend_write_deduped",
			Help:      "Number of write requests that were de-duplicated because an identical event already existed.",
		},
	)
	eventIDCollisions = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "dynamo_events_backend_event_id_collisions",
			Help:      "Number of distinct audit events that collided with an existing event sharing the same id and were re-inserted under a newly generated id.",
		},
	)

	prometheusCollectors = []prometheus.Collector{
		writeRequestsDeduped, eventIDCollisions,
	}
)
