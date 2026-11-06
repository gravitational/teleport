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
	"context"
	"crypto/x509"
	"iter"
	"log/slog"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/credentialencryption"
	"github.com/gravitational/teleport/lib/auth/integration/credentials"
	"github.com/gravitational/teleport/lib/auth/userexternalcredentials"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib/safetransport"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// Cache gets the resources used by the service, e.g. the Auth cache.
type Cache interface {
	services.AuthorityGetter
	services.GitServerGetter
	credentials.IntegrationGetter
	credentials.ByLabelsGetter

	// RangeApplicationServersWithName returns an iterator over application servers for a given app name.
	RangeApplicationServersWithName(ctx context.Context, appName string) iter.Seq2[types.AppServer, error]
}

// rateLimiter limits the rate of requests by token, e.g.
// [limiter.RateLimiter].
type rateLimiter interface {
	// RegisterRequest returns an error if the token exceeds the rate limit.
	RegisterRequest(token string) error
}

// resourceHandler handles user external credentials of one kind of resource,
// e.g. git servers.
type resourceHandler interface {
	// CheckRoute checks the user certificate is routed to the resource, e.g.
	// RouteToGit.Name for git servers.
	CheckRoute(ctx context.Context, identity *tlsca.Identity, resourceName string) error
	// CheckOwnership checks the decrypted credential belongs to the user of the
	// certificate, e.g. the GitHub user of the certificate.
	CheckOwnership(ctx context.Context, identity *tlsca.Identity, decrypted *credentialencryption.Payload) error
	// CheckAccess checks the user can access the resource, e.g. before
	// refreshing the credential of the user for the resource.
	CheckAccess(ctx context.Context, authCtx *authz.Context, resourceName string) error
	// RefreshOAuthToken exchanges the refresh token of the user for the
	// resource for a new OAuth token.
	RefreshOAuthToken(ctx context.Context, username, resourceName string, refreshToken []byte) (*oauth2.Token, error)
}

// ServiceConfig is the config for Service.
type ServiceConfig struct {
	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer
	// Backend is the backend for user external credentials.
	Backend services.UserExternalCredentials
	// Logger is the slog logger.
	Logger *slog.Logger
	// Encryptor encrypts and decrypts the Auth-layer of user external
	// credentials.
	Encryptor userexternalcredentials.Encryptor
	// Cache gets the user CA to verify user certificates.
	Cache Cache
	// ClusterName is the name of the local cluster.
	ClusterName string
	// Clock is used to check token expiry.
	Clock clockwork.Clock
	// UpstreamTransport (optional) overrides the HTTP transport used for
	// calling external services, e.g. GitHub. Used in tests.
	UpstreamTransport http.RoundTripper
	// Emitter emits audit events for credential refresh.
	Emitter apievents.Emitter
	// httpClient is the client for calling external services, e.g. GitHub.
	httpClient *http.Client
	// mcpHTTPClient is used to call the token endpoints of MCP servers. Users
	// choose those endpoints so it can only dial public addresses.
	// TODO(avatus): remove once httpClient becomes a safe transport in
	// https://github.com/gravitational/core/pull/3203
	mcpHTTPClient *http.Client
}

// Service implements the UserExternalCredentialService gRPC service.
type Service struct {
	pb.UnimplementedUserExternalCredentialServiceServer

	cfg ServiceConfig

	// handlers handle user external credentials by resource kind.
	handlers map[string]resourceHandler
	// decryptLimiter limits the rate of decryption per user.
	decryptLimiter rateLimiter
	// verifyUserCertificateOverride overrides verifyUserCertificate in tests.
	verifyUserCertificateOverride func(ctx context.Context, certDER []byte) (*tlsca.Identity, error)
	// writeLimiter limits the rate of refresh and sync per user.
	writeLimiter rateLimiter
}

// NewService creates a new UserExternalCredentialService.
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend is required")
	}
	if cfg.Encryptor == nil {
		return nil, trace.BadParameter("encryptor is required")
	}
	if cfg.Cache == nil {
		return nil, trace.BadParameter("cache is required")
	}
	if cfg.ClusterName == "" {
		return nil, trace.BadParameter("cluster name is required")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, teleport.Component(teleport.ComponentAuth, "user-external-credentials"))
	}
	httpClient, err := defaults.HTTPClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.UpstreamTransport != nil {
		httpClient.Transport = cfg.UpstreamTransport
	}
	cfg.httpClient = httpClient

	mcpTransport, err := safetransport.New()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg.mcpHTTPClient = &http.Client{
		Transport: mcpTransport,
		Timeout:   defaults.HTTPRequestTimeout,
		// token requests carry the code, verifier, refresh token and client secret in
		// the body, which a 307 or 308 would send on to the new location
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	// A per-user limit for decrypt calls for peace of mind. Callers are expected
	// to cache decrypted credentials per resource for minutes. With 20 calls per
	// minute and a burst of 40, a user can use ~30 apps behind 2 app services
	// that decrypt every 5 minutes.
	decryptLimiter, err := limiter.NewRateLimiter(limiter.Config{
		Rates: []limiter.Rate{{
			Period:  defaults.LimiterPeriod,
			Average: defaults.LimiterAverage,
			Burst:   defaults.LimiterBurst,
		}},
		Clock: cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Refresh and sync calls use a separate limiter with same limits.
	writeLimiter, err := limiter.NewRateLimiter(limiter.Config{
		Rates: []limiter.Rate{{
			Period:  defaults.LimiterPeriod,
			Average: defaults.LimiterAverage,
			Burst:   defaults.LimiterBurst,
		}},
		Clock: cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &Service{
		cfg:            cfg,
		decryptLimiter: decryptLimiter,
		writeLimiter:   writeLimiter,
	}
	s.handlers = map[string]resourceHandler{
		types.KindGitServer: &gitServerHandler{cfg: &s.cfg},
		// Currently, OAuth Proxy is the only integration that supports user external credentials,
		// so KindIntegration maps directly to the OAuth Proxy handler.
		types.KindIntegration: &oauthProxyHandler{cfg: &s.cfg},
	}
	return s, nil
}

func (s *Service) getResourceHandler(resourceKind string) (resourceHandler, error) {
	handler, ok := s.handlers[resourceKind]
	if !ok {
		return nil, trace.BadParameter("user external credentials of %q are not supported", resourceKind)
	}
	return handler, nil
}

// GetUserExternalCredentials retrieves the caller's credentials.
func (s *Service) GetUserExternalCredentials(ctx context.Context, _ *pb.GetUserExternalCredentialsRequest) (*pb.GetUserExternalCredentialsResponse, error) {
	authCtx, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authz.IsLocalUser(*authCtx) {
		return nil, trace.AccessDenied("only users can get user external credentials")
	}

	resource, err := s.getUserExternalCredentials(ctx, authCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pb.GetUserExternalCredentialsResponse_builder{
		Credentials: resource,
	}.Build(), nil
}

func (s *Service) getUserExternalCredentials(ctx context.Context, authCtx *authz.Context) (*pb.UserExternalCredentials, error) {
	identity := authCtx.Identity.GetIdentity()
	keyID, err := userexternalcredentials.KeyIDFromIdentity(&identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := s.cfg.Backend.GetUserExternalCredentials(ctx, keyID)
	return session, trace.Wrap(err)
}

// ListUserExternalCredentials returns a page of credentials matching the
// filter. The user field in the filter is required.
func (s *Service) ListUserExternalCredentials(ctx context.Context, req *pb.ListUserExternalCredentialsRequest) (*pb.ListUserExternalCredentialsResponse, error) {
	authCtx, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authz.IsLocalUser(*authCtx) {
		return nil, trace.AccessDenied("only users can list user external credentials")
	}
	identity := authCtx.Identity.GetIdentity()
	owner, err := credentialOwner(&identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if user := req.GetFilter().GetUser(); user != "" && user != owner {
		return nil, trace.AccessDenied("cannot list user external credentials of other users")
	}
	if req.GetFilter().GetKeyId() != "" {
		return nil, trace.BadParameter("filtering by key_id is not supported")
	}

	resources, nextPageToken, err := s.cfg.Backend.ListUserExternalCredentials(ctx, int(req.GetPageSize()), req.GetPageToken(), pb.EncryptionKeyID_builder{
		User:     owner,
		Delegate: req.GetFilter().GetDelegate(),
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pb.ListUserExternalCredentialsResponse_builder{
		Credentials:   resources,
		NextPageToken: nextPageToken,
	}.Build(), nil
}

// DecryptUserExternalCredential decrypts an auth-encrypted credential blob.
func (s *Service) DecryptUserExternalCredential(ctx context.Context, req *pb.DecryptUserExternalCredentialRequest) (*pb.DecryptUserExternalCredentialResponse, error) {
	if len(req.GetAuthEncryptedBlob()) == 0 {
		return nil, trace.BadParameter("missing auth_encrypted_blob")
	}
	if len(req.GetUserCertificate()) == 0 {
		return nil, trace.BadParameter("missing user_certificate")
	}

	// Only Proxy is allowed for decrypt.
	authCtx, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authz.HasBuiltinRole(*authCtx, string(types.RoleProxy)) {
		return nil, trace.AccessDenied("only proxies can decrypt user external credentials")
	}

	// User cert check.
	identity, err := s.verifyUserCertificate(ctx, req.GetUserCertificate())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	owner, err := credentialOwner(identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.decryptLimiter.RegisterRequest(owner); err != nil {
		return nil, trace.Wrap(err)
	}

	payload, err := s.cfg.Encryptor.DecryptPayload(ctx, req.GetAuthEncryptedBlob())
	if err != nil {
		return nil, trace.Wrap(err, "decrypting user external credential")
	}

	if payload.User != owner {
		return nil, trace.AccessDenied("user external credential does not belong to the user certificate")
	}

	// Check route from identity matches expectation and verify token ownership.
	handler, err := s.getResourceHandler(payload.ResourceKind)
	if err != nil {
		return nil, trace.Wrap(err)
	} else if err := handler.CheckRoute(ctx, identity, payload.ResourceName); err != nil {
		return nil, trace.Wrap(err)
	} else if err := handler.CheckOwnership(ctx, identity, payload); err != nil {
		return nil, trace.Wrap(err)
	}

	return pb.DecryptUserExternalCredentialResponse_builder{
		Plaintext: payload.Credential,
	}.Build(), nil
}

// RefreshOAuthTokens refreshes the OAuth tokens of a credential of the
// caller's session. The refresh is only allowed when the access token stored
// for the session is about to expire, regardless of what the client has
// cached.
//
// TODO(greedy52) currently client side always refetches the entire user
// credentials resource after successful refresh. To optimize, we can return the
// refreshed credential in the response.
func (s *Service) RefreshOAuthTokens(ctx context.Context, req *pb.RefreshOAuthTokensRequest) (*pb.RefreshOAuthTokensResponse, error) {
	if len(req.GetAuthEncryptedRefreshToken()) == 0 {
		return nil, trace.BadParameter("missing auth_encrypted_refresh_token")
	}

	authCtx, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authz.IsLocalUser(*authCtx) {
		return nil, trace.AccessDenied("only users can refresh user external credentials")
	}
	identity := authCtx.Identity.GetIdentity()
	owner, err := credentialOwner(&identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.writeLimiter.RegisterRequest(owner); err != nil {
		return nil, trace.Wrap(err)
	}

	// Check resource access.
	payload, err := s.cfg.Encryptor.DecryptPayload(ctx, req.GetAuthEncryptedRefreshToken())
	if err != nil {
		return nil, trace.Wrap(err, "decrypting refresh token")
	} else if payload.User != owner {
		return nil, trace.AccessDenied("refresh token does not belong to the caller")
	}
	handler, err := s.getResourceHandler(payload.ResourceKind)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := handler.CheckAccess(ctx, authCtx, payload.ResourceName); err != nil {
		return nil, trace.Wrap(err)
	}

	// If credential is not ready for refresh (either client clock is too far
	// ahead or another session has refreshed) return success without
	// refreshing.
	if ready, err := s.isOAuthTokenReadyForRefresh(ctx, authCtx, payload); err != nil {
		return nil, trace.Wrap(err)
	} else if !ready {
		return &pb.RefreshOAuthTokensResponse{}, nil
	}

	// Refresh and distribute.
	token, err := handler.RefreshOAuthToken(ctx, owner, payload.ResourceName, payload.Credential)
	if err != nil {
		return nil, trace.Wrap(err, "refreshing OAuth token")
	}
	authEncrypted, err := userexternalcredentials.MakeAuthEncryptedOAuthCredential(ctx, s.cfg.Encryptor, owner, payload.ResourceKind, payload.ResourceName, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := userexternalcredentials.Distribute(ctx, s.cfg.Backend, owner, authEncrypted); err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.RefreshOAuthTokensResponse{}, nil
}

// SyncUserExternalCredentials syncs credentials to a target session by
// re-encrypting with the target's public key.
func (s *Service) SyncUserExternalCredentials(ctx context.Context, req *pb.SyncUserExternalCredentialsRequest) (*pb.SyncUserExternalCredentialsResponse, error) {
	authCtx, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authz.IsLocalUser(*authCtx) {
		return nil, trace.AccessDenied("only users can sync user external credentials")
	}
	identity := authCtx.Identity.GetIdentity()
	owner, err := credentialOwner(&identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.writeLimiter.RegisterRequest(owner); err != nil {
		return nil, trace.Wrap(err)
	}

	target := req.GetCredentials()
	targetKeyID := target.GetSpec().GetEncryptionKeyId()
	if err := checkSyncTarget(&identity, targetKeyID); err != nil {
		return nil, trace.Wrap(err)
	}

	existing, err := s.cfg.Backend.GetUserExternalCredentials(ctx, targetKeyID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	merged := mergeCredentials(existing, target)

	// Skip the update if nothing changed.
	if proto.Equal(existing, merged) {
		return &pb.SyncUserExternalCredentialsResponse{}, nil
	}

	if _, err := s.cfg.Backend.UpdateUserExternalCredentials(ctx, merged); err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.SyncUserExternalCredentialsResponse{}, nil
}

// verifyUserCertificate verifies the user certificate is issued by the local
// user CA and returns its identity. Certificates of remote clusters are not
// supported.
func (s *Service) verifyUserCertificate(ctx context.Context, certDER []byte) (*tlsca.Identity, error) {
	if s.verifyUserCertificateOverride != nil {
		return s.verifyUserCertificateOverride(ctx, certDER)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, trace.Wrap(err, "parsing user certificate")
	}
	identity, err := tlsca.IdentityFromCert(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if identity.TeleportCluster != s.cfg.ClusterName {
		return nil, trace.AccessDenied("user certificate of cluster %q is not supported", identity.TeleportCluster)
	}

	roots, err := services.CertPoolFromAuthorityGetter(ctx, s.cfg.Cache, types.UserCA, s.cfg.ClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := services.VerifyUserCertificateWithClockSkew(cert, s.cfg.Clock, roots); err != nil {
		return nil, trace.AccessDenied("invalid user certificate: %v", err)
	}
	return identity, nil
}

func (s *Service) isOAuthTokenReadyForRefresh(ctx context.Context, authCtx *authz.Context, payload *credentialencryption.Payload) (bool, error) {
	session, err := s.getUserExternalCredentials(ctx, authCtx)
	if err != nil {
		return false, trace.Wrap(err, "getting user external credentials")
	}
	credential, found := services.FindUserExternalCredential(session, payload.ResourceKind, payload.ResourceName)
	if !found {
		return false, trace.NotFound("no credential for %s %q in the session", payload.ResourceKind, payload.ResourceName)
	}
	expiry := credential.GetOauth().GetAccessTokenExpiry().AsTime()
	if now := s.cfg.Clock.Now(); expiry.After(now.Add(time.Minute)) {
		s.cfg.Logger.DebugContext(ctx, "Access token not expired",
			"user", authCtx.User.GetName(),
			"resource_name", payload.ResourceName,
			"access_token_expiry", expiry,
		)
		return false, nil
	}
	return true, nil
}

// checkSyncTarget checks the target is another session of the caller.
func checkSyncTarget(identity *tlsca.Identity, target *pb.EncryptionKeyID) error {
	owner, err := credentialOwner(identity)
	if err != nil {
		return trace.Wrap(err)
	}
	switch {
	case target.GetUser() != owner:
		return trace.AccessDenied("cannot sync user external credentials to other users")
	case target.GetKeyId() == identity.EncryptionKeyID:
		return trace.BadParameter("cannot sync user external credentials to the caller's own session")
	}
	return nil
}

func mergeCredentials(existing, incoming *pb.UserExternalCredentials) *pb.UserExternalCredentials {
	merged := proto.CloneOf(existing)
	for _, credential := range incoming.GetSpec().GetCredentials() {
		services.PutUserExternalCredential(merged, credential)
	}
	return merged
}

func credentialOwner(identity *tlsca.Identity) (string, error) {
	if chain := identity.Delegation.GetChain(); len(chain) > 0 {
		rootDelegator, err := services.DelegatorUsername(chain[0])
		if err != nil {
			return "", trace.Wrap(err)
		}
		return rootDelegator, nil
	}
	return identity.Username, nil
}
