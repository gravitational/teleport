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

package gitserverv1

import (
	"context"
	"crypto/x509"
	"log/slog"
	"time"

	gogotypes "github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/gitserver/v1"
	userexternalcredentialsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/integration/credentials"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// CredentialsCache provides cached read access for lookups that don't need
// real-time consistency.
type CredentialsCache interface {
	GetGitServer(ctx context.Context, name string) (types.Server, error)
	services.IntegrationsGetter
	GetPluginStaticCredentialsByLabels(ctx context.Context, labels map[string]string) ([]types.PluginStaticCredentials, error)
}

// CredentialsBackend provides read-write access for credential mutations.
type CredentialsBackend interface {
	services.UserExternalCredentialsService
}

// CertVerifier verifies that a TLS certificate was signed by the Auth CA.
type CertVerifier func(certDER []byte) (*x509.Certificate, error)

// CredentialsServiceConfig holds configuration for GitCredentialsService.
type CredentialsServiceConfig struct {
	Authorizer   authz.Authorizer
	Cache        CredentialsCache
	Backend      CredentialsBackend
	Emitter      apievents.Emitter
	CertVerifier CertVerifier
	Logger       *slog.Logger
	Clock        clockwork.Clock
}

// CredentialsService implements the GitCredentialsService gRPC service.
type CredentialsService struct {
	pb.UnimplementedGitCredentialsServiceServer

	authorizer   authz.Authorizer
	cache        CredentialsCache
	backend      CredentialsBackend
	emitter      apievents.Emitter
	certVerifier CertVerifier
	logger       *slog.Logger
	clock        clockwork.Clock
}

// NewCredentialsService creates a new GitCredentialsService.
func NewCredentialsService(cfg CredentialsServiceConfig) (*CredentialsService, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if cfg.Cache == nil {
		return nil, trace.BadParameter("cache is required")
	}
	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend is required")
	}
	if cfg.Emitter == nil {
		return nil, trace.BadParameter("emitter is required")
	}
	if cfg.CertVerifier == nil {
		return nil, trace.BadParameter("cert verifier is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return &CredentialsService{
		authorizer:   cfg.Authorizer,
		cache:        cfg.Cache,
		backend:      cfg.Backend,
		emitter:      cfg.Emitter,
		certVerifier: cfg.CertVerifier,
		logger:       cfg.Logger,
		clock:        cfg.Clock,
	}, nil
}

// gitHubInfo holds resolved GitHub integration details.
type gitHubInfo struct {
	gitServer   types.Server
	github      *types.GitHubServerMetadata
	integration types.Integration
	clientID    string
}

// resolveGitHub resolves a git server name to its GitHub integration details.
// The clientID is read from the integration status (cached, no extra lookup).
func (s *CredentialsService) resolveGitHub(ctx context.Context, gitServerName string) (*gitHubInfo, error) {
	if gitServerName == "" {
		return nil, trace.BadParameter("missing git server name")
	}
	gitServer, err := s.cache.GetGitServer(ctx, gitServerName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	github := gitServer.GetGitHub()
	if github == nil {
		return nil, trace.BadParameter("git server %v is not a GitHub server", gitServerName)
	}

	ig, err := s.cache.GetIntegration(ctx, github.Integration)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var clientID string
	if status := ig.GetStatus().GitHub; status != nil {
		clientID = status.ClientID
	}
	if clientID == "" {
		return nil, trace.BadParameter("integration %v has no client ID in status; re-create the integration", ig.GetName())
	}

	return &gitHubInfo{
		gitServer:   gitServer,
		github:      github,
		integration: ig,
		clientID:    clientID,
	}, nil
}

// resolveClientSecret looks up the client secret from static credentials.
// Only needed for operations that call the GitHub API (token refresh, revocation).
func (s *CredentialsService) resolveClientSecret(ctx context.Context, ig types.Integration) (string, error) {
	ref := ig.GetCredentials().GetStaticCredentialsRef()
	if ref == nil {
		return "", trace.BadParameter("integration %v has no credentials", ig.GetName())
	}
	oauthCred, err := credentials.GetByPurpose(ctx, ref, credentials.PurposeGitHubOAuth, s.cache)
	if err != nil {
		return "", trace.Wrap(err)
	}
	_, clientSecret := oauthCred.GetOAuthClientSecret()
	return clientSecret, nil
}

// CheckGitCredentials checks whether stored git credentials exist for the
// calling user.
func (s *CredentialsService) CheckGitCredentials(ctx context.Context, in *pb.CheckGitCredentialsRequest) (*pb.CheckGitCredentialsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	info, err := s.resolveGitHub(ctx, in.GetGitServerName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	creds, err := s.backend.GetUserExternalCredentials(ctx, authCtx.User.GetName(), info.clientID)
	if err != nil {
		if trace.IsNotFound(err) {
			return pb.CheckGitCredentialsResponse_builder{Valid: false}.Build(), nil
		}
		return nil, trace.Wrap(err)
	}

	resp := pb.CheckGitCredentialsResponse_builder{Valid: true}
	if githubOAuth := creds.GetSpec().GetGithubOauth(); githubOAuth != nil {
		resp.GithubUsername = githubOAuth.GetUsername()
	}
	return resp.Build(), nil
}

// RevokeGitCredentials revokes stored git credentials for the calling user.
func (s *CredentialsService) RevokeGitCredentials(ctx context.Context, in *pb.RevokeGitCredentialsRequest) (*pb.RevokeGitCredentialsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username := authCtx.User.GetName()

	info, err := s.resolveGitHub(ctx, in.GetGitServerName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	creds, err := s.backend.GetUserExternalCredentials(ctx, username, info.clientID)
	if err != nil {
		if trace.IsNotFound(err) {
			return pb.RevokeGitCredentialsResponse_builder{}.Build(), nil
		}
		return nil, trace.Wrap(err)
	}

	if accessToken := creds.GetSpec().GetGithubOauth().GetAccessToken(); accessToken != "" {
		if clientSecret, err := s.resolveClientSecret(ctx, info.integration); err == nil && clientSecret != "" {
			if err := credentials.RevokeGitHubTokenGrant(ctx, info.clientID, clientSecret, accessToken); err != nil {
				s.logger.WarnContext(ctx, "Failed to revoke GitHub token", "user", username, "error", err)
			}
		}
	}

	if err := s.backend.DeleteUserExternalCredentials(ctx, username, info.clientID); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.emitter.EmitAuditEvent(ctx, &apievents.GitCredentialRevoke{
		Metadata: apievents.Metadata{
			Type: events.GitCredentialRevokeEvent,
			Code: events.GitCredentialRevokeCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		GitMetadata: apievents.GitMetadata{
			GitServerName: info.gitServer.GetName(),
			Organization:  info.github.Organization,
			Integration:   info.github.Integration,
		},
	}); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit git credential revoke event", "error", err)
	}

	return pb.RevokeGitCredentialsResponse_builder{}.Build(), nil
}

// GenerateGitHubAppToken generates a GitHub App access token for git
// operations. Auth verifies the provided user certificate and returns a valid
// access token.
func (s *CredentialsService) GenerateGitHubAppToken(ctx context.Context, in *pb.GenerateGitHubAppTokenRequest) (*pb.GenerateGitHubAppTokenResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authz.HasBuiltinRole(*authCtx, string(types.RoleProxy)) {
		return nil, trace.AccessDenied("GenerateGitHubAppToken is only available to proxy services")
	}

	if len(in.GetUserCert()) == 0 {
		return nil, trace.BadParameter("missing user certificate")
	}

	cert, err := s.certVerifier(in.GetUserCert())
	if err != nil {
		return nil, trace.Wrap(err, "verifying user certificate")
	}

	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if identity.RouteToGit.GitServerName == "" {
		return nil, trace.BadParameter("certificate does not contain RouteToGit")
	}

	if identity.PrivateKeyPolicy == "web_session" {
		return nil, trace.AccessDenied("web sessions cannot be used for git access")
	}

	info, err := s.resolveGitHub(ctx, identity.RouteToGit.GitServerName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	creds, err := s.backend.GetUserExternalCredentials(ctx, identity.Username, info.clientID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	githubOAuth := creds.GetSpec().GetGithubOauth()
	if githubOAuth == nil {
		return nil, trace.NotFound("no GitHub OAuth credentials found for user %v", identity.Username)
	}

	accessToken := githubOAuth.GetAccessToken()

	// Proactively refresh the token if it expires within 5 minutes, ensuring
	// the returned token is usable for at least one session chunk.
	if expiry := githubOAuth.GetAccessTokenExpiry(); expiry != nil && expiry.IsValid() {
		if s.clock.Now().Add(5 * time.Minute).After(expiry.AsTime()) {
			refreshToken := githubOAuth.GetRefreshToken()
			if refreshToken == "" {
				return nil, trace.AccessDenied("GitHub access token expired and no refresh token available for user %v; re-run 'tsh git login'", identity.Username)
			}

			newToken, err := s.refreshGitHubToken(ctx, info.integration, refreshToken)
			if err != nil {
				return nil, trace.Wrap(err, "refreshing GitHub token")
			}
			accessToken = newToken.AccessToken

			s.saveRefreshedCredentials(ctx, info.integration, creds, newToken)

			s.logger.DebugContext(ctx, "Generated GitHub app token",
				"user", identity.Username,
				"git_server", identity.RouteToGit.GitServerName,
				"refreshed", true,
				"new_expiry", newToken.Expiry,
			)
		}
	}

	return pb.GenerateGitHubAppTokenResponse_builder{
		AccessToken: accessToken,
	}.Build(), nil
}

func (s *CredentialsService) refreshGitHubToken(ctx context.Context, ig types.Integration, refreshToken string) (*oauth2.Token, error) {
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

const defaultMaxCredentialTTL = 7 * 24 * time.Hour

func (s *CredentialsService) saveRefreshedCredentials(ctx context.Context, ig types.Integration, creds *userexternalcredentialsv1.UserExternalCredentials, token *oauth2.Token) {
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

	ttl := defaultMaxCredentialTTL
	if spec := ig.GetGitHubIntegrationSpec(); spec != nil && spec.MaxCredentialTTL != nil {
		if d, err := gogotypes.DurationFromProto(spec.MaxCredentialTTL); err == nil && d > 0 {
			ttl = d
		}
	}
	creds.GetMetadata().Expires = timestamppb.New(s.clock.Now().Add(ttl))

	if _, err := s.backend.UpsertUserExternalCredentials(ctx, creds); err != nil {
		s.logger.WarnContext(ctx, "Failed to save refreshed GitHub credentials", "error", err)
	}
}
