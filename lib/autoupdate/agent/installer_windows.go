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

package agent

import (
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

// changeServiceBinPath sets the binPath of an open Windows service, leaving all other configuration unchanged.
func changeServiceBinPath(service *mgr.Service, binPath string) error {
	p, err := windows.UTF16PtrFromString(binPath)
	if err != nil {
		return trace.Wrap(err, "converting binPath to UTF16")
	}
	err = windows.ChangeServiceConfig(service.Handle,
		windows.SERVICE_NO_CHANGE, // serviceType
		windows.SERVICE_NO_CHANGE, // startType
		windows.SERVICE_NO_CHANGE, // errorControl
		p,                         // binaryPathName
		nil, nil, nil, nil, nil, nil)
	return trace.Wrap(err, "changing binPath of Windows service %s", service.Name)
}

// splitBinPath splits a Windows service binPath into the executable path and its arguments.
// The executable may be double-quoted; an unquoted executable ends at the first space.
func splitBinPath(binPath string) (exe, args string) {
	binPath = strings.TrimLeft(binPath, " ")
	if rest, ok := strings.CutPrefix(binPath, `"`); ok {
		exe, args, _ = strings.Cut(rest, `"`)
		return exe, strings.TrimLeft(args, " ")
	}
	exe, args, _ = strings.Cut(binPath, " ")
	return exe, strings.TrimLeft(args, " ")
}

// joinBinPath builds a Windows service binPath from a quoted executable path and its arguments.
func joinBinPath(exe, args string) string {
	if args == "" {
		return `"` + exe + `"`
	}
	return `"` + exe + `" ` + args
}

// samePath returns true if both paths refer to the same file, ignoring case.
func samePath(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// isWithinDir returns true if path is located within dir, ignoring case.
func isWithinDir(dir, path string) bool {
	rel, err := filepath.Rel(strings.ToLower(filepath.Clean(dir)), strings.ToLower(filepath.Clean(path)))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
