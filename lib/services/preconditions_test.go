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

	"github.com/stretchr/testify/require"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
)

func TestPreconditions_Add(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		initial  []*decisionpb.Precondition
		toAdd    []*decisionpb.Precondition
		expected []decisionpb.PreconditionKind
	}{
		{
			name:    "add to empty",
			initial: nil,
			toAdd: []*decisionpb.Precondition{
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
			},
			expected: []decisionpb.PreconditionKind{decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
		},
		{
			name: "add multiple maintains sorted order",
			initial: []*decisionpb.Precondition{
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
			},
			toAdd: []*decisionpb.Precondition{
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED},
			},
			expected: []decisionpb.PreconditionKind{
				decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED,
				decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA,
			},
		},
		{
			name: "add duplicate does not increase length",
			initial: []*decisionpb.Precondition{
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
			},
			toAdd: []*decisionpb.Precondition{
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
			},
			expected: []decisionpb.PreconditionKind{decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
		},
		{
			name: "add to middle maintains order",
			initial: []*decisionpb.Precondition{
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED},
			},
			toAdd: []*decisionpb.Precondition{
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
			},
			expected: []decisionpb.PreconditionKind{
				decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED,
				decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			preconds := newPreconditions(tc.initial)

			for _, precondition := range tc.toAdd {
				preconds.Add(precondition)
			}

			var got []decisionpb.PreconditionKind
			for precondition := range preconds.All() {
				got = append(got, precondition.Kind)
			}

			require.True(t, slices.Equal(got, tc.expected), "After Add(), All() = %v, want %v", got, tc.expected)
		})

	}
}

func TestPreconditions_All(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    *Preconditions
		expected []decisionpb.PreconditionKind
	}{
		{
			name:     "empty",
			input:    newPreconditions(nil),
			expected: []decisionpb.PreconditionKind{},
		},
		{
			name: "single element",
			input: newPreconditions([]*decisionpb.Precondition{
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
			}),
			expected: []decisionpb.PreconditionKind{
				decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA,
			},
		},
		{
			name: "multiple elements unordered",
			input: newPreconditions([]*decisionpb.Precondition{
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED},
			}),
			expected: []decisionpb.PreconditionKind{
				decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED,
				decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA,
			},
		},
		{
			name: "already sorted",
			input: newPreconditions([]*decisionpb.Precondition{
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED},
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
			}),
			expected: []decisionpb.PreconditionKind{
				decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED,
				decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []decisionpb.PreconditionKind

			for precondition := range tc.input.All() {
				got = append(got, precondition.Kind)
			}

			require.True(t, slices.Equal(got, tc.expected), "All() = %v, want %v", got, tc.expected)
		})

	}
}

func TestPreconditions_Contains(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    *Preconditions
		kind     decisionpb.PreconditionKind
		expected bool
	}{
		{
			name: "contains existing element",
			input: newPreconditions([]*decisionpb.Precondition{
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED},
			}),
			kind:     decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA,
			expected: true,
		},
		{
			name: "contains another existing element",
			input: newPreconditions([]*decisionpb.Precondition{
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED},
			}),
			kind:     decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED,
			expected: true,
		},
		{
			name: "does not contain non-existing element",
			input: newPreconditions([]*decisionpb.Precondition{
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED},
			}),
			kind:     decisionpb.PreconditionKind(999),
			expected: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.input.Contains(tc.kind)
			require.Equal(t, tc.expected, got, "Contains(%v) = %v, want %v", tc.kind, got, tc.expected)
		})

	}
}

func TestPreconditions_Len(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    *Preconditions
		expected int
	}{
		{
			name:     "empty set",
			input:    newPreconditions(nil),
			expected: 0,
		},
		{
			name: "single element",
			input: newPreconditions([]*decisionpb.Precondition{
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
			}),
			expected: 1,
		},
		{
			name: "multiple elements",
			input: newPreconditions([]*decisionpb.Precondition{
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED},
			}),
			expected: 2,
		},
		{
			name: "duplicates counted once",
			input: newPreconditions([]*decisionpb.Precondition{
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
				{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
			}),
			expected: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.input.Len()
			require.Equal(t, tc.expected, got, "Len() = %v, want %v", got, tc.expected)
		})
	}
}

func newPreconditions(preconds []*decisionpb.Precondition) *Preconditions {
	p := NewPreconditions()

	for _, precond := range preconds {
		p.Add(precond)
	}

	return p
}
