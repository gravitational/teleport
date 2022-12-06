/*
Copyright 2020 Gravitational, Inc.

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

package ui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestMakeAppsLabelFilter(t *testing.T) {
	type testCase struct {
		types.Apps
		expected []App
		name     string
	}

	testCases := []testCase{
		{
			name: "Single App with teleport.internal/ label",
			Apps: types.Apps{
				&types.AppV3{
					Metadata: types.Metadata{
						Name: "App1",
						Labels: map[string]string{
							"first":                "value1",
							"teleport.internal/dd": "hidden1",
						},
					},
				},
			},
			expected: []App{
				{
					Name: "App1",
					Labels: []Label{
						{
							Name:  "first",
							Value: "value1",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := MakeAppsConfig{
				Apps: tc.Apps,
			}
			apps := MakeApps(config)

			for i, app := range apps {
				expectedLabels := tc.expected[i].Labels

				require.Equal(t, expectedLabels, app.Labels)
			}
		})
	}
}
