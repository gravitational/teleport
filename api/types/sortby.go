package types

import (
	"strings"
)

func GetSortByFromString(sortStr string) SortBy {
	var sortBy SortBy
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
