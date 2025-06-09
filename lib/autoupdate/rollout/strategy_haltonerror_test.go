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
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/lib/utils"
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
	log := utils.NewSlogLoggerForTests()

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
							startVersion:  {Count: 5},
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

	tests := []struct {
		name             string
		initialState     []*autoupdate.AutoUpdateAgentRolloutStatusGroup
		reports          []*autoupdate.AutoUpdateAgentReport
		rolloutStartTime *timestamppb.Timestamp
		expectedState    []*autoupdate.AutoUpdateAgentRolloutStatusGroup
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
					PresentCount:  20,
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
					PresentCount:  20,
					UpToDateCount: 10,
					// InitialCount must not be changed during active -> active transitions
					InitialCount: 25,
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
					InitialCount:  20,
					PresentCount:  20,
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
					InitialCount:     10,
					PresentCount:     10,
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

			stubs := mockClientStubs{}
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
			// We use require.Equal instead of Elements match because group order matters.
			// It's not super important for time-based, but is crucial for halt-on-error.
			// So it's better to be more conservative and validate order never changes for
			// both strategies.
			require.Equal(t, tt.expectedState, tt.initialState)
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
