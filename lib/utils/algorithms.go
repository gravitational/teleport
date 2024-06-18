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

package utils

// Combinations yields all unique sub-slices of the input slice.
func Combinations(verbs []string) [][]string {
	var result [][]string
	length := len(verbs)

	for i := 0; i < (1 << length); i++ {
		subslice := make([]string, 0)
		for j := 0; j < length; j++ {
			if i&(1<<j) != 0 {
				subslice = append(subslice, verbs[j])
			}
		}
		result = append(result, subslice)
	}

	return result
}
