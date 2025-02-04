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

package aws

import (
	"maps"
	"slices"
	"sync"

	"github.com/aws/aws-sdk-go/aws/endpoints"
)

// IsKnownRegion returns true if provided region is one of the "well-known"
// AWS regions.
func IsKnownRegion(region string) bool {
	return slices.Contains(GetKnownRegions(), region)
}

// GetKnownRegions returns a list of "well-known" AWS regions generated from
// AWS SDK.
func GetKnownRegions() []string {
	knownRegionsOnce.Do(func() {
		var regions []string
		partitions := endpoints.DefaultPartitions()
		for _, partition := range partitions {
			regions = append(regions, slices.Collect(maps.Keys(partition.Regions()))...)
		}
		knownRegions = regions
	})
	return knownRegions
}

var (
	knownRegions     []string
	knownRegionsOnce sync.Once
)
