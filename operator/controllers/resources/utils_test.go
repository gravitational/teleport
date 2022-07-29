/*
Copyright 2022 Gravitational, Inc.

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

package resources

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCheckOwnership(t *testing.T) {
	type check func(t2 *testing.T, err error)

	hasNoErr := func() check {
		return func(t2 *testing.T, err error) {
			require.NoError(t2, err)
		}
	}

	hasAlreadyExistsErr := func() check {
		return func(t2 *testing.T, err error) {
			require.IsType(t2, &trace.AlreadyExistsError{}, err.(*trace.TraceErr).OrigError())
		}
	}

	tests := []struct {
		name                    string
		existingResource        types.Resource
		expectedConditionStatus metav1.ConditionStatus
		expectedConditionReason string
		check                   check
	}{
		{
			name:                    "new resource",
			existingResource:        nil,
			expectedConditionStatus: metav1.ConditionTrue,
			expectedConditionReason: ConditionReasonNewResource,
			check:                   hasNoErr(),
		},
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
			check:                   hasNoErr(),
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
			check:                   hasAlreadyExistsErr(),
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
			check:                   hasAlreadyExistsErr(),
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
			check:                   hasAlreadyExistsErr(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			condition, err := checkOwnership(tc.existingResource)

			tc.check(t, err)
			require.Equal(t, condition.Type, ConditionTypeTeleportResourceOwned)
			require.Equal(t, condition.Status, tc.expectedConditionStatus)
			require.Equal(t, condition.Reason, tc.expectedConditionReason)
		})
	}
}
