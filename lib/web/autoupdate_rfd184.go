/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package web

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/client/webclient"
	autoupdatepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types/autoupdate"
)

// automaticUpdateSettings184 crafts the automatic updates part of the ping/find response
// as described in RFD-184 (agents) and RFD-144 (tools).
// TODO: add the request as a parameter when we'll need to modulate the content based on the UUID and group
func (h *Handler) automaticUpdateSettings184(ctx context.Context) webclient.AutoUpdateSettings {
	autoUpdateConfig, err := h.cfg.AccessPoint.GetAutoUpdateConfig(ctx)
	// TODO(vapopov) DELETE IN v18.0.0 check of IsNotImplemented, must be backported to all latest supported versions.
	if err != nil && !trace.IsNotFound(err) && !trace.IsNotImplemented(err) {
		h.logger.ErrorContext(ctx, "failed to receive AutoUpdateConfig", "error", err)
	}

	autoUpdateVersion, err := h.cfg.AccessPoint.GetAutoUpdateVersion(ctx)
	// TODO(vapopov) DELETE IN v18.0.0 check of IsNotImplemented, must be backported to all latest supported versions.
	if err != nil && !trace.IsNotFound(err) && !trace.IsNotImplemented(err) {
		h.logger.ErrorContext(ctx, "failed to receive AutoUpdateVersion", "error", err)
	}

	return webclient.AutoUpdateSettings{
		ToolsAutoUpdate:          getToolsAutoUpdate(autoUpdateConfig),
		ToolsVersion:             getToolsVersion(autoUpdateVersion),
		AgentUpdateJitterSeconds: DefaultAgentUpdateJitterSeconds,
		AgentVersion:             getAgentVersion184(autoUpdateVersion),
		AgentAutoUpdate:          agentShouldUpdate184(autoUpdateConfig, autoUpdateVersion),
	}
}

func getToolsAutoUpdate(config *autoupdatepb.AutoUpdateConfig) bool {
	// If we can't get the AU config or if AUs are not configured, we default to "disabled".
	// This ensures we fail open and don't accidentally update agents if something is going wrong.
	// If we want to enable AUs by default, it would be better to create a default "autoupdate_config" resource
	// than changing this logic.
	if config.GetSpec().GetTools() != nil {
		return config.GetSpec().GetTools().GetMode() == autoupdate.ToolsUpdateModeEnabled
	}
	return false
}

func getToolsVersion(version *autoupdatepb.AutoUpdateVersion) string {
	// If we can't get the AU version or tools AU version is not specified, we default to the current proxy version.
	// This ensures we always advertise a version compatible with the cluster.
	if version.GetSpec().GetTools() == nil {
		return api.Version
	}
	return version.GetSpec().GetTools().GetTargetVersion()
}

func getAgentVersion184(version *autoupdatepb.AutoUpdateVersion) string {
	// If we can't get the AU version or tools AU version is not specified, we default to the current proxy version.
	// This ensures we always advertise a version compatible with the cluster.
	// TODO: read the version from the autoupdate_agent_rollout when the resource is implemented
	if version.GetSpec().GetAgents() == nil {
		return api.Version
	}

	return version.GetSpec().GetAgents().GetTargetVersion()
}

func agentShouldUpdate184(config *autoupdatepb.AutoUpdateConfig, version *autoupdatepb.AutoUpdateVersion) bool {
	// TODO: read the data from the autoupdate_agent_rollout when the resource is implemented

	// If we can't get the AU config or if AUs are not configured, we default to "disabled".
	// This ensures we fail open and don't accidentally update agents if something is going wrong.
	// If we want to enable AUs by default, it would be better to create a default "autoupdate_config" resource
	// than changing this logic.
	if config.GetSpec().GetAgents() == nil {
		return false
	}
	if version.GetSpec().GetAgents() == nil {
		return false
	}
	configMode := config.GetSpec().GetAgents().GetMode()
	versionMode := version.GetSpec().GetAgents().GetMode()

	// We update only if both version and config agent modes are "enabled"
	if configMode != autoupdate.AgentsUpdateModeEnabled || versionMode != autoupdate.AgentsUpdateModeEnabled {
		return false
	}

	scheduleName := version.GetSpec().GetAgents().GetSchedule()
	return scheduleName == autoupdate.AgentsScheduleImmediate
}
