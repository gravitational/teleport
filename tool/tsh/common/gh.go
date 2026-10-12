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
	"os"
	"os/exec"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type ghCommand struct {
	*kingpin.CmdClause

	gitHubOrganization string
	args               []string
}

func newGHCommand(app *kingpin.Application) *ghCommand {
	cmd := &ghCommand{
		CmdClause: app.Command("gh", "Run gh CLI commands through Teleport.").Interspersed(false),
	}
	cmd.Flag("github-org", "GitHub organization. Required when more than one GitHub organization is available.").StringVar(&cmd.gitHubOrganization)
	cmd.Alias(ghHelp)
	cmd.Arg("args", "Arguments for the gh CLI, after '--'.").StringsVar(&cmd.args)
	return cmd
}

func (c *ghCommand) run(cf *CLIConf) error {
	ghPath, err := cf.LookPath("gh")
	if err != nil {
		return trace.NotFound("could not locate the gh CLI, see https://cli.github.com for installation: %v", err)
	}

	gitCtx, err := newGitContextForGitHubOrg(cf, c.gitHubOrganization)
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
	return trace.Wrap(c.runGH(gitCtx, ghPath, proxy.GetAddr(), proxy.middleware))
}

// runGH runs "gh" with GH_HOST=github.localhost, which gh treats as a special
// host that uses plain HTTP, and HTTP_PROXY pointing to the local git HTTP
// proxy, which forwards the requests to Teleport over TLS. GH_TOKEN is a dummy
// token so gh does not start its own authentication flow.
func (c *ghCommand) runGH(gitCtx *gitContext, ghPath, proxyAddr string, middleware *gitHTTPMiddleware) error {
	cmd := exec.CommandContext(gitCtx.Context, ghPath, c.args...)
	cmd.Stdin = gitCtx.Stdin()
	cmd.Stdout = gitCtx.Stdout()
	cmd.Stderr = gitCtx.Stderr()
	cmd.Env = append(os.Environ(),
		"GH_HOST=github.localhost",
		"GH_TOKEN=teleport",
		"HTTP_PROXY=http://"+proxyAddr,
		"http_proxy=http://"+proxyAddr,
		// Requests must go through the local proxy, e.g. NO_PROXY=localhost
		// matches github.localhost.
		"NO_PROXY=",
		"no_proxy=",
	)
	logger.DebugContext(gitCtx.Context, "Running gh", "path", ghPath, "args", c.args, "git_server", gitCtx.gitServer.GetName())
	err := gitCtx.RunCommand(cmd)
	if middleware.unauthorized.Load() {
		printGitHubCredentialsRejected(gitCtx)
	}
	return trace.Wrap(err)
}
