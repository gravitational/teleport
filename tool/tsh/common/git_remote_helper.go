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

package common

import (
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/utils"
)

// gitRemoteHTTPCommand implements `tsh git remote-http`, which git runs for
// teleport://github.com/org/repo.git remotes. Internally, it calls `git
// remote-http` with appropriate flags and environment variables while hosting a
// local proxy that forwards the traffic to backend.
type gitRemoteHTTPCommand struct {
	*kingpin.CmdClause

	remote string
	url    string
}

func newGitRemoteHTTPCommand(parent *kingpin.CmdClause) *gitRemoteHTTPCommand {
	cmd := &gitRemoteHTTPCommand{
		CmdClause: parent.Command("remote-http", "Git remote helper for Git over HTTPS through Teleport.").Hidden(),
	}
	cmd.Arg("remote", "Name of the git remote.").Required().StringVar(&cmd.remote)
	cmd.Arg("url", "URL of the git remote.").Required().StringVar(&cmd.url)
	return cmd
}

func (c *gitRemoteHTTPCommand) run(cf *CLIConf) (err error) {
	// This command is invoked by git, try to prompt from the terminal instead.
	prompt.EnableStdinTerminalFallback()
	defer func() {
		if utils.IsCertExpiredError(err) {
			err = trace.AccessDenied("Your Teleport session has expired. Please login using 'tsh login'.")
		}
	}()

	u, err := parseGitURL(c.url)
	if err != nil {
		return trace.Wrap(err)
	}
	if !u.matches(gitRemoteHelperScheme, "github.com") {
		return trace.BadParameter("expecting %s://github.com/<org>/<repo> but got %q", gitRemoteHelperScheme, c.url)
	}

	gitCtx, err := newGitContextForGitHubOrg(cf, u.owner())
	if err != nil {
		return trace.Wrap(err)
	}
	defer gitCtx.close()

	if err := gitCtx.checkGitOverHTTPS(); err != nil {
		return trace.Wrap(err)
	}

	proxy, err := startGitHTTPProxy(gitCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxy.Close()

	// The GitHub credentials come from Teleport. When GitHub rejects them, fail
	// instead of prompting for a password or running credential helpers.
	proxyURL := fmt.Sprintf("http://%s", proxy.GetAddr())
	env := []string{
		"GIT_TERMINAL_PROMPT=0",
		// Requests must go through the local proxy, e.g. NO_PROXY=github.com
		// would bypass it.
		"NO_PROXY=",
		"no_proxy=",
	}
	err = execGitWithEnv(cf, cf.Stdout(), cf.Stderr(), env,
		"-c", "credential.helper=",
		"-c", fmt.Sprintf("http.proxy=%s", proxyURL),
		// Per-remote proxy settings take precedence over http.proxy.
		"-c", fmt.Sprintf("remote.%s.proxy=%s", c.remote, proxyURL),
		"remote-http", c.remote, fmt.Sprintf("http://github.com/%s", u.repoPath()),
	)
	if proxy.middleware.unauthorized.Load() {
		printGitHubCredentialsRejected(gitCtx)
	}
	return trace.Wrap(err)
}
