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

package services

import (
	"slices"
	"testing"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
)

func TestPreconditions_Sorted(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    Preconditions
		expected []decisionpb.PreconditionKind
	}{
		{
			name:     "empty",
			input:    Preconditions{},
			expected: []decisionpb.PreconditionKind{},
		},
		{
			name: "single element",
			input: Preconditions{
				decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA: {},
			},
			expected: []decisionpb.PreconditionKind{
				decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA,
			},
		},
		{
			name: "multiple elements unordered",
			input: Preconditions{
				decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA: {},
				decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED: {},
			},
			expected: []decisionpb.PreconditionKind{
				decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED,
				decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA,
			},
		},
		{
			name: "already sorted",
			input: Preconditions{
				decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED: {},
				decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA: {},
			},
			expected: []decisionpb.PreconditionKind{
				decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED,
				decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.input.Sorted()
			if !slices.Equal(got, tc.expected) {
				t.Errorf("Sorted() = %v, want %v", got, tc.expected)
			}
		})
	}
}
