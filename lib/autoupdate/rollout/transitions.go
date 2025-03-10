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

package rollout

import (
	"time"

	"github.com/gravitational/trace"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types/autoupdate"
)

const (
	updateReasonManualTrigger  = "manual_trigger"
	updateReasonForcedDone     = "manual_forced_done"
	updateReasonManualRollback = "manual_rollback"
)

// TriggerGroups mutates a rollout to trigger updates for the given groups.
// The function does not apply the rollout back, it is the caller's responsibility
// to commit it in the backend.
// The function takes a desired State parameter to leave room for future canary
// state support as specified in RFD 184.
func TriggerGroups(rollout *autoupdatev1pb.AutoUpdateAgentRollout, groupNames []string, desiredState autoupdatev1pb.AutoUpdateAgentGroupState, now time.Time) error {
	// Validation part, we look for everything not in order or unsupported.
	if rollout == nil {
		return trace.BadParameter("rollout cannot be nil")
	}

	if err := checkSchedule(rollout); err != nil {
		return trace.Wrap(err)
	}

	if err := strategySupportsManualTransition(rollout); err != nil {
		return trace.Wrap(err)
	}

	switch desiredState {
	case autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSPECIFIED:
		// When unspecified, we default to active
		desiredState = autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE
	case autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:
	default:
		return trace.BadParameter("unsupported desired state: %s, supported states are 'unspecified' and 'active'", desiredState)
	}

	groups := rollout.GetStatus().GetGroups()
	if len(groups) == 0 {
		return trace.BadParameter("rollout has no groups")
	}

	// Part where we do the real work, doing a state transition for every requested group.
	for _, groupName := range groupNames {
		group, err := getGroup(groups, groupName)
		if err != nil {
			return trace.Wrap(err)
		}

		// We check if the group state transition is legal.
		switch group.GetState() {
		case autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSPECIFIED,
			autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
			autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK:
		case autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:
			return trace.AlreadyExists("group %q is already active", groupName)
		case autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:
			return trace.AlreadyExists("group %q is already done", groupName)
		default:
			return trace.BadParameter("group %q in unexpected state %s", groupName, group.GetState())
		}

		setGroupState(group, desiredState, updateReasonManualTrigger, now)
	}

	return nil
}

// ForceGroupsDone mutates a rollout to forcefully transition groups to the done state.
// The function does not apply the rollout back, it is the caller's responsibility
// to commit it in the backend.
func ForceGroupsDone(rollout *autoupdatev1pb.AutoUpdateAgentRollout, groupNames []string, now time.Time) error {
	// Validation part, we look for everything not in order or unsupported.
	if rollout == nil {
		return trace.BadParameter("rollout cannot be nil")
	}

	if err := checkSchedule(rollout); err != nil {
		return trace.Wrap(err)
	}

	if err := strategySupportsManualTransition(rollout); err != nil {
		return trace.Wrap(err)
	}

	groups := rollout.GetStatus().GetGroups()
	if len(groups) == 0 {
		return trace.BadParameter("rollout has no groups")
	}

	// Part where we do the real work, doing a state transition for every requested group.
	for _, groupName := range groupNames {
		group, err := getGroup(groups, groupName)
		if err != nil {
			return trace.Wrap(err)
		}

		// We check if the group state transition is legal.
		switch group.GetState() {
		case autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSPECIFIED,
			autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
			autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
			autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:
		case autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:
			return trace.AlreadyExists("group %q is already done", groupName)
		default:
			return trace.BadParameter("group %q in unexpected state %s", groupName, group.GetState())
		}

		setGroupState(group, autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE, updateReasonForcedDone, now)
	}

	return nil
}

// RollbackStartedGroups mutates a rollout to rollback every group that started updating.
// The function does not apply the rollout back, it is the caller's responsibility
// to commit it in the backend.
func RollbackStartedGroups(rollout *autoupdatev1pb.AutoUpdateAgentRollout, now time.Time) error {
	groups := rollout.GetStatus().GetGroups()
	startedGroups := make([]string, 0, len(groups))

	for _, group := range groups {
		switch group.GetState() {
		case autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:
			startedGroups = append(startedGroups, group.GetName())
		}
	}

	if len(startedGroups) == 0 {
		return trace.NotFound("no already started group found")
	}

	return RollbackGroups(rollout, startedGroups, now)
}

// RollbackGroups mutates a rollout to rollback groups (to the rolledback state).
// The function does not apply the rollout back, it is the caller's responsibility
// to commit it in the backend.
func RollbackGroups(rollout *autoupdatev1pb.AutoUpdateAgentRollout, groupNames []string, now time.Time) error {
	// Validation part, we look for everything not in order or unsupported.
	if rollout == nil {
		return trace.BadParameter("rollout cannot be nil")
	}

	if err := checkSchedule(rollout); err != nil {
		return trace.Wrap(err)
	}

	if err := strategySupportsManualTransition(rollout); err != nil {
		return trace.Wrap(err)
	}

	groups := rollout.GetStatus().GetGroups()
	if len(groups) == 0 {
		return trace.BadParameter("rollout has no groups")
	}

	// Part where we do the real work, doing a state transition for every requested group.
	for _, groupName := range groupNames {
		group, err := getGroup(groups, groupName)
		if err != nil {
			return trace.Wrap(err)
		}

		if group.GetState() == autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK {
			return trace.AlreadyExists("group %q is already in a rolled-back state", groupName)
		}

		setGroupState(group, autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK, updateReasonManualRollback, now)
	}

	return nil
}

// getGroup returns the group of a given name in a rollout.
func getGroup(groups []*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup, groupName string) (*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup, error) {
	if groupName == "" {
		return nil, trace.BadParameter("group name cannot be empty")
	}
	for _, group := range groups {
		if group.GetName() == groupName {
			return group, nil
		}
	}
	return nil, trace.NotFound("group %q not found", groupName)
}

// checkSchedule fails early if this is an immediate rollout or some unknown schedule.
func checkSchedule(rollout *autoupdatev1pb.AutoUpdateAgentRollout) error {
	switch rollout.GetSpec().GetSchedule() {
	case autoupdate.AgentsScheduleImmediate:
		return trace.BadParameter("rollout schedule is immediate, it has no groups and cannot be triggered")
	case autoupdate.AgentsScheduleRegular:
		return nil
	default:
		return trace.BadParameter("unknown rollout schedule: %v", rollout.GetSpec().GetSchedule())
	}
}

// strategySupportsManualTransition checks if the current rollout strategy can be manually transitioned.
// We don't currently support manual transitions in time-based rollouts.
// We might do so in the future by introducing a new manual active state.
func strategySupportsManualTransition(rollout *autoupdatev1pb.AutoUpdateAgentRollout) error {
	switch rollout.GetSpec().GetStrategy() {
	case autoupdate.AgentsStrategyHaltOnError:
		return nil
	default:
		return trace.NotImplemented("manual group state transition is not supported for rollout strategy %q", rollout.GetSpec().GetStrategy())
	}
}
