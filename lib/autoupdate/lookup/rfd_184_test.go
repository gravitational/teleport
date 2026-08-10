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

package lookup

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	autoupdatepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
)

func TestGetVersionFromRollout(t *testing.T) {
	t.Parallel()
	groupName := "test-group"

	// This test matrix is written based on:
	// https://github.com/gravitational/teleport/blob/master/rfd/0184-agent-auto-updates.md#rollout-status-disabled
	latestAllTheTime := map[autoupdatepb.AutoUpdateAgentGroupState]string{
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:  testVersionHigh,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:       testVersionHigh,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:     testVersionHigh,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK: testVersionHigh,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY:     testVersionHigh,
	}

	activeDoneOnly := map[autoupdatepb.AutoUpdateAgentGroupState]string{
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:  testVersionLow,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:       testVersionHigh,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:     testVersionHigh,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK: testVersionLow,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY:     testVersionLow,
	}

	activeCanaryDone := map[autoupdatepb.AutoUpdateAgentGroupState]string{
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:  testVersionLow,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:       testVersionHigh,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:     testVersionHigh,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK: testVersionLow,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY:     testVersionHigh,
	}

	tests := map[string]map[string]map[autoupdatepb.AutoUpdateAgentGroupState]string{
		autoupdate.AgentsUpdateModeDisabled: {
			autoupdate.AgentsScheduleImmediate: latestAllTheTime,
			autoupdate.AgentsScheduleRegular:   latestAllTheTime,
		},
		autoupdate.AgentsUpdateModeSuspended: {
			autoupdate.AgentsScheduleImmediate: latestAllTheTime,
			autoupdate.AgentsScheduleRegular:   activeDoneOnly,
		},
		autoupdate.AgentsUpdateModeEnabled: {
			autoupdate.AgentsScheduleImmediate: latestAllTheTime,
			autoupdate.AgentsScheduleRegular:   activeDoneOnly,
		},
	}
	for mode, scheduleCases := range tests {
		for schedule, stateCases := range scheduleCases {
			for state, expectedVersion := range stateCases {
				t.Run(fmt.Sprintf("%s/%s/%s", mode, schedule, state), func(t *testing.T) {
					expectedSemVersion, err := version.EnsureSemver(expectedVersion)
					require.NoError(t, err)
					rollout := &autoupdatepb.AutoUpdateAgentRollout{
						Spec: &autoupdatepb.AutoUpdateAgentRolloutSpec{
							StartVersion:   testVersionLow,
							TargetVersion:  testVersionHigh,
							Schedule:       schedule,
							AutoupdateMode: mode,
							// Strategy does not affect which version are served
							Strategy: autoupdate.AgentsStrategyTimeBased,
						},
						Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
							Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
								{
									Name:  groupName,
									State: state,
								},
							},
						},
					}
					version, err := getVersionFromRollout(rollout, groupName, "")
					require.NoError(t, err)
					require.Equal(t, expectedSemVersion, version)
				})
			}
		}
	}

	canaryTestCases := map[bool]map[autoupdatepb.AutoUpdateAgentGroupState]string{
		true:  activeCanaryDone,
		false: activeDoneOnly,
	}

	for canaryMatching, statesCases := range canaryTestCases {
		const (
			schedule = autoupdate.AgentsScheduleRegular
			mode     = autoupdate.AgentsUpdateModeEnabled
		)

		for state, expectedVersion := range statesCases {
			t.Run(fmt.Sprintf("canary(%s)/%s", strconv.FormatBool(canaryMatching), state), func(t *testing.T) {
				expectedSemVersion, err := version.EnsureSemver(expectedVersion)
				require.NoError(t, err)

				rollout := &autoupdatepb.AutoUpdateAgentRollout{
					Spec: &autoupdatepb.AutoUpdateAgentRolloutSpec{
						StartVersion:   testVersionLow,
						TargetVersion:  testVersionHigh,
						Schedule:       schedule,
						AutoupdateMode: mode,
						// Strategy does not affect which version are served
						Strategy: autoupdate.AgentsStrategyTimeBased,
					},
					Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
						Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
							{
								Name:  groupName,
								State: state,
								Canaries: []*autoupdatepb.Canary{
									{
										UpdaterId: uuid.NewString(),
										HostId:    uuid.NewString(),
										Hostname:  "test-host",
										Success:   false,
									},
								},
							},
						},
					},
				}
				var updaterID string
				if canaryMatching {
					updaterID = rollout.GetStatus().GetGroups()[0].GetCanaries()[0].GetUpdaterId()
				} else {
					updaterID = uuid.NewString()
				}
				version, err := getVersionFromRollout(rollout, groupName, updaterID)
				require.NoError(t, err)
				require.Equal(t, expectedSemVersion, version)
			})

		}

	}
}

func TestGetTriggerFromRollout(t *testing.T) {
	t.Parallel()
	groupName := "test-group"

	// This test matrix is written based on:
	// https://github.com/gravitational/teleport/blob/master/rfd/0184-agent-auto-updates.md#rollout-status-disabled
	neverUpdate := map[autoupdatepb.AutoUpdateAgentGroupState]bool{
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:  false,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:       false,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:     false,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK: false,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY:     false,
	}
	alwaysUpdate := map[autoupdatepb.AutoUpdateAgentGroupState]bool{
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:  true,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:       true,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:     true,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK: true,
		autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY:     true,
	}

	tests := map[string]map[string]map[string]map[autoupdatepb.AutoUpdateAgentGroupState]bool{
		autoupdate.AgentsUpdateModeDisabled: {
			autoupdate.AgentsStrategyTimeBased: {
				autoupdate.AgentsScheduleImmediate: neverUpdate,
				autoupdate.AgentsScheduleRegular:   neverUpdate,
			},
			autoupdate.AgentsStrategyHaltOnError: {
				autoupdate.AgentsScheduleImmediate: neverUpdate,
				autoupdate.AgentsScheduleRegular:   neverUpdate,
			},
		},
		autoupdate.AgentsUpdateModeSuspended: {
			autoupdate.AgentsStrategyTimeBased: {
				autoupdate.AgentsScheduleImmediate: neverUpdate,
				autoupdate.AgentsScheduleRegular:   neverUpdate,
			},
			autoupdate.AgentsStrategyHaltOnError: {
				autoupdate.AgentsScheduleImmediate: neverUpdate,
				autoupdate.AgentsScheduleRegular:   neverUpdate,
			},
		},
		autoupdate.AgentsUpdateModeEnabled: {
			autoupdate.AgentsStrategyTimeBased: {
				autoupdate.AgentsScheduleImmediate: alwaysUpdate,
				autoupdate.AgentsScheduleRegular: {
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:  false,
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:       false,
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:     true,
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK: true,
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY:     false,
				},
			},
			autoupdate.AgentsStrategyHaltOnError: {
				autoupdate.AgentsScheduleImmediate: alwaysUpdate,
				autoupdate.AgentsScheduleRegular: {
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:  false,
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:       true,
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:     true,
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK: true,
					autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY:     false,
				},
			},
		},
	}
	for mode, strategyCases := range tests {
		for strategy, scheduleCases := range strategyCases {
			for schedule, stateCases := range scheduleCases {
				for state, expectedTrigger := range stateCases {
					t.Run(fmt.Sprintf("%s/%s/%s/%s", mode, strategy, schedule, state), func(t *testing.T) {
						rollout := &autoupdatepb.AutoUpdateAgentRollout{
							Spec: &autoupdatepb.AutoUpdateAgentRolloutSpec{
								StartVersion:   testVersionLow,
								TargetVersion:  testVersionHigh,
								Schedule:       schedule,
								AutoupdateMode: mode,
								Strategy:       strategy,
							},
							Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
								Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
									{
										Name:  groupName,
										State: state,
									},
								},
							},
						}
						shouldUpdate, err := getTriggerFromRollout(rollout, groupName, "")
						require.NoError(t, err)
						require.Equal(t, expectedTrigger, shouldUpdate)
					})
				}
			}
		}
	}

	canaryTestCases := map[bool]map[string]map[autoupdatepb.AutoUpdateAgentGroupState]bool{
		true: {
			autoupdate.AgentsStrategyTimeBased: {
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:  false,
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:       false,
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:     true,
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK: true,
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY:     true,
			},
			autoupdate.AgentsStrategyHaltOnError: {
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:  false,
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:       true,
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:     true,
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK: true,
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY:     true,
			},
		},
		false: {
			autoupdate.AgentsStrategyTimeBased: {
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:  false,
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:       false,
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:     true,
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK: true,
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY:     false,
			},
			autoupdate.AgentsStrategyHaltOnError: {
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_UNSTARTED:  false,
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_DONE:       true,
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ACTIVE:     true,
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_ROLLEDBACK: true,
				autoupdatepb.AutoUpdateAgentGroupState_AUTO_UPDATE_AGENT_GROUP_STATE_CANARY:     false,
			},
		},
	}

	for canaryMatching, strategyCases := range canaryTestCases {
		const (
			schedule = autoupdate.AgentsScheduleRegular
			mode     = autoupdate.AgentsUpdateModeEnabled
		)

		for strategy, statesCases := range strategyCases {
			for state, expectedTrigger := range statesCases {
				t.Run(fmt.Sprintf("canary(%s)/%s/%s", strconv.FormatBool(canaryMatching), strategy, state), func(t *testing.T) {
					rollout := &autoupdatepb.AutoUpdateAgentRollout{
						Spec: &autoupdatepb.AutoUpdateAgentRolloutSpec{
							StartVersion:   testVersionLow,
							TargetVersion:  testVersionHigh,
							Schedule:       schedule,
							AutoupdateMode: mode,
							Strategy:       strategy,
						},
						Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
							Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
								{
									Name:  groupName,
									State: state,
									Canaries: []*autoupdatepb.Canary{
										{
											UpdaterId: uuid.NewString(),
											HostId:    uuid.NewString(),
											Hostname:  "test-host",
											Success:   false,
										},
									},
								},
							},
						},
					}
					var updaterID string
					if canaryMatching {
						updaterID = rollout.GetStatus().GetGroups()[0].GetCanaries()[0].GetUpdaterId()
					} else {
						updaterID = uuid.NewString()
					}
					trigger, err := getTriggerFromRollout(rollout, groupName, updaterID)
					require.NoError(t, err)
					require.Equal(t, expectedTrigger, trigger)
				})

			}

		}

	}
}

func TestGetGroup(t *testing.T) {
	groupName := "test-group"
	t.Parallel()
	tests := []struct {
		name           string
		rollout        *autoupdatepb.AutoUpdateAgentRollout
		expectedResult *autoupdatepb.AutoUpdateAgentRolloutStatusGroup
		expectError    require.ErrorAssertionFunc
	}{
		{
			name:        "nil",
			expectError: require.Error,
		},
		{
			name:        "nil status",
			rollout:     &autoupdatepb.AutoUpdateAgentRollout{},
			expectError: require.Error,
		},
		{
			name:        "nil status groups",
			rollout:     &autoupdatepb.AutoUpdateAgentRollout{Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{}},
			expectError: require.Error,
		},
		{
			name: "empty status groups",
			rollout: &autoupdatepb.AutoUpdateAgentRollout{
				Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{},
				},
			},
			expectError: require.Error,
		},
		{
			name: "group matching name",
			rollout: &autoupdatepb.AutoUpdateAgentRollout{
				Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
						{Name: "foo", State: 1},
						{Name: "bar", State: 1},
						{Name: groupName, State: 2},
						{Name: "baz", State: 1},
					},
				},
			},
			expectedResult: &autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
				Name:  groupName,
				State: 2,
			},
			expectError: require.NoError,
		},
		{
			name: "no group matching name, should fallback to default",
			rollout: &autoupdatepb.AutoUpdateAgentRollout{
				Status: &autoupdatepb.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
						{Name: "foo", State: 1},
						{Name: "bar", State: 1},
						{Name: "baz", State: 1},
					},
				},
			},
			expectedResult: &autoupdatepb.AutoUpdateAgentRolloutStatusGroup{
				Name:  "baz",
				State: 1,
			},
			expectError: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getGroup(tt.rollout, groupName)
			tt.expectError(t, err)
			require.Equal(t, tt.expectedResult, result)
		})
	}
}
