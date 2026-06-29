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

	"github.com/gravitational/trace"
)

const gitRemoteHelperBinary = "git-remote-teleport"

// ensureGitRemoteHelper ensures that git-remote-teleport.cmd exists in the
// same directory as the tsh binary. Git on Windows looks for
// git-remote-<scheme>.cmd when handling custom URL schemes. The .cmd wrapper
// delegates to "tsh git http-remote".
func ensureGitRemoteHelper(cf *CLIConf) error {
	tshPath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "could not determine tsh path")
	}
	tshPath, err = filepath.EvalSymlinks(tshPath)
	if err != nil {
		return trace.Wrap(err, "could not resolve tsh path")
	}

	tshDir := filepath.Dir(tshPath)
	helperPath := filepath.Join(tshDir, gitRemoteHelperBinary+".cmd")

	if _, err := os.Stat(helperPath); err == nil {
		return nil
	}

	// The .cmd wrapper passes all arguments through to tsh git remote-http.
	// %* forwards all arguments from the .cmd invocation.
	content := fmt.Sprintf("@\"%s\" git remote-http %%*\r\n", tshPath)

	if err := os.WriteFile(helperPath, []byte(content), 0755); err != nil {
		return trace.Errorf("could not create %s.cmd; create %s manually with: %s",
			gitRemoteHelperBinary, helperPath, content)
	}

	logger.DebugContext(cf.Context, "Created git-remote-teleport.cmd wrapper",
		"path", helperPath,
		"tsh", tshPath,
	)
	return nil
}
