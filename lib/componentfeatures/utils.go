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

import componentfeaturesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/componentfeatures/v1"

// New creates a new [componentfeaturesv1.ComponentFeatures] struct containing the provided FeatureIDs.
func New(features ...FeatureID) *componentfeaturesv1.ComponentFeatures {
	out := &componentfeaturesv1.ComponentFeatures{}
	seen := make(map[FeatureID]struct{})
	for _, f := range features {
		if _, exists := seen[f]; exists {
			continue
		}
		seen[f] = struct{}{}
		out.Features = append(out.Features, f.ToProto())
	}
	return out
}

// Join combines [componentfeaturesv1.ComponentFeatures] sets into a single set containing all unique features.
func Join(sets ...*componentfeaturesv1.ComponentFeatures) *componentfeaturesv1.ComponentFeatures {
	out := &componentfeaturesv1.ComponentFeatures{}
	seen := make(map[componentfeaturesv1.ComponentFeatureID]struct{})

	for _, fs := range sets {
		if fs == nil || len(fs.Features) == 0 {
			continue
		}
		for _, f := range fs.Features {
			if _, exists := seen[f]; !exists {
				seen[f] = struct{}{}
				out.Features = append(out.Features, f)
			}
		}
	}

	return out
}

// InAllSets reports whether a given [componentfeaturesv1.ComponentFeatureID] is
// present in *every* [componentfeaturesv1.ComponentFeatures] set.
//
// If no sets are provided, or any set is nil, it returns false.
func InAllSets(
	feature FeatureID,
	sets ...*componentfeaturesv1.ComponentFeatures,
) bool {
	if len(sets) == 0 {
		return false
	}
	proto := feature.ToProto()

	for _, fs := range sets {
		if fs == nil || len(fs.Features) == 0 {
			return false
		}

		found := false
		for _, f := range fs.Features {
			if f == proto {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// Intersect returns a new [componentfeaturesv1.ComponentFeatures] containing only features present in all input
// [componentfeaturesv1.ComponentFeatures].
//
// If no sets are provided, or any set is nil or empty, it returns an empty [componentfeaturesv1.ComponentFeatures].
func Intersect(
	sets ...*componentfeaturesv1.ComponentFeatures,
) *componentfeaturesv1.ComponentFeatures {
	out := &componentfeaturesv1.ComponentFeatures{}

	if len(sets) == 0 {
		return out
	}

	counts := make(map[componentfeaturesv1.ComponentFeatureID]int)

	for _, fs := range sets {
		if fs == nil || len(fs.Features) == 0 {
			return out
		}

		seenInThisSet := make(map[componentfeaturesv1.ComponentFeatureID]struct{})
		for _, f := range fs.Features {
			if _, seen := seenInThisSet[f]; seen {
				continue
			}
			seenInThisSet[f] = struct{}{}
			counts[f]++
		}
	}

	for f, c := range counts {
		if c == len(sets) {
			out.Features = append(out.Features, f)
		}
	}

	return out
}
