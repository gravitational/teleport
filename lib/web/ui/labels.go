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

func transformCommandLabels(commandLabels map[string]types.CommandLabel) map[string]string {
	labels := make(map[string]string, len(commandLabels))

	for name, cmd := range commandLabels {
		labels[name] = cmd.GetResult()
	}

	return labels
}
