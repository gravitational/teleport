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
package sortcache

// NextKey returns the lexographically next key of equivalent length. This can be used to find the higher
// bound for a range whose lower bound is known (e.g. keys of the form `"prefix/suffix"` will all fall between
// `"prefix/"` and `NextKey("prefix/")`).
func NextKey(key string) string {
	end := []byte(key)
	for i := len(end) - 1; i >= 0; i-- {
		if end[i] < 0xff {
			end[i] = end[i] + 1
			end = end[:i+1]
			return string(end)
		}
	}

	// key is already the lexographically last value for this length and therefore there is no
	// true 'next' value. using a prefix of this form is somewhat nonsensical and unlikely to
	// come up in real-world usage. for the purposes of this helper, we treat this scenario as
	// something analogous to a zero-length slice indexing (`s[:0]`, `s[len(s):]`, etc), and
	// return the key unmodified. A valid alternative might be to append `0xff`, making the
	// range effectively be over all suffixes of key whose leading character were less than
	// `0xff`, but that is arguably more confusing since ranges would return some but not all
	// of the valid suffixes.
	return key
}
