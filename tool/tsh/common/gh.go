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
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

// gitHubLocalhost is the GH_HOST of `tsh gh`, which gh treats as a special
// host that uses plain HTTP. `tsh gh` rewrites teleport://github.com/
// remotes to http://github.localhost/ so gh finds the GitHub repository
// from the remotes.
const gitHubLocalhost = "github.localhost"

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
	// For teleport-configured repos, the remote url is teleport://github.com/<owner>/<repo>.
	// However, when gh operates on the repo, it needs the remote url to match GH_HOST.
	// Here the git config is used to translate the teleport:// urls for
	// duration of the gh command.
	configEnv, err := gitConfigEnv(
		fmt.Sprintf("url.http://%s/.insteadOf", gitHubLocalhost),
		gitRemoteHelperScheme+"://github.com/",
	)
	if err != nil {
		return trace.Wrap(err)
	}

	cmd := exec.CommandContext(gitCtx.Context, ghPath, c.args...)
	cmd.Stdin = gitCtx.Stdin()
	cmd.Stdout = gitCtx.Stdout()
	cmd.Stderr = gitCtx.Stderr()
	cmd.Env = append(os.Environ(),
		"GH_HOST="+gitHubLocalhost,
		"GH_TOKEN=teleport",
		"HTTP_PROXY=http://"+proxyAddr,
		"http_proxy=http://"+proxyAddr,
		// Requests must go through the local proxy, e.g. NO_PROXY=localhost
		// matches github.localhost. CONNECT requests are still forwarded with
		// the system proxy settings of tsh.
		"NO_PROXY=",
		"no_proxy=",
	)
	cmd.Env = append(cmd.Env, configEnv...)

	logger.DebugContext(gitCtx.Context, "Running gh", "path", ghPath, "args", c.args, "git_server", gitCtx.gitServer.GetName())
	err = gitCtx.RunCommand(cmd)
	switch {
	case middleware.unauthorized.Load():
		printGitHubCredentialsRejected(gitCtx)
	case err != nil:
		// Print a hint if gh might have failed because of the current repo is
		// not configured by Teleport.
		var remotes bytes.Buffer
		if err := execGitWithStdoutAndStderr(gitCtx.CLIConf, &remotes, io.Discard, "remote", "-v"); err == nil &&
			!strings.Contains(remotes.String(), "teleport://") {
			fmt.Fprintln(gitCtx.Stderr(), `
hint: no Git remote of the current repository is configured with Teleport.
      If this command is for the current repository, run 'tsh git config update' to configure it.`)
		}
	}
	return trace.Wrap(err)
}

// gitConfigEnv returns the environment variables that add the git config key
// and value to the git commands run by a child process, after any existing
// ones.
// https://git-scm.com/docs/git-config#Documentation/git-config.txt-GITCONFIGCOUNT
func gitConfigEnv(key, value string) ([]string, error) {
	var count int
	if existing := os.Getenv("GIT_CONFIG_COUNT"); existing != "" {
		var err error
		if count, err = strconv.Atoi(existing); err != nil || count < 0 {
			return nil, trace.BadParameter("invalid GIT_CONFIG_COUNT %q", existing)
		}
	}
	return []string{
		fmt.Sprintf("GIT_CONFIG_KEY_%d=%s", count, key),
		fmt.Sprintf("GIT_CONFIG_VALUE_%d=%s", count, value),
		fmt.Sprintf("GIT_CONFIG_COUNT=%d", count+1),
	}, nil
}
