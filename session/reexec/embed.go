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

package reexec

// EmbeddedReexecAvailable is true if the embedded session helper could be
// enabled in this build.
const EmbeddedReexecAvailable bool = embeddedReexecAvailable

// InitEmbeddedReexec tries to set up the embedded session helper for execution.
// This function can be called multiple times with no adverse effects, and, if
// successful, any subsequent call to [CommandOSTweaks] will result in a
// reexecution using the embedded helper rather than the current process. On
// builds without support for the embedded session helper, this function will
// unconditionally fail. Such a condition can be more clearly checked by using
// [EmbeddedReexecAvailable].
func InitEmbeddedReexec() error {
	return initEmbeddedReexec()
}
