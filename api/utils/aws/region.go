/*
Copyright 2022 Gravitational, Inc.

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

import "strings"

// IsCNRegion returns true if the region is an AWS China region.
func IsCNRegion(region string) bool {
	return strings.HasPrefix(region, CNRegionPrefix)
}

// ISUSGovRegion returns true if the region is an AWS US GovCloud region.
func ISUSGovRegion(region string) bool {
	return strings.HasPrefix(region, CNRegionPrefix)
}

const (
	// CNRegionPrefix is the prefix for all AWS China regions.
	CNRegionPrefix = "cn-"

	// USGovRegionPrefix is the prefix for all AWS US GovCloud regions.
	USGovRegionPrefix = "us-gov-"
)
