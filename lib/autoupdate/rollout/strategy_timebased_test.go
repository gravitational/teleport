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
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func Test_progressGroupsTimeBased(t *testing.T) {
	clock := clockwork.NewFakeClockAt(testSunday)
	log := logtest.NewLogger()

	groupName := "test-group"
	canStartToday := everyWeekday
	cannotStartToday := everyWeekdayButSunday
	lastUpdate := timestamppb.New(clock.Now().Add(-5 * time.Minute))
	ctx := context.Background()

	startVersion := "1.2.3"
	targetVersion := "1.2.4"
	fewSecondsAgo := clock.Now().Add(-5 * time.Second)
	fewMinutesAgo := clock.Now().Add(-7 * time.Minute)
	spec := autoupdate.AutoUpdateAgentRolloutSpec_builder{
		MaintenanceWindowDuration: durationpb.New(time.Hour),
		StartVersion:              startVersion,
		TargetVersion:             targetVersion,
	}.Build()

	var tests = []struct {
		name             string
		initialState     []*autoupdate.AutoUpdateAgentRolloutStatusGroup
		rolloutStartTime *timestamppb.Timestamp
		reports          []*autoupdate.AutoUpdateAgentReport
		expectedState    []*autoupdate.AutoUpdateAgentRolloutStatusGroup
	}{
		{
			name: "unstarted -> unstarted",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   lastUpdate,
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonOutsideWindow,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
			},
		},
		{
			name: "unstarted -> unstarted, with reports",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   lastUpdate,
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
			},
			reports: []*autoupdate.AutoUpdateAgentReport{
				autoupdate.AutoUpdateAgentReport_builder{
					Metadata: headerv1.Metadata_builder{Name: "auth1"}.Build(),
					Spec: autoupdate.AutoUpdateAgentReportSpec_builder{
						Timestamp: timestamppb.New(fewSecondsAgo),
						Groups: map[string]*autoupdate.AutoUpdateAgentReportSpecGroup{
							groupName: autoupdate.AutoUpdateAgentReportSpecGroup_builder{
								Versions: map[string]*autoupdate.AutoUpdateAgentReportSpecGroupVersion{
									startVersion:  autoupdate.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 5}.Build(),
									targetVersion: autoupdate.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 5}.Build(),
								},
							}.Build(),
						},
					}.Build(),
				}.Build(),
				autoupdate.AutoUpdateAgentReport_builder{
					Metadata: headerv1.Metadata_builder{Name: "auth2 (expired)"}.Build(),
					Spec: autoupdate.AutoUpdateAgentReportSpec_builder{
						Timestamp: timestamppb.New(fewMinutesAgo),
						Groups: map[string]*autoupdate.AutoUpdateAgentReportSpecGroup{
							groupName: autoupdate.AutoUpdateAgentReportSpecGroup_builder{
								Versions: map[string]*autoupdate.AutoUpdateAgentReportSpecGroupVersion{
									startVersion:  autoupdate.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 5}.Build(),
									targetVersion: autoupdate.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 5}.Build(),
								},
							}.Build(),
						},
					}.Build(),
				}.Build(),
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonOutsideWindow,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
					PresentCount:     10,
					UpToDateCount:    5,
				}.Build(),
			},
		},
		{
			name: "unstarted -> unstarted because rollout just changed",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   lastUpdate,
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
			},
			rolloutStartTime: timestamppb.New(clock.Now()),
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonRolloutChanged,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
			},
		},
		{
			name: "unstarted -> active",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:   lastUpdate,
					LastUpdateReason: updateReasonCreated,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonInWindow,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
			},
		},
		{
			name: "done -> done",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					LastUpdateTime:   lastUpdate,
					LastUpdateReason: updateReasonOutsideWindow,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					LastUpdateTime:   lastUpdate,
					LastUpdateReason: updateReasonOutsideWindow,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
			},
		},
		{
			name: "done -> active",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					LastUpdateTime:   lastUpdate,
					StartTime:        lastUpdate,
					LastUpdateReason: updateReasonOutsideWindow,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonInWindow,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
			},
		},
		{
			name: "active -> active",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        lastUpdate,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonInWindow,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        lastUpdate,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonInWindow,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
			},
		},
		{
			name: "active -> done",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        lastUpdate,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonInWindow,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             groupName,
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					StartTime:        lastUpdate,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonOutsideWindow,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
			},
		},
		{
			name: "rolledback is a dead end",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:            groupName + "-in-maintenance",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					LastUpdateTime:  lastUpdate,
					ConfigDays:      canStartToday,
					ConfigStartHour: matchingStartHour,
				}.Build(),
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:            groupName + "-out-of-maintenance",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					LastUpdateTime:  lastUpdate,
					ConfigDays:      cannotStartToday,
					ConfigStartHour: matchingStartHour,
				}.Build(),
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:            groupName + "-in-maintenance",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					LastUpdateTime:  lastUpdate,
					ConfigDays:      canStartToday,
					ConfigStartHour: matchingStartHour,
				}.Build(),
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:            groupName + "-out-of-maintenance",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					LastUpdateTime:  lastUpdate,
					ConfigDays:      cannotStartToday,
					ConfigStartHour: matchingStartHour,
				}.Build(),
			},
		},
		{
			name: "mix of everything",
			initialState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:            "new group should start",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
					LastUpdateTime:  lastUpdate,
					ConfigDays:      canStartToday,
					ConfigStartHour: matchingStartHour,
				}.Build(),
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:            "done group should start",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					LastUpdateTime:  lastUpdate,
					StartTime:       lastUpdate,
					ConfigDays:      canStartToday,
					ConfigStartHour: matchingStartHour,
				}.Build(),
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:            "rolledback group should do nothing",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					LastUpdateTime:  lastUpdate,
					ConfigDays:      canStartToday,
					ConfigStartHour: matchingStartHour,
				}.Build(),
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:            "old group should stop",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					LastUpdateTime:  lastUpdate,
					StartTime:       lastUpdate,
					ConfigDays:      cannotStartToday,
					ConfigStartHour: matchingStartHour,
				}.Build(),
			},
			expectedState: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             "new group should start",
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonInWindow,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             "done group should start",
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					StartTime:        timestamppb.New(clock.Now()),
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonInWindow,
					ConfigDays:       canStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:            "rolledback group should do nothing",
					State:           autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					LastUpdateTime:  lastUpdate,
					ConfigDays:      canStartToday,
					ConfigStartHour: matchingStartHour,
				}.Build(),
				autoupdate.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:             "old group should stop",
					State:            autoupdate.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					StartTime:        lastUpdate,
					LastUpdateTime:   timestamppb.New(clock.Now()),
					LastUpdateReason: updateReasonOutsideWindow,
					ConfigDays:       cannotStartToday,
					ConfigStartHour:  matchingStartHour,
				}.Build(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := autoupdate.AutoUpdateAgentRolloutStatus_builder{
				Groups:    tt.initialState,
				State:     0,
				StartTime: tt.rolloutStartTime,
			}.Build()
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
			strategy, err := newTimeBasedStrategy(log, newMockClient(t, stubs))
			require.NoError(t, err)
			err = strategy.progressRollout(ctx, spec, status, clock.Now())
			require.NoError(t, err)
			// We use require.Equal instead of Elements match because group order matters.
			// It's not super important for time-based, but is crucial for halt-on-error.
			// So it's better to be more conservative and validate order never changes for
			// both strategies.
			require.Equal(t, tt.expectedState, status.GetGroups())
		})
	}
}
