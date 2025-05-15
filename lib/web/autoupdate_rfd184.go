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

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/webclient"
	autoupdatepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
)

// automaticUpdateSettings184 crafts the automatic updates part of the ping/find response
// as described in RFD-184 (agents) and RFD-144 (tools).
func (h *Handler) automaticUpdateSettings184(ctx context.Context, group, updaterUUID string) webclient.AutoUpdateSettings {
	// Tools auto updates section.
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

	// Agent auto updates section.
	agentVersion, err := h.autoUpdateAgentVersion(ctx, group, updaterUUID)
	if err != nil {
		h.logger.ErrorContext(ctx, "failed to resolve AgentVersion", "error", err)
		// Defaulting to current version
		agentVersion = teleport.SemVer()
	}
	// If the source of truth is RFD 109 configuration (channels + CMC) we must emulate the
	// RFD109 agent maintenance window behavior by looking up the CMC and checking if
	// we are in a maintenance window.
	shouldUpdate, err := h.autoUpdateAgentShouldUpdate(ctx, group, updaterUUID, true /* window lookup */)
	if err != nil {
		h.logger.ErrorContext(ctx, "failed to resolve AgentAutoUpdate", "error", err)
		// Failing open
		shouldUpdate = false
	}

	toolsVersion, err := getToolsVersion(autoUpdateVersion)
	if err != nil {
		h.logger.ErrorContext(ctx, "failed to get tools version", "error", err)
		toolsVersion = teleport.SemVer()
	}

	return webclient.AutoUpdateSettings{
		ToolsAutoUpdate:          getToolsAutoUpdate(autoUpdateConfig),
		ToolsVersion:             toolsVersion.String(),
		AgentUpdateJitterSeconds: DefaultAgentUpdateJitterSeconds,
		AgentVersion:             agentVersion.String(),
		AgentAutoUpdate:          shouldUpdate,
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

func getToolsVersion(v *autoupdatepb.AutoUpdateVersion) (*semver.Version, error) {
	// If we can't get the AU version or tools AU version is not specified, we default to the current proxy version.
	// This ensures we always advertise a version compatible with the cluster.
	if v.GetSpec().GetTools() == nil {
		return teleport.SemVer(), nil
	}
	return version.EnsureSemver(v.GetSpec().GetTools().GetTargetVersion())
}
