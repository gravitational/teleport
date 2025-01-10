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

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
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
	strategy, err := newHaltOnErrorStrategy(log)
	require.NoError(t, err)

	fewMinutesAgo := clock.Now().Add(-5 * time.Minute)
	yesterday := testSaturday
	canStartToday := everyWeekday
	cannotStartToday := everyWeekdayButSunday
	ctx := context.Background()

	group1Name := "group1"
	group2Name := "group2"
	group3Name := "group3"

	tests := []struct {
		name             string
		initialState     []*autoupdate.AutoUpdateAgentRolloutStatusGroup
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &autoupdate.AutoUpdateAgentRolloutStatus{
				Groups:    tt.initialState,
				State:     0,
				StartTime: tt.rolloutStartTime,
			}
			err := strategy.progressRollout(ctx, nil, status, clock.Now())
			require.NoError(t, err)
			// We use require.Equal instead of Elements match because group order matters.
			// It's not super important for time-based, but is crucial for halt-on-error.
			// So it's better to be more conservative and validate order never changes for
			// both strategies.
			require.Equal(t, tt.expectedState, tt.initialState)
		})
	}
}
