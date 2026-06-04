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

package integrationv1

import (
	"context"

	"github.com/gravitational/trace"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/types/known/timestamppb"

	userexternalcredentialsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/integration/credentials"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/tlsca"
)

// GenerateGitHubAppToken returns a GitHub access token for the user identified by
// the git session. Only proxy services can call this.
func (s *Service) GenerateGitHubAppToken(ctx context.Context, in *integrationpb.GenerateGitHubAppTokenRequest) (*integrationpb.GenerateGitHubAppTokenResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authz.HasBuiltinRole(*authCtx, string(types.RoleProxy)) {
		return nil, trace.AccessDenied("GenerateGitHubAppToken is only available to proxy services")
	}

	if in.SessionId == "" {
		return nil, trace.BadParameter("missing session ID")
	}

	// Look up the session to get user and git server info.
	session, err := s.backend.GetAppSession(ctx, types.GetAppSessionRequest{SessionID: in.SessionId})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Verify session ownership.
	if session.GetUser() == "" {
		return nil, trace.AccessDenied("session %v has no user", in.SessionId)
	}

	// Parse the session cert to get RouteToGit.
	identity, err := identityFromSession(session)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if identity.RouteToGit.GitServerName == "" {
		return nil, trace.BadParameter("session %v is not a git session", in.SessionId)
	}

	// Verify the session cert user matches the session user.
	if identity.Username != session.GetUser() {
		return nil, trace.AccessDenied("session user mismatch: cert=%q session=%q", identity.Username, session.GetUser())
	}

	// Reject web sessions.
	if identity.PrivateKeyPolicy == "web_session" {
		return nil, trace.AccessDenied("web sessions cannot be used for git access")
	}

	// Resolve git server -> integration -> client_id.
	gitServer, err := s.backend.GetGitServer(ctx, identity.RouteToGit.GitServerName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	github := gitServer.GetGitHub()
	if github == nil {
		return nil, trace.BadParameter("git server %v is not a GitHub server", identity.RouteToGit.GitServerName)
	}

	ig, err := s.cache.GetIntegration(ctx, github.Integration)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientID, err := s.resolveClientID(ctx, ig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Look up stored credentials.
	creds, err := s.backend.GetUserExternalCredentials(ctx, session.GetUser(), clientID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	githubOAuth := creds.GetSpec().GetGithubOauth()
	if githubOAuth == nil {
		return nil, trace.NotFound("no GitHub OAuth credentials found for user %v", session.GetUser())
	}

	accessToken := githubOAuth.GetAccessToken()

	// Check if access token is expired and refresh if needed.
	if expiry := githubOAuth.GetAccessTokenExpiry(); expiry != nil && expiry.IsValid() {
		if s.clock.Now().After(expiry.AsTime()) {
			refreshToken := githubOAuth.GetRefreshToken()
			if refreshToken == "" {
				return nil, trace.AccessDenied("GitHub access token expired and no refresh token available for user %v; re-run 'tsh git login'", session.GetUser())
			}

			newToken, err := s.refreshGitHubToken(ctx, ig, refreshToken)
			if err != nil {
				return nil, trace.Wrap(err, "refreshing GitHub token")
			}
			accessToken = newToken.AccessToken

			// Save the refreshed credentials.
			s.saveRefreshedCredentials(ctx, creds, newToken)
		}
	}

	return &integrationpb.GenerateGitHubAppTokenResponse{
		AccessToken: accessToken,
	}, nil
}

// GetGitCredentialsStatus checks whether stored GitHub OAuth credentials
// exist for the calling user.
func (s *Service) GetGitCredentialsStatus(ctx context.Context, in *integrationpb.GetGitCredentialsStatusRequest) (*integrationpb.GetGitCredentialsStatusResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username := authCtx.User.GetName()

	if in.Integration == "" {
		return nil, trace.BadParameter("missing integration name")
	}

	ig, err := s.cache.GetIntegration(ctx, in.Integration)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientID, err := s.resolveClientID(ctx, ig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	creds, err := s.backend.GetUserExternalCredentials(ctx, username, clientID)
	if err != nil {
		if trace.IsNotFound(err) {
			return &integrationpb.GetGitCredentialsStatusResponse{
				Exists: false,
			}, nil
		}
		return nil, trace.Wrap(err)
	}

	githubOAuth := creds.GetSpec().GetGithubOauth()
	if githubOAuth == nil {
		return &integrationpb.GetGitCredentialsStatusResponse{
			Exists: false,
		}, nil
	}

	return &integrationpb.GetGitCredentialsStatusResponse{
		Exists:            true,
		AccessTokenExpiry: githubOAuth.GetAccessTokenExpiry(),
	}, nil
}

func (s *Service) refreshGitHubToken(ctx context.Context, ig types.Integration, refreshToken string) (*oauth2.Token, error) {
	ref := ig.GetCredentials().GetStaticCredentialsRef()
	if ref == nil {
		return nil, trace.BadParameter("integration %v has no credentials", ig.GetName())
	}
	oauthCred, err := credentials.GetByPurpose(ctx, ref, credentials.PurposeGitHubOAuth, s.cache)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientID, clientSecret := oauthCred.GetOAuthClientSecret()

	config := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://github.com/login/oauth/access_token",
		},
	}

	token, err := config.TokenSource(ctx, &oauth2.Token{
		RefreshToken: refreshToken,
	}).Token()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return token, nil
}

func (s *Service) saveRefreshedCredentials(ctx context.Context, creds *userexternalcredentialsv1.UserExternalCredentials, token *oauth2.Token) {
	ghCreds := creds.GetSpec().GetGithubOauth()
	if ghCreds == nil {
		return
	}
	ghCreds.SetAccessToken(token.AccessToken)
	if !token.Expiry.IsZero() {
		ghCreds.SetAccessTokenExpiry(timestamppb.New(token.Expiry))
	}
	if token.RefreshToken != "" {
		ghCreds.SetRefreshToken(token.RefreshToken)
	}
	if _, err := s.backend.UpsertUserExternalCredentials(ctx, creds); err != nil {
		s.logger.WarnContext(ctx, "Failed to save refreshed GitHub credentials", "error", err)
	}
}

func (s *Service) resolveClientID(ctx context.Context, ig types.Integration) (string, error) {
	ref := ig.GetCredentials().GetStaticCredentialsRef()
	if ref == nil {
		return "", trace.BadParameter("integration %v has no credentials", ig.GetName())
	}
	oauthCred, err := s.getStaticCredentialsWithPurpose(ctx, ig, "github-oauth")
	if err != nil {
		return "", trace.Wrap(err)
	}
	clientID := oauthCred.GetOAuthClientID()
	if clientID == "" {
		return "", trace.BadParameter("integration %v has no OAuth client ID", ig.GetName())
	}
	return clientID, nil
}

func identityFromSession(session types.WebSession) (*tlsca.Identity, error) {
	certPEM := session.GetTLSCert()
	if len(certPEM) == 0 {
		return nil, trace.BadParameter("session has no TLS certificate")
	}
	cert, err := tlsca.ParseCertificatePEM(certPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return identity, nil
}
