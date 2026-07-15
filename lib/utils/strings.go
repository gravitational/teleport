/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import "strings"

// TrimNonEmpty trims leading and trailing whitespace from s and reports whether
// the trimmed value is non-empty. It is shaped to be a drop-in callback for
// filter-and-transform helpers such as lib/utils/slices.FilterMapUnique, where
// the boolean return decides whether the transformed value is kept.
//
// Typical use is normalizing user-edited string lists (matcher selectors,
// config arrays) where stray whitespace and empty entries should both be
// dropped before downstream processing.
func TrimNonEmpty(s string) (string, bool) {
	s = strings.TrimSpace(s)
	return s, s != ""
}
