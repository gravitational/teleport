package utils

import "github.com/gravitational/teleport/api/types"

func LabelsToMap(labels types.Labels) map[string][]string {
	result := make(map[string][]string)
	for key, values := range labels {
		result[key] = values
	}
	return result
}
