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
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	usercredpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/credentialencryption"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/services"
)

type gitCommands struct {
	list       *gitListCommand
	login      *gitLoginCommand
	ssh        *gitSSHCommand
	config     *gitConfigCommand
	clone      *gitCloneCommand
	remoteHTTP *gitRemoteHTTPCommand
}

func newGitCommands(app *kingpin.Application) gitCommands {
	git := app.Command("git", "Git server commands.")
	return gitCommands{
		login:      newGitLoginCommand(git),
		list:       newGitListCommand(git),
		ssh:        newGitSSHCommand(git),
		config:     newGitConfigCommand(git),
		clone:      newGitCloneCommand(git),
		remoteHTTP: newGitRemoteHTTPCommand(git),
	}
}

type gitURL transport.Endpoint

func (g gitURL) check() error {
	switch {
	case g.isGitHub():
		if err := types.ValidateGitHubOrganizationName(g.owner()); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (g gitURL) isGitHub() bool {
	return g.Host == "github.com"
}

// owner returns the first part of the path. If the path does not have an owner,
// an empty string is returned.
//
// For GitHub, owner is either the user or the organization that owns the repo.
//
// For example, if the SSH url is git@github.com:gravitational/teleport.git, the
// owner would be "gravitational".
func (g gitURL) owner() string {
	// g.Path may have a preceding "/" from url.Parse.
	owner, _, ok := strings.Cut(strings.TrimPrefix(g.Path, "/"), "/")
	if !ok {
		return ""
	}
	return owner
}

// repoPath returns the path without the preceding and trailing "/", like
// "gravitational/teleport.git".
func (g gitURL) repoPath() string {
	return strings.Trim(g.Path, "/")
}

// matches returns true if the URL uses the protocol and the host, and has an
// owner.
func (g gitURL) matches(protocol, host string) bool {
	return g.Protocol == protocol && g.Host == host && g.owner() != ""
}

// parseGitURL parses and validates a Git URL.
func parseGitURL(originalURL string) (*gitURL, error) {
	endpoint, err := transport.NewEndpoint(originalURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u := gitURL(*endpoint)
	if err := u.check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &u, nil
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
func parseGitSSHURL(originalURL string) (*gitURL, error) {
	u, err := parseGitURL(originalURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if u.Protocol != "ssh" {
		return nil, trace.BadParameter("unsupported protocol %q. Please provide the SSH URL of the repository.", u.Protocol)
	}
	return u, nil
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
	return trace.Wrap(execGitWithEnv(cf, stdout, stderr, nil, args...))
}

// execGitWithEnv runs git with env added to the environment of tsh.
func execGitWithEnv(cf *CLIConf, stdout, stderr io.Writer, env []string, args ...string) error {
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
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	return trace.Wrap(cf.RunCommand(cmd))
}

// gitContext holds the state shared by the steps of a git command. It is not
// designed for concurrent use; callers that need concurrency are expected to
// synchronize outside of it.
type gitContext struct {
	*CLIConf
	tc            *client.TeleportClient
	clusterClient *client.ClusterClient
	gitServer     types.Server

	// doOAuthFlow performs the GitHub OAuth flow. Overridden in tests.
	// TODO(greedy52) support github oauth flow with git server
	doOAuthFlow func(cf *CLIConf, org string) error
}

// newGitContextForGitHubOrg returns a new gitContext with the GitHub git server
// of the organization, or the only GitHub git server if org is empty. close
// must be called when done.
func newGitContextForGitHubOrg(cf *CLIConf, org string) (*gitContext, error) {
	tc, err := makeClient(cf)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gitCtx := &gitContext{
		CLIConf:     cf,
		tc:          tc,
		doOAuthFlow: doGitHubOAuthFlow,
	}
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		gitServer, err := findGitHubServer(cf.Context, clusterClient.AuthClient, org)
		if err != nil {
			_ = clusterClient.Close()
			return trace.Wrap(err)
		}
		gitCtx.clusterClient = clusterClient
		gitCtx.gitServer = gitServer
		return nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	return gitCtx, nil
}

func (c *gitContext) close() error {
	return trace.Wrap(c.clusterClient.Close())
}

func (c *gitContext) userExternalCredentialsCachePath() string {
	return keypaths.UserExternalCredentialPath(profile.FullProfilePath(c.HomePath), c.tc.WebProxyHost(), c.tc.Username)
}

// findGitHubServer finds the GitHub git server of the GitHub organization, or
// the only GitHub git server if no organization is specified.
func findGitHubServer(ctx context.Context, clt apiclient.ListUnifiedResourcesClient, inputOrg string) (types.Server, error) {
	var labels map[string]string
	if inputOrg != "" {
		labels = map[string]string{types.GitHubOrgLabel: inputOrg}
	}
	resources, err := apiclient.GetAllUnifiedResources(ctx, clt, &proto.ListUnifiedResourcesRequest{
		Kinds:  []string{types.KindGitServer},
		SortBy: types.SortBy{Field: types.ResourceMetadataName},
		Labels: labels,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers, err := types.EnrichedResources(resources).ToResourcesWithLabels().AsServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	checkGitHubServer := func(server types.Server) (types.Server, error) {
		if server.GetGitHub() == nil {
			return nil, trace.BadParameter("git server %q is not a GitHub server", server.GetName())
		}
		return server, nil
	}

	if inputOrg != "" {
		if len(servers) == 0 {
			return nil, trace.NotFound("GitHub organization %q not found", inputOrg)
		}
		// Multiple git servers for same org. Just pick one for now.
		return checkGitHubServer(servers[0])
	}

	if len(servers) == 1 {
		return checkGitHubServer(servers[0])
	}
	// Zero or more than one git servers found, ask user to input --github-org.
	return nil, trace.BadParameter("please specify a GitHub organization with --github-org, run 'tsh git ls' to see the available Git servers")
}

// checkGitOverHTTPS returns an error if the git server or the current session
// does not support Git over HTTPS.
func (c *gitContext) checkGitOverHTTPS() error {
	if !types.IsGitProtocolAllowed(c.gitServer.GetGitHub().AllowProtocols, types.GitProtocolHTTPS) {
		return trace.BadParameter("Git over HTTPS is not allowed for GitHub organization %q, contact your Teleport administrator", c.gitServer.GetGitHub().Organization)
	}
	profile, err := c.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(checkSessionEncryptionKey(profile))
}

// checkSessionEncryptionKey returns an error if the session of the profile
// does not support Git over HTTPS.
func checkSessionEncryptionKey(profile *client.ProfileStatus) error {
	if profile.EncryptionKeyID == "" {
		return trace.BadParameter("the current session does not support Git over HTTPS. Run 'tsh login' again, or contact your Teleport administrator if the problem persists.")
	}
	return nil
}

// getUserDecryptedAccessToken returns the GitHub access token of the session
// for the git server, decrypted with the session's encryption key.
func (c *gitContext) getUserDecryptedAccessToken() (string, error) {
	keyRing, err := c.tc.LocalAgent().GetKeyRing(c.tc.SiteName)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if keyRing.EncryptionPrivateKey == nil {
		return "", trace.BadParameter("the current session does not support Git over HTTPS. Run 'tsh login' again, or contact your Teleport administrator if the problem persists.")
	}

	credential, err := c.getUserGitCredentialWithFallbacks(
		c.getUserGitCredentialFromCache,
		c.getUserGitCredentialFromBackendWithRefresh(keyRing.EncryptionPrivateKey),
		c.getUserGitCredentialFromOAuth,
	)
	if err != nil {
		return "", trace.Wrap(err)
	}
	authEncrypted, err := credentialencryption.Decrypt(keyRing.EncryptionPrivateKey, credential.GetOauth().GetAccessTokenBlob())
	if err != nil {
		return "", trace.Wrap(err, "decrypting GitHub credentials")
	}
	logger.DebugContext(c.Context, "Loaded GitHub credentials for git HTTP proxy",
		"git_server", c.gitServer.GetName(),
		"access_token_expiry", credential.GetOauth().GetAccessTokenExpiry().AsTime(),
	)
	return base64.StdEncoding.EncodeToString(authEncrypted), nil
}

func (c *gitContext) getUserGitCredentialFromBackendWithRefresh(encryptionKey *ecdsa.PrivateKey) func() (*usercredpb.Credential, error) {
	return func() (*usercredpb.Credential, error) {
		latest, err := c.getUserGitCredentialFromBackend()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !isGitCredentialExpiring(latest) {
			return latest, nil
		}

		logger.DebugContext(c.Context, "Refreshing GitHub credentials",
			"git_server", c.gitServer.GetName(),
			"access_token_expiry", latest.GetOauth().GetAccessTokenExpiry().AsTime(),
		)
		if len(latest.GetOauth().GetRefreshTokenBlob()) == 0 {
			return nil, trace.NotFound("GitHub credentials for organization %[1]q cannot be refreshed, run 'tsh git login --github-org %[1]s --force'",
				c.gitServer.GetGitHub().Organization)
		}
		authEncrypted, err := credentialencryption.Decrypt(encryptionKey, latest.GetOauth().GetRefreshTokenBlob())
		if err != nil {
			return nil, trace.Wrap(err, "decrypting GitHub refresh token")
		}
		if _, err := c.clusterClient.AuthClient.UserExternalCredentialClient().RefreshOAuthTokens(c.Context, usercredpb.RefreshOAuthTokensRequest_builder{
			AuthEncryptedRefreshToken: authEncrypted,
		}.Build()); err != nil {
			return nil, trace.Wrap(err, "refreshing GitHub credentials")
		}
		// Fetch the user credentials again to update the local cache.
		return c.getUserGitCredentialFromBackend()
	}
}

func (c *gitContext) getUserGitCredentialFromCache() (*usercredpb.Credential, error) {
	logger.DebugContext(c.Context, "Getting GitHub credentials from local cache", "git_server", c.gitServer.GetName())
	profile, err := c.ProfileStatus()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userCreds, err := client.LoadUserExternalCredentialsCache(c.Context, c.userExternalCredentialsCachePath(), profile.EncryptionKeyID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gitCred, ok := services.FindUserExternalCredential(userCreds, c.gitServer.GetKind(), c.gitServer.GetName())
	if !ok {
		return nil, trace.NotFound("credentials for %s %s not found", c.gitServer.GetKind(), c.gitServer.GetName())
	}
	if isGitCredentialExpiring(gitCred) {
		return nil, trace.NotFound("cached credentials for %s %s are expiring", c.gitServer.GetKind(), c.gitServer.GetName())
	}
	return gitCred, nil
}

func (c *gitContext) getUserGitCredentialFromBackend() (*usercredpb.Credential, error) {
	logger.DebugContext(c.Context, "Getting GitHub credentials from Auth", "git_server", c.gitServer.GetName())
	resp, err := c.clusterClient.AuthClient.UserExternalCredentialClient().GetUserExternalCredentials(c.Context, &usercredpb.GetUserExternalCredentialsRequest{})
	if trace.IsNotImplemented(err) {
		return nil, trace.BadParameter("the Teleport cluster does not support Git over HTTPS yet, contact your Teleport administrator")
	} else if err != nil {
		return nil, trace.Wrap(err)
	}
	userCreds := resp.GetCredentials()
	if err := client.SaveUserExternalCredentialsCache(c.Context, c.userExternalCredentialsCachePath(), userCreds); err != nil {
		return nil, trace.Wrap(err)
	}
	if gitCred, ok := services.FindUserExternalCredential(userCreds, c.gitServer.GetKind(), c.gitServer.GetName()); ok {
		return gitCred, nil
	}
	return nil, trace.NotFound("credentials for %s %s not found", c.gitServer.GetKind(), c.gitServer.GetName())
}

func (c *gitContext) getUserGitCredentialFromOAuth() (*usercredpb.Credential, error) {
	logger.DebugContext(c.Context, "Getting GitHub credentials from GitHub OAuth flow", "git_server", c.gitServer.GetName())
	github := c.gitServer.GetGitHub()
	// Without a browser on this machine, the user authorizes on the web UI
	// from any browser.
	//
	// TODO(greedy52) poll/watch for the credentials instead of asking the user
	// to run the command again.
	if !sso.OpenBrowserPathAvailable(c.Browser, c.LookPath) {
		nextStep := "run this command again"
		// Authorizing on the web UI adds the GitHub identity to the user but not
		// to the current cert, which Git over SSH needs.
		if types.IsGitProtocolAllowed(github.AllowProtocols, types.GitProtocolSSH) {
			if profile, err := c.ProfileStatus(); err == nil && profile.GitHubIdentity == nil {
				nextStep = "run 'tsh login --force' and this command again"
			}
		}
		return nil, trace.NotFound("GitHub credentials for organization %q not found. "+
			"Open this link in a browser logged in to Teleport as %s, then %s:\n\n  %s\n",
			github.Organization, c.tc.Username, nextStep, c.gitServerAuthorizeOnWebURL())
	}
	fmt.Fprintf(c.Stderr(), "Authorizing GitHub organization %q for Git over HTTPS.\n", github.Organization)
	if err := c.doOAuthFlow(c.CLIConf, github.Organization); err != nil {
		return nil, trace.Wrap(err)
	}
	return c.getUserGitCredentialFromBackend()
}

func (c *gitContext) getUserGitCredentialWithFallbacks(getFuncs ...func() (*usercredpb.Credential, error)) (*usercredpb.Credential, error) {
	var notFoundErr error = trace.NotFound("credentials for %s %s not found", c.gitServer.GetKind(), c.gitServer.GetName())
	for _, getFunc := range getFuncs {
		cred, err := getFunc()
		switch {
		case trace.IsNotFound(err):
			notFoundErr = err
		case err != nil:
			return nil, trace.Wrap(err)
		default:
			return cred, nil
		}
	}
	return nil, trace.Wrap(notFoundErr)
}

// isGitCredentialExpiring returns true if the access token expires within 30
// seconds. Auth refreshes within a longer window, so a local clock slightly
// ahead of Auth still gets the token refreshed.
func isGitCredentialExpiring(credential *usercredpb.Credential) bool {
	oauth := credential.GetOauth()
	return oauth.HasAccessTokenExpiry() && time.Until(oauth.GetAccessTokenExpiry().AsTime()) < 30*time.Second
}

func (c *gitContext) gitServerAuthorizeOnWebURL() string {
	authorizeURL := url.URL{
		Scheme: "https",
		Host:   c.tc.WebProxyAddr,
		Path:   "/web/gitservers/" + url.PathEscape(c.gitServer.GetName()) + "/authorize",
	}
	return authorizeURL.String()
}

type gitRemote struct {
	name string
	gitURL
	// hasSeparatePushURLs is true if the remote has a push URL or more than one
	// URL, which "git remote set-url" does not update.
	hasSeparatePushURLs bool
}

// listGitRemotes returns the remotes of the current Git directory.
func listGitRemotes(cf *CLIConf) ([]gitRemote, error) {
	out, err := execGitAndCaptureStdout(cf, "remote")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var remotes []gitRemote
	for _, name := range strings.Fields(out) {
		urls, err := execGitAndCaptureStdout(cf, "remote", "get-url", "--all", name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		fetchURL, _, hasMoreURLs := strings.Cut(urls, "\n")
		remote := gitRemote{
			name: name,
		}
		// Remotes that fail to parse or validate keep an empty protocol and
		// are reported as not supported.
		if u, err := parseGitURL(strings.TrimSpace(fetchURL)); err == nil {
			remote.gitURL = *u
		}
		pushURL, err := execGitAndCaptureStdout(cf, "config", "--default", "", "--get", fmt.Sprintf("remote.%s.pushurl", name))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Git pushes to all URLs.
		remote.hasSeparatePushURLs = pushURL != "" || hasMoreURLs
		remotes = append(remotes, remote)
	}
	return remotes, nil
}
