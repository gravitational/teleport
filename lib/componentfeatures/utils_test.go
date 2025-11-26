/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package componentfeatures

import (
	"testing"

	componentfeaturesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/componentfeatures/v1"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	const (
		featA = FeatureResourceConstraintsV1
		featB = FeatureID(200)
	)

	tests := []struct {
		name     string
		features []FeatureID
		want     []componentfeaturesv1.ComponentFeatureID
	}{
		{
			name:     "no features yields empty set",
			features: []FeatureID{},
			want:     []componentfeaturesv1.ComponentFeatureID{},
		},
		{
			name:     "duplicates are ignored",
			features: []FeatureID{featA, featB, featA, featB},
			want:     []componentfeaturesv1.ComponentFeatureID{featA.ToProto(), featB.ToProto()},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := New(tt.features...)
			require.NotNil(t, got, "New(%v) returned nil", tt.features)
			require.ElementsMatch(t, tt.want, got.Features, "New(%v) = %v; want %v", tt.features, got.Features, tt.want)
		})
	}
}

func TestJoin(t *testing.T) {
	t.Parallel()

	const (
		featA = FeatureResourceConstraintsV1
		featB = FeatureID(200)
		featC = FeatureID(201)
	)

	tests := []struct {
		name string
		sets []*componentfeaturesv1.ComponentFeatures
		want []componentfeaturesv1.ComponentFeatureID
	}{
		{
			name: "no sets returns empty set",
			sets: nil,
			want: []componentfeaturesv1.ComponentFeatureID{},
		},
		{
			name: "single set returns same set",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA, featB)},
			want: []componentfeaturesv1.ComponentFeatureID{featA.ToProto(), featB.ToProto()},
		},
		{
			name: "multiple sets combined",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA), New(featB), New(featA, featC)},
			want: []componentfeaturesv1.ComponentFeatureID{featA.ToProto(), featB.ToProto(), featC.ToProto()},
		},
		{
			name: "nil and empty sets ignored",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA), nil, New(), New(featB)},
			want: []componentfeaturesv1.ComponentFeatureID{featA.ToProto(), featB.ToProto()},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Join(tt.sets...)
			require.NotNil(t, got, "Join(%v) returned nil", tt.sets)
			require.ElementsMatch(t, tt.want, got.Features, "Join(%v) = %v; want %v", tt.sets, got.Features, tt.want)
		})
	}
}

func TestInAllSets(t *testing.T) {
	t.Parallel()

	const (
		featA = FeatureResourceConstraintsV1
		featB = FeatureID(200)
		featC = FeatureID(201)
	)

	tests := []struct {
		name    string
		sets    []*componentfeaturesv1.ComponentFeatures
		feature FeatureID
		want    bool
	}{
		{
			name:    "no sets",
			sets:    nil,
			feature: featA,
			want:    false,
		},
		{
			name:    "single set contains feature",
			sets:    []*componentfeaturesv1.ComponentFeatures{New(featA)},
			feature: featA,
			want:    true,
		},
		{
			name:    "single set missing feature",
			sets:    []*componentfeaturesv1.ComponentFeatures{New(featB)},
			feature: featA,
			want:    false,
		},
		{
			name:    "multiple sets all contain feature",
			sets:    []*componentfeaturesv1.ComponentFeatures{New(featA, featB), New(featA), New(featA, featB, featC)},
			feature: featA,
			want:    true,
		},
		{
			name:    "one set missing feature",
			sets:    []*componentfeaturesv1.ComponentFeatures{New(featA), New(featA, featB), New(featB, featC)},
			feature: featA,
			want:    false,
		},
		{
			name:    "nil set treated as unsupported",
			sets:    []*componentfeaturesv1.ComponentFeatures{New(featB, featC), nil, New(featA, featB)},
			feature: featB,
			want:    false,
		},
		{
			name:    "empty set treated as unsupported",
			sets:    []*componentfeaturesv1.ComponentFeatures{New(featA), New(), New(featA, featB)},
			feature: featA,
			want:    false,
		},
		{
			name:    "duplicate feature in set",
			sets:    []*componentfeaturesv1.ComponentFeatures{New(featA, featA, featB), New(featA, featC, featC)},
			feature: featA,
			want:    true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := InAllSets(tt.feature, tt.sets...)
			require.Equal(t, tt.want, got, "FeatureInAllSets(%v, %v) = %v; want %v", tt.feature, tt.sets, got, tt.want)
		})
	}
}

func TestIntersect(t *testing.T) {
	t.Parallel()

	const (
		featA = FeatureResourceConstraintsV1
		featB = FeatureID(200)
		featC = FeatureID(201)
		featD = FeatureID(202)
	)

	tests := []struct {
		name string
		sets []*componentfeaturesv1.ComponentFeatures
		want []componentfeaturesv1.ComponentFeatureID
	}{
		{
			name: "no sets returns empty set",
			sets: nil,
			want: []componentfeaturesv1.ComponentFeatureID{},
		},
		{
			name: "nil set returns empty set",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA, featB), nil, New(featA, featC)},
			want: []componentfeaturesv1.ComponentFeatureID{},
		},
		{
			name: "disjoint sets return empty set",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA, featB), New(featC, featD)},
			want: []componentfeaturesv1.ComponentFeatureID{},
		},
		{
			name: "single set returns same set",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA, featB)},
			want: []componentfeaturesv1.ComponentFeatureID{featA.ToProto(), featB.ToProto()},
		},
		{
			name: "sets with full intersection",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA, featB, featD), New(featD, featA, featB), New(featB, featD, featA)},
			want: []componentfeaturesv1.ComponentFeatureID{featA.ToProto(), featB.ToProto(), featD.ToProto()},
		},
		{
			name: "sets with partial intersection",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA, featB, featC), New(featB, featC, featD), New(featB, featD)},
			want: []componentfeaturesv1.ComponentFeatureID{featB.ToProto()},
		},
		{
			name: "duplicate features do not affect intersection",
			sets: []*componentfeaturesv1.ComponentFeatures{New(featA, featB, featB), New(featA, featA, featB, featC), New(featA, featD)},
			want: []componentfeaturesv1.ComponentFeatureID{featA.ToProto()},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Intersect(tt.sets...)
			require.NotNil(t, got, "Intersect should not return nil")

			// Order not guaranteed, so compare as sets.
			require.ElementsMatch(t, tt.want, got.Features, "Intersect(%v) = %v; want %v", tt.sets, got.Features, tt.want)
		})
	}
}
