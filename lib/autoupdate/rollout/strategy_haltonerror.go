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

package rollout

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	update "github.com/gravitational/teleport/api/types/autoupdate"
)

const (
	updateReasonCanStart              = "can_start"
	updateReasonCannotStart           = "cannot_start"
	updateReasonPreviousGroupsNotDone = "previous_groups_not_done"
	updateReasonUpdateComplete        = "update_complete"
	updateReasonUpdateInProgress      = "update_in_progress"
	haltOnErrorWindowDuration         = time.Hour
)

type haltOnErrorStrategy struct {
	log *slog.Logger
	clt Client
}

func (h *haltOnErrorStrategy) name() string {
	return update.AgentsStrategyHaltOnError
}

func newHaltOnErrorStrategy(log *slog.Logger, clt Client) (rolloutStrategy, error) {
	if log == nil {
		return nil, trace.BadParameter("missing log")
	}
	if clt == nil {
		return nil, trace.BadParameter("missing Client")
	}
	return &haltOnErrorStrategy{
		log: log.With("strategy", update.AgentsStrategyHaltOnError),
		clt: clt,
	}, nil
}

func (h *haltOnErrorStrategy) progressRollout(ctx context.Context, spec *autoupdate.AutoUpdateAgentRolloutSpec, status *autoupdate.AutoUpdateAgentRolloutStatus, now time.Time) error {
	reports, err := getAllValidReports(ctx, h.clt, now)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	countByGroup, upToDateByGroup := countUpToDate(reports, spec.GetTargetVersion())

	// We process every group in order, all the previous groups must be in the DONE state
	// for the next group to become active. Even if some early groups are not DONE,
	// later groups might be ACTIVE and need to transition to DONE, so we cannot
	// return early and must process every group.
	//
	// For example, in a dev/staging/prod setup, the "dev" group might get rolled
	// back while "staging" is still ACTIVE. We must not start PROD but still need
	// to transition "staging" to DONE.
	previousGroupsAreDone := true

	for i, group := range status.Groups {
		var agentCount, agentUpToDateCount int
		if i == len(status.Groups)-1 {
			agentCount, agentUpToDateCount = countCatchAll(status, countByGroup, upToDateByGroup)
		} else {
			agentCount = countByGroup[group.GetName()]
			agentUpToDateCount = upToDateByGroup[group.GetName()]
		}

		group.PresentCount = uint64(agentCount)
		group.UpToDateCount = uint64(agentUpToDateCount)

		switch group.State {
		case autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:
			var previousGroup *autoupdate.AutoUpdateAgentRolloutStatusGroup
			if i != 0 {
				previousGroup = status.Groups[i-1]
			}
			canStart, err := canStartHaltOnError(group, previousGroup, now)
			if err != nil {
				// In halt-on-error rollouts, groups are dependent.
				// Failing to transition a group should prevent other groups from transitioning.
				setGroupState(group, group.State, updateReasonReconcilerError, now)
				return err
			}

			// Check if the rollout got created after the theoretical group start time
			rolloutChangedDuringWindow, err := rolloutChangedInWindow(group, now, status.StartTime.AsTime(), haltOnErrorWindowDuration)
			if err != nil {
				setGroupState(group, group.State, updateReasonReconcilerError, now)
				return err
			}

			switch {
			case !previousGroupsAreDone:
				// All previous groups are not DONE
				setGroupState(group, group.State, updateReasonPreviousGroupsNotDone, now)
			case !canStart:
				// All previous groups are DONE, but time-related criteria are not met
				// This can be because we are outside an update window, or because the
				// specified wait_hours doesn't let us update yet.
				setGroupState(group, group.State, updateReasonCannotStart, now)
			case rolloutChangedDuringWindow:
				// All previous groups are DONE and time-related criteria are met.
				// However, the rollout changed during the maintenance window.
				setGroupState(group, group.State, updateReasonRolloutChanged, now)
			default:
				// All previous groups are DONE and time-related criteria are met.
				// We can start.
				setGroupState(group, autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE, updateReasonCanStart, now)
				group.InitialCount = uint64(agentCount)
			}
			previousGroupsAreDone = false
		case autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK:
			// The group has been manually rolled back. We don't touch anything and
			// don't process the next groups.
			previousGroupsAreDone = false
		case autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:
			// The group has already been updated, we can look at the next group
		case autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:
			// The group is currently being updated. We check if we can transition it to the done state
			done, reason := isDoneHaltOnError(group, now)

			if done {
				// We transition to the done state. We continue processing the groups as we might be able to start the next one.
				setGroupState(group, autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE, reason, now)
			} else {
				setGroupState(group, autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE, reason, now)
				previousGroupsAreDone = false
			}

		default:
			return trace.BadParameter("unknown autoupdate group state: %v", group.State)
		}
	}
	return nil
}

func canStartHaltOnError(group, previousGroup *autoupdate.AutoUpdateAgentRolloutStatusGroup, now time.Time) (bool, error) {
	// check wait hours
	if group.ConfigWaitHours != 0 {
		if previousGroup == nil {
			return false, trace.BadParameter("the first group cannot have non-zero wait hours")
		}

		previousStart := previousGroup.StartTime.AsTime()
		if previousStart.IsZero() || previousStart.Unix() == 0 {
			return false, trace.BadParameter("the previous group doesn't have a start time, cannot check the 'wait_hours' criterion")
		}

		// Check if the wait_hours criterion is OK, if we are at least after 'wait_hours' hours since the previous start.
		if now.Before(previousGroup.StartTime.AsTime().Add(time.Duration(group.ConfigWaitHours) * time.Hour)) {
			return false, nil
		}
	}

	return inWindow(group, now, haltOnErrorWindowDuration)
}

const (
	// Currently hardcoded maxInFlight, we might add a user-facing per-group
	// value in the future.
	maxInFlight   = 0.10
	doneThreshold = 1 - maxInFlight
)

func isDoneHaltOnError(group *autoupdate.AutoUpdateAgentRolloutStatusGroup, now time.Time) (bool, string) {
	switch {
	case group.InitialCount == 0:
		// Currently we don't implement status reporting from groups/agents.
		// So we just wait 60 minutes and consider the maintenance done.
		if group.StartTime.AsTime().Add(haltOnErrorWindowDuration).Before(now) {
			return true, updateReasonUpdateComplete
		}
		return false, updateReasonUpdateInProgress
	case float64(group.PresentCount)/float64(group.InitialCount) >= doneThreshold && float64(group.UpToDateCount)/float64(group.PresentCount) >= doneThreshold:
		return true, updateReasonUpdateComplete
	default:
		return false, updateReasonUpdateInProgress
	}
}

// countUpToDate iterates over all the reports and aggregates the counts by reported groups.
// The function returns two maps:
// - the number of agents belonging to each reported group
// - the number of up-to-date agents belonging to each reported group
func countUpToDate(
	reports []*autoupdate.AutoUpdateAgentReport,
	targetVersion string,
) (countByGroup, upToDateByGroup map[string]int) {
	countByGroup = make(map[string]int)
	upToDateByGroup = make(map[string]int)

	for _, report := range reports {
		for group, groupCount := range report.GetSpec().GetGroups() {
			for version, versionCount := range groupCount.GetVersions() {
				countByGroup[group] += int(versionCount.GetCount())
				if version == targetVersion {
					upToDateByGroup[group] += int(versionCount.GetCount())
				}
			}
		}
	}
	return countByGroup, upToDateByGroup
}

// CountCatchAll counts the number of agents belonging to the last group which is acting like a catch-all.
// The function returns two integers:
// - the number of agents belonging to the last group
// - the number of up-to-date agents belonging to the last group
func countCatchAll(rolloutStatus *autoupdate.AutoUpdateAgentRolloutStatus, countByGroup, upToDateByGroup map[string]int) (int, int) {
	if len(rolloutStatus.GetGroups()) == 0 {
		return 0, 0
	}

	rolloutGroups := make([]string, 0, len(rolloutStatus.GetGroups())-1)
	// We don't count the last group as it is the default one
	for _, group := range rolloutStatus.GetGroups()[:len(rolloutStatus.GetGroups())-1] {
		rolloutGroups = append(rolloutGroups, group.GetName())
	}

	var defaultGroupCount, upToDateDefaultGroupCount int

	for group, count := range countByGroup {
		if !slices.Contains(rolloutGroups, group) {
			defaultGroupCount += count
		}
	}

	for group, count := range upToDateByGroup {
		if !slices.Contains(rolloutGroups, group) {
			upToDateDefaultGroupCount += count
		}
	}

	return defaultGroupCount, upToDateDefaultGroupCount
}
