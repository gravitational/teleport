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

import (
	"fmt"
	"strconv"
	"strings"
)

// IsCNRegion returns true if the region is an AWS China region.
func IsCNRegion(region string) bool {
	return strings.HasPrefix(strings.ToLower(region), CNRegionPrefix)
}

// IsUSGovRegion returns true if the region is an AWS US GovCloud region.
func IsUSGovRegion(region string) bool {
	return strings.HasPrefix(strings.ToLower(region), USGovRegionPrefix)
}

// ShortRegionToRegion converts short region codes to regular region names. For
// example, a short region "use1" maps to region "us-east-1".
//
// There is no official documentation on this mapping. Here is gist of others
// collecting these naming schemes:
// https://gist.github.com/colinvh/14e4b7fb6b66c29f79d3
//
// This function currently does not support regions in secert partitions.
func ShortRegionToRegion(shortRegion string) (string, bool) {
	var prefix, direction string

	// Determine region prefix.
	remain := strings.ToLower(shortRegion)
	switch {
	case strings.HasPrefix(remain, "usg"):
		prefix = USGovRegionPrefix
		remain = remain[3:]

	case strings.HasPrefix(remain, "cn"):
		prefix = CNRegionPrefix
		remain = remain[2:]

	default:
		// For regions in standard partition, the first two letters is the
		// continent or country code (e.g. "eu" for Europe, "us" for US).
		if len(remain) < 2 {
			return "", false
		}

		prefix = remain[:2] + "-"
		remain = remain[2:]
	}

	// Map direction codes.
	switch {
	case strings.HasPrefix(remain, "nw"):
		direction = "northwest"
		remain = remain[2:]
	case strings.HasPrefix(remain, "ne"):
		direction = "northeast"
		remain = remain[2:]
	case strings.HasPrefix(remain, "se"):
		direction = "southeast"
		remain = remain[2:]
	case strings.HasPrefix(remain, "sw"):
		direction = "southwest"
		remain = remain[2:]
	case strings.HasPrefix(remain, "n"):
		direction = "north"
		remain = remain[1:]
	case strings.HasPrefix(remain, "e"):
		direction = "east"
		remain = remain[1:]
	case strings.HasPrefix(remain, "w"):
		direction = "west"
		remain = remain[1:]
	case strings.HasPrefix(remain, "s"):
		direction = "south"
		remain = remain[1:]
	case strings.HasPrefix(remain, "c"):
		direction = "central"
		remain = remain[1:]
	default:
		return "", false
	}

	// Remain should be a number.
	if _, err := strconv.Atoi(remain); err != nil {
		return "", false
	}

	return fmt.Sprintf("%s%s-%s", prefix, direction, remain), true
}

const (
	// CNRegionPrefix is the prefix for all AWS China regions.
	CNRegionPrefix = "cn-"

	// USGovRegionPrefix is the prefix for all AWS US GovCloud regions.
	USGovRegionPrefix = "us-gov-"
)
