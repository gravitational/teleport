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

package mcp

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
)

var (
	setupErrors = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "setup_errors_total",
			Subsystem: "mcp",
			Help:      "Number of errors encountered when setting up MCP sessions",
		},
		[]string{"transport"},
	)

	accumulatedSessions = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "sessions_total",
			Subsystem: "mcp",
			Help:      "Number of accumulated MCP sessions",
		},
		[]string{"transport"},
	)

	activeSessions = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "active_sessions_total",
			Subsystem: "mcp",
			Help:      "Number of active MCP sessions",
		},
		[]string{"transport"},
	)

	messagesFromClient = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "messages_from_client_total",
			Subsystem: "mcp",
			Help:      "Number of messages received from the MCP client",
		},
		[]string{"transport", "type", "method"},
	)

	messagesFromServer = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "messages_from_server_total",
			Subsystem: "mcp",
			Help:      "Number of messages received from the MCP server",
		},
		[]string{"transport", "type", "method"},
	)

	allPrometheusCollectors = []prometheus.Collector{
		setupErrors,
		accumulatedSessions, activeSessions,
		messagesFromClient, messagesFromServer,
	}
)
