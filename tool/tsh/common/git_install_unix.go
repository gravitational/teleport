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

//go:build !windows

package common

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/gravitational/trace"
)

const gitRemoteHelperBinary = "git-remote-teleport"

// ensureGitRemoteHelper ensures that git-remote-teleport symlink exists in
// PATH. It tries the directory of the tsh executable first (before resolving
// symlinks, e.g. /usr/local/bin/), then the resolved path. If the directory
// requires elevated permissions on macOS, it uses osascript to prompt for
// admin privileges. Returns an error if the helper cannot be installed.
func ensureGitRemoteHelper(cf *CLIConf) error {
	// Check if already in PATH.
	if _, err := cf.LookPath(gitRemoteHelperBinary); err == nil {
		return nil
	}

	tshPath := cf.executablePath
	installDir := filepath.Dir(tshPath)
	helperPath := filepath.Join(installDir, gitRemoteHelperBinary)

	// Check if it already exists.
	if _, err := os.Stat(helperPath); err == nil {
		return nil
	}

	// Try to create the symlink directly.
	if err := os.Symlink(tshPath, helperPath); err == nil {
		logger.DebugContext(cf.Context, "Created git-remote-teleport symlink",
			"symlink", helperPath,
			"target", tshPath,
		)
		return nil
	}

	// Could not create symlink without elevated permissions.
	// On macOS, use osascript to prompt for admin privileges.
	if runtime.GOOS == "darwin" {
		return trace.Wrap(symlinkWithElevation(cf, tshPath, helperPath))
	}

	return trace.Errorf("%s is a helper that allows git to proxy HTTPS operations through Teleport.\n"+
		"  To install, run: sudo ln -sf %s %s\n"+
		"  To install to a different location (must be a directory in your PATH):\n"+
		"    ln -sf %s <dir-in-PATH>/%s",
		gitRemoteHelperBinary, tshPath, helperPath,
		tshPath, gitRemoteHelperBinary)
}

func symlinkWithElevation(cf *CLIConf, target, linkPath string) error {
	fmt.Fprintf(cf.Stderr(), "%s is a helper that allows git to proxy HTTPS operations through Teleport.\n", gitRemoteHelperBinary)
	fmt.Fprintf(cf.Stderr(), "Symlinking %s -> %s. You may be prompted for your password.\n", linkPath, target)

	script := `on run argv
  do shell script "ln -sf " & quoted form of item 1 of argv & " " & quoted form of item 2 of argv ` +
		`with prompt "Teleport needs to install the git-remote-teleport helper." ` +
		`with administrator privileges
end run`

	cmd := exec.CommandContext(cf.Context, "osascript", "-e", script, target, linkPath)
	cmd.Stderr = cf.Stderr()
	if err := cmd.Run(); err != nil {
		return trace.Errorf("could not install %s.\n"+
			"  To install manually, run: sudo ln -sf %s %s\n"+
			"  To install to a different location: ln -sf %s <dir-in-PATH>/%s",
			gitRemoteHelperBinary, target, linkPath,
			target, gitRemoteHelperBinary)
	}

	fmt.Fprintf(cf.Stderr(), "Installed %s -> %s\n", linkPath, target)
	return nil
}
