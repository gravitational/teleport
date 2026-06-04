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

package common

import (
	"fmt"
	"os"
	"path/filepath"
)

const gitRemoteHelperBinary = "git-remote-teleport"

// ensureGitRemoteHelper ensures that git-remote-teleport symlink exists in the
// same directory as the tsh binary. If it doesn't exist, it creates it. If the
// directory requires elevated permissions, it prints instructions instead.
func ensureGitRemoteHelper(cf *CLIConf) {
	tshPath, err := os.Executable()
	if err != nil {
		logger.DebugContext(cf.Context, "Could not determine tsh path", "error", err)
		return
	}
	tshPath, err = filepath.EvalSymlinks(tshPath)
	if err != nil {
		logger.DebugContext(cf.Context, "Could not resolve tsh symlinks", "error", err)
		return
	}

	tshDir := filepath.Dir(tshPath)
	helperPath := filepath.Join(tshDir, gitRemoteHelperBinary)

	// Check if it already exists and points to the right place.
	if target, err := os.Readlink(helperPath); err == nil {
		if target == tshPath {
			return
		}
	}

	// Check if it exists as a regular file (not symlink).
	if _, err := os.Stat(helperPath); err == nil {
		return
	}

	// Try to create the symlink.
	if err := os.Symlink(tshPath, helperPath); err != nil {
		fmt.Fprintf(cf.Stderr(), "Note: could not create %s symlink automatically.\n", gitRemoteHelperBinary)
		fmt.Fprintf(cf.Stderr(), "To enable 'teleport://' git URLs, run:\n")
		fmt.Fprintf(cf.Stderr(), "  sudo ln -sf %s %s\n\n", tshPath, helperPath)
		return
	}

	logger.DebugContext(cf.Context, "Created git-remote-teleport symlink",
		"symlink", helperPath,
		"target", tshPath,
	)
}
