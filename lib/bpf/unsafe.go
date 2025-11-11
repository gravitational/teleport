/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package bpf

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// ConvertString converts a NUL-terminated string in a slice of int8 to
// a Go string. If the slice does not contain a NUL, the whole slice is
// copied.
func ConvertString(s []int8) string {
	bs := unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(s))), len(s))
	return unix.ByteSliceToString(bs)
}
