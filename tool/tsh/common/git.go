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
	"os/exec"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
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
	return gitCommands{
		login:  newGitLoginCommand(git),
		list:   newGitListCommand(git),
		ssh:    newGitSSHCommand(git),
		config: newGitConfigCommand(git),
		clone:  newGitCloneCommand(git),
	}
}

type gitSSHURL transport.Endpoint

func (g gitSSHURL) check() error {
	switch {
	case g.isGitHub():
		if err := types.ValidateGitHubOrganizationName(g.owner()); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (g gitSSHURL) isGitHub() bool {
	return g.Host == "github.com"
}

// owner returns the first part of the path. If the path does not have an owner,
// an empty string is returned.
//
// For GitHub, owner is either the user or the organization that owns the repo.
//
// For example, if the SSH url is git@github.com:gravitational/teleport.git, the
// owner would be "gravitational".
func (g gitSSHURL) owner() string {
	// g.Path may have a preceding "/" from url.Parse.
	owner, _, ok := strings.Cut(strings.TrimPrefix(g.Path, "/"), "/")
	if !ok {
		return ""
	}
	return owner
}

// parseGitSSHURL parse a Git SSH URL.
//
// Git URL Spec:
// - spec: https://git-scm.com/docs/git-clone#_git_urls
// - example: ssh://example.org/path/to/repo.git
//
// GitHub (SCP-like) URL:
// - spec: https://docs.github.com/en/get-started/getting-started-with-git/about-remote-repositories
// - example: git@github.com:gravitational/teleport.git
func parseGitSSHURL(originalURL string) (*gitSSHURL, error) {
	endpoint, err := transport.NewEndpoint(originalURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if endpoint.Protocol != "ssh" {
		return nil, trace.BadParameter("unsupported protocol %q. Please provide the SSH URL of the repository.", endpoint.Protocol)
	}
	s := gitSSHURL(*endpoint)
	if err := s.check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &s, nil
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
	const gitExecutable = "git"
	gitPath, err := cf.LookPath(gitExecutable)
	if err != nil {
		return trace.NotFound(`could not locate the executable %q. The following error occurred:
%s

tsh requires that the %q executable to be installed.
You can install it by following the instructions at https://git-scm.com/book/en/v2/Getting-Started-Installing-Git`,
			gitExecutable, err.Error(), gitExecutable)
	}
	logger.DebugContext(cf.Context, "Executing git command", "path", gitPath, "args", args)
	cmd := exec.CommandContext(cf.Context, gitPath, args...)
	cmd.Stdin = cf.Stdin()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return trace.Wrap(cf.RunCommand(cmd))
}
