// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

//go:build !(linux && arm64)

package common

import "syscall"

// dup2 wraps syscall.Dup2(oldfd, newfd) on platforms that have it, allowing
// platforms that do not to implement dup2() with syscall.Dup3() instead.
func dup2(oldfd, newfd int) error {
	return syscall.Dup2(oldfd, newfd)
}
