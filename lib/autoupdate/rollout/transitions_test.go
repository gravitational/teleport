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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types/autoupdate"
)

func TestTriggerGroups(t *testing.T) {
	now := time.Now()
	nowPb := timestamppb.New(now)
	fewSecondsAgo := now.Add(-3 * time.Second)
	fewMinutesAgo := now.Add(-6 * time.Minute)
	startVersion := "1.2.3"
	targetVersion := "1.2.4"
	otherVersion := "1.2.5"

	spec := autoupdatev1pb.AutoUpdateAgentRolloutSpec_builder{
		StartVersion:   startVersion,
		TargetVersion:  targetVersion,
		Schedule:       autoupdate.AgentsScheduleRegular,
		AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
		Strategy:       autoupdate.AgentsStrategyHaltOnError,
	}.Build()
	status := autoupdatev1pb.AutoUpdateAgentRolloutStatus_builder{
		Groups: []*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup{
			autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
				Name:  "blue",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
			}.Build(),
			autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
				Name:  "dev",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
			}.Build(),
			autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
				Name:  "stage",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			}.Build(),
			autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
				Name:  "prod",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
			}.Build(),
			autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
				Name:  "backup",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSPECIFIED,
			}.Build(),
		},
	}.Build()
	testReports := []*autoupdatev1pb.AutoUpdateAgentReport{
		autoupdatev1pb.AutoUpdateAgentReport_builder{
			Metadata: headerv1.Metadata_builder{Name: "auth1"}.Build(),
			Spec: autoupdatev1pb.AutoUpdateAgentReportSpec_builder{
				Timestamp: timestamppb.New(fewSecondsAgo),
				Groups: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroup{
					"blue": autoupdatev1pb.AutoUpdateAgentReportSpecGroup_builder{
						Versions: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion{
							startVersion:  autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 4}.Build(),
							targetVersion: autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 5}.Build(),
							otherVersion:  autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 1}.Build(),
						},
					}.Build(),
					"dev": autoupdatev1pb.AutoUpdateAgentReportSpecGroup_builder{
						Versions: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion{
							startVersion:  autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 5}.Build(),
							targetVersion: autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 5}.Build(),
						},
					}.Build(),
				},
			}.Build(),
		}.Build(),
		autoupdatev1pb.AutoUpdateAgentReport_builder{
			// This report is expired, it must be ignored
			Metadata: headerv1.Metadata_builder{Name: "auth2"}.Build(),
			Spec: autoupdatev1pb.AutoUpdateAgentReportSpec_builder{
				Timestamp: timestamppb.New(fewMinutesAgo),
				Groups: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroup{
					"blue": autoupdatev1pb.AutoUpdateAgentReportSpecGroup_builder{
						Versions: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion{
							startVersion:  autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 123}.Build(),
							targetVersion: autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 123}.Build(),
							otherVersion:  autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 123}.Build(),
						},
					}.Build(),
					"stage": autoupdatev1pb.AutoUpdateAgentReportSpecGroup_builder{
						Versions: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion{
							startVersion:  autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 123}.Build(),
							targetVersion: autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 123}.Build(),
						},
					}.Build(),
				},
			}.Build(),
		}.Build(),
	}

	tests := []struct {
		name           string
		rollout        *autoupdatev1pb.AutoUpdateAgentRollout
		groupNames     []string
		desiredState   autoupdatev1pb.AutoUpdateAgentGroupState
		reports        []*autoupdatev1pb.AutoUpdateAgentReport
		expectedStatus *autoupdatev1pb.AutoUpdateAgentRolloutStatus
		expectErr      require.ErrorAssertionFunc
	}{
		{
			name: "valid transition",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec:   spec,
				Status: proto.CloneOf(status),
			}.Build(),
			groupNames:   []string{"blue", "prod", "backup"},
			desiredState: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			expectErr:    require.NoError,
			expectedStatus: autoupdatev1pb.AutoUpdateAgentRolloutStatus_builder{
				Groups: []*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup{
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:             "blue",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
						StartTime:        nowPb,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonManualTrigger,
					}.Build(),
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:  "dev",
						State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					}.Build(),
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:  "stage",
						State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					}.Build(),
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:             "prod",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
						StartTime:        nowPb,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonManualTrigger,
					}.Build(),
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:             "backup",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
						StartTime:        nowPb,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonManualTrigger,
					}.Build(),
				},
			}.Build(),
		},
		{
			name: "valid transition, with reports",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec:   spec,
				Status: proto.CloneOf(status),
			}.Build(),
			groupNames:   []string{"blue", "prod", "backup"},
			desiredState: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			reports:      testReports,
			expectErr:    require.NoError,
			expectedStatus: autoupdatev1pb.AutoUpdateAgentRolloutStatus_builder{
				Groups: []*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup{
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:             "blue",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
						StartTime:        nowPb,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonManualTrigger,
						// The group transitioned, the count must be set
						InitialCount:  10,
						PresentCount:  10,
						UpToDateCount: 5,
					}.Build(),
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:  "dev",
						State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					}.Build(),
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:  "stage",
						State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
					}.Build(),
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:             "prod",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
						StartTime:        nowPb,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonManualTrigger,
					}.Build(),
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:             "backup",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
						StartTime:        nowPb,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonManualTrigger,
					}.Build(),
				},
			}.Build(),
		},
		{
			name: "no groups in rollout",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec:   spec,
				Status: &autoupdatev1pb.AutoUpdateAgentRolloutStatus{},
			}.Build(),
			groupNames:   []string{"prod", "backup"},
			desiredState: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			expectErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "rollout has no groups")
			},
		},
		{
			name: "unsupported desired state",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec:   spec,
				Status: proto.CloneOf(status),
			}.Build(),
			groupNames:   []string{"prod", "backup"},
			desiredState: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
			expectErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "unsupported desired state")
			},
		},
		{
			name: "unsupported strategy",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec: autoupdatev1pb.AutoUpdateAgentRolloutSpec_builder{
					StartVersion:   startVersion,
					TargetVersion:  targetVersion,
					Schedule:       autoupdate.AgentsScheduleRegular,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
					Strategy:       autoupdate.AgentsStrategyTimeBased,
				}.Build(),
				Status: proto.CloneOf(status),
			}.Build(),
			groupNames:   []string{"prod", "backup"},
			desiredState: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			expectErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "not supported for rollout strategy")
			},
		},
		{
			name: "unsupported schedule",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec: autoupdatev1pb.AutoUpdateAgentRolloutSpec_builder{
					StartVersion:   startVersion,
					TargetVersion:  targetVersion,
					Schedule:       autoupdate.AgentsScheduleImmediate,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
				}.Build(),
				Status: proto.CloneOf(status),
			}.Build(),
			groupNames:   []string{"prod", "backup"},
			desiredState: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			expectErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "rollout schedule is immediate")
			},
		},
		{
			name: "group already active",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec:   spec,
				Status: status,
			}.Build(),
			groupNames:   []string{"stage"},
			desiredState: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			expectErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "is already active")
			},
		},
		{
			name: "group already done",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec:   spec,
				Status: status,
			}.Build(),
			groupNames:   []string{"dev"},
			desiredState: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			expectErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "is already done")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TriggerGroups(tt.rollout, tt.reports, GroupListToGroupSet(tt.groupNames), tt.desiredState, now)
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
	spec := autoupdatev1pb.AutoUpdateAgentRolloutSpec_builder{
		StartVersion:   "1.2.3",
		TargetVersion:  "1.2.4",
		Schedule:       autoupdate.AgentsScheduleRegular,
		AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
		Strategy:       autoupdate.AgentsStrategyHaltOnError,
	}.Build()
	status := autoupdatev1pb.AutoUpdateAgentRolloutStatus_builder{
		Groups: []*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup{
			autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
				Name:  "blue",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
			}.Build(),
			autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
				Name:  "dev",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
			}.Build(),
			autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
				Name:  "stage",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			}.Build(),
			autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
				Name:  "prod",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
			}.Build(),
			autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
				Name:  "backup",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSPECIFIED,
			}.Build(),
		},
	}.Build()

	tests := []struct {
		name           string
		rollout        *autoupdatev1pb.AutoUpdateAgentRollout
		groupNames     []string
		expectedStatus *autoupdatev1pb.AutoUpdateAgentRolloutStatus
		expectErr      require.ErrorAssertionFunc
	}{
		{
			name: "valid transition",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec:   spec,
				Status: status,
			}.Build(),
			groupNames: []string{"blue", "stage", "prod", "backup"},
			expectErr:  require.NoError,
			expectedStatus: autoupdatev1pb.AutoUpdateAgentRolloutStatus_builder{
				Groups: []*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup{
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:             "blue",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonForcedDone,
					}.Build(),
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:  "dev",
						State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
					}.Build(),
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:             "stage",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonForcedDone,
					}.Build(),
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:             "prod",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonForcedDone,
					}.Build(),
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:             "backup",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonForcedDone,
					}.Build(),
				},
			}.Build(),
		},
		{
			name: "no groups in rollout",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec:   spec,
				Status: &autoupdatev1pb.AutoUpdateAgentRolloutStatus{},
			}.Build(),
			groupNames: []string{"prod", "backup"},
			expectErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "rollout has no groups")
			},
		},
		{
			name: "unsupported strategy",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec: autoupdatev1pb.AutoUpdateAgentRolloutSpec_builder{
					StartVersion:   "1.2.3",
					TargetVersion:  "1.2.4",
					Schedule:       autoupdate.AgentsScheduleRegular,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
					Strategy:       autoupdate.AgentsStrategyTimeBased,
				}.Build(),
				Status: status,
			}.Build(),
			groupNames: []string{"prod", "backup"},
			expectErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "not supported for rollout strategy")
			},
		},
		{
			name: "unsupported schedule",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec: autoupdatev1pb.AutoUpdateAgentRolloutSpec_builder{
					StartVersion:   "1.2.3",
					TargetVersion:  "1.2.4",
					Schedule:       autoupdate.AgentsScheduleImmediate,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
				}.Build(),
				Status: nil,
			}.Build(),
			groupNames: []string{"prod", "backup"},
			expectErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "rollout schedule is immediate")
			},
		},
		{
			name: "group already done",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec:   spec,
				Status: status,
			}.Build(),
			groupNames: []string{"dev"},
			expectErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "is already done")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ForceGroupsDone(tt.rollout, GroupListToGroupSet(tt.groupNames), now)
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
	spec := autoupdatev1pb.AutoUpdateAgentRolloutSpec_builder{
		StartVersion:   "1.2.3",
		TargetVersion:  "1.2.4",
		Schedule:       autoupdate.AgentsScheduleRegular,
		AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
		Strategy:       autoupdate.AgentsStrategyHaltOnError,
	}.Build()
	status := autoupdatev1pb.AutoUpdateAgentRolloutStatus_builder{
		Groups: []*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup{
			autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
				Name:  "blue",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
			}.Build(),
			autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
				Name:  "dev",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
			}.Build(),
			autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
				Name:  "stage",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
			}.Build(),
			autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
				Name:  "prod",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
			}.Build(),
			autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
				Name:  "backup",
				State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSPECIFIED,
			}.Build(),
		},
	}.Build()

	tests := []struct {
		name           string
		rollout        *autoupdatev1pb.AutoUpdateAgentRollout
		groupNames     []string
		expectedStatus *autoupdatev1pb.AutoUpdateAgentRolloutStatus
		expectErr      require.ErrorAssertionFunc
	}{
		{
			name: "valid transition",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec:   spec,
				Status: status,
			}.Build(),
			groupNames: []string{"dev", "stage", "prod", "backup"},
			expectErr:  require.NoError,
			expectedStatus: autoupdatev1pb.AutoUpdateAgentRolloutStatus_builder{
				Groups: []*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup{
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:  "blue",
						State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
					}.Build(),
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:             "dev",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonManualRollback,
					}.Build(),
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:             "stage",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonManualRollback,
					}.Build(),
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:             "prod",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonManualRollback,
					}.Build(),
					autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
						Name:             "backup",
						State:            autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
						LastUpdateTime:   nowPb,
						LastUpdateReason: updateReasonManualRollback,
					}.Build(),
				},
			}.Build(),
		},
		{
			name: "no groups in rollout",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec:   spec,
				Status: &autoupdatev1pb.AutoUpdateAgentRolloutStatus{},
			}.Build(),
			groupNames: []string{"prod", "backup"},
			expectErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "rollout has no groups")
			},
		},
		{
			name: "unsupported strategy",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec: autoupdatev1pb.AutoUpdateAgentRolloutSpec_builder{
					StartVersion:   "1.2.3",
					TargetVersion:  "1.2.4",
					Schedule:       autoupdate.AgentsScheduleRegular,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
					Strategy:       autoupdate.AgentsStrategyTimeBased,
				}.Build(),
				Status: status,
			}.Build(),
			groupNames: []string{"prod", "backup"},
			expectErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "not supported for rollout strategy")
			},
		},
		{
			name: "unsupported schedule",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec: autoupdatev1pb.AutoUpdateAgentRolloutSpec_builder{
					StartVersion:   "1.2.3",
					TargetVersion:  "1.2.4",
					Schedule:       autoupdate.AgentsScheduleImmediate,
					AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
				}.Build(),
				Status: nil,
			}.Build(),
			groupNames: []string{"prod", "backup"},
			expectErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "rollout schedule is immediate")
			},
		},
		{
			name: "group already rolledback",
			rollout: autoupdatev1pb.AutoUpdateAgentRollout_builder{
				Spec:   spec,
				Status: status,
			}.Build(),
			groupNames: []string{"blue"},
			expectErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "is already in a rolled-back state")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RollbackGroups(tt.rollout, GroupListToGroupSet(tt.groupNames), now)
			tt.expectErr(t, err)

			if err == nil {
				require.Empty(t, cmp.Diff(tt.expectedStatus, tt.rollout.GetStatus(), protocmp.Transform()))
			}
		})
	}
}

func TestStartedGroups(t *testing.T) {
	rollout := autoupdatev1pb.AutoUpdateAgentRollout_builder{
		Spec: autoupdatev1pb.AutoUpdateAgentRolloutSpec_builder{
			StartVersion:   "1.2.3",
			TargetVersion:  "1.2.4",
			Schedule:       autoupdate.AgentsScheduleRegular,
			AutoupdateMode: autoupdate.AgentsUpdateModeEnabled,
			Strategy:       autoupdate.AgentsStrategyHaltOnError,
		}.Build(),
		Status: autoupdatev1pb.AutoUpdateAgentRolloutStatus_builder{
			Groups: []*autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup{
				autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:  "blue",
					State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK,
				}.Build(),
				autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:  "dev",
					State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE,
				}.Build(),
				autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:  "stage",
					State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE,
				}.Build(),
				autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:  "prod",
					State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED,
				}.Build(),
				autoupdatev1pb.AutoUpdateAgentRolloutStatusGroup_builder{
					Name:  "backup",
					State: autoupdatev1pb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSPECIFIED,
				}.Build(),
			},
		}.Build(),
	}.Build()

	expectedGroups := GroupListToGroupSet([]string{"dev", "stage"})
	result := GetStartedGroups(rollout)

	require.Equal(t, expectedGroups, result)
}
