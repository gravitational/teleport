package auth

import (
	"context"
	"github.com/gravitational/teleport/api/client/proto"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/inventory"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

type instanceReport struct {
	data      map[string]instanceGroupReport
	timestamp time.Time
}

func (ir instanceReport) collectInstance(handle inventory.UpstreamHandle) {
	// We skip servers that joined less than a minute ago as they might have been
	// connected to another auth instance a few seconds ago, which would lead to double-counting.
	if handle.RegistrationTime().After(ir.timestamp.Add(-time.Minute)) {
		return
	}
	// We skip auth and proxy instances because we don't update them.
	if handle.HasService(types.RoleAuth) || handle.HasService(types.RoleProxy) {
		return
	}

	hello := handle.Hello()

	// We skip instances whose updater status is not unknown or OK.
	// Note: is it OK to allow unknown? Discuss this with Stephen.
	status := hello.GetUpdaterStatus()
	if status != proto.UpdaterStatus_UpdaterStatusOK && status != proto.UpdaterStatus_UpdaterStatusUnknown {
		return
	}

	if _, ok := ir.data[hello.UpdateGroup]; !ok {
		ir.data[hello.UpdateGroup] = instanceGroupReport{}
	}

	ir.data[hello.UpdateGroup].collectInstance(handle)
}

type instanceGroupReport map[string]instanceGroupVersionReport

func (ir instanceGroupReport) collectInstance(handle inventory.UpstreamHandle) {
	hello := handle.Hello()

	// Note: not validating if the semver is correct, is this an issue?
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

func (a *Server) generateAgentVersionReport(ctx context.Context) {
	now := a.clock.Now()
	a.logger.DebugContext(ctx, "Periodic agent version report routine started")

	a.logger.DebugContext(ctx, "Collecting agent versions from inventory")
	rawreport := instanceReport{timestamp: now, data: make(map[string]instanceGroupReport)}
	a.inventory.Iter(rawreport.collectInstance)

	a.logger.DebugContext(ctx, "Building the agent version report")
	spec := &autoupdatev1pb.AutoUpdateAgentReportSpec{
		Timestamp: timestamppb.New(a.clock.Now()),
		Groups:    make(map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroup, len(rawreport.data)),
	}

	// TODO: cap the group and version map size

	for groupName, groupData := range rawreport.data {
		versions := make(map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion, len(groupData))
		for versionName, groupVersionData := range groupData {
			versions[versionName] = &autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion{
				Count: uint32(groupVersionData.count),
			}
		}
		spec.Groups[groupName] = &autoupdatev1pb.AutoUpdateAgentReportSpecGroup{
			Versions: versions,
		}
	}

	report, err := autoupdate.NewAutoUpdateAgentReport(spec, a.ServerID)
	if err != nil {
		a.logger.ErrorContext(ctx, "Failed to generate agent version report: %v", err)
		return
	}

	a.logger.DebugContext(ctx, "Writing agent version report to the backend")
	_, err = a.UpsertAutoUpdateAgentReport(ctx, report)
	if err != nil {
		a.logger.ErrorContext(ctx, "Failed to write agent version report: %v", err)
	}
	a.logger.DebugContext(ctx, "Finished exporting the agent version report")
}
