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

package vnet

import (
	"context"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"

	"github.com/gravitational/teleport/lib/windowsservice"
)

const eventSource = "vnet"

// InstallService installs the VNet Windows service.
func InstallService(ctx context.Context) error {
	tshPath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "getting current exe path")
	}
	if err := assertWintunInstalled(tshPath); err != nil {
		return trace.Wrap(err, "checking if wintun.dll is installed next to %s", tshPath)
	}
	return trace.Wrap(windowsservice.Install(ctx, &windowsservice.InstallConfig{
		Name:              serviceName,
		Command:           ServiceCommand,
		EventSourceName:   eventSource,
		AccessPermissions: windows.SERVICE_QUERY_STATUS | windows.SERVICE_START | windows.SERVICE_STOP,
	}))
}

// UninstallService uninstalls the Windows VNet service.
func UninstallService(ctx context.Context) error {
	return trace.Wrap(windowsservice.Uninstall(ctx, &windowsservice.UninstallConfig{
		Name:            serviceName,
		EventSourceName: eventSource,
	}))
}

// asertWintunInstalled returns an error if wintun.dll is not a regular file
// installed in the same directory as tshPath.
func assertWintunInstalled(tshPath string) error {
	dir := filepath.Dir(tshPath)
	wintunPath := filepath.Join(dir, "wintun.dll")
	return trace.Wrap(assertRegularFile(wintunPath))
}

func assertRegularFile(path string) error {
	switch info, err := os.Lstat(path); {
	case os.IsNotExist(err):
		return trace.Wrap(err, "%s not found", path)
	case err != nil:
		return trace.Wrap(err, "unexpected error checking %s", path)
	case !info.Mode().IsRegular():
		return trace.BadParameter("%s is not a regular file", path)
	}
	return nil
}
