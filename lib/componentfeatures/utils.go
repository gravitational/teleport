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
	"context"
	"log/slog"

	componentfeaturesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/componentfeatures/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
)

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

// Join copies, deduplicates, and combines [componentfeaturesv1.ComponentFeatures] sets into a single set
// containing all unique features.
func Join(sets ...*componentfeaturesv1.ComponentFeatures) *componentfeaturesv1.ComponentFeatures {
	out := &componentfeaturesv1.ComponentFeatures{}
	seen := make(map[componentfeaturesv1.ComponentFeatureID]struct{})

	for _, fs := range sets {
		if fs == nil || len(fs.GetFeatures()) == 0 {
			continue
		}
		for _, f := range fs.GetFeatures() {
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
func InAllSets(feature FeatureID, sets ...*componentfeaturesv1.ComponentFeatures) bool {
	if len(sets) == 0 {
		return false
	}
	proto := feature.ToProto()

	for _, fs := range sets {
		if fs == nil || len(fs.GetFeatures()) == 0 {
			return false
		}

		found := false
		for _, f := range fs.GetFeatures() {
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
func Intersect(sets ...*componentfeaturesv1.ComponentFeatures) *componentfeaturesv1.ComponentFeatures {
	out := &componentfeaturesv1.ComponentFeatures{}

	if len(sets) == 0 {
		return out
	}

	counts := make(map[componentfeaturesv1.ComponentFeatureID]int)

	for _, fs := range sets {
		if fs == nil || len(fs.GetFeatures()) == 0 {
			return out
		}

		seenInThisSet := make(map[componentfeaturesv1.ComponentFeatureID]struct{})
		for _, f := range fs.GetFeatures() {
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

// ToIntegers deduplicates and converts [componentfeaturesv1.ComponentFeatures] into a list of integers.
func ToIntegers(features *componentfeaturesv1.ComponentFeatures) []int {
	out := make([]int, 0)
	seen := make(map[componentfeaturesv1.ComponentFeatureID]struct{})

	for _, f := range features.GetFeatures() {
		if _, seen := seen[f]; seen {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, int(f))
	}

	return out
}

type AuthProxyServersLister interface {
	GetProxies() ([]types.Server, error)
	GetAuthServers() ([]types.Server, error)
	ListProxyServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error)
	ListAuthServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error)
}

// GetClusterAuthProxyServerFeatures fetches all Auth and Proxy servers in the cluster and returns
// the intersection of their supported ComponentFeatures.
func GetClusterAuthProxyServerFeatures(ctx context.Context, clt AuthProxyServersLister, logger *slog.Logger) *componentfeaturesv1.ComponentFeatures {
	features := make([]*componentfeaturesv1.ComponentFeatures, 0)

	allProxies, err := clientutils.CollectWithFallback(
		ctx,
		clt.ListProxyServers,
		func(context.Context) ([]types.Server, error) {
			//nolint:staticcheck // TODO(kiosion): DELETE IN 21.0.0
			return clt.GetProxies()
		},
	)
	if err != nil {
		// If we fail to get proxies & can't be sure about feature support,
		// intersecting on `nil` ensures any intersection of ComponentFeatures will be empty.
		logger.ErrorContext(ctx, "Failed to get proxy servers to collect ComponentFeatures", "error", err)
		features = append(features, nil)
	} else {
		for _, srv := range allProxies {
			features = append(features, srv.GetComponentFeatures())
		}
	}

	allAuthServers, err := clientutils.CollectWithFallback(
		ctx,
		clt.ListAuthServers,
		func(context.Context) ([]types.Server, error) {
			//nolint:staticcheck // TODO(kiosion): DELETE IN 21.0.0
			return clt.GetAuthServers()
		},
	)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get auth servers to collect ComponentFeatures", "error", err)
		features = append(features, nil)
	} else {
		for _, srv := range allAuthServers {
			features = append(features, srv.GetComponentFeatures())
		}
	}

	return Intersect(features...)
}
