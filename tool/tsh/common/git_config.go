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

	"github.com/gravitational/teleport/api/types"
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
	configured := false

	sshCommand, err := c.getCoreSSHCommand(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	wantPrefix := makeGitCoreSSHCommand(cf.executablePath, "")
	if strings.HasPrefix(sshCommand, wantPrefix) {
		_, org, _ := strings.Cut(sshCommand, wantPrefix)
		fmt.Fprintf(cf.Stdout(), "Git SSH is configured with Teleport for GitHub organization %q.\n", org)
		configured = true
	}

	remoteURL, err := execGitAndCaptureStdout(cf, "config", "--local", "--default", "", "--get", "remote.origin.url")
	if err == nil && strings.HasPrefix(remoteURL, "teleport://") {
		fmt.Fprintf(cf.Stdout(), "Git HTTPS is configured with Teleport remote %q.\n", remoteURL)
		configured = true
	}

	if !configured {
		c.printDirNotConfigured(cf.Stdout(), true, sshCommand)
	}
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
	url, err := execGitAndCaptureStdout(cf, "ls-remote", "--get-url")
	if err != nil {
		return trace.Wrap(err)
	}
	url = strings.TrimSpace(url)

	if isHTTPSGitURL(url) {
		return trace.Wrap(c.doUpdateHTTPS(cf, url))
	}

	u, err := parseGitSSHURL(url)
	if err != nil {
		return trace.BadParameter("unsupported remote URL %q", url)
	}
	if !u.isGitHub() {
		return trace.BadParameter("unsupported non-GitHub host %q", u.Host)
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	gitServer, err := findGitServerByOrg(cf, tc, u.owner())
	if err != nil {
		return trace.Wrap(err)
	}

	github := gitServer.GetGitHub()
	if github == nil {
		return trace.BadParameter("git server %v is not a GitHub server", gitServer.GetName())
	}
	if !types.GitServerSSHEnabled(github) {
		return trace.BadParameter("git server %v does not have SSH proxying enabled", gitServer.GetName())
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

func (c *gitConfigCommand) doUpdateHTTPS(cf *CLIConf, rawURL string) error {
	org, repoPath, err := parseGitHTTPSURL(rawURL)
	if err != nil {
		return trace.Wrap(err)
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	gitServer, err := findGitServerByOrg(cf, tc, org)
	if err != nil {
		return trace.Wrap(err)
	}

	github := gitServer.GetGitHub()
	if github == nil || !types.GitServerHTTPEnabled(github) {
		return trace.BadParameter("git server %v does not have HTTP proxying enabled", gitServer.GetName())
	}

	teleportURL := fmt.Sprintf("teleport://github.com/%s", repoPath)
	if err := execGit(cf, "remote", "set-url", "origin", teleportURL); err != nil {
		return trace.Wrap(err)
	}

	ensureGitRemoteHelper(cf)
	fmt.Fprintf(cf.Stdout(), "Remote origin rewritten to %q.\n", teleportURL)
	return trace.Wrap(c.doCheck(cf))
}

func (c *gitConfigCommand) doReset(cf *CLIConf) error {
	resetSSH := false
	sshCommand, err := c.getCoreSSHCommand(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	wantPrefix := makeGitCoreSSHCommand(cf.executablePath, "")
	if strings.HasPrefix(sshCommand, wantPrefix) {
		if err := execGit(cf, "config", "--local", "--unset-all", gitCoreSSHCommand); err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintln(cf.Stdout(), "Teleport SSH configuration removed.")
		resetSSH = true
	}

	resetHTTPS := false
	remoteURL, err := execGitAndCaptureStdout(cf, "config", "--local", "--default", "", "--get", "remote.origin.url")
	if err == nil {
		remoteURL = strings.TrimSpace(remoteURL)
		if strings.HasPrefix(remoteURL, "teleport://") {
			httpsURL := teleportURLToHTTPS(remoteURL)
			if err := execGit(cf, "remote", "set-url", "origin", httpsURL); err != nil {
				return trace.Wrap(err)
			}
			fmt.Fprintf(cf.Stdout(), "Remote origin restored to %q.\n", httpsURL)
			resetHTTPS = true
		}
	}

	if !resetSSH && !resetHTTPS {
		c.printDirNotConfigured(cf.Stdout(), false, sshCommand)
	}
	return nil
}

// teleportURLToHTTPS converts teleport://github.com/org/repo.git back to
// https://github.com/org/repo.git.
func teleportURLToHTTPS(teleportURL string) string {
	return strings.Replace(teleportURL, "teleport://", "https://", 1)
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
