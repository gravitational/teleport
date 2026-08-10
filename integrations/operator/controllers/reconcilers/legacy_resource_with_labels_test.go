// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package reconcilers

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

type fakeResourceWithLabels struct {
	types.ResourceWithLabels
	labels map[string]string
	scope  string
}

func (r *fakeResourceWithLabels) GetLabel(key string) (string, bool) {
	value, ok := r.labels[key]
	return value, ok
}

func (r *fakeResourceWithLabels) SetStaticLabels(labels map[string]string) {
	r.labels = labels
}

func (r *fakeResourceWithLabels) GetScope() string {
	return r.scope
}

func TestResourceWithLabelsAdapter_CheckOwnership(t *testing.T) {
	tests := []struct {
		name           string
		resource       *fakeResourceWithLabels
		expected       bool
		expectedReason string
	}{
		{
			name:           "no labels",
			resource:       &fakeResourceWithLabels{labels: map[string]string{}},
			expected:       false,
			expectedReason: ownershipIssueMissingOriginLabel,
		},
		{
			name:           "labels but no origin",
			resource:       &fakeResourceWithLabels{labels: map[string]string{"foo": "bar"}},
			expected:       false,
			expectedReason: ownershipIssueMissingOriginLabel,
		},
		{
			name:           "origin label mismatch",
			resource:       &fakeResourceWithLabels{labels: map[string]string{types.OriginLabel: types.OriginDefaults}},
			expected:       false,
			expectedReason: fmt.Sprintf(ownershipIssueMismatchOriginLabel, types.OriginDefaults),
		},
		{
			name:     "origin label match",
			resource: &fakeResourceWithLabels{labels: map[string]string{types.OriginLabel: types.OriginKubernetes}},
			expected: true,
		},
	}

	adapter := ResourceWithLabelsAdapter[*fakeResourceWithLabels]{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owned, reason := adapter.CheckOwnership(tt.resource, OperatorMetadata{})
			require.Equal(t, tt.expected, owned)
			require.Equal(t, tt.expectedReason, reason)
		})
	}
}

func TestResourceWithLabelsAdapter_SetResourceLabels(t *testing.T) {
	adapter := ResourceWithLabelsAdapter[*fakeResourceWithLabels]{}
	resource := &fakeResourceWithLabels{labels: map[string]string{}}
	adapter.SetResourceLabels(resource, map[string]string{
		"foo": "bar",
	}, OperatorMetadata{
		Namespace: "unused",
		ID:        "unused",
		TokenName: "unused",
		Scope:     "unused",
		Owner:     "unused",
	}, customResourceMetadata{
		name:      "unused",
		namespace: "unused",
		gvk:       "unused",
	})
	require.Equal(t, map[string]string{types.OriginLabel: types.OriginKubernetes, "foo": "bar"}, resource.labels)
}

func TestScopedResourceWithLabelsAdapter_CheckOwnership(t *testing.T) {
	const (
		testID        = "test-id"
		conflictingID = "conflicting-id"
		testScope     = "/team/a"
	)
	tests := []struct {
		name           string
		resource       *fakeResourceWithLabels
		expected       bool
		expectedReason string
	}{
		{
			name:           "unscoped - no labels",
			resource:       &fakeResourceWithLabels{labels: map[string]string{}},
			expected:       false,
			expectedReason: ownershipIssueMissingOriginLabel,
		},
		{
			name:           "uncscope - labels but no origin",
			resource:       &fakeResourceWithLabels{labels: map[string]string{"foo": "bar"}},
			expected:       false,
			expectedReason: ownershipIssueMissingOriginLabel,
		},
		{
			name:           "unscoped - origin label mismatch",
			resource:       &fakeResourceWithLabels{labels: map[string]string{types.OriginLabel: types.OriginDefaults}},
			expected:       false,
			expectedReason: fmt.Sprintf(ownershipIssueMismatchOriginLabel, types.OriginDefaults),
		},
		{
			name:     "unscoped - origin label match but no id",
			resource: &fakeResourceWithLabels{labels: map[string]string{types.OriginLabel: types.OriginKubernetes}},
			expected: true,
		},
		{
			name:     "unscoped - origin label match but mismatch id",
			resource: &fakeResourceWithLabels{labels: map[string]string{types.OriginLabel: types.OriginKubernetes, OperatorIDLabel: conflictingID}},
			expected: true,
		},
		{
			name:     "unscoped - origin label match and matching id",
			resource: &fakeResourceWithLabels{labels: map[string]string{types.OriginLabel: types.OriginKubernetes, OperatorIDLabel: testID}},
			expected: true,
		},
		{
			name: "scoped - no labels",
			resource: &fakeResourceWithLabels{
				labels: map[string]string{},
				scope:  testScope,
			},
			expected:       false,
			expectedReason: ownershipIssueMissingOriginLabel,
		},
		{
			name: "scoped - labels but no origin",
			resource: &fakeResourceWithLabels{
				labels: map[string]string{"foo": "bar"},
				scope:  testScope,
			},
			expected:       false,
			expectedReason: ownershipIssueMissingOriginLabel,
		},
		{
			name: "scoped - origin label mismatch",
			resource: &fakeResourceWithLabels{
				labels: map[string]string{types.OriginLabel: types.OriginDefaults},
				scope:  testScope,
			},
			expected:       false,
			expectedReason: fmt.Sprintf(ownershipIssueMismatchOriginLabel, types.OriginDefaults),
		},
		{
			name: "scoped - origin label match but no id",
			resource: &fakeResourceWithLabels{
				labels: map[string]string{types.OriginLabel: types.OriginKubernetes},
				scope:  testScope,
			},
			expected:       false,
			expectedReason: ownershipIssueMissingOperatorID,
		},
		{
			name: "scoped - origin label match but mismatch id",
			resource: &fakeResourceWithLabels{
				labels: map[string]string{types.OriginLabel: types.OriginKubernetes, OperatorIDLabel: conflictingID},
				scope:  testScope,
			},
			expected:       false,
			expectedReason: fmt.Sprintf(ownershipIssueMismatchOperatorID, conflictingID, testID),
		},
		{
			name: "scoped - origin label match and matching id",
			resource: &fakeResourceWithLabels{
				labels: map[string]string{types.OriginLabel: types.OriginKubernetes, OperatorIDLabel: testID},
				scope:  testScope,
			},
			expected: true,
		},
	}

	adapter := ScopedResourceWithLabelsAdapter[*fakeResourceWithLabels]{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owned, reason := adapter.CheckOwnership(tt.resource, OperatorMetadata{ID: testID})
			require.Equal(t, tt.expected, owned)
			require.Equal(t, tt.expectedReason, reason)
		})
	}

}

func TestScopedResourceWithLabelsAdapter_SetLabels(t *testing.T) {
	const testScope = "/team/a"
	initialLabels := map[string]string{
		"foo": "bar",
	}
	kubeLabels := map[string]string{
		"kube": "label",
	}

	operatorMetadata := OperatorMetadata{
		Namespace: "namespace",
		ID:        "id",
		TokenName: "token",
		Scope:     "scope",
		Owner:     "owner",
	}
	resourceMetadata := customResourceMetadata{
		namespace: "cr-namespace",
		name:      "name",
		gvk:       "gvk",
	}

	tests := []struct {
		name           string
		resource       *fakeResourceWithLabels
		expectedLabels map[string]string
	}{
		{
			name: "unscoped",
			resource: &fakeResourceWithLabels{
				labels: initialLabels,
			},
			expectedLabels: map[string]string{
				"kube":            "label",
				types.OriginLabel: types.OriginKubernetes,
			},
		},
		{
			name: "scoped",
			resource: &fakeResourceWithLabels{
				labels: initialLabels,
				scope:  testScope,
			},
			expectedLabels: map[string]string{
				"kube":                       "label",
				types.OriginLabel:            types.OriginKubernetes,
				OperatorIDLabel:              operatorMetadata.ID,
				operatorNamespaceLabel:       operatorMetadata.Namespace,
				operatorOwnerLabel:           operatorMetadata.Owner,
				operatorTokenNameLabel:       operatorMetadata.TokenName,
				customResourceNameLabel:      resourceMetadata.name,
				customResourceNamespaceLabel: resourceMetadata.namespace,
				customResourceGVKLabel:       resourceMetadata.gvk,
			},
		},
	}

	adapter := ScopedResourceWithLabelsAdapter[*fakeResourceWithLabels]{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter.SetResourceLabels(tt.resource, kubeLabels, operatorMetadata, resourceMetadata)
			require.Equal(t, tt.expectedLabels, tt.resource.labels)
		})
	}
}
