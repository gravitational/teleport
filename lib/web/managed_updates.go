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

package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport"
	autoupdatepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/api/utils/clientutils"
	aur "github.com/gravitational/teleport/lib/autoupdate/report"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/web/ui"
)

// getManagedUpdatesDetails returns managed updates details.
func (h *Handler) getManagedUpdatesDetails(w http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext) (any, error) {
	ctx := r.Context()
	clt, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &ui.ManagedUpdatesDetails{}

	autoUpdateConfig, err := clt.GetAutoUpdateConfig(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		autoUpdateConfig = nil
	}

	autoUpdateVersion, err := clt.GetAutoUpdateVersion(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		autoUpdateVersion = nil
	}

	response.Tools = getToolsInfo(autoUpdateConfig, autoUpdateVersion)

	rollout, err := clt.GetAutoUpdateAgentRollout(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		rollout = nil
	}

	if rollout != nil {
		response.Rollout = getRolloutInfo(rollout)
	}

	reports, err := stream.Collect(clientutils.Resources(ctx, clt.ListAutoUpdateAgentReports))
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		reports = nil
	}

	// Filter and aggregate version counts from the reports
	validReports := aur.ValidReports(reports, time.Now())
	versionCountsByGroup := aur.AggregateVersionCounts(validReports)

	if rollout != nil {
		response.Groups = getGroupsInfo(rollout, versionCountsByGroup)
		response.OrphanedAgentVersionCounts = getOrphanedAgentCounts(rollout, versionCountsByGroup)
	}

	// Get cluster maintenance info if this is a cloud cluster
	if features := h.GetClusterFeatures(); features.GetCloud() {
		maintenanceConfig, err := clt.GetClusterMaintenanceConfig(ctx)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			maintenanceConfig = nil
		}
		if maintenanceConfig != nil {
			response.ClusterMaintenance = getClusterMaintenanceInfo(maintenanceConfig)
		}
	}

	return response, nil
}

// getToolsInfo builds the ToolsAutoUpdateInfo object.
func getToolsInfo(config *autoupdatepb.AutoUpdateConfig, version *autoupdatepb.AutoUpdateVersion) *ui.ToolsAutoUpdateInfo {
	var mode, targetVersion string

	if config != nil {
		mode = config.GetSpec().GetTools().GetMode()
	}
	if version != nil {
		targetVersion = version.GetSpec().GetTools().GetTargetVersion()
	}

	// If empty, return nil
	if mode == "" && targetVersion == "" {
		return nil
	}

	return &ui.ToolsAutoUpdateInfo{
		Mode:          mode,
		TargetVersion: targetVersion,
	}
}

// getRolloutInfo builds the RolloutInfo object.
func getRolloutInfo(rollout *autoupdatepb.AutoUpdateAgentRollout) *ui.RolloutInfo {
	if rollout == nil || rollout.GetSpec() == nil {
		return nil
	}

	spec := rollout.GetSpec()
	status := rollout.GetStatus()

	info := &ui.RolloutInfo{
		StartVersion:  spec.GetStartVersion(),
		TargetVersion: spec.GetTargetVersion(),
		Strategy:      spec.GetStrategy(),
		Schedule:      spec.GetSchedule(),
		State:         strings.ToLower(aur.UserFriendlyState(status.GetState())),
		Mode:          spec.GetAutoupdateMode(),
	}

	// Set the rollout start time
	if status != nil {
		if startTime := status.GetStartTime(); startTime != nil && startTime.IsValid() {
			t := startTime.AsTime()
			if !t.IsZero() && t.Unix() != 0 {
				info.StartTime = &t
			}
		}
	}

	return info
}

// getGroupsInfo builds the list of RolloutGroupInfo objects.
func getGroupsInfo(rollout *autoupdatepb.AutoUpdateAgentRollout, versionCountsByGroup map[string]map[string]int) []ui.RolloutGroupInfo {
	if rollout == nil {
		return nil
	}

	groups := rollout.GetStatus().GetGroups()
	if len(groups) == 0 {
		return nil
	}

	out := make([]ui.RolloutGroupInfo, 0, len(groups))

	for i, group := range groups {
		groupInfo := ui.RolloutGroupInfo{
			Name:          group.GetName(),
			State:         strings.ToLower(aur.UserFriendlyState(group.GetState())),
			InitialCount:  group.GetInitialCount(),
			PresentCount:  group.GetPresentCount(),
			UpToDateCount: group.GetUpToDateCount(),
			StateReason:   group.GetLastUpdateReason(),
			CanaryCount:   group.GetCanaryCount(),
			IsCatchAll:    i == len(groups)-1,
		}

		// Only set the position if the strategy is halt-on-error
		if rollout.GetSpec().GetStrategy() == autoupdate.AgentsStrategyHaltOnError {
			groupInfo.Position = i + 1
		}

		// Set the group start time
		if startTime := group.GetStartTime(); startTime != nil && startTime.IsValid() {
			t := startTime.AsTime()
			if !t.IsZero() && t.Unix() != 0 {
				groupInfo.StartTime = &t
			}
		}

		// Set the last update time
		if lastUpdateTime := group.GetLastUpdateTime(); lastUpdateTime != nil && lastUpdateTime.IsValid() {
			t := lastUpdateTime.AsTime()
			if !t.IsZero() && t.Unix() != 0 {
				groupInfo.LastUpdateTime = &t
			}
		}

		// Add the version counts from aggregated reports
		if counts, ok := versionCountsByGroup[group.GetName()]; ok && len(counts) > 0 {
			groupInfo.AgentVersionCounts = counts
		}

		// Calculate the CanarySuccessCount
		if groupInfo.CanaryCount > 0 {
			var successCount uint64
			for _, canary := range group.GetCanaries() {
				if canary.GetSuccess() {
					successCount++
				}
			}
			groupInfo.CanarySuccessCount = successCount
		}

		out = append(out, groupInfo)
	}

	return out
}

// getClusterMaintenaceInfo builds the ClusterMaintenanceInfo object.
func getClusterMaintenanceInfo(cmc types.ClusterMaintenanceConfig) *ui.ClusterMaintenanceInfo {
	window, ok := cmc.GetAgentUpgradeWindow()
	if !ok {
		return nil
	}

	return &ui.ClusterMaintenanceInfo{
		ControlPlaneVersion:  teleport.Version,
		MaintenanceWeekdays:  window.Weekdays,
		MaintenanceStartHour: int(window.UTCStartHour),
	}
}

// getOrphanedAgentCounts returns version counts for agents reporting group names
// that don't match any defined rollout group.
func getOrphanedAgentCounts(rollout *autoupdatepb.AutoUpdateAgentRollout, versionCountsByGroup map[string]map[string]int) map[string]int {
	if rollout == nil || len(versionCountsByGroup) == 0 {
		return nil
	}

	// Get the defined rollout groups
	definedGroups := make(map[string]bool)
	for _, group := range rollout.GetStatus().GetGroups() {
		definedGroups[group.GetName()] = true
	}

	// Calculate how many agents don't belong to any of those groups, and their version.
	orphanedCounts := make(map[string]int)
	for groupName, versionCounts := range versionCountsByGroup {
		if !definedGroups[groupName] {
			for version, count := range versionCounts {
				orphanedCounts[version] += count
			}
		}
	}

	if len(orphanedCounts) == 0 {
		return nil
	}

	return orphanedCounts
}

func getAutoUpdateServiceClient(sctx *SessionContext) autoupdatepb.AutoUpdateServiceClient {
	return autoupdatepb.NewAutoUpdateServiceClient(sctx.GetClientConnection())
}

// startGroupUpdate starts an update for a specified rollout group.
func (h *Handler) startGroupUpdate(w http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext) (any, error) {
	ctx := r.Context()

	groupName := params.ByName("groupName")
	if groupName == "" {
		return nil, trace.BadParameter("group name is required")
	}

	var req ui.StartGroupUpdateRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	state := autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSPECIFIED
	// If the force flag is set to true, set the desired state to active to skip canary phase.
	if req.Force {
		state = autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE
	}

	client := getAutoUpdateServiceClient(sctx)
	rollout, err := client.TriggerAutoUpdateAgentGroup(ctx, &autoupdatepb.TriggerAutoUpdateAgentGroupRequest{
		Groups:       []string{groupName},
		DesiredState: state,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	group, err := findGroupInfo(rollout, groupName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.GroupActionResponse{Group: group}, nil
}

// markGroupDone marks a specified rollout group as done.
func (h *Handler) markGroupDone(w http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext) (any, error) {
	ctx := r.Context()

	groupName := params.ByName("groupName")
	if groupName == "" {
		return nil, trace.BadParameter("group name is required")
	}

	client := getAutoUpdateServiceClient(sctx)
	rollout, err := client.ForceAutoUpdateAgentGroup(ctx, &autoupdatepb.ForceAutoUpdateAgentGroupRequest{
		Groups: []string{groupName},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	group, err := findGroupInfo(rollout, groupName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.GroupActionResponse{Group: group}, nil
}

// rollbackGroup rolls back a specified rollout group.
func (h *Handler) rollbackGroup(w http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext) (any, error) {
	ctx := r.Context()

	groupName := params.ByName("groupName")
	if groupName == "" {
		return nil, trace.BadParameter("group name is required")
	}

	auClient := getAutoUpdateServiceClient(sctx)
	rollout, err := auClient.RollbackAutoUpdateAgentGroup(ctx, &autoupdatepb.RollbackAutoUpdateAgentGroupRequest{
		Groups:           []string{groupName},
		AllStartedGroups: false,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	group, err := findGroupInfo(rollout, groupName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.GroupActionResponse{Group: group}, nil
}

// findGroupInfo gets the RolloutGroupInfo for a specified group name.
func findGroupInfo(rollout *autoupdatepb.AutoUpdateAgentRollout, groupName string) (*ui.RolloutGroupInfo, error) {
	if rollout == nil || rollout.GetStatus() == nil {
		return nil, trace.NotFound("group %q not found in rollout", groupName)
	}

	groups := getGroupsInfo(rollout, nil)
	for i := range groups {
		if groups[i].Name == groupName {
			return &groups[i], nil
		}
	}
	return nil, trace.NotFound("group %q not found in rollout", groupName)
}
