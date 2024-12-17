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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// TestNewAutoUpdateConfig verifies validation for AutoUpdateConfig resource.
func TestNewAutoUpdateConfig(t *testing.T) {
	tests := []struct {
		name      string
		spec      *autoupdate.AutoUpdateConfigSpec
		want      *autoupdate.AutoUpdateConfig
		assertErr func(*testing.T, error, ...any)
	}{
		{
			name: "success tools autoupdate disabled",
			spec: &autoupdate.AutoUpdateConfigSpec{
				Tools: &autoupdate.AutoUpdateConfigSpecTools{
					Mode: ToolsUpdateModeDisabled,
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.NoError(t, err)
			},
			want: &autoupdate.AutoUpdateConfig{
				Kind:    types.KindAutoUpdateConfig,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateConfig,
				},
				Spec: &autoupdate.AutoUpdateConfigSpec{
					Tools: &autoupdate.AutoUpdateConfigSpecTools{
						Mode: ToolsUpdateModeDisabled,
					},
				},
			},
		},
		{
			name: "success tools autoupdate enabled",
			spec: &autoupdate.AutoUpdateConfigSpec{
				Tools: &autoupdate.AutoUpdateConfigSpecTools{
					Mode: ToolsUpdateModeEnabled,
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.NoError(t, err)
			},
			want: &autoupdate.AutoUpdateConfig{
				Kind:    types.KindAutoUpdateConfig,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateConfig,
				},
				Spec: &autoupdate.AutoUpdateConfigSpec{
					Tools: &autoupdate.AutoUpdateConfigSpecTools{
						Mode: ToolsUpdateModeEnabled,
					},
				},
			},
		},
		{
			name: "invalid spec",
			spec: nil,
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "Spec is nil")
			},
		},
		{
			name: "invalid tools mode",
			spec: &autoupdate.AutoUpdateConfigSpec{
				Tools: &autoupdate.AutoUpdateConfigSpecTools{
					Mode: "invalid-mode",
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "unsupported tools mode: \"invalid-mode\"")
			},
		},
		{
			name: "invalid agents mode",
			spec: &autoupdate.AutoUpdateConfigSpec{
				Agents: &autoupdate.AutoUpdateConfigSpecAgents{
					Mode:     "invalid-mode",
					Strategy: AgentsStrategyHaltOnError,
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "unsupported agents mode: \"invalid-mode\"")
			},
		},
		{
			name: "invalid agents strategy",
			spec: &autoupdate.AutoUpdateConfigSpec{
				Agents: &autoupdate.AutoUpdateConfigSpecAgents{
					Mode:     AgentsUpdateModeEnabled,
					Strategy: "invalid-strategy",
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "unsupported agents strategy: \"invalid-strategy\"")
			},
		},
		{
			name: "invalid agents non-nil maintenance window with halt-on-error",
			spec: &autoupdate.AutoUpdateConfigSpec{
				Agents: &autoupdate.AutoUpdateConfigSpecAgents{
					Mode:                      AgentsUpdateModeEnabled,
					Strategy:                  AgentsStrategyHaltOnError,
					MaintenanceWindowDuration: durationpb.New(time.Hour),
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "maintenance_window_duration must be zero")
			},
		},
		{
			name: "invalid agents nil maintenance window with time-based strategy",
			spec: &autoupdate.AutoUpdateConfigSpec{
				Agents: &autoupdate.AutoUpdateConfigSpecAgents{
					Mode:     AgentsUpdateModeEnabled,
					Strategy: AgentsStrategyTimeBased,
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "maintenance_window_duration must be greater than 10 minutes")
			},
		},
		{
			name: "invalid agents short maintenance window",
			spec: &autoupdate.AutoUpdateConfigSpec{
				Agents: &autoupdate.AutoUpdateConfigSpecAgents{
					Mode:                      AgentsUpdateModeEnabled,
					Strategy:                  AgentsStrategyTimeBased,
					MaintenanceWindowDuration: durationpb.New(time.Minute),
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "maintenance_window_duration must be greater than 10 minutes")
			},
		},
		{
			name: "success agents autoupdate halt-on-failure",
			spec: &autoupdate.AutoUpdateConfigSpec{
				Agents: &autoupdate.AutoUpdateConfigSpecAgents{
					Mode:     AgentsUpdateModeEnabled,
					Strategy: AgentsStrategyHaltOnError,
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.NoError(t, err)
			},
			want: &autoupdate.AutoUpdateConfig{
				Kind:    types.KindAutoUpdateConfig,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateConfig,
				},
				Spec: &autoupdate.AutoUpdateConfigSpec{
					Agents: &autoupdate.AutoUpdateConfigSpecAgents{
						Mode:     AgentsUpdateModeEnabled,
						Strategy: AgentsStrategyHaltOnError,
					},
				},
			},
		},
		{
			name: "success agents autoupdate time-based",
			spec: &autoupdate.AutoUpdateConfigSpec{
				Agents: &autoupdate.AutoUpdateConfigSpecAgents{
					Mode:                      AgentsUpdateModeEnabled,
					Strategy:                  AgentsStrategyTimeBased,
					MaintenanceWindowDuration: durationpb.New(time.Hour),
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.NoError(t, err)
			},
			want: &autoupdate.AutoUpdateConfig{
				Kind:    types.KindAutoUpdateConfig,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateConfig,
				},
				Spec: &autoupdate.AutoUpdateConfigSpec{
					Agents: &autoupdate.AutoUpdateConfigSpecAgents{
						Mode:                      AgentsUpdateModeEnabled,
						Strategy:                  AgentsStrategyTimeBased,
						MaintenanceWindowDuration: durationpb.New(time.Hour),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewAutoUpdateConfig(tt.spec)
			tt.assertErr(t, err)
			require.Empty(t, cmp.Diff(got, tt.want, protocmp.Transform()))
		})
	}
}
