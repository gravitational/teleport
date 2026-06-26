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

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	proto "github.com/gravitational/teleport/api/client/proto"
	gitserverv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/gitserver/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils"
)

// gitLoginCommand implements `tsh git login`.
type gitLoginCommand struct {
	*kingpin.CmdClause

	gitServerName      string
	gitHubOrganization string
	force              bool
}

func newGitLoginCommand(parent *kingpin.CmdClause) *gitLoginCommand {
	cmd := &gitLoginCommand{
		CmdClause: parent.Command("login", "Opens a browser and retrieves your login from GitHub."),
	}

	cmd.Arg("git-server", "Name of the git server.").StringVar(&cmd.gitServerName)
	cmd.Flag("github-org", "GitHub organization (deprecated, use git-server argument instead).").StringVar(&cmd.gitHubOrganization)
	cmd.Flag("force", "Force a login.").BoolVar(&cmd.force)
	return cmd
}

func (c *gitLoginCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	gitServer, err := resolveGitServer(cf, tc, c.gitServerName, c.gitHubOrganization)
	if err != nil {
		return trace.Wrap(err)
	}

	github := gitServer.GetGitHub()
	if github == nil {
		return trace.BadParameter("git server %v is not a GitHub server", gitServer.GetName())
	}

	if !types.GitServerSSHEnabled(github) && !types.GitServerHTTPEnabled(github) {
		return trace.BadParameter("git server %v has no protocols enabled", gitServer.GetName())
	}

	if c.force {
		if _, err := getGitHubIdentity(cf, github.Organization, withForceOAuthFlow(true)); err != nil {
			return trace.Wrap(err)
		}
	} else {
		if err := ensureGitHubCredentials(cf, tc, gitServer, types.GitServerSSHEnabled(github), types.GitServerHTTPEnabled(github)); err != nil {
			return trace.Wrap(err)
		}
	}

	profile, err := cf.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	if profile.GitHubIdentity != nil {
		fmt.Fprintf(cf.Stdout(), "Logged in as GitHub user %q.\n", profile.GitHubIdentity.Username)
	}

	sshOK := types.GitServerSSHEnabled(github)
	httpOK := types.GitServerHTTPEnabled(github)

	if httpOK {
		ensureGitRemoteHelper(cf)

		valid, reason := hasValidGitCert(tc, gitServer.GetName())
		logger.DebugContext(cf.Context, "Checking git cert validity",
			"git_server", gitServer.GetName(),
			"valid", valid,
			"reason", reason,
		)
		if !valid {
			if err := issueGitCert(cf, tc, gitServer.GetName()); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	fmt.Fprintln(cf.Stdout())
	if sshOK {
		fmt.Fprintf(cf.Stdout(), "You can now use Git over SSH:\n")
		fmt.Fprintf(cf.Stdout(), "  tsh git clone git@github.com:%s/<repo>.git\n", github.Organization)
		fmt.Fprintln(cf.Stdout())
	}
	if httpOK {
		fmt.Fprintf(cf.Stdout(), "You can now use Git over HTTPS:\n")
		fmt.Fprintf(cf.Stdout(), "  tsh git clone https://github.com/%s/<repo>.git\n", github.Organization)
		fmt.Fprintln(cf.Stdout())
		fmt.Fprintf(cf.Stdout(), "You can now use the GitHub CLI:\n")
		fmt.Fprintf(cf.Stdout(), "  tsh gh -- api /user\n")
		fmt.Fprintln(cf.Stdout())
	}
	return nil
}

// ensureGitHubCredentials ensures the user has the necessary GitHub credentials
// for the given git server and protocol. For SSH, it ensures GitHub identity is
// bound. For HTTP, it also ensures the access token is stored.
func ensureGitHubCredentials(cf *CLIConf, tc *client.TeleportClient, gitServer types.Server, needSSH, needHTTP bool) error {
	github := gitServer.GetGitHub()
	if github == nil {
		return trace.BadParameter("git server %v is not a GitHub server", gitServer.GetName())
	}

	needOAuth := false
	if needSSH {
		profile, err := cf.ProfileStatus()
		if err != nil {
			return trace.Wrap(err)
		}
		if profile.GitHubIdentity == nil {
			needOAuth = true
		}
	}
	if !needOAuth && needHTTP {
		hasCredentials, err := checkGitHubCredentials(cf, tc, gitServer.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
		if !hasCredentials {
			needOAuth = true
		}
	}

	if needOAuth {
		if _, err := getGitHubIdentity(cf, github.Organization, withForceOAuthFlow(true)); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func hasValidGitCert(tc *client.TeleportClient, gitServerName string) (bool, string) {
	keyRing, err := tc.LocalAgent().GetKeyRing(tc.SiteName, client.WithAllCerts...)
	if err != nil {
		return false, fmt.Sprintf("failed to get key ring: %v", err)
	}
	cert, err := keyRing.AppTLSCert(gitServerName)
	if err != nil {
		return false, fmt.Sprintf("no cert found: %v", err)
	}
	if err := utils.VerifyTLSCertLeafExpiry(cert, nil); err != nil {
		return false, fmt.Sprintf("cert expired: %v", err)
	}
	return true, "valid"
}

func issueGitCert(cf *CLIConf, tc *client.TeleportClient, gitServerName string) error {
	return client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		result, err := clusterClient.IssueUserCertsWithMFA(cf.Context, client.ReissueParams{
			RouteToGit: proto.RouteToGit{
				GitServerName: gitServerName,
			},
		})
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(tc.LocalAgent().AddAppKeyRing(result.KeyRing))
	})
}

func checkGitHubCredentials(cf *CLIConf, tc *client.TeleportClient, gitServerName string) (bool, error) {
	var valid bool
	err := client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		checkReq := &gitserverv1.CheckGitCredentialsRequest{}
		checkReq.SetGitServerName(gitServerName)
		resp, err := clusterClient.AuthClient.GitCredentialsClient().CheckGitCredentials(cf.Context, checkReq)
		if err != nil {
			return trace.Wrap(err)
		}
		valid = resp.GetValid()
		return nil
	})
	return valid, trace.Wrap(err)
}

// resolveGitServer finds a git server by name, org, or auto-selects if only
// one exists.
func resolveGitServer(cf *CLIConf, tc *client.TeleportClient, name, org string) (types.Server, error) {
	switch {
	case name != "":
		return findGitServerByName(cf, tc, name)
	case org != "":
		return findGitServerByOrg(cf, tc, org)
	}

	var servers []types.Server
	err := client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		servers, _, err = clusterClient.AuthClient.GitServerReadOnlyClient().ListGitServers(cf.Context, 0, "")
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch len(servers) {
	case 0:
		return nil, trace.NotFound("no git servers found")
	case 1:
		return servers[0], nil
	default:
		var names []string
		for _, s := range servers {
			names = append(names, s.GetName())
		}
		return nil, trace.BadParameter("multiple git servers found, specify one: %v", names)
	}
}

func findGitServerByName(cf *CLIConf, tc *client.TeleportClient, name string) (types.Server, error) {
	var server types.Server
	err := client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		server, err = clusterClient.AuthClient.GitServerReadOnlyClient().GetGitServer(cf.Context, name)
		return trace.Wrap(err)
	})
	return server, trace.Wrap(err)
}

func findGitServerByOrg(cf *CLIConf, tc *client.TeleportClient, org string) (types.Server, error) {
	var server types.Server
	err := client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		servers, _, err := clusterClient.AuthClient.GitServerReadOnlyClient().ListGitServers(cf.Context, 0, "")
		if err != nil {
			return trace.Wrap(err)
		}
		for _, s := range servers {
			if github := s.GetGitHub(); github != nil && github.Organization == org {
				server = s
				return nil
			}
		}
		return trace.NotFound("git server for organization %q not found", org)
	})
	return server, trace.Wrap(err)
}

func getGitHubIdentity(cf *CLIConf, org string, applyOpts ...getGitHubIdentityOption) (*client.GitHubIdentity, error) {
	opts := getGitHubIdentityOptions{
		doOAuthFlow: doGitHubOAuthFlow,
	}
	for _, applyOpt := range applyOpts {
		applyOpt(&opts)
	}

	// See if GitHub identity already present.
	profile, err := cf.ProfileStatus()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if profile.GitHubIdentity != nil && !opts.forceOAuthFlow {
		return profile.GitHubIdentity, nil
	}

	// Do GitHub OAuth flow to get GitHub identity.
	if err := opts.doOAuthFlow(cf, org); err != nil {
		return nil, trace.Wrap(err)
	}

	// Check profile again.
	profile, err = cf.ProfileStatus()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if profile.GitHubIdentity == nil {
		// This should not happen if the OAuth is successful.
		return nil, trace.NotFound("GitHub identity not found after GitHub OAuth flow")
	}
	return profile.GitHubIdentity, nil
}

type getGitHubIdentityOptions struct {
	forceOAuthFlow bool
	doOAuthFlow    func(cf *CLIConf, org string) error
}

type getGitHubIdentityOption func(*getGitHubIdentityOptions)

func withForceOAuthFlow(force bool) getGitHubIdentityOption {
	return func(o *getGitHubIdentityOptions) {
		o.forceOAuthFlow = force
	}
}

func withOAuthFlowOverride(override func(*CLIConf, string) error) getGitHubIdentityOption {
	return func(o *getGitHubIdentityOptions) {
		o.doOAuthFlow = override
	}
}

func doGitHubOAuthFlow(cf *CLIConf, org string) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	// Capture active requests before starting the OAuth flow.
	profile, err := cf.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	err = client.RetryWithRelogin(
		cf.Context,
		tc,
		func() error {
			return tc.ReissueWithGitHubOAuth(cf.Context, org)
		},
		client.WithAfterLoginHook(func() error {
			// Update profile if a re-login is performed.
			profile, err = cf.ProfileStatus()
			return trace.Wrap(err)
		}),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	// Ideally active requests should be handled during the OAuth flow in one
	// shot but that complicates the OAuth flow by a lot. For now, we work
	// around this by manually reissuing the request IDs after the oauth flow.
	// The oauth flow is usually only a one time login anyway so we don't expect
	// this happen often.
	if len(profile.ActiveRequests) > 0 {
		// Send to stderr in case called by `git`.
		fmt.Fprintln(cf.Stderr(), "Reissuing certificates for access requests ...")
		var emptyDropRequests []string
		if err := reissueWithRequests(cf, tc, profile.ActiveRequests, emptyDropRequests); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
