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

package rollout

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types/autoupdate"
)

func TestTriggerGroups(t *testing.T) {
	now := time.Now()
	nowPb := timestamppb.New(now)
	spec := &autoupdatev1pb.AutoUpdateAgentRolloutSpec{
		StartVersion:   "1.2.3",
		TargetVersion:  "1.2.4",
		Schedule:       autoupdate.AgentsScheduleRegular,
		AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
		Strategy:       autoupdate.AgentsStrategyHaltOnError,
	}
	status := &autoupdatev1pb.AutoUpdateAgentRolloutStatus{
		Groups: []*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup{
			{
				Name:  "blue",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
			},
			{
				Name:  "dev",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
			},
			{
				Name:  "stage",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			},
			{
				Name:  "prod",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
			},
			{
				Name:  "backup",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSPECIFIED,
			},
		},
	}

	tests := []struct {
		name           string
		rollout        *autoupdatev1pb.AutoUpdateAgentRollout
		groupNames     []string
		desiredState   autoupdatev1pb.AutoUpdateAgentGroupState
		expectedStatus *autoupdatev1pb.AutoUpdateAgentRolloutStatus
		expectErr      require.ErrorAssertionFunc
	}{
		{
			name: "valid transition",
			rollout: &autoupdatev1pb.AutoUpdateAgentRollout{
				Spec:   spec,
				Status: status,
			},
			groupNames:   []string{"blue", "prod", "backup"},
			desiredState: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			expectErr:    require.NoError,
			expectedStatus: &autoupdatev1pb.AutoUpdateAgentRolloutStatus{
				Groups: []*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup{
					{
						Name:             "blue",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
						StartTime:        nowPb,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonManualTrigger,
					},
					{
						Name:  "dev",
						State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					},
					{
						Name:  "stage",
						State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					},
					{
						Name:             "prod",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
						StartTime:        nowPb,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonManualTrigger,
					},
					{
						Name:             "backup",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
						StartTime:        nowPb,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonManualTrigger,
					},
				},
			},
		},
		{
			name: "no groups in rollout",
			rollout: &autoupdatev1pb.AutoUpdateAgentRollout{
				Spec:   spec,
				Status: &autoupdatev1pb.AutoUpdateAgentRolloutStatus{},
			},
			groupNames:   []string{"prod", "backup"},
			desiredState: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			expectErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "rollout has no groups")
			},
		},
		{
			name: "unsupported desired state",
			rollout: &autoupdatev1pb.AutoUpdateAgentRollout{
				Spec:   spec,
				Status: status,
			},
			groupNames:   []string{"prod", "backup"},
			desiredState: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
			expectErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "unsupported desired state")
			},
		},
		{
			name: "unsupported strategy",
			rollout: &autoupdatev1pb.AutoUpdateAgentRollout{
				Spec: &autoupdatev1pb.AutoUpdateAgentRolloutSpec{
					StartVersion:   "1.2.3",
					TargetVersion:  "1.2.4",
					Schedule:       autoupdate.AgentsScheduleRegular,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
					Strategy:       autoupdate.AgentsStrategyTimeBased,
				},
				Status: status,
			},
			groupNames:   []string{"prod", "backup"},
			desiredState: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			expectErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "not supported for rollout strategy")
			},
		},
		{
			name: "unsupported schedule",
			rollout: &autoupdatev1pb.AutoUpdateAgentRollout{
				Spec: &autoupdatev1pb.AutoUpdateAgentRolloutSpec{
					StartVersion:   "1.2.3",
					TargetVersion:  "1.2.4",
					Schedule:       autoupdate.AgentsScheduleImmediate,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
				},
				Status: nil,
			},
			groupNames:   []string{"prod", "backup"},
			desiredState: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			expectErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "rollout schedule is immediate")
			},
		},
		{
			name: "group already active",
			rollout: &autoupdatev1pb.AutoUpdateAgentRollout{
				Spec:   spec,
				Status: status,
			},
			groupNames:   []string{"stage"},
			desiredState: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			expectErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "is already active")
			},
		},
		{
			name: "group already done",
			rollout: &autoupdatev1pb.AutoUpdateAgentRollout{
				Spec:   spec,
				Status: status,
			},
			groupNames:   []string{"dev"},
			desiredState: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			expectErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "is already done")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TriggerGroups(tt.rollout, tt.groupNames, tt.desiredState, now)
			tt.expectErr(t, err)

			if err == nil {
				require.Empty(t, cmp.Diff(tt.expectedStatus, tt.rollout.GetStatus(), protocmp.Transform()))
			}
		})
	}
}

func TestForceGroupsDone(t *testing.T) {
	now := time.Now()
	nowPb := timestamppb.New(now)
	spec := &autoupdatev1pb.AutoUpdateAgentRolloutSpec{
		StartVersion:   "1.2.3",
		TargetVersion:  "1.2.4",
		Schedule:       autoupdate.AgentsScheduleRegular,
		AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
		Strategy:       autoupdate.AgentsStrategyHaltOnError,
	}
	status := &autoupdatev1pb.AutoUpdateAgentRolloutStatus{
		Groups: []*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup{
			{
				Name:  "blue",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
			},
			{
				Name:  "dev",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
			},
			{
				Name:  "stage",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			},
			{
				Name:  "prod",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
			},
			{
				Name:  "backup",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSPECIFIED,
			},
		},
	}

	tests := []struct {
		name           string
		rollout        *autoupdatev1pb.AutoUpdateAgentRollout
		groupNames     []string
		expectedStatus *autoupdatev1pb.AutoUpdateAgentRolloutStatus
		expectErr      require.ErrorAssertionFunc
	}{
		{
			name: "valid transition",
			rollout: &autoupdatev1pb.AutoUpdateAgentRollout{
				Spec:   spec,
				Status: status,
			},
			groupNames: []string{"blue", "stage", "prod", "backup"},
			expectErr:  require.NoError,
			expectedStatus: &autoupdatev1pb.AutoUpdateAgentRolloutStatus{
				Groups: []*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup{
					{
						Name:             "blue",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonForcedDone,
					},
					{
						Name:  "dev",
						State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					},
					{
						Name:             "stage",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonForcedDone,
					},
					{
						Name:             "prod",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonForcedDone,
					},
					{
						Name:             "backup",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonForcedDone,
					},
				},
			},
		},
		{
			name: "no groups in rollout",
			rollout: &autoupdatev1pb.AutoUpdateAgentRollout{
				Spec:   spec,
				Status: &autoupdatev1pb.AutoUpdateAgentRolloutStatus{},
			},
			groupNames: []string{"prod", "backup"},
			expectErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "rollout has no groups")
			},
		},
		{
			name: "unsupported strategy",
			rollout: &autoupdatev1pb.AutoUpdateAgentRollout{
				Spec: &autoupdatev1pb.AutoUpdateAgentRolloutSpec{
					StartVersion:   "1.2.3",
					TargetVersion:  "1.2.4",
					Schedule:       autoupdate.AgentsScheduleRegular,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
					Strategy:       autoupdate.AgentsStrategyTimeBased,
				},
				Status: status,
			},
			groupNames: []string{"prod", "backup"},
			expectErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "not supported for rollout strategy")
			},
		},
		{
			name: "unsupported schedule",
			rollout: &autoupdatev1pb.AutoUpdateAgentRollout{
				Spec: &autoupdatev1pb.AutoUpdateAgentRolloutSpec{
					StartVersion:   "1.2.3",
					TargetVersion:  "1.2.4",
					Schedule:       autoupdate.AgentsScheduleImmediate,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
				},
				Status: nil,
			},
			groupNames: []string{"prod", "backup"},
			expectErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "rollout schedule is immediate")
			},
		},
		{
			name: "group already done",
			rollout: &autoupdatev1pb.AutoUpdateAgentRollout{
				Spec:   spec,
				Status: status,
			},
			groupNames: []string{"dev"},
			expectErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "is already done")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ForceGroupsDone(tt.rollout, tt.groupNames, now)
			tt.expectErr(t, err)

			if err == nil {
				require.Empty(t, cmp.Diff(tt.expectedStatus, tt.rollout.GetStatus(), protocmp.Transform()))
			}
		})
	}
}

func TestRollbackGroups(t *testing.T) {
	now := time.Now()
	nowPb := timestamppb.New(now)
	spec := &autoupdatev1pb.AutoUpdateAgentRolloutSpec{
		StartVersion:   "1.2.3",
		TargetVersion:  "1.2.4",
		Schedule:       autoupdate.AgentsScheduleRegular,
		AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
		Strategy:       autoupdate.AgentsStrategyHaltOnError,
	}
	status := &autoupdatev1pb.AutoUpdateAgentRolloutStatus{
		Groups: []*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup{
			{
				Name:  "blue",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
			},
			{
				Name:  "dev",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
			},
			{
				Name:  "stage",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			},
			{
				Name:  "prod",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
			},
			{
				Name:  "backup",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSPECIFIED,
			},
		},
	}

	tests := []struct {
		name           string
		rollout        *autoupdatev1pb.AutoUpdateAgentRollout
		groupNames     []string
		expectedStatus *autoupdatev1pb.AutoUpdateAgentRolloutStatus
		expectErr      require.ErrorAssertionFunc
	}{
		{
			name: "valid transition",
			rollout: &autoupdatev1pb.AutoUpdateAgentRollout{
				Spec:   spec,
				Status: status,
			},
			groupNames: []string{"dev", "stage", "prod", "backup"},
			expectErr:  require.NoError,
			expectedStatus: &autoupdatev1pb.AutoUpdateAgentRolloutStatus{
				Groups: []*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup{
					{
						Name:  "blue",
						State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					},
					{
						Name:             "dev",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonManualRollback,
					},
					{
						Name:             "stage",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonManualRollback,
					},
					{
						Name:             "prod",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonManualRollback,
					},
					{
						Name:             "backup",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonManualRollback,
					},
				},
			},
		},
		{
			name: "no groups in rollout",
			rollout: &autoupdatev1pb.AutoUpdateAgentRollout{
				Spec:   spec,
				Status: &autoupdatev1pb.AutoUpdateAgentRolloutStatus{},
			},
			groupNames: []string{"prod", "backup"},
			expectErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "rollout has no groups")
			},
		},
		{
			name: "unsupported strategy",
			rollout: &autoupdatev1pb.AutoUpdateAgentRollout{
				Spec: &autoupdatev1pb.AutoUpdateAgentRolloutSpec{
					StartVersion:   "1.2.3",
					TargetVersion:  "1.2.4",
					Schedule:       autoupdate.AgentsScheduleRegular,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
					Strategy:       autoupdate.AgentsStrategyTimeBased,
				},
				Status: status,
			},
			groupNames: []string{"prod", "backup"},
			expectErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "not supported for rollout strategy")
			},
		},
		{
			name: "unsupported schedule",
			rollout: &autoupdatev1pb.AutoUpdateAgentRollout{
				Spec: &autoupdatev1pb.AutoUpdateAgentRolloutSpec{
					StartVersion:   "1.2.3",
					TargetVersion:  "1.2.4",
					Schedule:       autoupdate.AgentsScheduleImmediate,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
				},
				Status: nil,
			},
			groupNames: []string{"prod", "backup"},
			expectErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "rollout schedule is immediate")
			},
		},
		{
			name: "group already rolledback",
			rollout: &autoupdatev1pb.AutoUpdateAgentRollout{
				Spec:   spec,
				Status: status,
			},
			groupNames: []string{"blue"},
			expectErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "is already in a rolled-back state")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RollbackGroups(tt.rollout, tt.groupNames, now)
			tt.expectErr(t, err)

			if err == nil {
				require.Empty(t, cmp.Diff(tt.expectedStatus, tt.rollout.GetStatus(), protocmp.Transform()))
			}
		})
	}
}

func TestRollbackStartedGroups(t *testing.T) {
	now := time.Now()
	nowPb := timestamppb.New(now)

	rollout := &autoupdatev1pb.AutoUpdateAgentRollout{
		Spec: &autoupdatev1pb.AutoUpdateAgentRolloutSpec{
			StartVersion:   "1.2.3",
			TargetVersion:  "1.2.4",
			Schedule:       autoupdate.AgentsScheduleRegular,
			AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
			Strategy:       autoupdate.AgentsStrategyHaltOnError,
		},
		Status: &autoupdatev1pb.AutoUpdateAgentRolloutStatus{
			Groups: []*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup{
				{
					Name:  "blue",
					State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
				},
				{
					Name:  "dev",
					State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
				},
				{
					Name:  "stage",
					State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
				},
				{
					Name:  "prod",
					State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
				},
				{
					Name:  "backup",
					State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSPECIFIED,
				},
			},
		},
	}

	expectedGroups := []*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup{
		{
			// Already rolledback group is not changed.
			Name:  "blue",
			State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
		},
		{
			// Active and done groups are rolledback.
			Name:             "dev",
			State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
			LastUpdateTime:   nowPb,
			LastUpdateReason: updateReasonManualRollback,
		},
		{
			Name:             "stage",
			State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
			LastUpdateTime:   nowPb,
			LastUpdateReason: updateReasonManualRollback,
		},
		{
			// Unstarted and unknown groups are not changed.
			Name:  "prod",
			State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
		},
		{
			Name:  "backup",
			State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSPECIFIED,
		},
	}

	require.NoError(t, RollbackStartedGroups(rollout, now))
	require.Empty(t, cmp.Diff(expectedGroups, rollout.GetStatus().GetGroups(), protocmp.Transform()))

}
