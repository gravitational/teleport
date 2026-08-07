package reconcilers

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

type fakeResourceWithoutLabels struct {
	resourceWithoutLabels
	labels map[string]string
}

func (r *fakeResourceWithoutLabels) Origin() string {
	return r.labels[types.OriginLabel]
}

func (r *fakeResourceWithoutLabels) SetOrigin(origin string) {
	r.labels[types.OriginLabel] = origin
}

func TestResourceWithoutLabelsAdapter_CheckOwnership(t *testing.T) {
	tests := []struct {
		name           string
		resource       *fakeResourceWithoutLabels
		expected       bool
		expectedReason string
	}{
		{
			name:           "no labels",
			resource:       &fakeResourceWithoutLabels{labels: map[string]string{}},
			expected:       false,
			expectedReason: ownershipIssueMissingOriginLabel,
		},
		{
			name:           "labels but no origin",
			resource:       &fakeResourceWithoutLabels{labels: map[string]string{"foo": "bar"}},
			expected:       false,
			expectedReason: ownershipIssueMissingOriginLabel,
		},
		{
			name:           "origin label mismatch",
			resource:       &fakeResourceWithoutLabels{labels: map[string]string{types.OriginLabel: types.OriginDefaults}},
			expected:       false,
			expectedReason: fmt.Sprintf(ownershipIssueMismatchOriginLabel, types.OriginDefaults),
		},
		{
			name:     "origin label match",
			resource: &fakeResourceWithoutLabels{labels: map[string]string{types.OriginLabel: types.OriginKubernetes}},
			expected: true,
		},
	}

	adapter := ResourceWithoutLabelsAdapter[*fakeResourceWithoutLabels]{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owned, reason := adapter.CheckOwnership(tt.resource, OperatorMetadata{})
			require.Equal(t, tt.expected, owned)
			require.Equal(t, tt.expectedReason, reason)
		})
	}

}

func TestResourceWithoutLabelsAdapter_SetResourceLabels(t *testing.T) {
	adapter := ResourceWithoutLabelsAdapter[*fakeResourceWithoutLabels]{}
	resource := &fakeResourceWithoutLabels{labels: map[string]string{}}
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
	require.Equal(t, map[string]string{types.OriginLabel: types.OriginKubernetes}, resource.labels)
}
