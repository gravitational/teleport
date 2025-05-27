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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

func newFakeControlStream() fakeControlStream {
	return fakeControlStream{
		msgChan:  make(chan proto.UpstreamInventoryMessage),
		doneChan: make(chan struct{}),
	}
}

type fakeControlStream struct {
	client.UpstreamInventoryControlStream
	msgChan  chan proto.UpstreamInventoryMessage
	doneChan chan struct{}
}

func (f fakeControlStream) CloseWithError(err error) error {
	return nil
}

func (f fakeControlStream) Close() error {
	return nil
}

func (f fakeControlStream) Recv() <-chan proto.UpstreamInventoryMessage {
	return f.msgChan
}

func (f fakeControlStream) Done() <-chan struct{} {
	return f.doneChan
}

func (f fakeControlStream) fakeMsg(msg proto.UpstreamInventoryMessage) {
	f.msgChan <- msg
}

func (f fakeControlStream) close() {
	close(f.msgChan)
}

type fakeServer struct {
	version       string
	updateGroup   string
	delay         time.Duration
	roles         types.SystemRoles
	updaterStatus types.UpdaterStatus
	goodbye       *proto.UpstreamInventoryGoodbye
}

func TestServer_generateAgentVersionReport(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	twoMinutesAgo := now.Add(-time.Minute * 2)
	// agentRole are typicial roles an agent can have
	agentRoles := types.SystemRoles{types.RoleNode, types.RoleApp}
	updaterOK := types.UpdaterStatus_UPDATER_STATUS_OK

	tests := []struct {
		name     string
		fixtures []fakeServer
		expected *autoupdatev1pb.AutoUpdateAgentReportSpec
	}{
		{
			name: "no servers",
			expected: &autoupdatev1pb.AutoUpdateAgentReportSpec{
				Timestamp: timestamppb.New(now),
			},
		},
		{
			name: "no group, same version",
			fixtures: []fakeServer{
				{version: "1.2.3", roles: agentRoles, updateGroup: "default"},
				{version: "1.2.3", roles: agentRoles, updateGroup: "default"},
				{version: "1.2.3", roles: agentRoles, updateGroup: "default"},
				{version: "1.2.3", roles: agentRoles, updateGroup: "default"},
			},
			expected: &autoupdatev1pb.AutoUpdateAgentReportSpec{
				Timestamp: timestamppb.New(now),
				Groups: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroup{
					"default": {
						Versions: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion{
							"1.2.3": {Count: 4},
						},
					},
				},
			},
		},
		{
			name: "control plane servers are ignored",
			fixtures: []fakeServer{
				{version: "1.2.3", roles: types.SystemRoles{types.RoleKube, types.RoleAuth}, updateGroup: "default"},
				{version: "1.2.3", roles: types.SystemRoles{types.RoleApp, types.RoleProxy}, updateGroup: "default"},
				{version: "1.2.3", roles: types.SystemRoles{types.RoleApp, types.RoleKube}, updateGroup: "default"},
			},
			expected: &autoupdatev1pb.AutoUpdateAgentReportSpec{
				Timestamp: timestamppb.New(now),
				Groups: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroup{
					"default": {
						Versions: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion{
							"1.2.3": {Count: 1},
						},
					},
				},
			},
		},
		{
			name: "disabled or pinned updaters are ignored",
			fixtures: []fakeServer{
				{version: "1.2.3", roles: agentRoles, updaterStatus: updaterOK, updateGroup: "default"},
				{version: "1.2.3", roles: agentRoles, updaterStatus: types.UpdaterStatus_UPDATER_STATUS_PINNED, updateGroup: "default"},
				{version: "1.2.3", roles: agentRoles, updaterStatus: types.UpdaterStatus_UPDATER_STATUS_DISABLED, updateGroup: "default"},
			},
			expected: &autoupdatev1pb.AutoUpdateAgentReportSpec{
				Timestamp: timestamppb.New(now),
				Groups: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroup{
					"default": {
						Versions: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion{
							"1.2.3": {Count: 1},
						},
					},
				},
				Omitted: []*autoupdatev1pb.AutoUpdateAgentReportSpecOmitted{
					{Reason: omissionReasonUpdaterPinned, Count: 1},
					{Reason: omissionReasonUpdaterDisabled, Count: 1},
				},
			},
		},
		{
			name: "reloaded and terminating instances are ignored",
			fixtures: []fakeServer{
				{version: "1.2.3", roles: agentRoles, updaterStatus: updaterOK, updateGroup: "default"},
				{version: "1.2.3", roles: agentRoles, updaterStatus: updaterOK, updateGroup: "default", goodbye: &proto.UpstreamInventoryGoodbye{SoftReload: true}},
				{version: "1.2.3", roles: agentRoles, updaterStatus: updaterOK, updateGroup: "default", goodbye: &proto.UpstreamInventoryGoodbye{DeleteResources: true}},
			},
			expected: &autoupdatev1pb.AutoUpdateAgentReportSpec{
				Timestamp: timestamppb.New(now),
				Groups: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroup{
					"default": {
						Versions: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion{
							"1.2.3": {Count: 1},
						},
					},
				},
			},
		},
		{
			name: "too recent servers are ignored",
			fixtures: []fakeServer{
				{version: "1.2.3", roles: agentRoles, updaterStatus: updaterOK, updateGroup: "default"},
				{version: "1.2.3", delay: 90 * time.Second, roles: agentRoles, updaterStatus: updaterOK, updateGroup: "default"},
				{version: "1.2.3", roles: agentRoles, updaterStatus: updaterOK, updateGroup: "default"},
				{version: "1.2.3", roles: agentRoles, updaterStatus: updaterOK, updateGroup: "default"},
			},
			expected: &autoupdatev1pb.AutoUpdateAgentReportSpec{
				Timestamp: timestamppb.New(now),
				Groups: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroup{
					"default": {
						Versions: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion{
							"1.2.3": {Count: 1},
						},
					},
				},
			},
		},
		{
			name: "multiple versions and groups",
			fixtures: []fakeServer{
				{version: "1.2.3", updateGroup: "default", roles: agentRoles, updaterStatus: updaterOK},
				{version: "1.2.4", updateGroup: "default", roles: agentRoles, updaterStatus: updaterOK},
				{version: "1.2.5", updateGroup: "prod", roles: agentRoles, updaterStatus: updaterOK},
				{version: "1.2.5", updateGroup: "prod", roles: agentRoles, updaterStatus: updaterOK},
				{version: "1.2.5", updateGroup: "dev", roles: agentRoles, updaterStatus: updaterOK},
			},
			expected: &autoupdatev1pb.AutoUpdateAgentReportSpec{
				Timestamp: timestamppb.New(now),
				Groups: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroup{
					"default": {
						Versions: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion{
							"1.2.3": {Count: 1},
							"1.2.4": {Count: 1},
						},
					},
					"dev": {
						Versions: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion{
							"1.2.5": {Count: 1},
						},
					},
					"prod": {
						Versions: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion{
							"1.2.5": {Count: 2},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := clockwork.NewFakeClockAt(twoMinutesAgo)
			auth := &Server{
				logger:   utils.NewSlogLoggerForTests(),
				ServerID: uuid.NewString(),
			}
			controller := inventory.NewController(auth, nil, inventory.WithClock(clock))
			for _, fixture := range tt.fixtures {
				clock.Advance(fixture.delay)
				stream := newFakeControlStream()
				status := fixture.updaterStatus
				if status == types.UpdaterStatus_UPDATER_STATUS_UNSPECIFIED {
					status = types.UpdaterStatus_UPDATER_STATUS_OK
				}
				controller.RegisterControlStream(stream, &proto.UpstreamInventoryHello{
					Services:         fixture.roles.StringSlice(),
					ServerID:         uuid.New().String(),
					Version:          fixture.version,
					ExternalUpgrader: types.UpgraderKindTeleportUpdate,
					UpdaterInfo:      &types.UpdaterV2Info{UpdaterStatus: status, UpdateGroup: fixture.updateGroup},
				})
				if fixture.goodbye != nil {
					stream.fakeMsg(fixture.goodbye)
				}
				t.Cleanup(stream.close)
			}
			auth.inventory = controller
			auth.clock = clockwork.NewFakeClockAt(now)

			report, err := auth.generateAgentVersionReport(ctx)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(
				tt.expected, report.GetSpec(),
				protocmp.Transform(),
				protocmp.SortRepeatedFields(&autoupdatev1pb.AutoUpdateAgentReportSpec{}, "omitted")))
		})
	}
}

func TestServer_reportAgentVersions(t *testing.T) {
	// Test setup: create auth.
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	svc, err := local.NewAutoUpdateService(bk)
	require.NoError(t, err)

	now := time.Now()
	twoMinutesAgo := now.Add(-time.Minute * 2)
	clock := clockwork.NewFakeClockAt(twoMinutesAgo)

	// Test setup: load fixtures.
	const testNodeCount = 10
	auth := &Server{
		clock:    clock,
		ServerID: uuid.NewString(),
		Services: &Services{AutoUpdateService: svc},
		logger:   utils.NewSlogLoggerForTests(),
	}
	auth.Cache = auth.Services
	controller := inventory.NewController(auth, nil, inventory.WithClock(clock))
	auth.inventory = controller

	for range testNodeCount {
		stream := newFakeControlStream()
		controller.RegisterControlStream(stream, &proto.UpstreamInventoryHello{
			Services:         types.SystemRoles{types.RoleNode}.StringSlice(),
			Version:          "1.2.3",
			ServerID:         uuid.NewString(),
			ExternalUpgrader: types.UpgraderKindTeleportUpdate,
			UpdaterInfo: &types.UpdaterV2Info{
				UpdaterStatus: types.UpdaterStatus_UPDATER_STATUS_OK,
				UpdateGroup:   "default",
			},
		})
		t.Cleanup(stream.close)
	}
	ctx := context.Background()
	rollout, err := autoupdate.NewAutoUpdateAgentRollout(&autoupdatev1pb.AutoUpdateAgentRolloutSpec{
		StartVersion:              "1.2.3",
		TargetVersion:             "1.2.4",
		Schedule:                  autoupdate.AgentsScheduleRegular,
		AutoupdateMode:            autoupdate.AgentsUpdateModeEnabled,
		Strategy:                  autoupdate.AgentsStrategyHaltOnError,
		MaintenanceWindowDuration: nil,
	})
	require.NoError(t, err)
	_, err = svc.CreateAutoUpdateAgentRollout(ctx, rollout)
	require.NoError(t, err)

	// Test execution: compute and write report.
	clock.Advance(2 * time.Minute)
	auth.reportAgentVersions(ctx)

	// Test validation
	report, err := svc.GetAutoUpdateAgentReport(ctx, auth.ServerID)
	require.NoError(t, err)

	require.NotNil(t, report)
	require.NotEmpty(t, report.GetSpec().GetGroups())
	require.NotNil(t, report.GetSpec().GetGroups()["default"])
	require.NotNil(t, report.GetSpec().GetGroups()["default"].GetVersions())
	require.NotNil(t, report.GetSpec().GetGroups()["default"].GetVersions()["1.2.3"])
	require.Equal(t, testNodeCount, int(report.GetSpec().GetGroups()["default"].GetVersions()["1.2.3"].GetCount()))
}
