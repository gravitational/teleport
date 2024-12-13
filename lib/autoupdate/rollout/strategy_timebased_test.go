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

func Test_progressGroupsTimeBased(t *testing.T) {
	clock := clockwork.NewFakeClockAt(testSunday)
	log := utils.NewSlogLoggerForTests()
	strategy, err := newTimeBasedStrategy(log, clock)
	require.NoError(t, err)

	groupName := "test-group"
	canStartToday := everyWeekday
	cannotStartToday := everyWeekdayButSunday
	lastUpdate := timestamppb.New(clock.Now().Add(-5 * time.Minute))
	ctx := context.Background()

	tests := []struct {
		name          string
		initialState  []*autoupdate.AutoUpdateAgentRolloutStatusGroup
		expectedState []*autoupdate.AutoUpdateAgentRolloutStatusGroup
	}{
		{
			name: "unstarted -> unstarted",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   lastUpdate,
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonOutsideWindow,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
		},
		{
			name: "unstarted -> active",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   lastUpdate,
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonInWindow,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
		},
		{
			name: "done -> done",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					LastUpdateTime:   lastUpdate,
					LastUpdateReason: updateReasonOutsideWindow,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					LastUpdateTime:   lastUpdate,
					LastUpdateReason: updateReasonOutsideWindow,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
		},
		{
			name: "done -> active",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					LastUpdateTime:   lastUpdate,
					StartTime:        lastUpdate,
					LastUpdateReason: updateReasonOutsideWindow,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonInWindow,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
		},
		{
			name: "active -> active",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        lastUpdate,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonInWindow,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        lastUpdate,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonInWindow,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
		},
		{
			name: "active -> done",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        lastUpdate,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonInWindow,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					StartTime:        lastUpdate,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonOutsideWindow,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
		},
		{
			name: "rolledback is a dead end",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:            groupName + "-in-maintenance",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					LastUpdateTime:  lastUpdate,
					ConfigDays:      canStartToday,
					ConfigStartHour: matchingStartHour,
				},
				{
					Name:            groupName + "-out-of-maintenance",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					LastUpdateTime:  lastUpdate,
					ConfigDays:      cannotStartToday,
					ConfigStartHour: matchingStartHour,
				},
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:            groupName + "-in-maintenance",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					LastUpdateTime:  lastUpdate,
					ConfigDays:      canStartToday,
					ConfigStartHour: matchingStartHour,
				},
				{
					Name:            groupName + "-out-of-maintenance",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					LastUpdateTime:  lastUpdate,
					ConfigDays:      cannotStartToday,
					ConfigStartHour: matchingStartHour,
				},
			},
		},
		{
			name: "mix of everything",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:            "new group should start",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:  lastUpdate,
					ConfigDays:      canStartToday,
					ConfigStartHour: matchingStartHour,
				},
				{
					Name:            "done group should start",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					LastUpdateTime:  lastUpdate,
					StartTime:       lastUpdate,
					ConfigDays:      canStartToday,
					ConfigStartHour: matchingStartHour,
				},
				{
					Name:            "rolledback group should do nothing",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					LastUpdateTime:  lastUpdate,
					ConfigDays:      canStartToday,
					ConfigStartHour: matchingStartHour,
				},
				{
					Name:            "old group should stop",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					LastUpdateTime:  lastUpdate,
					StartTime:       lastUpdate,
					ConfigDays:      cannotStartToday,
					ConfigStartHour: matchingStartHour,
				},
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:             "new group should start",
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonInWindow,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
				{
					Name:             "done group should start",
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonInWindow,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				},
				{
					Name:            "rolledback group should do nothing",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					LastUpdateTime:  lastUpdate,
					ConfigDays:      canStartToday,
					ConfigStartHour: matchingStartHour,
				},
				{
					Name:             "old group should stop",
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					StartTime:        lastUpdate,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonOutsideWindow,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := strategy.progressRollout(ctx, tt.initialState)
			require.NoError(t, err)
			// We use require.Equal instead of Elements match because group order matters.
			// It's not super important for time-based, but is crucial for halt-on-error.
			// So it's better to be more conservative and validate order never changes for
			// both strategies.
			require.Equal(t, tt.expectedState, tt.initialState)
		})
	}
}
