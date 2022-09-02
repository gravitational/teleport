package types

import (
	"strings"
)

// GetSortByFromString expects a string in format `<fieldName>:<asc|desc>` where
// index 0 is fieldName and index 1 is direction.
// If a direction is not set, or is not recognized, it defaults to ASC.
func GetSortByFromString(sortStr string) (sortBy SortBy) {
	if sortStr != "" {
		vals := strings.Split(sortStr, ":")
		if vals[0] != "" {
			sortBy.Field = vals[0]
			if len(vals) > 1 && vals[1] == "desc" {
				sortBy.IsDesc = true
			}
		}
	}

	return sortBy
}
