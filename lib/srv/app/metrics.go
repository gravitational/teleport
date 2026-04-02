/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package app

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

func init() {
	_ = metrics.RegisterPrometheusCollectors(activeSessions)
}

// activeSessions tracks the number of active HTTP app sessions on
// this agent. Each session maps to one cached session chunk in the
// ConnectionsHandler. TCP and MCP sessions are not included because
// they bypass the session chunk cache.
var activeSessions = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: teleport.MetricNamespace,
	Subsystem: "app",
	Name:      "active_sessions",
	Help:      "Number of active HTTP app sessions on this agent. Does not include TCP or MCP sessions.",
}, []string{"app"})
