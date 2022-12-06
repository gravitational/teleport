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

func TestMakeLabels(t *testing.T) {
	type testCase struct {
		name      string
		labelMaps []map[string]string
		expected  []Label
	}

	testCases := []testCase{
		{
			name: "Single map single label case",
			labelMaps: []map[string]string{
				{
					"label1": "value1",
				},
			},
			expected: []Label{
				{
					Name:  "label1",
					Value: "value1",
				},
			},
		},
		{
			name: "Single map multiple labels case",
			labelMaps: []map[string]string{
				{
					"label1": "value1",
					"label2": "value2",
				},
			},
			expected: []Label{
				{
					Name:  "label1",
					Value: "value1",
				},
				{
					Name:  "label2",
					Value: "value2",
				},
			},
		},
		{
			name: "Multiple maps single label case",
			labelMaps: []map[string]string{
				{
					"label1": "value1",
				},
				{
					"label2": "value2",
				},
			},
			expected: []Label{
				{
					Name:  "label1",
					Value: "value1",
				},
				{
					Name:  "label2",
					Value: "value2",
				},
			},
		},
		{
			name: "Multiple maps with internal labels",
			labelMaps: []map[string]string{
				{
					"label1":                   "value1",
					"teleport.internal/label3": "value3",
				},
				{
					"label2":                   "value2",
					"teleport.internal/label4": "value4",
				},
			},
			expected: []Label{
				{
					Name:  "label1",
					Value: "value1",
				},
				{
					Name:  "label2",
					Value: "value2",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			labels := makeLabels(tc.labelMaps...)

			require.Equal(t, tc.expected, labels)
		})
	}
}

func TestTransformCommandLabels(t *testing.T) {
	commandLabels := map[string]types.CommandLabel{
		"label1": &types.CommandLabelV2{
			Result: "value1",
		},
	}
	labels := transformCommandLabels(commandLabels)
	expected := map[string]string{
		"label1": "value1",
	}

	require.Equal(t, expected, labels)
}
