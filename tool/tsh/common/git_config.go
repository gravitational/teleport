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
	"io"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

// gitConfigCommand implements `tsh git config`.
//
// This command internally executes `git` commands like `git config xxx`.
// can generally assume the user has `git` binary installed (otherwise there is
// no point using the `git` proxy feature).
//
// TODO(greedy52) investigate using `go-git` library instead of calling `git
// config`.
type gitConfigCommand struct {
	*kingpin.CmdClause

	action string
}

const (
	gitConfigActionDefault = ""
	gitConfigActionUpdate  = "update"
	gitConfigActionReset   = "reset"

	// gitCoreSSHCommand is the Git config used for setting up alternative SSH
	// command. For Git-proxying, the command should point to "tsh git ssh".
	//
	// https://git-scm.com/docs/git-config#Documentation/git-config.txt-coresshCommand
	gitCoreSSHCommand = "core.sshcommand"
)

func newGitConfigCommand(parent *kingpin.CmdClause) *gitConfigCommand {
	cmd := &gitConfigCommand{
		CmdClause: parent.Command("config", "Check Teleport config on the working Git directory. Or provide an action ('update' or 'reset') to configure the Git repo."),
	}

	cmd.Arg("action", "Optional action to perform. 'update' to configure the Git repo to proxy Git commands through Teleport. 'reset' to clear Teleport configuration from the Git repo.").
		EnumVar(&cmd.action, gitConfigActionUpdate, gitConfigActionReset)
	return cmd
}

func (c *gitConfigCommand) run(cf *CLIConf) error {
	// Make sure we are in a Git dir.
	err := execGitWithStdoutAndStderr(cf, io.Discard, io.Discard, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		// In case git is not found, return the look path error.
		if trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		// This error message is a slight alternation of the original error
		// message from the above command.
		return trace.BadParameter("the current directory is not a Git repository (or any of the parent directories)")
	}

	switch c.action {
	case gitConfigActionDefault:
		return trace.Wrap(c.doCheck(cf))
	case gitConfigActionUpdate:
		return trace.Wrap(c.doUpdate(cf))
	case gitConfigActionReset:
		return trace.Wrap(c.doReset(cf))
	default:
		return trace.BadParameter("unknown action '%v'", c.action)
	}
}

func (c *gitConfigCommand) doCheck(cf *CLIConf) error {
	sshCommand, err := c.getCoreSSHCommand(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	wantPrefix := makeGitCoreSSHCommand(cf.executablePath, "")
	if strings.HasPrefix(sshCommand, wantPrefix) {
		_, org, _ := strings.Cut(sshCommand, wantPrefix)
		fmt.Fprintf(cf.Stdout(), "The current Git directory is configured with Teleport for GitHub organization %q.\n", org)
		return nil
	}

	c.printDirNotConfigured(cf.Stdout(), true, sshCommand)
	return nil
}

func (c *gitConfigCommand) printDirNotConfigured(w io.Writer, withUpdate bool, existingSSHCommand string) {
	fmt.Fprintln(w, "The current Git directory is not configured with Teleport.")
	if withUpdate {
		if existingSSHCommand != "" {
			fmt.Fprintf(w, "%q currently has value %q.\n", gitCoreSSHCommand, existingSSHCommand)
			fmt.Fprintf(w, "Run 'tsh git config update' to configure Git directory with Teleport but %q will be overwritten.\n", gitCoreSSHCommand)
		} else {
			fmt.Fprintln(w, "Run 'tsh git config update' to configure it.")
		}
	}
}

func (c *gitConfigCommand) doUpdate(cf *CLIConf) error {
	urls, err := execGitAndCaptureStdout(cf, "ls-remote", "--get-url")
	if err != nil {
		return trace.Wrap(err)
	}
	for _, url := range strings.Split(urls, "\n") {
		u, err := parseGitSSHURL(url)
		if err != nil {
			logger.DebugContext(cf.Context, "Skipping URL", "error", err, "url", url)
			continue
		}
		if !u.isGitHub() {
			logger.DebugContext(cf.Context, "Skipping non-GitHub host", "host", u.Host)
			continue
		}

		logger.DebugContext(cf.Context, "Configuring repo to use tsh.", "url", url, "owner", u.owner())
		args := []string{
			"config", "--local",
			"--replace-all", gitCoreSSHCommand,
			makeGitCoreSSHCommand(cf.executablePath, u.owner()),
		}
		if err := execGit(cf, args...); err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintln(cf.Stdout(), "Teleport configuration added.")
		return trace.Wrap(c.doCheck(cf))
	}
	return trace.NotFound("no GitHub SSH URL found from 'git ls-remote --get-url'")
}

func (c *gitConfigCommand) doReset(cf *CLIConf) error {
	sshCommand, err := c.getCoreSSHCommand(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	wantPrefix := makeGitCoreSSHCommand(cf.executablePath, "")
	if !strings.HasPrefix(sshCommand, wantPrefix) {
		c.printDirNotConfigured(cf.Stdout(), false, sshCommand)
		return nil
	}

	if err := execGit(cf, "config", "--local", "--unset-all", gitCoreSSHCommand); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintln(cf.Stdout(), "Teleport configuration removed.")
	return nil
}

func (c *gitConfigCommand) getCoreSSHCommand(cf *CLIConf) (string, error) {
	return execGitAndCaptureStdout(cf,
		"config", "--local",
		// set default to empty to avoid non-zero exit when config is missing
		"--default", "",
		"--get", gitCoreSSHCommand,
	)
}

// makeGitCoreSSHCommand generates the value for Git config "core.sshcommand".
func makeGitCoreSSHCommand(tshBin, githubOrg string) string {
	// Quote the path in case it has spaces
	return fmt.Sprintf("\"%s\" git ssh --github-org %s",
		tshBin,
		githubOrg,
	)
}
