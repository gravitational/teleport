// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package teleport

import "github.com/gravitational/teleport/session/logconstants"

// static assertions that [logconstants.ComponentKey] and [logconstants.ComponentFields]
// are equal to the respective consts defined in this package; we can't just
// define them to be equal because the true definition belongs here and we want
// to avoid circular module requirements
func _() {
	const mustBeTrue = ComponentKey == logconstants.ComponentKey
	_ = map[bool]struct{}{false: struct{}{}, mustBeTrue: struct{}{}}
}
func _() {
	const mustBeTrue = ComponentFields == logconstants.ComponentFields
	_ = map[bool]struct{}{false: struct{}{}, mustBeTrue: struct{}{}}
}
