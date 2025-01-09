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

package common

import (
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

// gitCloneCommand implements `tsh git clone`.
//
// This command internally executes `git clone` while setting `core.sshcommand`.
// You can generally assume the user has `git` binary installed (otherwise there
// is no point using the `git` proxy feature).
//
// TODO(greedy52) investigate using `go-git` library instead of calling `git
// clone`.
type gitCloneCommand struct {
	*kingpin.CmdClause

	repository string
	directory  string
}

func newGitCloneCommand(parent *kingpin.CmdClause) *gitCloneCommand {
	cmd := &gitCloneCommand{
		CmdClause: parent.Command("clone", "Clone a Git repository."),
	}

	cmd.Arg("repository", "Git URL of the repository to clone.").Required().StringVar(&cmd.repository)
	cmd.Arg("directory", "The name of a new directory to clone into.").StringVar(&cmd.directory)
	// TODO(greedy52) support passing extra args to git like --branch/--depth.
	return cmd
}

func (c *gitCloneCommand) run(cf *CLIConf) error {
	u, err := parseGitSSHURL(c.repository)
	if err != nil {
		return trace.Wrap(err)
	}
	if !u.isGitHub() {
		return trace.BadParameter("%s is not a GitHub repository", c.repository)
	}

	sshCommand := makeGitCoreSSHCommand(cf.executablePath, u.owner())
	args := []string{
		"clone",
		"--config", fmt.Sprintf("%s=%s", gitCoreSSHCommand, sshCommand),
		c.repository,
	}
	if c.directory != "" {
		args = append(args, c.directory)
	}
	return trace.Wrap(execGit(cf, args...))
}
