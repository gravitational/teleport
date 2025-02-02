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
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	autoupdatepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/utils"
)

// autoUpdateAgentVersion returns the version the agent should install/update to based on
// its group and updater UUID.
// If the cluster contains an autoupdate_agent_rollout resource from RFD184 it should take precedence.
// If the resource is not there, we fall back to RFD109-style updates with channels
// and maintenance window derived from the cluster_maintenance_config resource.
// Version returned follows semver without the leading "v".
func (h *Handler) autoUpdateAgentVersion(ctx context.Context, group, updaterUUID string) (string, error) {
	rollout, err := h.cfg.AccessPoint.GetAutoUpdateAgentRollout(ctx)
	if err != nil {
		// Fallback to channels if there is no autoupdate_agent_rollout.
		if trace.IsNotFound(err) {
			return getVersionFromChannel(ctx, h.cfg.AutomaticUpgradesChannels, group)
		}
		// Something is broken, we don't want to fallback to channels, this would be harmful.
		return "", trace.Wrap(err, "getting autoupdate_agent_rollout")
	}

	return getVersionFromRollout(rollout, group, updaterUUID)
}

// handlerVersionGetter is a dummy struct implementing version.Getter by wrapping Handler.autoUpdateAgentVersion.
type handlerVersionGetter struct {
	*Handler
}

// GetVersion implements version.Getter.
func (h *handlerVersionGetter) GetVersion(ctx context.Context) (string, error) {
	const group, updaterUUID = "", ""
	agentVersion, err := h.autoUpdateAgentVersion(ctx, group, updaterUUID)
	if err != nil {
		return "", trace.Wrap(err)
	}
	// We add the leading v required by the version.Getter interface.
	return fmt.Sprintf("v%s", agentVersion), nil
}

// autoUpdateAgentShouldUpdate returns if the agent should update now to based on its group
// and updater UUID.
// If the cluster contains an autoupdate_agent_rollout resource from RFD184 it should take precedence.
// If the resource is not there, we fall back to RFD109-style updates with channels
// and maintenance window derived from the cluster_maintenance_config resource.
func (h *Handler) autoUpdateAgentShouldUpdate(ctx context.Context, group, updaterUUID string, windowLookup bool) (bool, error) {
	rollout, err := h.cfg.AccessPoint.GetAutoUpdateAgentRollout(ctx)
	if err != nil {
		// Fallback to channels if there is no autoupdate_agent_rollout.
		if trace.IsNotFound(err) {
			// Updaters using the RFD184 API are not aware of maintenance windows
			// like RFD109 updaters are. To have both updaters adopt the same behavior
			// we must do the CMC window lookup for them.
			if windowLookup {
				return h.getTriggerFromWindowThenChannel(ctx, group)
			}
			return getTriggerFromChannel(ctx, h.cfg.AutomaticUpgradesChannels, group)
		}
		// Something is broken, we don't want to fallback to channels, this would be harmful.
		return false, trace.Wrap(err, "failed to get auto-update rollout")
	}

	return getTriggerFromRollout(rollout, group, updaterUUID)
}

// getVersionFromRollout returns the version we should serve to the agent based
// on the RFD184 agent rollout, the agent group name, and its UUID.
// This logic is pretty complex and described in RFD 184.
// The spec is summed up in the following table:
// https://github.com/gravitational/teleport/blob/master/rfd/0184-agent-auto-updates.md#rollout-status-disabled
// Version returned follows semver without the leading "v".
func getVersionFromRollout(
	rollout *autoupdatepb.AutoUpdateAgentRollout,
	groupName, updaterUUID string,
) (string, error) {
	switch rollout.GetSpec().GetAutoupdateMode() {
	case autoupdate.AgentsUpdateModeDisabled:
		// If AUs are disabled, we always answer the target version
		return rollout.GetSpec().GetTargetVersion(), nil
	case autoupdate.AgentsUpdateModeSuspended, autoupdate.AgentsUpdateModeEnabled:
		// If AUs are enabled or suspended, we modulate the response based on the schedule and agent group state
	default:
		return "", trace.BadParameter("unsupported agent update mode %q", rollout.GetSpec().GetAutoupdateMode())
	}

	// If the schedule is immediate, agents always update to the latest version
	if rollout.GetSpec().GetSchedule() == autoupdate.AgentsScheduleImmediate {
		return rollout.GetSpec().GetTargetVersion(), nil
	}

	// Else we follow the regular schedule and answer based on the agent group state
	group, err := getGroup(rollout, groupName)
	if err != nil {
		return "", trace.Wrap(err, "getting group %q", groupName)
	}

	switch group.GetState() {
	case autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK:
		return rollout.GetSpec().GetStartVersion(), nil
	case autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:
		return rollout.GetSpec().GetTargetVersion(), nil
	default:
		return "", trace.NotImplemented("unsupported group state %q", group.GetState())
	}
}

// getTriggerFromRollout returns the version we should serve to the agent based
// on the RFD184 agent rollout, the agent group name, and its UUID.
// This logic is pretty complex and described in RFD 184.
// The spec is summed up in the following table:
// https://github.com/gravitational/teleport/blob/master/rfd/0184-agent-auto-updates.md#rollout-status-disabled
func getTriggerFromRollout(rollout *autoupdatepb.AutoUpdateAgentRollout, groupName, updaterUUID string) (bool, error) {
	// If the mode is "paused" or "disabled", we never tell to update
	switch rollout.GetSpec().GetAutoupdateMode() {
	case autoupdate.AgentsUpdateModeDisabled, autoupdate.AgentsUpdateModeSuspended:
		// If AUs are disabled or suspended, never tell to update
		return false, nil
	case autoupdate.AgentsUpdateModeEnabled:
		// If AUs are enabled, we modulate the response based on the schedule and agent group state
	default:
		return false, trace.BadParameter("unsupported agent update mode %q", rollout.GetSpec().GetAutoupdateMode())
	}

	// If the schedule is immediate, agents always update to the latest version
	if rollout.GetSpec().GetSchedule() == autoupdate.AgentsScheduleImmediate {
		return true, nil
	}

	// Else we follow the regular schedule and answer based on the agent group state
	group, err := getGroup(rollout, groupName)
	if err != nil {
		return false, trace.Wrap(err, "getting group %q", groupName)
	}

	switch group.GetState() {
	case autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:
		return false, nil
	case autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK:
		return true, nil
	case autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:
		return rollout.GetSpec().GetStrategy() == autoupdate.AgentsStrategyHaltOnError, nil
	default:
		return false, trace.NotImplemented("Unsupported group state %q", group.GetState())
	}
}

// getGroup returns the agent rollout group the requesting agent belongs to.
// If a group matches the agent-provided group name, this group is returned.
// Else the default group is returned. The default group currently is the last
// one. This might change in the future.
func getGroup(
	rollout *autoupdatepb.AutoUpdateAgentRollout,
	groupName string,
) (*autoupdatepb.AutoUpdateAgentRolloutStatusGroup, error) {
	groups := rollout.GetStatus().GetGroups()
	if len(groups) == 0 {
		return nil, trace.BadParameter("no groups found")
	}

	// Try to find a group with our name
	for _, group := range groups {
		if group.Name == groupName {
			return group, nil
		}
	}

	// Fallback to the default group (currently the last one but this might change).
	return groups[len(groups)-1], nil
}

// getVersionFromChannel gets the target version from the RFD109 channels.
// Version returned follows semver without the leading "v".
func getVersionFromChannel(ctx context.Context, channels automaticupgrades.Channels, groupName string) (version string, err error) {
	// RFD109 channels return the version with the 'v' prefix.
	// We can't change the internals for backward compatibility, so we must trim the prefix if it's here.
	defer func() {
		version = strings.TrimPrefix(version, "v")
	}()

	if channel, ok := channels[groupName]; ok {
		return channel.GetVersion(ctx)
	}
	return channels.DefaultVersion(ctx)
}

// getTriggerFromWindowThenChannel gets the target version from the RFD109 maintenance window and channels.
func (h *Handler) getTriggerFromWindowThenChannel(ctx context.Context, groupName string) (bool, error) {
	// Caching the CMC for 10 seconds because this resource is cached neither by the auth nor the proxy.
	// And this function can be accessed via unauthenticated endpoints.
	cmc, err := utils.FnCacheGet[types.ClusterMaintenanceConfig](ctx, h.clusterMaintenanceConfigCache, "cmc", func(ctx context.Context) (types.ClusterMaintenanceConfig, error) {
		return h.cfg.ProxyClient.GetClusterMaintenanceConfig(ctx)
	})

	// If we have a CMC, we check if the window is active, else we just check if the update is critical.
	if err == nil && cmc.WithinUpgradeWindow(h.clock.Now()) {
		return true, nil
	}

	return getTriggerFromChannel(ctx, h.cfg.AutomaticUpgradesChannels, groupName)
}

// getTriggerFromWindowThenChannel gets the target version from the RFD109 channels.
func getTriggerFromChannel(ctx context.Context, channels automaticupgrades.Channels, groupName string) (bool, error) {
	if channel, ok := channels[groupName]; ok {
		return channel.GetCritical(ctx)
	}
	defaultChannel, err := channels.DefaultChannel()
	if err != nil {
		return false, trace.Wrap(err, "creating new default channel")
	}
	return defaultChannel.GetCritical(ctx)
}
