package auth

import (
	"context"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/inventory"
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

func (a *Server) generateAgentVersionReport(ctx context.Context) instanceReport {
	report := instanceReport{timestamp: a.clock.Now(), data: make(map[string]instanceGroupReport)}
	a.inventory.Iter(report.collectInstance)
	return report
}
