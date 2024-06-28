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
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"

	apiawsutils "github.com/gravitational/teleport/api/utils/aws"
)

func onGitRemote(cf *CLIConf) error {
	slog.InfoContext(cf.Context, "onGitRemote", "origin", cf.GitOrigin, "url", cf.GitURL)

	gitURL, err := url.Parse(cf.GitURL)
	if err != nil {
		return trace.Wrap(err)
	}

	if gitURL.User != nil {
		cf.AppName = gitURL.User.Username()
		if password, ok := gitURL.User.Password(); ok {
			cf.SiteName = password
		}
		gitURL.User = nil
	}

	if isCodeCommitURL(gitURL) {
		return trace.Wrap(onGitRemoteCodeCommit(cf, gitURL))
	}
	return trace.NotImplemented("unsupported %v", cf.GitURL)
}

func onGitRemoteCodeCommit(cf *CLIConf, codeCommitURL *url.URL) error {
	// TODO local proxy for per-session MFA and proxy behind L7 load balancer?
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	// Check if app cert exists and prompt for app login if necessary.
	if _, _, err := loadAppCertificate(tc, cf.AppName); err != nil {
		return trace.Wrap(err)
	}

	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	gitConfigs := &gitConfigs{
		sslCert:      profile.AppCertPath(tc.SiteName, cf.AppName),
		sslKey:       profile.KeyPath(),
		extraHeaders: []string{"X-Teleport-Original-Git-Url: " + codeCommitURL.String()},
	}

	runGitCommandAndExit(cf.Context, gitConfigs.envs(), "remote-http", cf.GitOrigin, "https://"+tc.WebProxyAddr+codeCommitURL.Path)
	return nil
}

func runGitCommandAndExit(ctx context.Context, extraEnv []string, commands ...string) {
	// TODO investigate why os.Environ() is required
	cmd := exec.Command("git", commands...)
	cmd.Env = append(os.Environ(), extraEnv...)
	slog.DebugContext(ctx, "Calling git.", "env", cmd.Env, "args", strings.Join(cmd.Args, " "))
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		slog.ErrorContext(ctx, "Git exited with error.", "error", err)
	}
	if cmd.ProcessState != nil {
		os.Exit(cmd.ProcessState.ExitCode())
	}
}

// TODO move to api/utils/aws?
// https://git-codecommit.ca-central-1.amazonaws.com/v1/repos/steve-codecommit
func isCodeCommitURL(u *url.URL) bool {
	return u.Scheme == "https" &&
		apiawsutils.IsAWSEndpoint(u.Host) &&
		strings.HasPrefix(u.Host, "git-codecommit")
}

// TODO move to a proper lib like lib/client/git
type gitConfigs struct {
	sslCert      string
	sslKey       string
	extraHeaders []string
	// TODO insecure, local proxy
}

func (g *gitConfigs) envs() []string {
	envs := []string{
		"GIT_SSL_CERT=" + g.sslCert,
		"GIT_SSL_KEY=" + g.sslKey,
	}
	envs = append(envs, fmt.Sprintf("GIT_CONFIG_COUNT=%d", len(g.extraHeaders)))
	for i, header := range g.extraHeaders {
		envs = append(envs,
			fmt.Sprintf("GIT_CONFIG_KEY_%d=http.extraHeader", i),
			fmt.Sprintf("GIT_CONFIG_VALUE_%d=%s", i, header),
		)
	}
	return envs
}
