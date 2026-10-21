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

package userexternalcredentialsv1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/oauth2"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/credentialencryption"
	"github.com/gravitational/teleport/lib/auth/integration/credentials"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// gitServerHandler handles user external credentials of git servers.
type gitServerHandler struct {
	cfg *ServiceConfig
}

// CheckRoute checks the user certificate is routed to the git server.
func (h *gitServerHandler) CheckRoute(ctx context.Context, identity *tlsca.Identity, gitServerName string) error {
	if identity.RouteToGit.Name == "" {
		return trace.BadParameter("user certificate has no git server route")
	}
	if identity.RouteToGit.Name != gitServerName {
		return trace.AccessDenied("user external credential does not belong to the git server of the user certificate")
	}
	return nil
}

// CheckOwnership checks with GitHub that the decrypted token belongs to the
// GitHub user of the certificate.
func (h *gitServerHandler) CheckOwnership(ctx context.Context, identity *tlsca.Identity, decrypted *credentialencryption.Payload) error {
	if identity.GitHubUserID == "" {
		return trace.AccessDenied("user certificate has no GitHub identity")
	}
	app, err := h.getOAuthApp(ctx, decrypted.ResourceName)
	if err != nil {
		return trace.Wrap(err)
	}
	githubUserID, err := h.checkToken(ctx, app, decrypted.Credential)
	if err != nil {
		return trace.Wrap(err)
	}
	if githubUserID != identity.GitHubUserID {
		h.cfg.Logger.WarnContext(ctx, "GitHub token does not belong to the GitHub user of the user certificate",
			"user", identity.Username,
			"git_server", decrypted.ResourceName,
			"identity_github_user_id", identity.GitHubUserID,
			"token_github_user_id", githubUserID,
		)
		return trace.AccessDenied("GitHub credential does not belong to the GitHub user of the user certificate")
	}
	return nil
}

// CheckAccess checks the user can access the git server.
func (h *gitServerHandler) CheckAccess(ctx context.Context, authCtx *authz.Context, gitServerName string) error {
	gitServer, err := h.cfg.Cache.GetGitServer(ctx, gitServerName)
	if err != nil {
		return trace.Wrap(err)
	}
	// MFA is not required for refreshing credentials of git servers but will be
	// enforced at the connection time.
	return trace.Wrap(authCtx.Checker.CheckAccess(gitServer, services.AccessState{MFAVerified: true}))
}

// RefreshOAuthToken exchanges the GitHub refresh token of the user for the
// git server for a new OAuth token, with the OAuth app of the git server's
// integration.
//
// TODO(greedy52) Concurrent refreshes may cause race conditions where some
// clients see refresh failures or receive invalid tokens. This is rare and
// recoverable. Consider using a semaphora if this proves to be an issue.
func (h *gitServerHandler) RefreshOAuthToken(ctx context.Context, username, gitServerName string, refreshToken []byte) (*oauth2.Token, error) {
	app, err := h.getOAuthApp(ctx, gitServerName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// oauth2 uses the HTTP client from the context.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, h.cfg.httpClient)
	// An expired token makes the token source use the refresh token.
	token, err := app.TokenSource(ctx, &oauth2.Token{
		RefreshToken: string(refreshToken),
		Expiry:       h.cfg.Clock.Now().Add(-time.Minute),
	}).Token()
	if err != nil {
		// https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps
		//
		// "If the refresh token that you specified is invalid or expired, you will receive a bad_refresh_token error."
		//
		// In this case, return NotFound so the client will try to force a GitHub OAuth flow.
		//
		// TODO(greedy52) ideally we should delete the credential from the caller's session.
		if retrieveErr, ok := errors.AsType[*oauth2.RetrieveError](err); ok && retrieveErr.ErrorCode == "bad_refresh_token" {
			return nil, trace.NotFound("GitHub rejected the refresh token for git server %q: %v", gitServerName, err)
		}
		return nil, trace.Wrap(err, "refreshing GitHub OAuth token")
	}
	if token.Expiry.IsZero() {
		return nil, trace.BadParameter("GitHub returned a non-expiring access token for git server %q", gitServerName)
	}
	h.cfg.Logger.DebugContext(ctx, "Refreshed GitHub OAuth token",
		"user", username,
		"git_server", gitServerName,
		"access_token_expiry", token.Expiry,
		"has_refresh_token", token.RefreshToken != "",
	)
	return token, nil
}

// getOAuthApp returns the OAuth app of the integration of the GitHub git
// server.
func (h *gitServerHandler) getOAuthApp(ctx context.Context, gitServerName string) (*oauth2.Config, error) {
	gitServer, err := h.cfg.Cache.GetGitServer(ctx, gitServerName)
	if err != nil {
		return nil, trace.Wrap(err)
	} else if gitServer.GetGitHub() == nil {
		return nil, trace.BadParameter("git server %q is not a GitHub server", gitServerName)
	}
	_, ref, err := credentials.GetIntegrationWithCredRef(ctx, gitServer.GetGitHub().Integration, h.cfg.Cache)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cred, err := credentials.GetByPurpose(ctx, ref, credentials.PurposeGitHubOAuth, h.cfg.Cache)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientID, clientSecret := cred.GetOAuthClientSecret()
	if clientID == "" || clientSecret == "" {
		return nil, trace.BadParameter("no OAuth client ID or secret found for integration %q", gitServer.GetGitHub().Integration)
	}
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: types.GithubURL + "/login/oauth/access_token",
		},
	}, nil
}

// checkToken returns the ID of the GitHub user of the token, with the "check a
// token" API of the OAuth app.
// https://docs.github.com/en/rest/apps/oauth-applications#check-a-token
func (h *gitServerHandler) checkToken(ctx context.Context, app *oauth2.Config, token []byte) (string, error) {
	body, err := json.Marshal(map[string]string{"access_token": string(token)})
	if err != nil {
		return "", trace.Wrap(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		types.GithubAPIURL+"/applications/"+url.PathEscape(app.ClientID)+"/token", bytes.NewReader(body))
	if err != nil {
		return "", trace.Wrap(err)
	}
	req.SetBasicAuth(app.ClientID, app.ClientSecret)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.cfg.httpClient.Do(req)
	if err != nil {
		return "", trace.Wrap(err, "checking GitHub token")
	}
	defer resp.Body.Close()
	respBody, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return "", trace.Wrap(err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return "", trace.AccessDenied("GitHub token is invalid or revoked")
	default:
		if err := trace.ReadError(resp.StatusCode, respBody); err != nil {
			return "", trace.Wrap(err, "checking GitHub token")
		}
		return "", trace.BadParameter("unexpected response from GitHub when checking token: %v %s", resp.StatusCode, respBody)
	}
	var result struct {
		User struct {
			ID int64 `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", trace.Wrap(err)
	}
	return strconv.FormatInt(result.User.ID, 10), nil
}
