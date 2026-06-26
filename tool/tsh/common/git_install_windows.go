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

//go:build windows

package common

import (
	"fmt"
	"os"
	"path/filepath"
)

const gitRemoteHelperBinary = "git-remote-teleport"

// ensureGitRemoteHelper ensures that git-remote-teleport.cmd exists in the
// same directory as the tsh binary. Git on Windows looks for
// git-remote-<scheme>.cmd when handling custom URL schemes. The .cmd wrapper
// delegates to "tsh git http-remote".
func ensureGitRemoteHelper(cf *CLIConf) {
	tshPath, err := os.Executable()
	if err != nil {
		logger.DebugContext(cf.Context, "Could not determine tsh path", "error", err)
		return
	}
	tshPath, err = filepath.EvalSymlinks(tshPath)
	if err != nil {
		logger.DebugContext(cf.Context, "Could not resolve tsh path", "error", err)
		return
	}

	tshDir := filepath.Dir(tshPath)
	helperPath := filepath.Join(tshDir, gitRemoteHelperBinary+".cmd")

	if _, err := os.Stat(helperPath); err == nil {
		return
	}

	// The .cmd wrapper passes all arguments through to tsh git http-remote.
	// %* forwards all arguments from the .cmd invocation.
	content := fmt.Sprintf("@\"%s\" git http-remote %%*\r\n", tshPath)

	if err := os.WriteFile(helperPath, []byte(content), 0755); err != nil {
		fmt.Fprintf(cf.Stderr(), "Note: could not create %s.cmd automatically.\n", gitRemoteHelperBinary)
		fmt.Fprintf(cf.Stderr(), "To enable 'teleport://' git URLs, create %s with:\n", helperPath)
		fmt.Fprintf(cf.Stderr(), "  %s\n\n", content)
		return
	}

	logger.DebugContext(cf.Context, "Created git-remote-teleport.cmd wrapper",
		"path", helperPath,
		"tsh", tshPath,
	)
}
