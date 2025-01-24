/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
)

// IsManagedByUpdater returns true if the local Teleport binary is managed by teleport-update.
// Note that true may be returned even if auto-updates is disabled or the version is pinned.
// The binary is considered managed if it lives under /opt/teleport, but not within the package
// path at /opt/teleport/system.
func IsManagedByUpdater() (bool, error) {
	teleportPath, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return false, trace.Wrap(err, "cannot find Teleport binary")
	}
	// Check if current binary is under the updater-managed path.
	managed, err := hasParentDir(teleportPath, teleportOptDir)
	if err != nil {
		return false, trace.Wrap(err)
	}
	if !managed {
		return false, nil
	}
	// Return false if the binary is under the updater-managed path, but in the system prefix reserved for the package.
	system, err := hasParentDir(teleportPath, filepath.Join(teleportOptDir, systemNamespace))
	return !system, err
}

// IsManagedAndDefault returns true if the local Teleport binary is both managed by teleport-update
// and the default installation (with teleport.service as the unit file name).
// The binary is considered managed and default if it lives within /opt/teleport/default.
func IsManagedAndDefault() (bool, error) {
	teleportPath, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return false, trace.Wrap(err, "cannot find Teleport binary")
	}
	return hasParentDir(teleportPath, filepath.Join(teleportOptDir, defaultNamespace))
}

// hasParentDir returns true if dir is any parent directory of parent.
// hasParentDir does not resolve symlinks, and requires that files be represented the same way in dir and parent.
func hasParentDir(dir, parent string) (bool, error) {
	// Note that os.Stat + os.SameFile would be more reliable,
	// but does not work well for arbitrarily nested subdirectories.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false, trace.Wrap(err, "cannot get absolute path for directory %s", dir)
	}
	absParent, err := filepath.Abs(parent)
	if err != nil {
		return false, trace.Wrap(err, "cannot get absolute path for parent directory %s", dir)
	}
	sep := string(filepath.Separator)
	if !strings.HasSuffix(absParent, sep) {
		absParent += sep
	}
	return strings.HasPrefix(absDir, absParent), nil
}
