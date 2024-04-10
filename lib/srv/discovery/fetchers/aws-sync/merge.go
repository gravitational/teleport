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

package aws_sync

import "reflect"

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
	resultElem := reflect.ValueOf(result).Elem()
	for _, r := range results {
		if r == nil {
			continue
		}
		mergable := reflect.ValueOf(r).Elem()
		for i := 0; i < resultElem.NumField(); i++ {
			field := resultElem.Field(i)
			mergableField := mergable.Field(i)
			field.Set(reflect.AppendSlice(field, mergableField))
		}
	}
	return result
}
