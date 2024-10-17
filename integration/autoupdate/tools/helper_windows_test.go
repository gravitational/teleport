//go:build windows

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

package tools_test

import (
	"syscall"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
)

var (
	kernel    = windows.NewLazyDLL("kernel32.dll")
	ctrlEvent = kernel.NewProc("GenerateConsoleCtrlEvent")
)

// sendInterrupt sends a Ctrl-Break event to the process.
func sendInterrupt(pid int) error {
	r, _, err := ctrlEvent.Call(uintptr(syscall.CTRL_BREAK_EVENT), uintptr(pid))
	if r == 0 {
		return trace.Wrap(err)
	}
	return nil
}
