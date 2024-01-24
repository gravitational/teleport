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

package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/gravitational/teleport/api/types"
)

func TestCheckOwnership(t *testing.T) {
	tests := []struct {
		name                    string
		existingResource        types.Resource
		expectedConditionStatus metav1.ConditionStatus
		expectedConditionReason string
		isOwned                 bool
	}{
		{
			name: "existing owned resource",
			existingResource: &types.UserV2{
				Metadata: types.Metadata{
					Name:   "existing owned user",
					Labels: map[string]string{types.OriginLabel: types.OriginKubernetes},
				},
			},
			expectedConditionStatus: metav1.ConditionTrue,
			expectedConditionReason: ConditionReasonOriginLabelMatching,
			isOwned:                 true,
		},
		{
			name: "existing unowned resource (no label)",
			existingResource: &types.UserV2{
				Metadata: types.Metadata{
					Name: "existing unowned user without label",
				},
			},
			expectedConditionStatus: metav1.ConditionFalse,
			expectedConditionReason: ConditionReasonOriginLabelNotMatching,
			isOwned:                 false,
		},
		{
			name: "existing unowned resource (bad origin)",
			existingResource: &types.UserV2{
				Metadata: types.Metadata{
					Name:   "existing owned user without origin label",
					Labels: map[string]string{types.OriginLabel: types.OriginConfigFile},
				},
			},
			expectedConditionStatus: metav1.ConditionFalse,
			expectedConditionReason: ConditionReasonOriginLabelNotMatching,
			isOwned:                 false,
		},
		{
			name: "existing unowned resource (no origin)",
			existingResource: &types.UserV2{
				Metadata: types.Metadata{
					Name:   "existing owned user without origin label",
					Labels: map[string]string{"foo": "bar"},
				},
			},
			expectedConditionStatus: metav1.ConditionFalse,
			expectedConditionReason: ConditionReasonOriginLabelNotMatching,
			isOwned:                 false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			condition, isOwned := checkOwnership(tc.existingResource)

			require.Equal(t, tc.isOwned, isOwned)
			require.Equal(t, ConditionTypeTeleportResourceOwned, condition.Type)
			require.Equal(t, tc.expectedConditionStatus, condition.Status)
			require.Equal(t, tc.expectedConditionReason, condition.Reason)
		})
	}
}

func TestCheckAnnotationFlag(t *testing.T) {
	testFlag := "foo"
	tests := []struct {
		name           string
		annotations    map[string]string
		defaultValue   bool
		expectedOutput bool
	}{
		{
			name:           "flag set true, default true",
			annotations:    map[string]string{testFlag: "true"},
			defaultValue:   true,
			expectedOutput: true,
		},
		{
			name:           "flag set false, default true",
			annotations:    map[string]string{testFlag: "false"},
			defaultValue:   true,
			expectedOutput: false,
		},
		{
			name:           "flag set true, default false",
			annotations:    map[string]string{testFlag: "true"},
			defaultValue:   false,
			expectedOutput: true,
		},
		{
			name:           "flag set false, default false",
			annotations:    map[string]string{testFlag: "false"},
			defaultValue:   false,
			expectedOutput: false,
		},
		{
			name:           "flag missing, default true",
			annotations:    map[string]string{},
			defaultValue:   true,
			expectedOutput: true,
		},
		{
			name:           "flag missing, default false",
			annotations:    map[string]string{},
			defaultValue:   false,
			expectedOutput: false,
		},
		{
			name:           "flag malformed, default true",
			annotations:    map[string]string{testFlag: "malformed"},
			defaultValue:   true,
			expectedOutput: true,
		},
		{
			name:           "flag malformed, default false",
			annotations:    map[string]string{testFlag: "malformed"},
			defaultValue:   false,
			expectedOutput: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			obj := &unstructured.Unstructured{}
			obj.SetAnnotations(tt.annotations)
			require.Equal(t, tt.expectedOutput, checkAnnotationFlag(obj, testFlag, tt.defaultValue))
		})
	}
}
