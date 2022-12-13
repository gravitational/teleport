/*
Copyright 2020 Gravitational, Inc.

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

package ui

import (
	"sort"
	"strings"

	"github.com/gravitational/teleport/api/types"
)

// makeLabels is a function that transforms map[string]string arguments passed to it to sorted slice of Labels.
// It also removes all Teleport internal labels from output.
func makeLabels(labelMaps ...map[string]string) []Label {
	length := 0
	for _, labelMap := range labelMaps {
		length += len(labelMap)
	}

	labels := make([]Label, 0, length)

	for _, labelMap := range labelMaps {
		for name, value := range labelMap {
			if strings.HasPrefix(name, types.TeleportInternalLabelPrefix) {
				continue
			}

			labels = append(labels, Label{Name: name, Value: value})
		}
	}

	sort.Sort(sortedLabels(labels))

	return labels
}

func transformCommandLabels(commandLabels map[string]types.CommandLabel) map[string]string {
	labels := make(map[string]string, len(commandLabels))

	for name, cmd := range commandLabels {
		labels[name] = cmd.GetResult()
	}

	return labels
}
