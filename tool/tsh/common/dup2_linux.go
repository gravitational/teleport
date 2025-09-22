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

//go:build linux

package common

import "syscall"

// dup2 implements syscall.Dup2(oldfd, newfd) in a way that works on all
// current Linux platforms, and likely on any new platforms. New platforms
// such as ARM64 do not implement syscall.Dup2() instead implementing
// syscall.Dup3() which is largely a superset, with one special case.
func dup2(oldfd, newfd int) error {
	if oldfd == newfd {
		// dup2 would do nothing in this case, but dup3 returns an error.
		// Emulate dup2 behavior.
		return nil
	}
	return syscall.Dup3(oldfd, newfd, 0)
}
