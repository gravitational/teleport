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

// TestNewAutoUpdateVersion verifies validation for AutoUpdateVersion resource.
func TestNewAutoUpdateVersion(t *testing.T) {
	tests := []struct {
		name      string
		spec      *autoupdate.AutoUpdateVersionSpec
		want      *autoupdate.AutoUpdateVersion
		assertErr func(*testing.T, error, ...any)
	}{
		{
			name: "success tools autoupdate version",
			spec: &autoupdate.AutoUpdateVersionSpec{
				Tools: &autoupdate.AutoUpdateVersionSpecTools{
					TargetVersion: "1.2.3-dev",
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.NoError(t, err)
			},
			want: &autoupdate.AutoUpdateVersion{
				Kind:    types.KindAutoUpdateVersion,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateVersion,
				},
				Spec: &autoupdate.AutoUpdateVersionSpec{
					Tools: &autoupdate.AutoUpdateVersionSpecTools{
						TargetVersion: "1.2.3-dev",
					},
				},
			},
		},
		{
			name: "invalid empty tools version",
			spec: &autoupdate.AutoUpdateVersionSpec{
				Tools: &autoupdate.AutoUpdateVersionSpecTools{
					TargetVersion: "",
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "target_version\n\tversion is unset")
			},
		},
		{
			name: "invalid semantic tools version",
			spec: &autoupdate.AutoUpdateVersionSpec{
				Tools: &autoupdate.AutoUpdateVersionSpecTools{
					TargetVersion: "17-0-0",
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "target_version\n\tversion \"17-0-0\" is not a valid semantic version")
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
			name: "success agents autoupdate version",
			spec: &autoupdate.AutoUpdateVersionSpec{
				Agents: &autoupdate.AutoUpdateVersionSpecAgents{
					StartVersion:  "1.2.3-dev.1",
					TargetVersion: "1.2.3-dev.2",
					Schedule:      AgentsScheduleImmediate,
					Mode:          AgentsUpdateModeEnabled,
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.NoError(t, err)
			},
			want: &autoupdate.AutoUpdateVersion{
				Kind:    types.KindAutoUpdateVersion,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameAutoUpdateVersion,
				},
				Spec: &autoupdate.AutoUpdateVersionSpec{
					Agents: &autoupdate.AutoUpdateVersionSpecAgents{
						StartVersion:  "1.2.3-dev.1",
						TargetVersion: "1.2.3-dev.2",
						Schedule:      AgentsScheduleImmediate,
						Mode:          AgentsUpdateModeEnabled,
					},
				},
			},
		},
		{
			name: "invalid empty agents start version",
			spec: &autoupdate.AutoUpdateVersionSpec{
				Agents: &autoupdate.AutoUpdateVersionSpecAgents{
					StartVersion:  "",
					TargetVersion: "1.2.3",
					Mode:          AgentsUpdateModeEnabled,
					Schedule:      AgentsScheduleImmediate,
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "start_version\n\tversion is unset")
			},
		},
		{
			name: "invalid empty agents target version",
			spec: &autoupdate.AutoUpdateVersionSpec{
				Agents: &autoupdate.AutoUpdateVersionSpecAgents{
					StartVersion:  "1.2.3-dev",
					TargetVersion: "",
					Mode:          AgentsUpdateModeEnabled,
					Schedule:      AgentsScheduleImmediate,
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "target_version\n\tversion is unset")
			},
		},
		{
			name: "invalid semantic agents start version",
			spec: &autoupdate.AutoUpdateVersionSpec{
				Agents: &autoupdate.AutoUpdateVersionSpecAgents{
					StartVersion:  "17-0-0",
					TargetVersion: "1.2.3",
					Mode:          AgentsUpdateModeEnabled,
					Schedule:      AgentsScheduleImmediate,
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "start_version\n\tversion \"17-0-0\" is not a valid semantic version")
			},
		},
		{
			name: "invalid semantic agents target version",
			spec: &autoupdate.AutoUpdateVersionSpec{
				Agents: &autoupdate.AutoUpdateVersionSpecAgents{
					StartVersion:  "1.2.3",
					TargetVersion: "17-0-0",
					Mode:          AgentsUpdateModeEnabled,
					Schedule:      AgentsScheduleImmediate,
				},
			},
			assertErr: func(t *testing.T, err error, a ...any) {
				require.ErrorContains(t, err, "target_version\n\tversion \"17-0-0\" is not a valid semantic version")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewAutoUpdateVersion(tt.spec)
			tt.assertErr(t, err)
			require.Empty(t, cmp.Diff(got, tt.want, protocmp.Transform()))
		})
	}
}
