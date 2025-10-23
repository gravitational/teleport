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
	"os/exec"
	"syscall"

	"github.com/gravitational/trace"
)

// newCommand creates command depends on platform.
func newCommand(path string, args ...string) *exec.Cmd {
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	return cmd
}

// sendInterrupt sends a Ctrl-Break event to the process.
func sendInterrupt(pid int) error {
	d, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return trace.Wrap(err)
	}
	p, err := d.FindProc("GenerateConsoleCtrlEvent")
	if err != nil {
		return trace.Wrap(err)
	}
	r, _, err := p.Call(syscall.CTRL_BREAK_EVENT, uintptr(pid))
	if r == 0 {
		return trace.Wrap(err)
	}
	return nil
}
