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

package autoupdate

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows/registry"
)

const (
	// Defined in electron-builder-config.js
	teleportConnectGUID    = "22539266-67e8-54a3-83b9-dfdca7b33ee1"
	teleportConnectKeyPath = `SOFTWARE\` + teleportConnectGUID
	installLocationKey     = "InstallLocation"
)

func isPerMachineInstall() (bool, error) {
	perMachineLocation, err := readPerMachineInstallLocation()
	if err != nil {
		if trace.IsNotFound(err) {
			return false, nil
		}
		return false, trace.Wrap(err)
	}

	exePath, err := os.Executable()
	if err != nil {
		return false, trace.Wrap(err)
	}

	// tsh is placed in <installation directory>/resources/bin/tsh.exe.
	exePathImperMachineLocation := filepath.Join(perMachineLocation, "resources", "bin", "tsh.exe")

	return exePath == exePathImperMachineLocation, nil
}

func readPerMachineInstallLocation() (path string, err error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, teleportConnectKeyPath, registry.READ)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", trace.NotFound("registry key %s not found", teleportConnectKeyPath)
		}
		return "", trace.Wrap(err, "opening registry key %s", teleportConnectKeyPath)
	}

	defer func() {
		if closeErr := key.Close(); closeErr != nil && err == nil {
			err = trace.Wrap(closeErr, "closing registry key %s", teleportConnectKeyPath)
		}
	}()

	path, _, err = key.GetStringValue(installLocationKey)
	if err != nil {
		return "", trace.Wrap(err, "reading registry value %s from %s", installLocationKey, teleportConnectKeyPath)
	}

	return path, nil
}
