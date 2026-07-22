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
	"encoding/hex"
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
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cryptoutils"
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
// TokenEncryptor encrypts and decrypts credential tokens.
type TokenEncryptor interface {
	EncryptionEnabled() bool
	EncryptTokens(ctx context.Context, accessToken, refreshToken string) ([]byte, error)
	DecryptTokens(ctx context.Context, ciphertext []byte) (accessToken, refreshToken string, err error)
}

// TokenDistributor distributes double-encrypted tokens to all sessions.
type TokenDistributor func(ctx context.Context, username, gitServerName, accessToken, refreshToken string, expiry time.Time, logger *slog.Logger) error

type CredentialsServiceConfig struct {
	Authorizer     authz.Authorizer
	Cache          CredentialsCache
	Backend        CredentialsBackend
	RawBackend     backend.Backend
	Emitter        apievents.Emitter
	CertVerifier   CertVerifier
	TokenEncryptor TokenEncryptor
	Distributor    TokenDistributor
	Semaphores     types.Semaphores
	Logger         *slog.Logger
	Clock          clockwork.Clock
}

// CredentialsService implements the GitCredentialsService gRPC service.
type CredentialsService struct {
	pb.UnimplementedGitCredentialsServiceServer

	authorizer     authz.Authorizer
	cache          CredentialsCache
	backend        CredentialsBackend
	rawBackend     backend.Backend
	emitter        apievents.Emitter
	certVerifier   CertVerifier
	tokenEncryptor TokenEncryptor
	distributor    TokenDistributor
	semaphores     types.Semaphores
	logger         *slog.Logger
	clock          clockwork.Clock
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
		authorizer:     cfg.Authorizer,
		cache:          cfg.Cache,
		backend:        cfg.Backend,
		rawBackend:     cfg.RawBackend,
		emitter:        cfg.Emitter,
		certVerifier:   cfg.CertVerifier,
		tokenEncryptor: cfg.TokenEncryptor,
		distributor:    cfg.Distributor,
		semaphores:     cfg.Semaphores,
		logger:         cfg.Logger,
		clock:          cfg.Clock,
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

	// If the client provided a KMS-encrypted access token, decrypt it
	// directly instead of looking up stored credentials. This supports
	// the double-encrypted token flow where tokens are stored client-side.
	if kmsToken := in.GetKmsEncryptedToken(); len(kmsToken) > 0 {
		// KMS-decrypt to get the EncryptedPayload JSON.
		payloadJSON, _, err := s.tokenEncryptor.DecryptTokens(ctx, kmsToken)
		if err != nil {
			return nil, trace.Wrap(err, "KMS-decrypting client-provided token")
		}

		// Verify the payload binding.
		payload, err := cryptoutils.UnmarshalEncryptedPayload([]byte(payloadJSON))
		if err != nil {
			return nil, trace.Wrap(err, "unmarshaling encrypted payload")
		}
		if payload.User != identity.Username {
			return nil, trace.AccessDenied("token does not belong to this user")
		}
		if payload.Resource.Kind != types.KindGitServer || payload.Resource.Name != identity.RouteToGit.GitServerName {
			return nil, trace.AccessDenied("token does not match the requested git server")
		}

		s.logger.DebugContext(ctx, "Decrypted client-provided KMS token",
			"user", identity.Username,
			"git_server", identity.RouteToGit.GitServerName,
		)
		return pb.GenerateGitHubAppTokenResponse_builder{
			AccessToken: string(payload.Payload),
		}.Build(), nil
	}

	return nil, trace.NotFound("no credentials provided; run 'tsh git login' to authenticate")
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

// getTokens extracts the access and refresh tokens, decrypting if needed.
func (s *CredentialsService) getTokens(ctx context.Context, ghCreds *userexternalcredentialsv1.GitHubOAuthCredentials) (accessToken, refreshToken string, err error) {
	if encrypted := ghCreds.GetEncryptedTokens(); len(encrypted) > 0 {
		accessToken, refreshToken, err = s.tokenEncryptor.DecryptTokens(ctx, encrypted)
		if err != nil {
			return "", "", trace.Wrap(err)
		}
		return accessToken, refreshToken, nil
	}
	return ghCreds.GetAccessToken(), ghCreds.GetRefreshToken(), nil
}

func (s *CredentialsService) saveRefreshedCredentials(ctx context.Context, ig types.Integration, creds *userexternalcredentialsv1.UserExternalCredentials, token *oauth2.Token) {
	ghCreds := creds.GetSpec().GetGithubOauth()
	if ghCreds == nil {
		return
	}

	if s.tokenEncryptor.EncryptionEnabled() {
		encrypted, err := s.tokenEncryptor.EncryptTokens(ctx, token.AccessToken, token.RefreshToken)
		if err != nil {
			s.logger.WarnContext(ctx, "Failed to encrypt refreshed tokens", "error", err)
			return
		}
		ghCreds.SetEncryptedTokens(encrypted)
		ghCreds.SetAccessToken("")
		ghCreds.SetRefreshToken("")
	} else {
		ghCreds.SetAccessToken(token.AccessToken)
		if token.RefreshToken != "" {
			ghCreds.SetRefreshToken(token.RefreshToken)
		}
	}

	if !token.Expiry.IsZero() {
		ghCreds.SetAccessTokenExpiry(timestamppb.New(token.Expiry))
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

// RefreshGitToken refreshes a GitHub access token using the provided
// auth-encrypted refresh token. Auth unseals it, calls GitHub, and
// distributes the new tokens to all sessions.
//
// A backend semaphore ensures only one auth server refreshes at a time for a
// given user+resource. If another auth server is already refreshing, this call
// waits for the semaphore and then returns success (the tokens were already
// distributed by the other server).
func (s *CredentialsService) RefreshGitToken(ctx context.Context, req *pb.RefreshGitTokenRequest) (*pb.RefreshGitTokenResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gitServerName := req.GetGitServerName()
	if gitServerName == "" {
		return nil, trace.BadParameter("git_server_name is required")
	}
	authEncryptedRefresh := req.GetAuthEncryptedRefreshToken()
	if len(authEncryptedRefresh) == 0 {
		return nil, trace.BadParameter("auth_encrypted_refresh_token is required")
	}

	// Verify user has access to the git server.
	gitServer, err := s.cache.GetGitServer(ctx, gitServerName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.Checker.CheckAccess(gitServer, services.AccessState{MFAVerified: true}); err != nil {
		return nil, trace.Wrap(err)
	}

	username := authCtx.User.GetName()

	// Acquire semaphore to prevent concurrent refreshes for the same
	// user+resource across multiple auth servers.
	if s.semaphores != nil {
		semName := hex.EncodeToString([]byte(username + "/" + gitServerName))
		lease, err := services.AcquireSemaphoreWithRetry(ctx, services.AcquireSemaphoreWithRetryConfig{
			Service: s.semaphores,
			Request: types.AcquireSemaphoreRequest{
				SemaphoreKind: "git_token_refresh",
				SemaphoreName: semName,
				MaxLeases:     1,
				Holder:        username,
			},
			Retry: retryutils.LinearConfig{
				Step:  time.Second,
				Max:   time.Second,
				Clock: s.clock,
			},
			TTL: time.Minute,
			Now: s.clock.Now,
		})
		if err != nil {
			s.logger.WarnContext(ctx, "Failed to acquire refresh semaphore",
				"user", username,
				"git_server", gitServerName,
				"error", err,
			)
			return nil, trace.Wrap(err, "failed to acquire refresh lock")
		}
		defer func() {
			if err := s.semaphores.CancelSemaphoreLease(ctx, *lease); err != nil {
				s.logger.WarnContext(ctx, "Failed to release refresh semaphore", "error", err)
			}
		}()
	}

	// Auth-unseal the refresh token.
	refreshPayloadJSON, _, err := s.tokenEncryptor.DecryptTokens(ctx, authEncryptedRefresh)
	if err != nil {
		return nil, trace.Wrap(err, "auth-decrypting refresh token")
	}

	// Verify the payload binding.
	refreshPayload, err := cryptoutils.UnmarshalEncryptedPayload([]byte(refreshPayloadJSON))
	if err != nil {
		return nil, trace.Wrap(err, "unmarshaling refresh token payload")
	}
	if refreshPayload.User != username {
		return nil, trace.AccessDenied("refresh token does not belong to this user")
	}

	// Resolve the integration for this git server.
	info, err := s.resolveGitHub(ctx, gitServerName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Call GitHub to refresh.
	newToken, err := s.refreshGitHubToken(ctx, info.integration, string(refreshPayload.Payload))
	if err != nil {
		return nil, trace.Wrap(err, "refreshing GitHub token")
	}

	// Distribute double-encrypted tokens to all sessions.
	if s.distributor != nil {
		if err := s.distributor(ctx, username, gitServerName, newToken.AccessToken, newToken.RefreshToken, newToken.Expiry, s.logger); err != nil {
			s.logger.WarnContext(ctx, "Failed to distribute refreshed tokens", "error", err)
		}
	}

	s.logger.DebugContext(ctx, "Refreshed git token",
		"user", username,
		"git_server", gitServerName,
	)

	return pb.RefreshGitTokenResponse_builder{}.Build(), nil
}

