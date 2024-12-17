/*
Copyright 2024 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package autoupdate

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// TestNewAutoUpdateConfig verifies validation for AutoUpdateConfig resource.
func TestNewAutoUpdateAgentRollout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		spec      *autoupdate.AutoUpdateAgentRolloutSpec
		want      *autoupdate.AutoUpdateAgentRollout
		assertErr func(*testing.T, error, ...any)
	}{
		{
			name: "success valid rollout",
			spec: &autoupdate.AutoUpdateAgentRolloutSpec{
				StartVersion:   "1.2.3",
				TargetVersion:  "2.3.4-dev",
				Schedule:       AgentsScheduleImmediate,
				AutoupdateMode: AgentsUpdateModeEnabled,
				Strategy:       AgentsStrategyHaltOnError,
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.NoError(t, err)
			},
			want: &autoupdate.AutoUpdateAgentRollout{
				Kind:    types.KindAutoUpdateAgentRollout,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateAgentRollout,
				},
				Spec: &autoupdate.AutoUpdateAgentRolloutSpec{
					StartVersion:   "1.2.3",
					TargetVersion:  "2.3.4-dev",
					Schedule:       AgentsScheduleImmediate,
					AutoupdateMode: AgentsUpdateModeEnabled,
					Strategy:       AgentsStrategyHaltOnError,
				},
			},
		},
		{
			name: "missing spec",
			spec: nil,
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "Spec is nil")
			},
		},
		{
			name: "missing start version",
			spec: &autoupdate.AutoUpdateAgentRolloutSpec{
				TargetVersion:  "2.3.4-dev",
				Schedule:       AgentsScheduleImmediate,
				AutoupdateMode: AgentsUpdateModeEnabled,
				Strategy:       AgentsStrategyHaltOnError,
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "start_version\n\tversion is unset")
			},
		},
		{
			name: "invalid target version",
			spec: &autoupdate.AutoUpdateAgentRolloutSpec{
				StartVersion:   "1.2.3",
				TargetVersion:  "2-3-4",
				Schedule:       AgentsScheduleImmediate,
				AutoupdateMode: AgentsUpdateModeEnabled,
				Strategy:       AgentsStrategyHaltOnError,
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "target_version\n\tversion \"2-3-4\" is not a valid semantic version")
			},
		},
		{
			name: "invalid autoupdate mode",
			spec: &autoupdate.AutoUpdateAgentRolloutSpec{
				StartVersion:   "1.2.3",
				TargetVersion:  "2.3.4-dev",
				Schedule:       AgentsScheduleImmediate,
				AutoupdateMode: "invalid-mode",
				Strategy:       AgentsStrategyHaltOnError,
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "unsupported agents mode: \"invalid-mode\"")
			},
		},
		{
			name: "invalid schedule name",
			spec: &autoupdate.AutoUpdateAgentRolloutSpec{
				StartVersion:   "1.2.3",
				TargetVersion:  "2.3.4-dev",
				Schedule:       "invalid-schedule",
				AutoupdateMode: AgentsUpdateModeEnabled,
				Strategy:       AgentsStrategyHaltOnError,
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "unsupported schedule type: \"invalid-schedule\"")
			},
		},
		{
			name: "invalid strategy",
			spec: &autoupdate.AutoUpdateAgentRolloutSpec{
				StartVersion:   "1.2.3",
				TargetVersion:  "2.3.4-dev",
				Schedule:       AgentsScheduleImmediate,
				AutoupdateMode: AgentsUpdateModeEnabled,
				Strategy:       "invalid-strategy",
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "unsupported agents strategy: \"invalid-strategy\"")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewAutoUpdateAgentRollout(tt.spec)
			tt.assertErr(t, err)
			require.Empty(t, cmp.Diff(got, tt.want, protocmp.Transform()))
		})
	}
}

var (
	timeBasedRolloutSpec = autoupdate.AutoUpdateAgentRolloutSpec{
		StartVersion:   "1.2.3",
		TargetVersion:  "2.3.4-dev",
		Schedule:       AgentsScheduleRegular,
		AutoupdateMode: AgentsUpdateModeEnabled,
		Strategy:       AgentsStrategyTimeBased,
	}
	haltOnErrorRolloutSpec = autoupdate.AutoUpdateAgentRolloutSpec{
		StartVersion:   "1.2.3",
		TargetVersion:  "2.3.4-dev",
		Schedule:       AgentsScheduleRegular,
		AutoupdateMode: AgentsUpdateModeEnabled,
		Strategy:       AgentsStrategyHaltOnError,
	}
)

func TestValidateAutoUpdateAgentRollout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		rollout   *autoupdate.AutoUpdateAgentRollout
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "valid time-based rollout with groups",
			rollout: &autoupdate.AutoUpdateAgentRollout{
				Kind:    types.KindAutoUpdateAgentRollout,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateAgentRollout,
				},
				Spec: &timeBasedRolloutSpec,
				Status: &autoupdate.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
						{Name: "g1", ConfigDays: []string{"*"}},
						{Name: "g2", ConfigDays: []string{"*"}},
					},
				},
			},
			assertErr: require.NoError,
		},
		{
			name: "valid halt-on-error rollout with groups",
			rollout: &autoupdate.AutoUpdateAgentRollout{
				Kind:    types.KindAutoUpdateAgentRollout,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateAgentRollout,
				},
				Spec: &haltOnErrorRolloutSpec,
				Status: &autoupdate.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
						{Name: "g1", ConfigDays: []string{"*"}},
						{Name: "g2", ConfigDays: []string{"*"}, ConfigWaitHours: 1},
					},
				},
			},
			assertErr: require.NoError,
		},
		{
			name: "group with negative wait days",
			rollout: &autoupdate.AutoUpdateAgentRollout{
				Kind:    types.KindAutoUpdateAgentRollout,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateAgentRollout,
				},
				Spec: &haltOnErrorRolloutSpec,
				Status: &autoupdate.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
						{Name: "g1", ConfigDays: []string{"*"}},
						{Name: "g2", ConfigDays: []string{"*"}, ConfigWaitHours: -1},
					},
				},
			},
			assertErr: require.Error,
		},
		{
			name: "group with invalid week days",
			rollout: &autoupdate.AutoUpdateAgentRollout{
				Kind:    types.KindAutoUpdateAgentRollout,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateAgentRollout,
				},
				Spec: &haltOnErrorRolloutSpec,
				Status: &autoupdate.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
						{Name: "g1", ConfigDays: []string{"*"}},
						{Name: "g2", ConfigDays: []string{"frurfday"}, ConfigWaitHours: 1},
					},
				},
			},
			assertErr: require.Error,
		},
		{
			name: "group with no week days",
			rollout: &autoupdate.AutoUpdateAgentRollout{
				Kind:    types.KindAutoUpdateAgentRollout,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateAgentRollout,
				},
				Spec: &haltOnErrorRolloutSpec,
				Status: &autoupdate.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
						{Name: "g1", ConfigDays: []string{"*"}},
						{Name: "g2", ConfigDays: []string{}, ConfigWaitHours: 1},
					},
				},
			},
			assertErr: require.Error,
		},
		{
			name: "group with empty name",
			rollout: &autoupdate.AutoUpdateAgentRollout{
				Kind:    types.KindAutoUpdateAgentRollout,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateAgentRollout,
				},
				Spec: &haltOnErrorRolloutSpec,
				Status: &autoupdate.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
						{Name: "g1", ConfigDays: []string{"*"}},
						{Name: "", ConfigDays: []string{"*"}, ConfigWaitHours: 1},
					},
				},
			},
			assertErr: require.Error,
		},
		{
			name: "first group with non zero wait days",
			rollout: &autoupdate.AutoUpdateAgentRollout{
				Kind:    types.KindAutoUpdateAgentRollout,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateAgentRollout,
				},
				Spec: &haltOnErrorRolloutSpec,
				Status: &autoupdate.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
						{Name: "g1", ConfigDays: []string{"*"}, ConfigWaitHours: 1},
						{Name: "g2", ConfigDays: []string{"*"}},
					},
				},
			},
			assertErr: require.Error,
		},
		{
			name: "group with non zero wait days on a time-based rollout",
			rollout: &autoupdate.AutoUpdateAgentRollout{
				Kind:    types.KindAutoUpdateAgentRollout,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateAgentRollout,
				},
				Spec: &timeBasedRolloutSpec,
				Status: &autoupdate.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
						{Name: "g1", ConfigDays: []string{"*"}},
						{Name: "g2", ConfigDays: []string{"*"}, ConfigWaitHours: 1},
					},
				},
			},
			assertErr: require.Error,
		},
		{
			name: "group with impossible start hour",
			rollout: &autoupdate.AutoUpdateAgentRollout{
				Kind:    types.KindAutoUpdateAgentRollout,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateAgentRollout,
				},
				Spec: &haltOnErrorRolloutSpec,
				Status: &autoupdate.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
						{Name: "g1", ConfigDays: []string{"*"}},
						{Name: "dark hour", ConfigDays: []string{"*"}, ConfigStartHour: 24},
					},
				},
			},
			assertErr: require.Error,
		},
		{
			name: "group with same name",
			rollout: &autoupdate.AutoUpdateAgentRollout{
				Kind:    types.KindAutoUpdateAgentRollout,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateAgentRollout,
				},
				Spec: &haltOnErrorRolloutSpec,
				Status: &autoupdate.AutoUpdateAgentRolloutStatus{
					Groups: []*autoupdate.AutoUpdateAgentRolloutStatusGroup{
						{Name: "g1", ConfigDays: []string{"*"}},
						{Name: "g1", ConfigDays: []string{"*"}},
					},
				},
			},
			assertErr: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAutoUpdateAgentRollout(tt.rollout)
			tt.assertErr(t, err)
		})
	}
}
