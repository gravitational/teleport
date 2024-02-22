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

package gitlab

// MergeResources merges multiple resources into a single Resources object.
// This is used to merge resources from multiple accounts and regions
// into a single object.
// It does not check for duplicates, so it is possible to have duplicates.
func MergeResources(results ...*Resources) *Resources {
	if len(results) == 0 {
		return &Resources{}
	}
	if len(results) == 1 {
		return results[0]
	}
	result := &Resources{}
	for _, r := range results {
		result.GroupMembers = append(result.GroupMembers, r.GroupMembers...)
		result.ProjectMembers = append(result.ProjectMembers, r.ProjectMembers...)
		result.Projects = append(result.Projects, r.Projects...)
		result.Groups = append(result.Groups, r.Groups...)
	}
	return result
}
