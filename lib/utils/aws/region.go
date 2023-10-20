/*
Copyright 2023 Gravitational, Inc.

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

package aws

import (
	"sync"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
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
			regions = append(regions, maps.Keys(partition.Regions())...)
		}
		knownRegions = regions
	})
	return knownRegions
}

var (
	knownRegions     []string
	knownRegionsOnce sync.Once
)
