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
	"os"
	"os/exec"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

// gitHTTPRemoteCommand implements `tsh git remote-helper`, which is invoked
// by git as `git-remote-teleport <remote> <url>`.
//
// When a git remote uses the "teleport://" scheme, git invokes
// git-remote-teleport (a symlink to tsh) with two args: the remote name and
// the URL. This command starts the local proxy infrastructure and delegates to
// git's built-in HTTP transport via `git remote-http`.
//
// Example:
//
//	git clone teleport://github.com/org/repo.git
//	# git invokes: git-remote-teleport origin teleport://github.com/org/repo.git
//	# tsh starts proxies, then runs: git remote-http origin https://github.com/org/repo.git
type gitHTTPRemoteCommand struct {
	*kingpin.CmdClause

	gitCmd    string
	remoteURL string
}

func newGitHTTPRemoteCommand(parent *kingpin.CmdClause) *gitHTTPRemoteCommand {
	cmd := &gitHTTPRemoteCommand{
		CmdClause: parent.Command("remote-http", "Git remote helper for teleport:// URLs (internal).").Hidden(),
	}
	cmd.Arg("git-cmd", "Git command (remote name).").Required().StringVar(&cmd.gitCmd)
	cmd.Arg("url", "Remote URL.").Required().StringVar(&cmd.remoteURL)
	return cmd
}

func (c *gitHTTPRemoteCommand) run(cf *CLIConf) error {
	httpsURL, org, err := parseTeleportURL(c.remoteURL)
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

	proxy, err := startGitProxy(cf, tc, gitServer.GetName())
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxy.Close()

	// Git (C binary, LibreSSL) respects GIT_SSL_CAINFO for custom CA certs
	// on all platforms including macOS (unlike Go binaries).
	// The ALPN proxy terminates git's HTTPS with a self-signed CA, then
	// forwards to the Teleport proxy with mTLS.
	cmd := exec.Command("git", "remote-http", c.gitCmd, httpsURL)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("HTTPS_PROXY=http://%s", proxy.forwardProxy.GetAddr()),
		fmt.Sprintf("https_proxy=http://%s", proxy.forwardProxy.GetAddr()),
		fmt.Sprintf("GIT_SSL_CAINFO=%s", proxy.localCAPath),
	)

	logger.DebugContext(cf.Context, "Running git remote-http",
		"git_cmd", c.gitCmd,
		"https_url", httpsURL,
		"forward_proxy", proxy.forwardProxy.GetAddr(),
		"ca_file", proxy.localCAPath,
	)

	if err := cf.RunCommand(cmd); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// parseTeleportURL parses a teleport:// URL and returns the HTTPS equivalent
// and the org name.
//
// Format: teleport://github.com/<org>/<repo>.git
// Example: teleport://github.com/gravitational/teleport.git
// Returns: https://github.com/gravitational/teleport.git, "gravitational"
func parseTeleportURL(rawURL string) (httpsURL, org string, err error) {
	const scheme = "teleport://"
	if !strings.HasPrefix(rawURL, scheme) {
		return "", "", trace.BadParameter("expected teleport:// URL, got %q", rawURL)
	}

	path := strings.TrimPrefix(rawURL, scheme)

	// Expected: github.com/<org>/<repo>.git
	if !strings.HasPrefix(path, "github.com/") {
		return "", "", trace.BadParameter("unsupported host in teleport URL %q, only github.com is supported", rawURL)
	}

	repoPath := strings.TrimPrefix(path, "github.com/")
	parts := strings.SplitN(repoPath, "/", 2)
	if len(parts) < 2 {
		return "", "", trace.BadParameter("invalid teleport URL %q, expected teleport://github.com/<org>/<repo>.git", rawURL)
	}

	org = parts[0]
	httpsURL = fmt.Sprintf("https://%s", path)
	return httpsURL, org, nil
}
