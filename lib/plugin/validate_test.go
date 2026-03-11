// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plugin

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func Test_validateOkta(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name         string
		oktaSettings *types.PluginOktaSettings
		errMatcher   func(error) bool
		errContains  string
	}

	for _, tt := range []testCase{
		{
			name: "malformed TimeBetweenImports",
			oktaSettings: &types.PluginOktaSettings{
				SyncSettings: &types.PluginOktaSyncSettings{
					TimeBetweenImports: "not_a_duration",
				},
			},
			errMatcher:  trace.IsBadParameter,
			errContains: "time_between_imports is not valid",
		},
		{
			name: "malformed TimeBetweenAssignmentProcessLoops",
			oktaSettings: &types.PluginOktaSettings{
				SyncSettings: &types.PluginOktaSyncSettings{
					TimeBetweenAssignmentProcessLoops: "not_a_duration",
				},
			},
			errMatcher:  trace.IsBadParameter,
			errContains: "time_between_assignment_process_loops is not valid",
		},
		// TODO(kopiczko): add the check when integration tests are fixed.
		//{
		//	name: "TimeBetweenAssignmentProcessLoops longer than TimeBetweenImports",
		//	oktaSettings: &types.PluginOktaSettings{
		//		SyncSettings: &types.PluginOktaSyncSettings{
		//			TimeBetweenImports:                "1m",
		//			TimeBetweenAssignmentProcessLoops: "1m6s",
		//		},
		//	},
		//	errMatcher:  trace.IsBadParameter,
		//	errContains: "time_between_assignment_process_loops cannot be longer than time_between_imports",
		//},
		//{
		//	name: "TimeBetweenAssignmentProcessLoops longer than implicit TimeBetweenImports",
		//	oktaSettings: &types.PluginOktaSettings{
		//		SyncSettings: &types.PluginOktaSyncSettings{
		//			// TimeBetweenImports is 30m by default
		//			TimeBetweenAssignmentProcessLoops: "30m1s",
		//		},
		//	},
		//	errMatcher:  trace.IsBadParameter,
		//	errContains: "time_between_assignment_process_loops cannot be longer than time_between_imports",
		//},
	} {
		t.Run(tt.name, func(t *testing.T) {
			plugin := newTestOktaPlugin(tt.oktaSettings)
			err := validateOkta(plugin)
			if tt.errMatcher == nil && tt.errContains == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.errContains)
			require.True(t, tt.errMatcher(err))
		})
	}
}

func newTestOktaPlugin(oktaSettings *types.PluginOktaSettings) *types.PluginV1 {
	return types.NewPluginV1(
		types.Metadata{
			Name: types.PluginTypeOkta,
		},
		types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{
				Okta: oktaSettings,
			},
		},
		nil,
	)
}
