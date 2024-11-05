/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
		{
			name: "Multiple maps with hidden labels",
			labelMaps: []map[string]string{
				{
					"label1":                 "value1",
					"teleport.hidden/label3": "value3",
				},
				{
					"label2":                 "value2",
					"teleport.hidden/label4": "value4",
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
			labels := MakeLabelsWithoutInternalPrefixes(tc.labelMaps...)

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
	labels := TransformCommandLabels(commandLabels)
	expected := map[string]string{
		"label1": "value1",
	}

	require.Equal(t, expected, labels)
}
