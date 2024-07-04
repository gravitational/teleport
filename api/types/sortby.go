// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"strings"
)

// GetSortByFromString expects a string in format `<fieldName>:<asc|desc>` where
// index 0 is fieldName and index 1 is direction.
// If a direction is not set, or is not recognized, it defaults to ASC.
func GetSortByFromString(sortStr string) SortBy {
	var sortBy SortBy

	if sortStr == "" {
		return sortBy
	}

	vals := strings.Split(sortStr, ":")
	if vals[0] != "" {
		sortBy.Field = vals[0]
		if len(vals) > 1 && strings.ToLower(vals[1]) == "desc" {
			sortBy.IsDesc = true
		}
	}

	return sortBy
}
