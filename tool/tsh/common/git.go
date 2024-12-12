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
	"bytes"
	"io"
	"net/url"
	"os/exec"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type gitCommands struct {
	list   *gitListCommand
	login  *gitLoginCommand
	ssh    *gitSSHCommand
	config *gitConfigCommand
	clone  *gitCloneCommand
}

func newGitCommands(app *kingpin.Application) gitCommands {
	git := app.Command("git", "Git server commands.")
	cmds := gitCommands{
		login:  newGitLoginCommand(git),
		list:   newGitListCommand(git),
		ssh:    newGitSSHCommand(git),
		config: newGitConfigCommand(git),
		clone:  newGitCloneCommand(git),
	}

	// TODO(greedy52) hide the commands until all basic features are implemented.
	git.Hidden()
	cmds.login.Hidden()
	cmds.list.Hidden()
	cmds.config.Hidden()
	cmds.clone.Hidden()
	return cmds
}

type gitSSHURL struct {
	user string
	host string
	port string
	path string
	// owner is the first part of the path.
	// For GitHub, owner is either the user or the organization that owns the
	// repo.
	owner string
}

func (g gitSSHURL) isGitHub() bool {
	return g.host == "github.com"
}

// parseGitSSHURL parse a Git SSH URL.
//
// Normal Git URL Spec: https://git-scm.com/docs/git-clone#_git_urls
// Example: ssh://example.org/path/to/repo.git
//
// GitHub URL Spec: https://docs.github.com/en/get-started/getting-started-with-git/about-remote-repositories
// Example: git@github.com:gravitational/teleport.git
func parseGitSSHURL(originalURL string) (*gitSSHURL, error) {
	if strings.HasPrefix(originalURL, "ssh://") {
		u, err := url.Parse(originalURL)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		path := strings.TrimLeft(u.Path, "/")
		owner, _, _ := strings.Cut(path, "/")
		gitSSHURL := &gitSSHURL{
			path:  path,
			host:  u.Hostname(),
			port:  u.Port(),
			owner: owner,
		}
		if u.User != nil {
			gitSSHURL.user = u.User.Username()
		}
		return gitSSHURL, nil
	}

	if strings.Contains(originalURL, "@github.com:") {
		return parseGitHubSSHURL(originalURL)
	}
	return nil, trace.BadParameter("unsupported git ssh URL %s", originalURL)
}

func parseGitHubSSHURL(originalURL string) (*gitSSHURL, error) {
	user, hostAndMore, ok := strings.Cut(originalURL, "@")
	if !ok {
		return nil, trace.BadParameter("invalid git ssh URL %s", originalURL)
	}
	host, path, ok := strings.Cut(hostAndMore, ":")
	if !ok {
		return nil, trace.BadParameter("invalid git ssh URL %s", originalURL)
	}
	owner, _, ok := strings.Cut(path, "/")
	if !ok {
		return nil, trace.BadParameter("invalid git ssh URL %s", originalURL)
	}
	return &gitSSHURL{
		user:  user,
		host:  host,
		owner: owner,
		path:  path,
	}, nil
}

func execGitAndCaptureStdout(cf *CLIConf, args ...string) (string, error) {
	var bufStd bytes.Buffer
	if err := execGitWithStdoutAndStderr(cf, &bufStd, cf.Stderr(), args...); err != nil {
		return "", trace.Wrap(err)
	}
	return strings.TrimSpace(bufStd.String()), nil
}

func execGit(cf *CLIConf, args ...string) error {
	return trace.Wrap(execGitWithStdoutAndStderr(cf, cf.Stdout(), cf.Stderr(), args...))
}

func execGitWithStdoutAndStderr(cf *CLIConf, stdout, stderr io.Writer, args ...string) error {
	log.Debugf("Executing 'git' with args: %v", args)
	cmd := exec.CommandContext(cf.Context, "git", args...)
	cmd.Stdin = cf.Stdin()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return trace.Wrap(cf.RunCommand(cmd))
}
