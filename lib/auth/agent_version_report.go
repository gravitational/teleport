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
	"math/rand/v2"
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

// filterHandler filters handles than can or cannot be used for automatic update
// purposes. It returns true if the handle can be used, false otherwise.
// If the handle cannot be used, the function might also return a non-empty string
// explaining why.
func filterHandler(handle inventory.UpstreamHandle, now time.Time) (bool, string) {
	// If the instance is being soft-reloaded or shut down, we ignore it.
	if goodbye := handle.Goodbye(); goodbye.GetSoftReload() || goodbye.GetDeleteResources() {
		return false, ""
	}

	// We skip servers that joined less than a minute ago as they might have been
	// connected to another auth instance a few seconds ago, which would lead to double-counting.
	if now.Sub(handle.RegistrationTime()) < constants.AutoUpdateAgentReportPeriod {
		return false, ""
	}
	// We skip control planes instances because we don't update them.
	if handle.HasControlPlaneService() {
		return false, ""
	}

	hello := handle.Hello()

	// If the machine has no updater, we skip it
	switch hello.ExternalUpgrader {
	case "":
		return false, omissionReasonNoUpdater
	case types.UpgraderKindSystemdUnit:
		return false, omissionReasonUpdaterV1
	}

	// Reject instance not advertising updater info
	updaterInfo := hello.GetUpdaterInfo()
	if updaterInfo == nil {
		return false, omissionReasonUpdaterTooOld
	}

	// Reject instances who are not advertising the group properly.
	// They might be running too old versions.
	updateGroup := updaterInfo.UpdateGroup
	if updateGroup == "" {
		return false, omissionReasonUpdaterTooOld
	}

	// We skip instances whose updater status is not OK.
	status := updaterInfo.UpdaterStatus
	switch status {
	case types.UpdaterStatus_UPDATER_STATUS_OK:
	case types.UpdaterStatus_UPDATER_STATUS_DISABLED:
		return false, omissionReasonUpdaterDisabled
	case types.UpdaterStatus_UPDATER_STATUS_PINNED:
		return false, omissionReasonUpdaterPinned
	case types.UpdaterStatus_UPDATER_STATUS_UNREADABLE:
		return false, omissionReasonUpdaterUnreadable
	default:
		return false, omissionReasonUpdaterUnknown
	}
	return true, ""
}

func (ir instanceReport) collectInstance(handle inventory.UpstreamHandle) {
	ok, reason := filterHandler(handle, ir.timestamp)
	if !ok {
		if reason != "" {
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

// SampleAgentsFromGroup iterates over every handle in the inventory to
// build a random sample of agents belonging to a given group.
// The main use-case for this function is to pick canaries that can be updated.
func (a *Server) SampleAgentsFromGroup(ctx context.Context, groupName string, sampleSize int) []*autoupdatev1pb.Canary {

	filter := func(handle inventory.UpstreamHandle) bool {
		ok, _ := filterHandler(handle, a.clock.Now())
		if !ok {
			return false
		}

		// No need to check for UpdaterInfo being nil, it would have been filtered
		// out by filterHandler().
		return handle.Hello().UpdaterInfo.UpdateGroup == groupName
	}
	sampler := newHandlerSampler(sampleSize, filter)

	a.inventory.UniqueHandles(sampler.visit)

	sampled := sampler.Sampled()
	canaries := make([]*autoupdatev1pb.Canary, len(sampled))
	for i, h := range sampled {
		hello := h.Hello()
		canaries[i] = &autoupdatev1pb.Canary{
			UpdaterId: string(hello.UpdaterInfo.UpdateUUID),
			HostId:    hello.ServerID,
			Hostname:  hello.Hostname,
			Success:   false,
		}
	}
	return canaries
}

// handleSampler randomly samples handles from the inventory.
// It implements Alan Waterman's Reservoir Sampling Algorithm R
// (The Art of Computer Programming Volume 2).
// See https://en.wikipedia.org/wiki/Reservoir_sampling for more details.
type handleSampler struct {
	sampleSize int
	seenCount  int
	filter     func(handle inventory.UpstreamHandle) bool
	// TODO for reviewers:
	// Do we feel confident about holding to the Handle even after we're done visiting?
	// I think so but @espadolini had doubts.
	// Alternatives are:
	// - using generics and taking a func(inventory.UpstreamHandle) (K, bool)
	// - making the sampler part of the inventory package
	sample []inventory.UpstreamHandle
}

func newHandlerSampler(sampleSize int, filter func(handle inventory.UpstreamHandle) bool) *handleSampler {
	return &handleSampler{
		sampleSize: sampleSize,
		seenCount:  sampleSize,
		filter:     filter,
		sample:     make([]inventory.UpstreamHandle, 0, sampleSize),
	}
}

func (h *handleSampler) visit(handle inventory.UpstreamHandle) {
	// filter out everything we don't want
	if !h.filter(handle) {
		return
	}

	// Fill the reservoir
	if len(h.sample) < h.sampleSize {
		h.sample = append(h.sample, handle)
		h.seenCount++
		return
	}

	// Reservoir is already filled, replace existing elements.
	if j := rand.N(h.seenCount); j < h.sampleSize {
		h.sample[j] = handle
	}
	h.seenCount++
}

func (h *handleSampler) Sampled() []inventory.UpstreamHandle {
	return h.sample
}
