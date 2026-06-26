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
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	toolcommon "github.com/gravitational/teleport/tool/common"
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
	if isHTTPSGitURL(c.repository) {
		return c.runHTTPS(cf)
	}
	return c.runSSH(cf)
}

func (c *gitCloneCommand) runSSH(cf *CLIConf) error {
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

func (c *gitCloneCommand) runHTTPS(cf *CLIConf) error {
	org, repoPath, err := parseGitHTTPSURL(c.repository)
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

	valid, _ := hasValidGitCert(tc, gitServer.GetName())
	if !valid {
		if err := ensureGitCredentialsAndCert(cf, tc, gitServer); err != nil {
			return trace.Wrap(err)
		}
	}

	ensureGitRemoteHelper(cf)

	teleportURL := fmt.Sprintf("teleport://github.com/%s", repoPath)
	args := []string{"clone", teleportURL}
	if c.directory != "" {
		args = append(args, c.directory)
	}

	fmt.Fprintf(cf.Stdout(), "Cloning via Teleport git server %q\n", gitServer.GetName())
	if err := execGit(cf, args...); err != nil {
		// git already printed the error to stderr. Use ExitCodeError to
		// exit without printing the error again.
		return trace.Wrap(&toolcommon.ExitCodeError{Code: 1})
	}
	return nil
}

func isHTTPSGitURL(url string) bool {
	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")
}

// parseGitHTTPSURL parses a GitHub HTTPS URL and returns the org and repo path.
// Example: https://github.com/gravitational/teleport.git -> "gravitational", "gravitational/teleport.git"
func parseGitHTTPSURL(rawURL string) (org, repoPath string, err error) {
	for _, prefix := range []string{"https://github.com/", "http://github.com/"} {
		if strings.HasPrefix(rawURL, prefix) {
			repoPath = strings.TrimPrefix(rawURL, prefix)
			parts := strings.SplitN(repoPath, "/", 2)
			if len(parts) < 2 {
				return "", "", trace.BadParameter("invalid GitHub URL %q", rawURL)
			}
			return parts[0], repoPath, nil
		}
	}
	return "", "", trace.BadParameter("unsupported HTTPS URL %q, only github.com is supported", rawURL)
}

// findGitServerByOrg is defined in git_login.go.
