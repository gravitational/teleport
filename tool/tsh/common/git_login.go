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
	"crypto/ecdsa"
	"fmt"
	"net"
	"path/filepath"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	proto "github.com/gravitational/teleport/api/client/proto"
	gitserverv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/gitserver/v1"
	hardwarekeyagentv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/hardwarekeyagent/v1"
	userexternalsecretsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalsecrets/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cryptoutils"
	libhwk "github.com/gravitational/teleport/lib/hardwarekey"
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

	needOAuth := c.force

	if !needOAuth {
		proxyHost, _, _ := net.SplitHostPort(tc.WebProxyAddr)
		if cred := findCachedCredential(cf.HomePath, proxyHost, tc.Username, gitServer.GetName()); cred != nil {
			logger.DebugContext(cf.Context, "Found cached credential",
				"git_server", gitServer.GetName(),
			)
		} else {
			if err := fetchAndCacheSessionCredentials(cf, tc); err != nil {
				logger.DebugContext(cf.Context, "No credentials available, need OAuth",
					"git_server", gitServer.GetName(),
					"error", err,
				)
				needOAuth = true
			}
		}
	}

	if needOAuth {
		if _, err := getGitHubIdentity(cf, github.Organization, withForceOAuthFlow(true)); err != nil {
			return trace.Wrap(err)
		}
		// After OAuth, fetch and cache the newly created encrypted token.
		if err := fetchAndCacheSessionCredentials(cf, tc); err != nil {
			logger.WarnContext(cf.Context, "Failed to fetch encrypted token after OAuth", "error", err)
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

		// Still need the git cert for ALPN mTLS.
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

// encryptionHelper abstracts ECIES decryption and key ID retrieval so it can
// be done either locally (with a private key from the key ring) or via the
// encryption agent (for beams where tbot holds the key).
type encryptionHelper interface {
	decrypt(ctx context.Context, ciphertext []byte) ([]byte, error)
	keyID(ctx context.Context) string
}

type localEncryptionHelper struct {
	key   *ecdsa.PrivateKey
	id    string
}

func (h *localEncryptionHelper) decrypt(_ context.Context, ciphertext []byte) ([]byte, error) {
	return cryptoutils.ECIESDecrypt(h.key, ciphertext)
}

func (h *localEncryptionHelper) keyID(_ context.Context) string {
	return h.id
}

type agentEncryptionHelper struct {
	client hardwarekeyagentv1.EncryptionAgentServiceClient
	cachedKeyID string
}

func (h *agentEncryptionHelper) decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	resp, err := h.client.Decrypt(ctx, hardwarekeyagentv1.DecryptRequest_builder{
		Ciphertext: ciphertext,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.GetPlaintext(), nil
}

func (h *agentEncryptionHelper) keyID(ctx context.Context) string {
	if h.cachedKeyID != "" {
		return h.cachedKeyID
	}
	resp, err := h.client.GetEncryptionKeyID(ctx, hardwarekeyagentv1.GetEncryptionKeyIDRequest_builder{}.Build())
	if err != nil {
		return ""
	}
	h.cachedKeyID = resp.GetEncryptionKeyId()
	return h.cachedKeyID
}

// getEncryptionHelper returns an encryptionHelper -- either using the local
// private key or falling back to the encryption agent.
func getEncryptionHelper(ctx context.Context, tc *client.TeleportClient) (encryptionHelper, error) {
	keyRing, err := tc.LocalAgent().GetKeyRing(tc.SiteName, client.WithSSHCerts{})
	if err == nil && keyRing.EncryptionPrivateKey != nil {
		profileStatus, err := tc.ProfileStatus()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &localEncryptionHelper{
			key: keyRing.EncryptionPrivateKey,
			id:  profileStatus.EncryptionKeyID,
		}, nil
	}

	agentDir := libhwk.AgentDirFromEnv(libhwk.DefaultAgentDir())
	encClient, err := newEncryptionAgentClient(ctx, agentDir)
	if err != nil {
		return nil, trace.Wrap(err, "no encryption key and no encryption agent available")
	}

	return &agentEncryptionHelper{client: encClient}, nil
}

// newEncryptionAgentClient connects to the encryption agent at the given dir.
func newEncryptionAgentClient(ctx context.Context, agentDir string) (hardwarekeyagentv1.EncryptionAgentServiceClient, error) {
	creds, err := credentials.NewClientTLSFromFile(
		filepath.Join(agentDir, libhwk.CertFileName), "localhost",
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conn, err := grpc.NewClient(
		"unix://"+filepath.Join(agentDir, libhwk.SocketFileName),
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return hardwarekeyagentv1.NewEncryptionAgentServiceClient(conn), nil
}

// fetchAndCacheSessionCredentials fetches the session credentials resource from
// the backend and caches it locally as protojson (with refresh tokens stripped).
// deleteSessionCredentials deletes the current session's credentials resource
// from the backend. Best-effort, called during tsh logout.
func deleteSessionCredentials(cf *CLIConf, tc *client.TeleportClient) error {
	ctx, cancel := context.WithTimeout(cf.Context, 5*time.Second)
	defer cancel()

	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	_, err = clusterClient.AuthClient.UserExternalSecretClient().DeleteUserSessionCredentials(
		ctx,
		userexternalsecretsv1.DeleteUserSessionCredentialsRequest_builder{}.Build(),
	)
	return trace.Wrap(err)
}

func fetchAndCacheSessionCredentials(cf *CLIConf, tc *client.TeleportClient) error {
	helper, err := getEncryptionHelper(cf.Context, tc)
	if err != nil {
		logger.DebugContext(cf.Context, "No encryption helper available, skipping credential fetch",
			"error", err,
		)
		return nil
	}

	encKeyID := helper.keyID(cf.Context)
	if encKeyID == "" {
		logger.DebugContext(cf.Context, "No encryption key ID available")
		return nil
	}

	return client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		resp, err := clusterClient.AuthClient.UserExternalSecretClient().GetUserSessionCredentials(cf.Context, userexternalsecretsv1.GetUserSessionCredentialsRequest_builder{
			EncryptionKeyId: encKeyID,
		}.Build())
		if err != nil {
			return trace.Wrap(err)
		}

		resource := resp.GetCredentials()
		if resource == nil || len(resource.GetSpec().GetCredentials()) == 0 {
			logger.DebugContext(cf.Context, "No credentials found on backend")
			return nil
		}

		proxyHost, _, _ := net.SplitHostPort(tc.WebProxyAddr)
		if err := client.SaveSessionCredentials(cf.HomePath, proxyHost, tc.Username, resource); err != nil {
			return trace.Wrap(err)
		}

		logger.DebugContext(cf.Context, "Cached session credentials locally",
			"credentials", len(resource.GetSpec().GetCredentials()),
		)
		return nil
	})
}

// ensureLocalSecret ensures a locally cached auth-encrypted access token
// exists. It checks local cache first, then backend, then triggers OAuth
// if needed.
func ensureLocalSecret(cf *CLIConf, tc *client.TeleportClient, gitServerName string, forceRefresh bool) error {
	if forceRefresh {
		return trace.Wrap(refreshGitToken(cf, tc, gitServerName))
	}

	proxyHost, _, _ := net.SplitHostPort(tc.WebProxyAddr)

	// Check local cache for a valid credential.
	if cred := findCachedCredential(cf.HomePath, proxyHost, tc.Username, gitServerName); cred != nil {
		if exp := cred.GetAccessTokenExpiry(); exp == nil || !exp.IsValid() || time.Now().Before(exp.AsTime()) {
			return nil
		}
	}

	// Cache miss or expired. Fetch from backend.
	if err := fetchAndCacheSessionCredentials(cf, tc); err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("no credentials found for %s, run 'tsh git login' first", gitServerName)
		}
		return trace.Wrap(err)
	}

	// Re-check after fetch.
	cred := findCachedCredential(cf.HomePath, proxyHost, tc.Username, gitServerName)
	if cred == nil {
		return trace.NotFound("no credentials found for %s, run 'tsh git login' first", gitServerName)
	}

	if exp := cred.GetAccessTokenExpiry(); exp != nil && exp.IsValid() && !time.Now().Before(exp.AsTime()) {
		logger.DebugContext(cf.Context, "Access token expired, refreshing",
			"git_server", gitServerName,
			"expires_at", exp.AsTime(),
		)
		if err := refreshGitToken(cf, tc, gitServerName); err != nil {
			return trace.Wrap(err, "failed to refresh expired token for %s, run 'tsh git login' to re-authenticate", gitServerName)
		}
	}
	return nil
}

// findCachedCredential loads the local cache and returns the OAuthSecret
// for the given git server, or nil if not found.
func findCachedCredential(homePath, proxyHost, username, gitServerName string) *userexternalsecretsv1.OAuthSecret {
	resource, err := client.LoadSessionCredentials(homePath, proxyHost, username)
	if err != nil || resource == nil {
		return nil
	}
	for _, cred := range resource.GetSpec().GetCredentials() {
		if cred.GetResourceKind() == types.KindGitServer && cred.GetResourceName() == gitServerName {
			return cred.GetOauth()
		}
	}
	return nil
}

// refreshGitToken refreshes the GitHub access token by fetching the latest
// refresh token from the backend, decrypting the ECIES layer, and sending the
// auth-encrypted blob to the git service for refresh.
func refreshGitToken(cf *CLIConf, tc *client.TeleportClient, gitServerName string) error {
	helper, err := getEncryptionHelper(cf.Context, tc)
	if err != nil {
		return trace.Wrap(err, "no encryption helper available for refresh")
	}

	encKeyID := helper.keyID(cf.Context)
	if encKeyID == "" {
		return trace.BadParameter("no encryption key ID available for refresh")
	}

	return client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		resp, err := clusterClient.AuthClient.UserExternalSecretClient().GetUserSessionCredentials(cf.Context, userexternalsecretsv1.GetUserSessionCredentialsRequest_builder{
			EncryptionKeyId: encKeyID,
		}.Build())
		if err != nil {
			return trace.Wrap(err, "fetching credentials for refresh")
		}

		var oauth *userexternalsecretsv1.OAuthSecret
		for _, cred := range resp.GetCredentials().GetSpec().GetCredentials() {
			if cred.GetResourceKind() == types.KindGitServer && cred.GetResourceName() == gitServerName {
				oauth = cred.GetOauth()
				break
			}
		}
		if oauth == nil || len(oauth.GetRefreshTokenBlob()) == 0 {
			return trace.NotFound("no refresh token available for %s", gitServerName)
		}

		// Decrypt the ECIES layer of the refresh token blob.
		authEncryptedRefresh, err := helper.decrypt(cf.Context, oauth.GetRefreshTokenBlob())
		if err != nil {
			return trace.Wrap(err, "decrypting refresh token blob")
		}

		// Call git service to refresh.
		_, err = clusterClient.AuthClient.GitCredentialsClient().RefreshGitToken(cf.Context, gitserverv1.RefreshGitTokenRequest_builder{
			GitServerName:             gitServerName,
			AuthEncryptedRefreshToken: authEncryptedRefresh,
		}.Build())
		if err != nil {
			return trace.Wrap(err, "refreshing git token")
		}

		logger.DebugContext(cf.Context, "Refreshed git token", "git_server", gitServerName)

		// Fetch and cache the new double-encrypted access token.
		return fetchAndCacheSessionCredentials(cf, tc)
	})
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
		if err := fetchAndCacheSessionCredentials(cf, tc); err != nil {
			logger.WarnContext(cf.Context, "Failed to fetch encrypted token after OAuth", "error", err)
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
