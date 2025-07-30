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

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	update "github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
)

const (
	updateReasonCanStart              = "can_start"
	updateReasonCannotStart           = "cannot_start"
	updateReasonPreviousGroupsNotDone = "previous_groups_not_done"
	updateReasonUpdateComplete        = "update_complete"
	updateReasonUpdateInProgress      = "update_in_progress"
	updateReasonCanariesAlive         = "canaries_are_alive"
	updateReasonWaitingForCanaries    = "waiting_for_canaries"
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

				// We pass the list of groups to the sampler because it must compute the catch-call group
				groups := make([]string, len(status.Groups))
				for j, g := range status.Groups {
					groups[j] = g.GetName()
				}
				h.startGroup(ctx, group, now, agentCount, status)
			}
			previousGroupsAreDone = false
		case autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY:
			// Sample the canaries if they were not sampled yet.
			if err := h.sampleCanaries(ctx, group, status); err != nil {
				return trace.Wrap(err, "failed to sample canaries")
			}
			// Check if the canaries are back online and running the right version
			targetVersion, err := version.EnsureSemver(spec.GetTargetVersion())
			if err != nil {
				return trace.Wrap(err, "failed to parse target version, rollout is malformed")
			}
			successfulCanaries := h.updateCanariesStatus(ctx, group, *targetVersion)

			// If all canaries are OK, we can transition to the active state
			if successfulCanaries == int(group.CanaryCount) {
				h.log.DebugContext(ctx, "All canaries came back alive, transitioning to the active state", "group", group, "got", successfulCanaries, "want", int(group.CanaryCount))
				setGroupState(group, autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE, updateReasonCanariesAlive, now)
			} else {
				h.log.DebugContext(ctx, "Not all canaries came back yet, staying into canary state", "group", group, "got", successfulCanaries, "want", int(group.CanaryCount))
				setGroupState(group, group.State, updateReasonWaitingForCanaries, now)

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

func shouldUseCanaries(group *autoupdate.AutoUpdateAgentRolloutStatusGroup) bool {
	// in the future we might change this logic to be a multiple of the required canary count
	// and make the canary count dynamic
	return group.CanaryCount > 0
}

func (h *haltOnErrorStrategy) startGroup(ctx context.Context, group *autoupdate.AutoUpdateAgentRolloutStatusGroup, now time.Time, agentCount int, status *autoupdate.AutoUpdateAgentRolloutStatus) {
	group.InitialCount = uint64(agentCount)

	if !shouldUseCanaries(group) {
		h.log.DebugContext(ctx, "Skipping canary rollout, transitioning directly to the active state", "group", group.Name)
		setGroupState(group, autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE, updateReasonCanStart, now)
		return
	}

	setGroupState(group, autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY, updateReasonCanStart, now)
	// This is a small optimization, as we just transitioned into the canary state we can sample canaries.
	// This will allow us to start updating without having to wait for the next reconciliation cycle.
	if err := h.sampleCanaries(ctx, group, status); err != nil {
		h.log.WarnContext(ctx, "Failed to sample canaries", "group", group.Name)
	}
}

func (h *haltOnErrorStrategy) sampleCanaries(ctx context.Context, group *autoupdate.AutoUpdateAgentRolloutStatusGroup, status *autoupdate.AutoUpdateAgentRolloutStatus) error {
	// Check if we need to pick more canaries
	if len(group.Canaries) < int(group.CanaryCount) {
		previousLength := len(group.Canaries)
		h.log.DebugContext(ctx, "Group is missing canaries, sampling some more", "group", group, "got", previousLength, "want", int(group.CanaryCount))

		// We pass the list of groups to the sampler because it must compute the catch-all group.
		groups := make([]string, len(status.Groups))
		for j, g := range status.Groups {
			groups[j] = g.GetName()
		}
		// We sample as many canaries as possible instead of just the missing ones
		// Because we might sample an already sampled canary.
		additionalCanaries, err := h.clt.SampleAgentsFromAutoUpdateGroup(ctx, group.Name, int(group.CanaryCount), groups)
		if err != nil {
			return trace.Wrap(err)
		}
		injectCanaries(group, additionalCanaries)
		h.log.DebugContext(ctx, "Additional canaries sampled", "group", group, "before", previousLength, "after", len(group.Canaries))
	} else {
		h.log.DebugContext(ctx, "Canaries already sampled", "group", group.Name, "got", len(group.Canaries))
	}
	return nil
}

func injectCanaries(group *autoupdate.AutoUpdateAgentRolloutStatusGroup, additionalCanaries []*autoupdate.Canary) {
	for _, canary := range additionalCanaries {
		// We first check if the canary has already been sampled
		alreadySampled := false
		for _, existingCanary := range group.Canaries {
			if existingCanary.UpdaterId == canary.UpdaterId {
				alreadySampled = true
				break
			}
		}

		// If it was not, great, we have a new canary.
		if !alreadySampled {
			group.Canaries = append(group.Canaries, canary)
		}

		// Stop adding canaries once we have the right amount
		if len(group.Canaries) == int(group.CanaryCount) {
			return
		}
	}
}

func (h *haltOnErrorStrategy) updateCanariesStatus(ctx context.Context, group *autoupdate.AutoUpdateAgentRolloutStatusGroup, targetVersion semver.Version) int {
	h.log.DebugContext(ctx, "Checking canaries", "group", group.Name)
	var successfulCanaries int
	for _, canary := range group.Canaries {
		// If the canary already came back healthy, nothing to do
		if canary.Success {
			successfulCanaries++
			continue
		}

		canaryLogInfo := slog.Group("canary", "host_id", canary.HostId, "updater_id", canary.UpdaterId, "hostname", canary.Hostname)
		log := h.log.With(canaryLogInfo).With("group", group.Name)

		// Check if the canary is connected to our auth
		hellos, err := h.clt.LookupAgentInInventory(ctx, canary.HostId)
		if err != nil {
			if trace.IsNotFound(err) {
				// Canary is not registered to our Auth Service.
				// Note: One old canary instance might still be connected to the auth,
				// be we are ignoring terminating instances.
				log.DebugContext(ctx, "Node not connected")
			} else {
				h.log.WarnContext(ctx, "Failed to lookup agent")
			}
			continue
		}

		if canaryIsRunningTargetVersion(ctx, hellos, targetVersion, log) {
			canary.Success = true
			successfulCanaries++
		}
	}
	return successfulCanaries
}

// canaryIsRunningTargetVersion returns true if at least one of the Hellos indicates
// the canary is running the target version.
func canaryIsRunningTargetVersion(ctx context.Context, hellos []*proto.UpstreamInventoryHello, targetVersion semver.Version, log *slog.Logger) bool {
	for _, hello := range hellos {
		canaryVersion, err := version.EnsureSemver(hello.Version)
		if err != nil {
			log.WarnContext(ctx, "Failed to parse canary version", "err", err, "current_version", hello.Version)
			continue
		}
		if !targetVersion.Equal(*canaryVersion) {
			log.DebugContext(ctx, "Canary is not running the target version", "current_version", canaryVersion, "expected_version", targetVersion)
			continue
		}
		log.DebugContext(ctx, "Canary is running the target version, marking it healthy", "current_version", canaryVersion, "expected_version", targetVersion)
		return true
	}
	return false
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
