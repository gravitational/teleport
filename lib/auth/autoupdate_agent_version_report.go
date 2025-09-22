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

package auth

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/constants"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/inventory"
)

const (
	omissionReasonUpdaterDisabled   = "updater is disabled"
	omissionReasonUpdaterPinned     = "updater is pinning a specific version"
	omissionReasonUpdaterUnreadable = "agent cannot read updater status"
	omissionReasonUpdaterUnknown    = "unknown updater status"
	omissionReasonUpdaterV1         = "managed updates v1 updater does not support agent reports"
	omissionReasonUpdaterTooOld     = "updater version does not support agent reports"
	omissionReasonNoUpdater         = "agent has no updater"
)

type instanceReport struct {
	data      map[string]instanceGroupReport
	timestamp time.Time
	omissions map[string]int
}

type autoUpdateFilterResult int

const (
	// The zero value should not be used, but if it is, it should not match anything.
	autoUpdateFilterResultUnknown autoUpdateFilterResult = iota
	autoUpdateFilterResultMatching
	autoUpdateFilterResultTerminating
	autoUpdateFilterResultRecentlyConnected
	autoUpdaterFilterResultIsControlPlane
	autoUpdateFilterResultNoUpdater
	autoUpdateFilterResultUpdaterTooOld
	autoUpdateFilterResultUpdaterDisabled
	autoUpdateFilterResultUpdaterPinned
	autoUpdateFilterResultUpdaterUnreadable
	autoUpdateFilterResultUpdaterUnknown
	autoUpdateFilterResultUpdaterV1
)

// String returns a user-facing message explaining the filter decision.
// If the filter result reason is not actionable, it returns an empty string.
func (r autoUpdateFilterResult) String() string {
	switch r {
	case autoUpdateFilterResultNoUpdater:
		return omissionReasonNoUpdater
	case autoUpdateFilterResultUpdaterTooOld:
		return omissionReasonUpdaterTooOld
	case autoUpdateFilterResultUpdaterDisabled:
		return omissionReasonUpdaterDisabled
	case autoUpdateFilterResultUpdaterPinned:
		return omissionReasonUpdaterPinned
	case autoUpdateFilterResultUpdaterUnreadable:
		return omissionReasonUpdaterUnreadable
	case autoUpdateFilterResultUpdaterUnknown:
		return omissionReasonUpdaterUnknown
	case autoUpdateFilterResultUpdaterV1:
		return omissionReasonUpdaterV1
	default:
		return ""
	}
}

// filterHandler filters handles than can or cannot be used for automatic update
// purposes. It returns true if the handle can be used, false otherwise.
// If the handle cannot be used, the function might also return a non-empty string
// explaining why.
func filterHandler(handle inventory.UpstreamHandle, now time.Time) autoUpdateFilterResult {
	// If the instance is being soft-reloaded or shut down, we ignore it.
	if goodbye := handle.Goodbye(); goodbye.GetSoftReload() || goodbye.GetDeleteResources() {
		return autoUpdateFilterResultTerminating
	}

	// We skip servers that joined less than a minute ago as they might have been
	// connected to another auth instance a few seconds ago, which would lead to double-counting.
	if now.Sub(handle.RegistrationTime()) < constants.AutoUpdateAgentReportPeriod {
		return autoUpdateFilterResultRecentlyConnected
	}
	// We skip control planes instances because we don't update them.
	if handle.HasControlPlaneService() {
		return autoUpdaterFilterResultIsControlPlane
	}

	hello := handle.Hello()

	// If the machine has no updater, we skip it
	switch hello.ExternalUpgrader {
	case "":
		return autoUpdateFilterResultNoUpdater
	case types.UpgraderKindSystemdUnit:
		return autoUpdateFilterResultUpdaterV1
	}

	// Reject instance not advertising updater info
	updaterInfo := hello.GetUpdaterInfo()
	if updaterInfo == nil {
		return autoUpdateFilterResultUpdaterTooOld
	}

	// Reject instances who are not advertising the group properly.
	// They might be running too old versions.
	updateGroup := updaterInfo.UpdateGroup
	if updateGroup == "" {
		return autoUpdateFilterResultUpdaterTooOld
	}

	// We skip instances whose updater status is not OK.
	status := updaterInfo.UpdaterStatus
	switch status {
	case types.UpdaterStatus_UPDATER_STATUS_OK:
	case types.UpdaterStatus_UPDATER_STATUS_DISABLED:
		return autoUpdateFilterResultUpdaterDisabled
	case types.UpdaterStatus_UPDATER_STATUS_PINNED:
		return autoUpdateFilterResultUpdaterPinned
	case types.UpdaterStatus_UPDATER_STATUS_UNREADABLE:
		return autoUpdateFilterResultUpdaterUnreadable
	default:
		return autoUpdateFilterResultUnknown
	}
	return autoUpdateFilterResultMatching
}

// collectInstance is meant to be a callback that is passed to inventory.AllHandles().
func (ir instanceReport) collectInstance(handle inventory.UpstreamHandle) {
	result := filterHandler(handle, ir.timestamp)
	if result != autoUpdateFilterResultMatching {
		if reason := result.String(); reason != "" {
			ir.omissions[reason] += 1
		}
		return
	}

	// No need to check for UpdaterInfo being nil, it would have been filtered
	// out by filterHandler().
	updateGroup := handle.Hello().UpdaterInfo.UpdateGroup

	if _, ok := ir.data[updateGroup]; !ok {
		ir.data[updateGroup] = instanceGroupReport{}
	}

	ir.data[updateGroup].collectInstance(handle)
}

type instanceGroupReport map[string]instanceGroupVersionReport

func (ir instanceGroupReport) collectInstance(handle inventory.UpstreamHandle) {
	hello := handle.Hello()

	stats, ok := ir[hello.Version]
	if !ok {
		stats = instanceGroupVersionReport{}
	}

	stats.count += 1

	ir[hello.Version] = stats
}

type instanceGroupVersionReport struct {
	count int
	// Leaving room here to add the lowest UUID, as described in RFD 184.
}

func (a *Server) generateAgentVersionReport(ctx context.Context) (*autoupdatev1pb.AutoUpdateAgentReport, error) {
	now := a.clock.Now()

	a.logger.DebugContext(ctx, "Collecting agent versions from inventory")
	rawreport := instanceReport{
		timestamp: now,
		data:      make(map[string]instanceGroupReport),
		omissions: make(map[string]int),
	}
	a.inventory.AllHandles(rawreport.collectInstance)

	a.logger.DebugContext(ctx, "Building the agent version report")
	spec := &autoupdatev1pb.AutoUpdateAgentReportSpec{
		Timestamp: timestamppb.New(a.clock.Now()),
		Groups:    make(map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroup, len(rawreport.data)),
		Omitted:   make([]*autoupdatev1pb.AutoUpdateAgentReportSpecOmitted, 0, len(rawreport.omissions)),
	}

	// TODO(hugoShaka): gracefully handle too many groups or versions (sort and report only the largest ones).
	// Currently the agent version report will just fail validation if there are too many groups.

	for groupName, groupData := range rawreport.data {
		versions := make(map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion, len(groupData))
		for versionName, groupVersionData := range groupData {
			versions[versionName] = &autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion{
				Count: int32(groupVersionData.count),
			}
		}
		spec.Groups[groupName] = &autoupdatev1pb.AutoUpdateAgentReportSpecGroup{
			Versions: versions,
		}
	}

	for reason, count := range rawreport.omissions {
		spec.Omitted = append(spec.Omitted, &autoupdatev1pb.AutoUpdateAgentReportSpecOmitted{
			Reason: reason,
			Count:  int64(count),
		})
	}

	report, err := autoupdate.NewAutoUpdateAgentReport(spec, a.ServerID)
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate agent version report")
	}

	return report, nil
}

func (a *Server) reportAgentVersions(ctx context.Context) {
	if _, err := a.GetAutoUpdateAgentRollout(ctx); err != nil {
		if trace.IsNotFound(err) {
			a.logger.DebugContext(ctx, "Skipping periodic agent report because the cluster doesn't contain an autoupdate_agent_rollout.")
			return
		}
		a.logger.WarnContext(ctx, "Failed to check if autoupdate_agent_rollout resource exists, aborting periodic agent report", "error", err)
	}

	a.logger.DebugContext(ctx, "Periodic agent version report routine started")
	report, err := a.generateAgentVersionReport(ctx)
	if err != nil {
		a.logger.WarnContext(ctx, "Failed to report agent versions", "error", err)
		return
	}

	a.logger.DebugContext(ctx, "Writing agent version report to the backend", "name", report.GetMetadata().GetName())
	_, err = a.UpsertAutoUpdateAgentReport(ctx, report)
	if err != nil {
		a.logger.ErrorContext(ctx, "Failed to write agent version report", "error", err)
	}
	a.logger.DebugContext(ctx, "Finished exporting the agent version report")
}
