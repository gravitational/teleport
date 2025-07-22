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
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func Test_canStartHaltOnError(t *testing.T) {
	now := testSunday
	yesterday := testSaturday

	tests := []struct {
		name          string
		group         *autoupdate.AutoUpdateAgentRolloutStatusGroup
		previousGroup *autoupdate.AutoUpdateAgentRolloutStatusGroup
		want          bool
		wantErr       require.ErrorAssertionFunc
	}{
		{
			name: "first group, no wait_hours",
			group: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:            "test-group",
				ConfigDays:      everyWeekday,
				ConfigStartHour: int32(now.Hour()),
				ConfigWaitHours: 0,
			},
			want:    true,
			wantErr: require.NoError,
		},
		{
			name: "first group, wait_days (invalid)",
			group: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:            "test-group",
				ConfigDays:      everyWeekday,
				ConfigStartHour: int32(now.Hour()),
				ConfigWaitHours: 1,
			},
			want:    false,
			wantErr: require.Error,
		},
		{
			name: "second group, no wait_days",
			group: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:            "test-group",
				ConfigDays:      everyWeekday,
				ConfigStartHour: int32(now.Hour()),
				ConfigWaitHours: 0,
			},
			previousGroup: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:            "previous-group",
				StartTime:       timestamppb.New(now),
				State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
				ConfigDays:      everyWeekday,
				ConfigStartHour: int32(now.Hour()),
				ConfigWaitHours: 0,
			},
			want:    true,
			wantErr: require.NoError,
		},
		{
			name: "second group, wait_days not over",
			group: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:            "test-group",
				ConfigDays:      everyWeekday,
				ConfigStartHour: int32(now.Hour()),
				ConfigWaitHours: 48,
			},
			previousGroup: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:            "previous-group",
				StartTime:       timestamppb.New(yesterday),
				State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
				ConfigDays:      everyWeekday,
				ConfigStartHour: int32(now.Hour()),
				ConfigWaitHours: 0,
			},
			want:    false,
			wantErr: require.NoError,
		},
		{
			name: "second group, wait_days over",
			group: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:            "test-group",
				ConfigDays:      everyWeekday,
				ConfigStartHour: int32(now.Hour()),
				ConfigWaitHours: 24,
			},
			previousGroup: &autoupdate.AutoUpdateAgentRolloutStatusGroup{
				Name:            "previous-group",
				StartTime:       timestamppb.New(yesterday),
				State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
				ConfigDays:      everyWeekday,
				ConfigStartHour: int32(now.Hour()),
				ConfigWaitHours: 0,
			},
			want:    true,
			wantErr: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := canStartHaltOnError(tt.group, tt.previousGroup, now)
			tt.wantErr(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_progressGroupsHaltOnError(t *testing.T) {
	clock := clockwork.NewFakeClockAt(testSunday)
	log := logtest.NewLogger()

	fewSecondsAgo := clock.Now().Add(-3 * time.Second)
	fewMinutesAgo := clock.Now().Add(-5 * time.Minute)
	yesterday := testSaturday
	canStartToday := everyWeekday
	cannotStartToday := everyWeekdayButSunday
	ctx := context.Background()

	startVersion := "1.2.3"
	targetVersion := "1.2.4"
	otherVersion := "1.2.5"

	group1Name := "group1"
	group2Name := "group2"
	group3Name := "group3"

	testReports := []*autoupdate.AutoUpdateAgentReport{
		{
			Metadata: &headerv1.Metadata{Name: "auth1"},
			Spec: &autoupdate.AutoUpdateAgentReportSpec{
				Timestamp: timestamppb.New(fewSecondsAgo),
				Groups: map[string]*autoupdate.AutoUpdateAgentReportSpecGroup{
					group1Name: {
						Versions: map[string]*autoupdate.AutoUpdateAgentReportSpecGroupVersion{
							startVersion:  {Count: 4},
							targetVersion: {Count: 5},
							otherVersion:  {Count: 1},
						},
					},
					group2Name: {
						Versions: map[string]*autoupdate.AutoUpdateAgentReportSpecGroupVersion{
							startVersion:  {Count: 3},
							targetVersion: {Count: 5},
						},
					},
				},
			},
		},
		{
			// This report is expired, it must be ignored
			Metadata: &headerv1.Metadata{Name: "auth2"},
			Spec: &autoupdate.AutoUpdateAgentReportSpec{
				Timestamp: timestamppb.New(fewMinutesAgo),
				Groups: map[string]*autoupdate.AutoUpdateAgentReportSpecGroup{
					group1Name: {
						Versions: map[string]*autoupdate.AutoUpdateAgentReportSpecGroupVersion{
							startVersion:  {Count: 123},
							targetVersion: {Count: 123},
							otherVersion:  {Count: 123},
						},
					},
					group2Name: {
						Versions: map[string]*autoupdate.AutoUpdateAgentReportSpecGroupVersion{
							startVersion:  {Count: 123},
							targetVersion: {Count: 123},
						},
					},
				},
			},
		},
	}

	// canaryTestReports contain more agents, so it triggers the canary logic
	canaryTestReports := []*autoupdate.AutoUpdateAgentReport{
		{
			Metadata: &headerv1.Metadata{Name: "auth1"},
			Spec: &autoupdate.AutoUpdateAgentReportSpec{
				Timestamp: timestamppb.New(fewSecondsAgo),
				Groups: map[string]*autoupdate.AutoUpdateAgentReportSpecGroup{
					group1Name: {
						Versions: map[string]*autoupdate.AutoUpdateAgentReportSpecGroupVersion{
							startVersion:  {Count: 20},
							targetVersion: {Count: 5},
							otherVersion:  {Count: 1},
						},
					},
					group2Name: {
						Versions: map[string]*autoupdate.AutoUpdateAgentReportSpecGroupVersion{
							startVersion:  {Count: 3},
							targetVersion: {Count: 5},
						},
					},
				},
			},
		},
	}

	var (
		testCanaries        []*autoupdate.Canary
		healthyTestCanaries []*autoupdate.Canary
	)
	for i := range 10 {
		updaterId := uuid.NewString()
		hostID := uuid.NewString()
		hostName := fmt.Sprintf("canary-%d", i)
		testCanaries = append(testCanaries, &autoupdate.Canary{
			UpdaterId: updaterId,
			HostId:    hostID,
			Hostname:  hostName,
			Success:   false,
		})
		healthyTestCanaries = append(healthyTestCanaries, &autoupdate.Canary{
			UpdaterId: updaterId,
			HostId:    hostID,
			Hostname:  hostName,
			Success:   true,
		})
	}

	testCanariesLookupNotFound := make(map[string][]callAnswer[[]*proto.UpstreamInventoryHello])
	testCanariesLookupStartVersion := make(map[string][]callAnswer[[]*proto.UpstreamInventoryHello])
	testCanariesLookupTargetVersion := make(map[string][]callAnswer[[]*proto.UpstreamInventoryHello])
	testCanariesLookupTargetVersionDualHandles := make(map[string][]callAnswer[[]*proto.UpstreamInventoryHello])

	for _, canary := range testCanaries {
		testCanariesLookupNotFound[canary.HostId] = []callAnswer[[]*proto.UpstreamInventoryHello]{
			{err: trace.NotFound("handle not found")},
		}
		testCanariesLookupStartVersion[canary.HostId] = []callAnswer[[]*proto.UpstreamInventoryHello]{
			{
				result: []*proto.UpstreamInventoryHello{
					{
						Version:                 startVersion,
						ServerID:                canary.HostId,
						Hostname:                canary.Hostname,
						ExternalUpgrader:        types.UpgraderKindTeleportUpdate,
						ExternalUpgraderVersion: startVersion,
					},
				},
				err: nil,
			},
		}
		testCanariesLookupTargetVersion[canary.HostId] = []callAnswer[[]*proto.UpstreamInventoryHello]{
			{
				result: []*proto.UpstreamInventoryHello{
					{
						Version:                 targetVersion,
						ServerID:                canary.HostId,
						Hostname:                canary.Hostname,
						ExternalUpgrader:        types.UpgraderKindTeleportUpdate,
						ExternalUpgraderVersion: targetVersion,
					},
				},
				err: nil,
			},
		}
		testCanariesLookupTargetVersionDualHandles[canary.HostId] = []callAnswer[[]*proto.UpstreamInventoryHello]{
			{
				result: []*proto.UpstreamInventoryHello{
					{
						Version:                 startVersion,
						ServerID:                canary.HostId,
						Hostname:                canary.Hostname,
						ExternalUpgrader:        types.UpgraderKindTeleportUpdate,
						ExternalUpgraderVersion: startVersion,
					},
					{
						Version:                 targetVersion,
						ServerID:                canary.HostId,
						Hostname:                canary.Hostname,
						ExternalUpgrader:        types.UpgraderKindTeleportUpdate,
						ExternalUpgraderVersion: targetVersion,
					},
				},
				err: nil,
			},
		}
	}

	tests := []struct {
		name             string
		initialState     []*autoupdate.AutoUpdateAgentRolloutStatusGroup
		reports          []*autoupdate.AutoUpdateAgentReport
		rolloutStartTime *timestamppb.Timestamp
		expectedState    []*autoupdate.AutoUpdateAgentRolloutStatusGroup
		canarySamples    []callAnswer[[]*autoupdate.Canary]
		agentLookups     map[string][]callAnswer[[]*proto.UpstreamInventoryHello]
	}{
		{
			name: "single group unstarted -> unstarted",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonCannotStart,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
		},
		{
			name: "single group unstarted -> unstarted because rollout changed in window",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
			rolloutStartTime: timestamppb.New(clock.Now()),
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonRolloutChanged,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
		},
		{
			name: "single group unstarted -> active",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonCanStart,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
		},
		{
			name: "single group active -> active",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(fewMinutesAgo),
					LastUpdateTime:   timestamppb.New(fewMinutesAgo),
					LastUpdateReason: updateReasonCanStart,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(fewMinutesAgo),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonUpdateInProgress,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
		},
		{
			name: "single group active -> done",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(yesterday),
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonUpdateInProgress,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					StartTime:        timestamppb.New(yesterday),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonUpdateComplete,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
		},
		{
			name: "single group done -> done",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					StartTime:        timestamppb.New(yesterday),
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonUpdateComplete,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					StartTime:        timestamppb.New(yesterday),
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonUpdateComplete,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
		},
		{
			name: "single group rolledback -> rolledback",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					StartTime:        timestamppb.New(yesterday),
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: "manual_rollback",
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					StartTime:        timestamppb.New(yesterday),
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: "manual_rollback",
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
		},
		{
			name: "first group done, second should activate, third should not progress",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					StartTime:        timestamppb.New(yesterday),
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonUpdateComplete,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
				{
					Name:             group2Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					ConfigWaitHours:  24,
				},
				{
					Name:             group3Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					ConfigWaitHours:  0,
				},
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					StartTime:        timestamppb.New(yesterday),
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonUpdateComplete,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
				{
					Name:             group2Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonCanStart,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					ConfigWaitHours:  24,
				},
				{
					Name:             group3Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonPreviousGroupsNotDone,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					ConfigWaitHours:  0,
				},
			},
		},
		{
			name: "first group rolledback, second should not start",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					StartTime:        timestamppb.New(yesterday),
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: "manual_rollback",
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
				{
					Name:             group2Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					ConfigWaitHours:  24,
				},
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					StartTime:        timestamppb.New(yesterday),
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: "manual_rollback",
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
				{
					Name:             group2Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonPreviousGroupsNotDone,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					ConfigWaitHours:  24,
				},
			},
		},
		{
			name: "first group rolledback, second is active and should become done, third should not progress",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					StartTime:        timestamppb.New(yesterday),
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: "manual_rollback",
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
				{
					Name:             group2Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(yesterday),
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonCanStart,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					ConfigWaitHours:  0,
				},
				{
					Name:             group3Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					ConfigWaitHours:  0,
				},
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					StartTime:        timestamppb.New(yesterday),
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: "manual_rollback",
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
				{
					Name:             group2Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					StartTime:        timestamppb.New(yesterday),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonUpdateComplete,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					ConfigWaitHours:  0,
				},
				{
					Name:             group3Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonPreviousGroupsNotDone,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					ConfigWaitHours:  0,
				},
			},
		},
		{
			name: "single group unstarted -> unstarted with reports",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
			reports: testReports,
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonCannotStart,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
					// Group1 is the catch-all group, so it should count group2 agents
					PresentCount:  18,
					UpToDateCount: 10,
				},
			},
		},
		{
			name: "single group active -> active with reports",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(fewMinutesAgo),
					LastUpdateTime:   timestamppb.New(fewMinutesAgo),
					LastUpdateReason: updateReasonCanStart,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					InitialCount:     25,
					UpToDateCount:    0,
					PresentCount:     10,
				},
			},
			reports: testReports,
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(fewMinutesAgo),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonUpdateInProgress,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					// Group1 is the catch-all group, so it should count group2 agents
					PresentCount:  18,
					UpToDateCount: 10,
					// InitialCount must not be changed during active -> active transitions
					InitialCount: 25,
				},
			},
		},
		{
			name: "single group unstarted -> active with reports",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					PresentCount:     12,
					UpToDateCount:    3,
				},
			},
			reports: testReports,
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonCanStart,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					// InitialCount must be set during unstarted -> active transition
					InitialCount:  18,
					PresentCount:  18,
					UpToDateCount: 10,
				},
			},
		},
		{
			name: "first group done, second should activate, third should not progress, with reports",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					StartTime:        timestamppb.New(yesterday),
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonUpdateComplete,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					InitialCount:     10,
					PresentCount:     8,
					UpToDateCount:    5,
				},
				{
					Name:             group2Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					ConfigWaitHours:  24,
					PresentCount:     2,
					UpToDateCount:    2,
				},
				{
					Name:             group3Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					ConfigWaitHours:  0,
				},
			},
			reports: testReports,
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					StartTime:        timestamppb.New(yesterday),
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonUpdateComplete,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					InitialCount:     10,
					PresentCount:     10,
					UpToDateCount:    5,
				},
				{
					Name:             group2Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonCanStart,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					ConfigWaitHours:  24,
					InitialCount:     8,
					PresentCount:     8,
					UpToDateCount:    5,
				},
				{
					Name:             group3Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonPreviousGroupsNotDone,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					ConfigWaitHours:  0,
				},
			},
		},
		{
			name: "single group unstarted -> canary, no canaries sampled",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(yesterday),
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					PresentCount:     12,
					UpToDateCount:    3,
					CanaryCount:      5,
				},
			},
			reports:       canaryTestReports,
			canarySamples: []callAnswer[[]*autoupdate.Canary]{{}},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY,
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonCanStart,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					// InitialCount must be set during unstarted -> active transition
					InitialCount:  34,
					PresentCount:  34,
					UpToDateCount: 10,
					CanaryCount:   5,
				},
			},
		},
		{
			name: "single group canary -> canary, sampling agents, agents not found",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonCanStart,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					// InitialCount must be set during unstarted -> active transition
					InitialCount:  34,
					PresentCount:  34,
					UpToDateCount: 10,
					CanaryCount:   5,
				},
			},
			reports:       canaryTestReports,
			canarySamples: mockResponseForCanaries(testCanaries[:5]),
			agentLookups:  testCanariesLookupNotFound,
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonWaitingForCanaries,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					// InitialCount must be set during unstarted -> active transition
					InitialCount:  34,
					PresentCount:  34,
					UpToDateCount: 10,
					CanaryCount:   5,
					Canaries:      testCanaries[:5],
				},
			},
		},
		{
			name: "single group canary -> canary, sampling agents, agents running old version",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonCanStart,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					// InitialCount must be set during unstarted -> active transition
					InitialCount:  34,
					PresentCount:  34,
					UpToDateCount: 10,
					CanaryCount:   5,
				},
			},
			reports:       canaryTestReports,
			canarySamples: mockResponseForCanaries(testCanaries[:5]),
			agentLookups:  testCanariesLookupStartVersion,
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonWaitingForCanaries,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					// InitialCount must be set during unstarted -> active transition
					InitialCount:  34,
					PresentCount:  34,
					UpToDateCount: 10,
					CanaryCount:   5,
					Canaries:      testCanaries[:5],
				},
			},
		},
		{
			name: "single group canary -> active, sampling agents, agents running target version",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY,
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonCanStart,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					// InitialCount must be set during unstarted -> active transition
					InitialCount:  34,
					PresentCount:  34,
					UpToDateCount: 10,
					CanaryCount:   5,
				},
			},
			reports:       canaryTestReports,
			canarySamples: mockResponseForCanaries(testCanaries[:5]),
			agentLookups:  testCanariesLookupTargetVersion,
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonCanariesAlive,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					// InitialCount must be set during unstarted -> active transition
					InitialCount:  34,
					PresentCount:  34,
					UpToDateCount: 10,
					CanaryCount:   5,
					Canaries:      healthyTestCanaries[:5],
				},
			},
		},
		{
			name: "single group canary -> active, already sampled agents, agents running target version",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY,
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonCanStart,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					// InitialCount must be set during unstarted -> active transition
					InitialCount:  34,
					PresentCount:  34,
					UpToDateCount: 10,
					CanaryCount:   5,
					Canaries:      testCanaries[:5],
				},
			},
			reports: canaryTestReports,
			// no canarySamples set, we don't expect a sampling call
			agentLookups: testCanariesLookupTargetVersion,
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonCanariesAlive,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					// InitialCount must be set during unstarted -> active transition
					InitialCount:  34,
					PresentCount:  34,
					UpToDateCount: 10,
					CanaryCount:   5,
					Canaries:      healthyTestCanaries[:5],
				},
			},
		},
		{
			name: "single group canary -> canary, incomplete sampled agents, agents running start version",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonCanStart,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					// InitialCount must be set during unstarted -> active transition
					InitialCount:  34,
					PresentCount:  34,
					UpToDateCount: 10,
					CanaryCount:   5,
					// Only 2 canaries
					Canaries: testCanaries[8:10],
				},
			},
			reports:       canaryTestReports,
			canarySamples: mockResponseForCanaries(testCanaries[:5]),
			agentLookups:  testCanariesLookupStartVersion,
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonWaitingForCanaries,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					// InitialCount must be set during unstarted -> active transition
					InitialCount:  34,
					PresentCount:  34,
					UpToDateCount: 10,
					CanaryCount:   5,
					// We expect the 2 initial agents to stay here, and 3 additional agents
					Canaries: append(testCanaries[8:10], testCanaries[0:3]...),
				},
			},
		},
		{
			name: "single group canary -> active, already sampled agents, agents running target version, several handles",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonCanStart,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					// InitialCount must be set during unstarted -> active transition
					InitialCount:  34,
					PresentCount:  34,
					UpToDateCount: 10,
					CanaryCount:   5,
					Canaries:      testCanaries[:5],
				},
			},
			reports: canaryTestReports,
			// no canarySamples set, we don't expect a sampling call
			agentLookups: testCanariesLookupTargetVersionDualHandles,
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             group1Name,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonCanariesAlive,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
					// InitialCount must be set during unstarted -> active transition
					InitialCount:  34,
					PresentCount:  34,
					UpToDateCount: 10,
					CanaryCount:   5,
					Canaries:      healthyTestCanaries[:5],
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &autoupdate.AutoUpdateAgentRolloutStatus{
				Groups:    tt.initialState,
				State:     0,
				StartTime: tt.rolloutStartTime,
			}
			spec := &autoupdate.AutoUpdateAgentRolloutSpec{
				StartVersion:  startVersion,
				TargetVersion: targetVersion,
			}

			stubs := mockClientStubs{
				agentSamples:          tt.canarySamples,
				inventoryAgentLookups: tt.agentLookups,
			}
			if tt.reports == nil {
				stubs.reportsAnswers = []callAnswer[[]*autoupdate.AutoUpdateAgentReport]{
					{
						result: []*autoupdate.AutoUpdateAgentReport{},
						err:    trace.NotFound("no report"),
					},
				}
			} else {
				stubs.reportsAnswers = []callAnswer[[]*autoupdate.AutoUpdateAgentReport]{
					{
						result: tt.reports,
						err:    nil,
					},
				}
			}

			clt := newMockClient(t, stubs)
			strategy, err := newHaltOnErrorStrategy(log, clt)
			require.NoError(t, err)
			err = strategy.progressRollout(ctx, spec, status, clock.Now())
			require.NoError(t, err)
			// Group order matters.
			// It's not super important for time-based, but is crucial for halt-on-error.
			// So it's better to be more conservative and validate order never changes for
			// both strategies.
			require.Empty(t, cmp.Diff(tt.expectedState, tt.initialState, protocmp.Transform()))
		})
	}
}

func TestCountCatchAll(t *testing.T) {
	countByGroup := map[string]int{
		"dev":   10,
		"stage": 25,
		"prod":  33,
	}
	upToDateByGroup := map[string]int{
		"dev":   5,
		"stage": 12,
		"prod":  1,
	}

	tests := []struct {
		name             string
		rolloutStatus    *autoupdate.AutoUpdateAgentRolloutStatus
		expectedCount    int
		expectedUpToDate int
	}{
		{
			name: "all group hit",
			rolloutStatus: &autoupdate.AutoUpdateAgentRolloutStatus{
				Groups: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
					{Name: "dev"},
					{Name: "stage"},
					{Name: "prod"},
				},
			},
			expectedCount:    countByGroup["prod"],
			expectedUpToDate: upToDateByGroup["prod"],
		},
		{
			name: "one group miss",
			rolloutStatus: &autoupdate.AutoUpdateAgentRolloutStatus{
				Groups: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
					{Name: "dev"},
					{Name: "prod"},
				},
			},
			expectedCount:    countByGroup["stage"] + countByGroup["prod"],
			expectedUpToDate: upToDateByGroup["stage"] + upToDateByGroup["prod"],
		},
		{
			name: "only catch-all group hit",
			rolloutStatus: &autoupdate.AutoUpdateAgentRolloutStatus{
				Groups: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
					{Name: "prod"},
				},
			},
			expectedCount:    countByGroup["dev"] + countByGroup["stage"] + countByGroup["prod"],
			expectedUpToDate: upToDateByGroup["dev"] + upToDateByGroup["stage"] + upToDateByGroup["prod"],
		},
		{
			name: "no common group",
			rolloutStatus: &autoupdate.AutoUpdateAgentRolloutStatus{
				Groups: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
					{Name: "foobar"},
				},
			},
			expectedCount:    countByGroup["dev"] + countByGroup["stage"] + countByGroup["prod"],
			expectedUpToDate: upToDateByGroup["dev"] + upToDateByGroup["stage"] + upToDateByGroup["prod"],
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, upToDate := countCatchAll(tt.rolloutStatus, countByGroup, upToDateByGroup)
			require.Equal(t, tt.expectedCount, count)
			require.Equal(t, tt.expectedUpToDate, upToDate)
		})
	}
}

func mockResponseForCanaries(canaries []*autoupdate.Canary) []callAnswer[[]*autoupdate.Canary] {
	return []callAnswer[[]*autoupdate.Canary]{
		{
			result: canaries,
		},
	}
}
