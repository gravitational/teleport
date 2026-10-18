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
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/oauth2"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/credentialencryption"
	"github.com/gravitational/teleport/lib/auth/userexternalcredentials"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// Cache gets the resources used by the service, e.g. the Auth cache.
type Cache interface {
	services.AuthorityGetter
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

	return &Service{
		cfg:            cfg,
		decryptLimiter: decryptLimiter,
		handlers:       map[string]resourceHandler{},
	}, nil
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
	session, err := s.cfg.Backend.Get(ctx, keyID)
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
	username := authCtx.User.GetName()
	if user := req.GetFilter().GetUser(); user != "" && user != username {
		return nil, trace.AccessDenied("cannot list user external credentials of other users")
	}
	if req.GetFilter().GetKeyId() != "" {
		return nil, trace.BadParameter("filtering by key_id is not supported")
	}

	resources, nextPageToken, err := s.cfg.Backend.List(ctx, int(req.GetPageSize()), req.GetPageToken(), pb.EncryptionKeyID_builder{
		User: username,
		// TODO(greedy52) support filter by delegate
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
	if err := s.decryptLimiter.RegisterRequest(identity.Username); err != nil {
		return nil, trace.Wrap(err)
	}

	payload, err := s.cfg.Encryptor.DecryptPayload(ctx, req.GetAuthEncryptedBlob())
	if err != nil {
		return nil, trace.Wrap(err, "decrypting user external credential")
	}
	if payload.User != identity.Username {
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
	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
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
