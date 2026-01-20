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

package common

import (
	"os"
	"os/exec"

	"github.com/gravitational/trace"
)

const (
	envTSHClaudeSession = "TSH_CLAUDE_SESSION"
)

func runClaude(cf *CLIConf) error {
	// TODO(greedy52) read profiles
	os.Setenv(envTSHClaudeSession, "true")

	cmd := exec.CommandContext(cf.Context,
		"claude",
		// TODO(greedy52) dump the plugin somewhere and use it instead of hard-coding the poc dir
		"--plugin-dir", "tsh-claude-poc",
	)
	cmd.Stdin = cf.Stdin()
	cmd.Stdout = cf.Stdout()
	cmd.Stderr = cf.Stderr()
	return trace.Wrap(cmd.Run())
}
