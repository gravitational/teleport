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

package utils

import (
	"unsafe"

	"github.com/gravitational/trace"
)

// UnsafeSliceData is a wrapper around unsafe.SliceData
// which ensures that instead of ever returning
// "a non-nil pointer to an unspecified memory address"
// (see unsafe.SliceData documentation), an error is
// returned instead.
func UnsafeSliceData[T any](slice []T) (*T, error) {
	if slice == nil || cap(slice) > 0 {
		return unsafe.SliceData(slice), nil
	}

	return nil, trace.BadParameter("non-nil slice had a capacity of zero")
}
