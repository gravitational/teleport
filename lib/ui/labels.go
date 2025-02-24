/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package ui

import (
	"sort"
	"strings"

	"github.com/gravitational/teleport/api/types"
)

// Label describes label for webapp
type Label struct {
	// Name is this label name
	Name string `json:"name"`
	// Value is this label value
	Value string `json:"value"`
}

// MakeLabelsWithoutInternalPrefixes is a function that transforms map[string]string arguments passed to it to sorted slice of Labels.
// It also removes all Teleport internal labels from output.
func MakeLabelsWithoutInternalPrefixes(labelMaps ...map[string]string) []Label {
	length := 0
	for _, labelMap := range labelMaps {
		length += len(labelMap)
	}

	labels := make([]Label, 0, length)

	for _, labelMap := range labelMaps {
		for name, value := range labelMap {
			if strings.HasPrefix(name, types.TeleportInternalLabelPrefix) ||
				strings.HasPrefix(name, types.TeleportHiddenLabelPrefix) {
				continue
			}

			labels = append(labels, Label{Name: name, Value: value})
		}
	}

	sort.Sort(sortedLabels(labels))

	return labels
}

// sortedLabels is a sort wrapper that sorts labels by name
type sortedLabels []Label

func (s sortedLabels) Len() int {
	return len(s)
}

func (s sortedLabels) Less(i, j int) bool {
	labelA := strings.ToLower(s[i].Name)
	labelB := strings.ToLower(s[j].Name)

	for _, sortName := range types.BackSortedLabelPrefixes {
		name := strings.ToLower(sortName)
		if strings.Contains(labelA, name) && !strings.Contains(labelB, name) {
			return false // labelA should be at the end
		}
		if !strings.Contains(labelA, name) && strings.Contains(labelB, name) {
			return true // labelB should be at the end
		}
	}

	// If neither label contains any of the sendToBackOfSortNames, sort them as usual
	return labelA < labelB
}

func (s sortedLabels) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func TransformCommandLabels(commandLabels map[string]types.CommandLabel) map[string]string {
	labels := make(map[string]string, len(commandLabels))

	for name, cmd := range commandLabels {
		labels[name] = cmd.GetResult()
	}

	return labels
}
