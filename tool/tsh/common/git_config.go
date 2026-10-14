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
	// gitRemoteHelperScheme is the URL scheme of git remotes that go through
	// Teleport over HTTPS, e.g. teleport://github.com/org/repo.git. For these
	// remotes, git runs `git remote-teleport <remote> <url>`, which is an
	// alias of the git repository that runs `tsh git remote-http`.
	gitRemoteHelperScheme = "teleport"
	// gitRemoteHelperAliasKey is the git config of the alias.
	gitRemoteHelperAliasKey = "alias.remote-" + gitRemoteHelperScheme
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
	w := cf.Stdout()
	remotes, err := listGitRemotes(cf)
	switch {
	case err != nil:
		return trace.Wrap(err)
	case len(remotes) == 0:
		fmt.Fprintln(w, "No remote found.")
		return nil
	}

	sshCommand, err := c.getCoreSSHCommand(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	wantPrefix := makeGitCoreSSHCommand(cf.executablePath, "")
	sshOrg, sshConfigured := strings.CutPrefix(sshCommand, wantPrefix)

	var notConfigured bool
	var existingSSHCommand string
	for _, remote := range remotes {
		switch {
		case remote.isGitHub() && remote.hasSeparatePushURLs:
			fmt.Fprintf(w, "Remote %q has a separate push URL, which is not supported by Teleport yet.\n", remote.name)
		case remote.matches(gitRemoteHelperScheme, "github.com"):
			fmt.Fprintf(w, "Remote %q is configured with Teleport over HTTPS.\n", remote.name)
		case remote.matches("https", "github.com"):
			fmt.Fprintf(w, "Remote %q is not configured with Teleport.\n", remote.name)
			notConfigured = true
		case remote.matches("ssh", "github.com"):
			if sshConfigured {
				fmt.Fprintf(w, "Remote %q is configured with Teleport over SSH for GitHub organization %q.\n", remote.name, sshOrg)
				continue
			}
			fmt.Fprintf(w, "Remote %q is not configured with Teleport.\n", remote.name)
			notConfigured = true
			existingSSHCommand = sshCommand
		default:
			fmt.Fprintf(w, "Remote %q is not supported by Teleport.\n", remote.name)
		}
	}

	if notConfigured {
		if existingSSHCommand != "" {
			fmt.Fprintf(w, "%q currently has value %q.\n", gitCoreSSHCommand, existingSSHCommand)
			fmt.Fprintf(w, "Run 'tsh git config update' to configure Git directory with Teleport but %q will be overwritten.\n", gitCoreSSHCommand)
		} else {
			fmt.Fprintln(w, "Run 'tsh git config update' to configure it.")
		}
	}
	return nil
}

func (c *gitConfigCommand) doUpdate(cf *CLIConf) error {
	remotes, err := listGitRemotes(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	// TODO(greedy52) support multiple remotes and push URLs, e.g. a fork of
	// another owner. Fetch the git servers to check whether the organization
	// of each remote is served by Teleport and which protocols it allows.
	// Consider offering a cross-protocol update (e.g. SSH to HTTPS) with a
	// warning that reset cannot restore the original URL, or printing
	// instructions to run `git remote set-url` first.
	const singleRemoteHint = "Make sure the repository has a single remote with an SSH or https://github.com URL and no separate push URL, " +
		"then run 'tsh git config update' again."
	switch len(remotes) {
	case 0:
		fmt.Fprintln(cf.Stdout(), "No remote found.")
		return nil
	case 1:
	default:
		return trace.BadParameter("multiple remotes are not supported yet. %s", singleRemoteHint)
	}

	remote := remotes[0]
	switch {
	case remote.isGitHub() && remote.hasSeparatePushURLs:
		return trace.BadParameter("remote %q has a separate push URL, which is not supported yet. %s", remote.name, singleRemoteHint)
	case remote.matches(gitRemoteHelperScheme, "github.com"), remote.matches("https", "github.com"):
		return trace.Wrap(c.updateHTTPSRemote(cf, remote))
	case remote.matches("ssh", "github.com"):
		return trace.Wrap(c.updateSSHRemote(cf, remote))
	default:
		return trace.BadParameter("remote %q is not supported by Teleport. %s", remote.name, singleRemoteHint)
	}
}

// updateSSHRemote configures core.sshCommand for the GitHub organization of
// the remote.
func (c *gitConfigCommand) updateSSHRemote(cf *CLIConf, remote gitRemote) error {
	args := []string{
		"config", "--local",
		"--replace-all", gitCoreSSHCommand,
		makeGitCoreSSHCommand(cf.executablePath, remote.owner()),
	}
	if err := execGit(cf, args...); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(cf.Stdout(), "Remote %q is updated to use Teleport over SSH.\n", remote.name)
	return nil
}

// resetSSHRemotes removes core.sshCommand set by Teleport, and returns true if
// it was set.
func (c *gitConfigCommand) resetSSHRemotes(cf *CLIConf) (bool, error) {
	sshCommand, err := c.getCoreSSHCommand(cf)
	if err != nil {
		return false, trace.Wrap(err)
	}
	wantPrefix := makeGitCoreSSHCommand(cf.executablePath, "")
	if !strings.HasPrefix(sshCommand, wantPrefix) {
		return false, nil
	}
	if err := execGit(cf, "config", "--local", "--unset-all", gitCoreSSHCommand); err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
}

func (c *gitConfigCommand) doReset(cf *CLIConf) error {
	httpsReset, err := c.resetHTTPSRemotes(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	sshReset, err := c.resetSSHRemotes(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	if !sshReset && !httpsReset {
		fmt.Fprintln(cf.Stdout(), "The current Git directory is not configured with Teleport.")
		return nil
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

func (c *gitConfigCommand) updateHTTPSRemote(cf *CLIConf, remote gitRemote) error {
	profile, err := cf.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkSessionEncryptionKey(profile); err != nil {
		return trace.Wrap(err)
	}
	for _, args := range [][]string{
		{"remote", "set-url", remote.name, gitRemoteHelperScheme + "://github.com/" + remote.repoPath()},
		{"config", "--local", gitRemoteHelperAliasKey, makeGitRemoteHelperAlias(cf.executablePath, profile.ProxyURL.Host)},
	} {
		if err := execGit(cf, args...); err != nil {
			return trace.Wrap(err)
		}
	}
	fmt.Fprintf(cf.Stdout(), "Remote %q is updated to use Teleport over HTTPS.\n", remote.name)
	return nil
}

func (c *gitConfigCommand) resetHTTPSRemotes(cf *CLIConf) (reset bool, err error) {
	remotes, err := listGitRemotes(cf)
	if err != nil {
		return false, trace.Wrap(err)
	}
	for _, remote := range remotes {
		if remote.matches(gitRemoteHelperScheme, "github.com") {
			if err := c.resetHTTPSRemote(cf, remote); err != nil {
				return false, trace.Wrap(err)
			}
			reset = true
		}
	}
	if reset {
		if err := execGit(cf, "config", "--local", "--unset", gitRemoteHelperAliasKey); err != nil {
			return false, trace.Wrap(err)
		}
	}
	return reset, nil
}

func (c *gitConfigCommand) resetHTTPSRemote(cf *CLIConf, remote gitRemote) error {
	httpsURL := "https://github.com/" + remote.repoPath()
	if err := execGit(cf, "remote", "set-url", remote.name, httpsURL); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(cf.Stdout(), "Remote %q is reset to %s.\n", remote.name, httpsURL)
	return nil
}

// makeGitRemoteHelperAlias returns the value of the git alias that runs the
// remote helper. For teleport://<address> remotes, git runs
// `git remote-teleport <remote> <url>`, and without a git-remote-teleport
// executable, git runs the alias of the same name.
//
// https://git-scm.com/docs/gitremote-helpers#_invocation
// https://git-scm.com/docs/git-config#Documentation/git-config.txt-alias
//
// The alias pins the Teleport proxy, so the remote helper always uses the
// cluster that configured the repository.
func makeGitRemoteHelperAlias(tshBin, proxyAddr string) string {
	// Quote the path in case it has spaces
	return fmt.Sprintf("!\"%s\" --proxy=%s git remote-http", tshBin, proxyAddr)
}
