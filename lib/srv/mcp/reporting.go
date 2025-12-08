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
	"slices"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

var (
	setupErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "setup_errors_total",
			Subsystem: "mcp",
			Help:      "Number of errors encountered when setting up MCP sessions",
		},
		[]string{"transport"},
	)

	accumulatedSessions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
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

	// knownNotificationMethods is a list of known method names for notifications.
	//
	// The list is obtained by searching these in addition to mcp-go:
	// - https://github.com/modelcontextprotocol/modelcontextprotocol
	// - https://github.com/modelcontextprotocol/typescript-sdk/blob/main/src/server/index.ts
	knownNotificationMethods = []string{
		//nolint:misspell // "cancelled" is "UK" spelling but our linter is set to use US locale
		"notifications/cancelled",
		"notifications/initialized",
		"notifications/message",
		"notifications/progress",
		mcputils.MethodNotificationPromptsListChanged,   // notifications/prompts/list_changed
		mcputils.MethodNotificationResourcesListChanged, // notifications/resources/list_changed
		mcputils.MethodNotificationResourceUpdated,      // notifications/resources/updated
		mcputils.MethodNotificationToolsListChanged,     // notifications/tools/list_changed
		mcputils.MethodNotificationRootsListChanged,     // notifications/roots/list_changed
	}

	// knownRequestMethods is a list of known method names for requests.
	//
	// The list is obtained by searching these in addition to mcp-go:
	// - https://github.com/modelcontextprotocol/modelcontextprotocol
	// - https://github.com/modelcontextprotocol/typescript-sdk/blob/main/src/server/index.ts
	knownRequestMethods = []string{
		mcputils.MethodInitialize,             // initialize
		mcputils.MethodPing,                   // ping
		mcputils.MethodResourcesList,          // resources/list
		mcputils.MethodResourcesTemplatesList, // resources/templates/list
		mcputils.MethodResourcesRead,          // resources/read
		mcputils.MethodPromptsList,            // prompts/list
		mcputils.MethodPromptsGet,             // prompts/get
		mcputils.MethodToolsList,              // tools/list
		mcputils.MethodToolsCall,              // tools/call
		mcputils.MethodSetLogLevel,            // logging/setLevel
		mcputils.MethodElicitationCreate,      // elicitation/create
		mcputils.MethodListRoots,              // roots/list
		mcputils.MethodSamplingCreateMessage,  // sampling/createMessage
	}
)

func reportNotificationMethod(method string) string {
	if slices.Contains(knownNotificationMethods, method) {
		return method
	}
	return "unknown"
}

func reportRequestMethod(method string) string {
	if slices.Contains(knownRequestMethods, method) {
		return method
	}
	return "unknown"
}
