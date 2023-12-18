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
