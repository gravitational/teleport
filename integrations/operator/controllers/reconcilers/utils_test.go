/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package reconcilers

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			obj := &unstructured.Unstructured{}
			obj.SetAnnotations(tt.annotations)
			require.Equal(t, tt.expectedOutput, checkAnnotationFlag(obj, testFlag, tt.defaultValue))
		})
	}
}

type fakeKubernetesResource struct {
	client.Object
}

func TestNewKubeResource(t *testing.T) {
	// Test with a value receiver
	resource := newKubeResource[fakeKubernetesResource]()
	require.IsTypef(t, fakeKubernetesResource{}, resource, "Should be of type FakeKubernetesResource")
	require.NotNil(t, resource)

	// Test with a pointer receiver
	resourcePtr := newKubeResource[*fakeKubernetesResource]()
	require.IsTypef(t, &fakeKubernetesResource{}, resourcePtr, "Should be a pointer on FakeKubernetesResourcePtrReceiver")
	require.NotNil(t, resourcePtr)
	require.NotNil(t, *resourcePtr)
}
