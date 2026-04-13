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

package autoupdate

import (
	"context"
	"log/slog"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	autoupdatepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
)

const (
	// DefaultAgentUpdateJitterSeconds is the default jitter agents should wait before updating.
	DefaultAgentUpdateJitterSeconds = 60
)

type accessPoint interface {
	GetAutoUpdateConfig(ctx context.Context) (*autoupdatepb.AutoUpdateConfig, error)
	GetAutoUpdateVersion(ctx context.Context) (*autoupdatepb.AutoUpdateVersion, error)
}

type lookupResolver interface {
	GetVersion(ctx context.Context, group, updaterUUID string) (*semver.Version, error)
	ShouldUpdate(ctx context.Context, group, updaterUUID string, windowLookup bool) (bool, error)
}

// AutoUpdateSettingsServiceConfig contains the configuration for AutoUpdateSettingsService.
type AutoUpdateSettingsServiceConfig struct {
	Logger *slog.Logger
	// AccessPoint provides access to the backend for retrieving auto update configuration and version.
	AccessPoint accessPoint
	// LookupResolver provides the logic for resolving the agent version and whether it should update based on the group, UUID, and RFD 184 rollout.
	LookupResolver lookupResolver
}

func (c *AutoUpdateSettingsServiceConfig) checkAndSetDefaults() error {
	if c == nil {
		return trace.BadParameter("missing parameter AutoUpdateSettingsServiceConfig")
	}

	if c.Logger == nil {
		return trace.BadParameter("missing parameter Logger")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing parameter AccessPoint")
	}
	if c.LookupResolver == nil {
		return trace.BadParameter("missing parameter LookupResolver")
	}

	return nil
}

// AutoUpdateSettingsService provides the auto update settings for agents and tools, as described in RFD-184 (agents) and RFD-144 (tools).
type AutoUpdateSettingsService struct {
	*AutoUpdateSettingsServiceConfig
}

// NewAutoUpdateSettingsService creates a new AutoUpdateSettingsService.
func NewAutoUpdateSettingsService(config *AutoUpdateSettingsServiceConfig) (*AutoUpdateSettingsService, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &AutoUpdateSettingsService{
		AutoUpdateSettingsServiceConfig: config,
	}, nil
}

// Get returns the auto update settings for agents and tools, as described in RFD-184 (agents) and RFD-144 (tools).
func (a *AutoUpdateSettingsService) Get(ctx context.Context, group, updaterUUID string) *proto.AutoUpdateSettings {
	// Tools auto updates section.
	autoUpdateConfig, err := a.AccessPoint.GetAutoUpdateConfig(ctx)
	// TODO(vapopov) DELETE IN v18.0.0 check of IsNotImplemented, must be backported to all latest supported versions.
	if err != nil && !trace.IsNotFound(err) && !trace.IsNotImplemented(err) {
		a.Logger.ErrorContext(ctx, "failed to receive AutoUpdateConfig", "error", err)
	}

	autoUpdateVersion, err := a.AccessPoint.GetAutoUpdateVersion(ctx)
	// TODO(vapopov) DELETE IN v18.0.0 check of IsNotImplemented, must be backported to all latest supported versions.
	if err != nil && !trace.IsNotFound(err) && !trace.IsNotImplemented(err) {
		a.Logger.ErrorContext(ctx, "failed to receive AutoUpdateVersion", "error", err)
	}

	// Agent auto updates section.
	agentVersion, err := a.LookupResolver.GetVersion(ctx, group, updaterUUID)
	if err != nil {
		a.Logger.ErrorContext(ctx, "failed to resolve AgentVersion", "error", err)
		// Defaulting to current version
		agentVersion = teleport.SemVer()
	}
	// If the source of truth is RFD 109 configuration (channels + CMC) we must emulate the
	// RFD109 agent maintenance window behavior by looking up the CMC and checking if
	// we are in a maintenance window.
	shouldUpdate, err := a.LookupResolver.ShouldUpdate(ctx, group, updaterUUID, true /* window lookup */)
	if err != nil {
		a.Logger.ErrorContext(ctx, "failed to resolve AgentAutoUpdate", "error", err)
		// Failing open
		shouldUpdate = false
	}

	toolsVersion, err := GetToolsVersion(autoUpdateVersion)
	if err != nil {
		a.Logger.ErrorContext(ctx, "failed to get tools version", "error", err)
		toolsVersion = teleport.SemVer()
	}

	return &proto.AutoUpdateSettings{
		ToolsAutoUpdate:          getToolsAutoUpdate(autoUpdateConfig),
		ToolsVersion:             toolsVersion.String(),
		AgentUpdateJitterSeconds: DefaultAgentUpdateJitterSeconds,
		AgentVersion:             agentVersion.String(),
		AgentAutoUpdate:          shouldUpdate,
	}
}

// GetToolsVersion returns the version tools should update to based on the auto update version resource.
func GetToolsVersion(v *autoupdatepb.AutoUpdateVersion) (*semver.Version, error) {
	// If we can't get the AU version or tools AU version is not specified, we default to the current proxy version.
	// This ensures we always advertise a version compatible with the cluster.
	if v.GetSpec().GetTools() == nil {
		return teleport.SemVer(), nil
	}
	return version.EnsureSemver(v.GetSpec().GetTools().GetTargetVersion())
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
