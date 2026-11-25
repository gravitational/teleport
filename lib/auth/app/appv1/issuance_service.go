// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package appv1

import (
	"cmp"
	"context"
	"crypto/x509"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/gravitational/teleport"
	appv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/app/v1"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/jwt"
	scopedapp "github.com/gravitational/teleport/lib/scopes/app"
	"github.com/gravitational/teleport/lib/services"
	appcommon "github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/oidc"
)

// Cache is the subset of cached reads required by the app issuance service.
type Cache interface {
	GetAppSession(ctx context.Context, req types.GetAppSessionRequest) (types.WebSession, error)
	ListAppServersForHost(ctx context.Context, req *presencev1.ListAppServersForHostRequest) ([]*types.AppServerV3, string, error)
	services.AuthorityGetter
	services.ClusterNameGetter
	services.ProxyGetter
}

// IssuanceServiceConfig is the config for [IssuanceService].
type IssuanceServiceConfig struct {
	// Authorizer is the authorizer to use.
	Authorizer authz.ScopedAuthorizer
	// Cache is the cache used to fetch resources.
	Cache Cache
	// KeyStore is used to sign JWT tokens.
	KeyStore *keystore.Manager

	clock clockwork.Clock
}

// IssuanceService implements teleport.app.v1.AppIssuanceService.
type IssuanceService struct {
	appv1.UnimplementedAppIssuanceServiceServer

	authorizer authz.ScopedAuthorizer
	cache      Cache
	keyStore   *keystore.Manager
	clock      clockwork.Clock
	logger     *slog.Logger
}

var _ appv1.AppIssuanceServiceServer = (*IssuanceService)(nil)

// NewIssuanceService returns a new AppIssuanceService.
func NewIssuanceService(cfg IssuanceServiceConfig) (*IssuanceService, error) {
	switch {
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Cache == nil:
		return nil, trace.BadParameter("cache is required")
	case cfg.KeyStore == nil:
		return nil, trace.BadParameter("key store is required")
	}
	if cfg.clock == nil {
		cfg.clock = clockwork.NewRealClock()
	}
	return &IssuanceService{
		authorizer: cfg.Authorizer,
		cache:      cfg.Cache,
		keyStore:   cfg.KeyStore,
		clock:      cfg.clock,
		logger:     slog.With(teleport.ComponentKey, "app.issuance.service"),
	}, nil
}

// IssueAppOIDCToken issues a JWT signed by the cluster's OIDC IdP CA for an
// active application session.
func (s *IssuanceService) IssueAppOIDCToken(ctx context.Context, req *appv1.IssueAppOIDCTokenRequest) (*appv1.IssueAppOIDCTokenResponse, error) {
	callerHostID, identity, err := s.authorizeAppService(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	appServer, err := s.getAppServer(ctx, identity, callerHostID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	audience, err := getOIDCTokenAudience(appServer.GetApp())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	expires, err := s.getOIDCTokenExpires(req.GetTtl(), identity.Expires)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	issuer, err := s.getOIDCTokenIssuer(ctx, appServer.GetApp())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roles, traits := appcommon.RolesAndTraitsForAppToken(identity, appServer.GetApp())
	token, err := s.signJWT(ctx, types.OIDCIdPCA, jwt.SignParams{
		Issuer:   issuer,
		Username: identity.Username,
		Roles:    roles,
		Traits:   traits,
		Audience: audience,
		Expires:  expires,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.logger.InfoContext(ctx, "Generated OIDC token",
		"app_session_id", req.GetAppSessionId(),
		"username", identity.Username,
		"app_public_addr", identity.RouteToApp.PublicAddr,
		"audience", audience,
		"ttl", req.GetTtl().AsDuration(),
	)
	return appv1.IssueAppOIDCTokenResponse_builder{Token: token}.Build(), nil
}

// authorizeAppService authorizes an app service to issue a token for the
// identity of the request, and returns the caller's host ID and the identity.
func (s *IssuanceService) authorizeAppService(ctx context.Context, req *appv1.IssueAppOIDCTokenRequest) (string, *tlsca.Identity, error) {
	authCtx, err := s.authorizer.AuthorizeScoped(ctx)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	if !authCtx.HasBuiltinRole(types.RoleApp) {
		return "", nil, trace.AccessDenied("this request can be only executed by the app service")
	}
	serverID, ok := authCtx.LocalServerID()
	if !ok {
		return "", nil, trace.AccessDenied("this request can be only executed by the app service")
	}

	identity, err := s.getIdentity(ctx, req)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAgentScope(identity.RouteToApp.Scope); err != nil {
		return "", nil, trace.Wrap(err)
	}
	return serverID, identity, nil
}

func (s *IssuanceService) getIdentity(ctx context.Context, req *appv1.IssueAppOIDCTokenRequest) (*tlsca.Identity, error) {
	// TODO(greedy52) require user cert in v20.
	if len(req.GetUserCertificate()) > 0 {
		identity, err := s.getIdentityFromUserCert(ctx, req.GetUserCertificate())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if identity.RouteToApp.SessionID != req.GetAppSessionId() {
			return nil, trace.AccessDenied("app session ID mismatch")
		}
		return identity, nil
	}
	return s.getIdentityFromAppSession(ctx, req.GetAppSessionId())
}

func (s *IssuanceService) getIdentityFromUserCert(ctx context.Context, rawCert []byte) (*tlsca.Identity, error) {
	cert, err := x509.ParseCertificate(rawCert)
	if err != nil {
		return nil, trace.Wrap(err, "parsing user certificate")
	}
	identity, err := tlsca.IdentityFromCert(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Verify the user certificate against the issuing cluster's User CA.
	ca, err := s.cache.GetCertAuthority(ctx, types.CertAuthID{
		Type:        types.UserCA,
		ClusterName: identity.TeleportCluster,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roots, err := services.CertPool(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := services.VerifyUserCertificateWithClockSkew(cert, s.clock, roots); err != nil {
		return nil, trace.Wrap(err, "invalid user certificate")
	}

	clusterName, err := s.cache.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch identity.TeleportCluster {
	// Check if app session exists on local cluster.
	case clusterName.GetClusterName():
		if _, err := s.cache.GetAppSession(ctx, types.GetAppSessionRequest{
			SessionID: identity.RouteToApp.SessionID,
		}); err != nil {
			return nil, trace.Wrap(err)
		}

	// If the identity is from a remote cluster, the identity needs some
	// mapping. Also app session check is skipped as the app session only exists
	// in root cluster.
	//
	// TODO(greedy52) normally remote user mapping (name, roles, traits) is
	// handled by the auth middleware + authorizer. Consider refactoring to
	// have a shared utility for identity mapping without going through the
	// authorizer.
	default:
		accessInfo, err := services.AccessInfoFromRemoteTLSIdentity(*identity, ca.CombinedMapping())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		identity.Username = services.UsernameForRemoteCluster(identity.Username, identity.TeleportCluster)
		identity.Groups = accessInfo.Roles
		identity.Traits = accessInfo.Traits
	}

	s.logger.DebugContext(ctx, "Resolved user identity from certificate",
		"username", identity.Username,
		"teleport_cluster", identity.TeleportCluster,
		"route_to_app", identity.RouteToApp,
	)
	return identity, nil
}

func (s *IssuanceService) getIdentityFromAppSession(ctx context.Context, sessionID string) (*tlsca.Identity, error) {
	if sessionID == "" {
		return nil, trace.BadParameter("app session id missing")
	}

	session, err := s.cache.GetAppSession(ctx, types.GetAppSessionRequest{
		SessionID: sessionID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := tlsca.ParseCertificatePEM(session.GetTLSCert())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity, err := tlsca.IdentityFromCert(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.logger.DebugContext(ctx, "Resolved user identity from app session ID", "username", identity.Username, "route_to_app", identity.RouteToApp)
	return identity, nil
}

func (s *IssuanceService) getAppServer(ctx context.Context, identity *tlsca.Identity, callerHostID string) (types.AppServer, error) {
	route := identity.RouteToApp
	// route.Name should always be populated past v19
	if route.Name == "" {
		return nil, trace.BadParameter("app session identity missing the app name")
	}

	// TODO (williamo): look into also adding the app name in the ListAppServersForHostRequest.
	for server, err := range clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageToken string) ([]*types.AppServerV3, string, error) {
		return s.cache.ListAppServersForHost(ctx, presencev1.ListAppServersForHostRequest_builder{
			HostId:    callerHostID,
			Scope:     route.Scope,
			PageSize:  int32(pageSize),
			PageToken: pageToken,
		}.Build())
	}) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if server.GetApp().GetName() == route.Name {
			return server, nil
		}

	}
	return nil, trace.NotFound("application %q not found for app service %q", cmp.Or(route.Name, route.PublicAddr), callerHostID)
}

// getOIDCTokenIssuer returns the issuer of OIDC tokens for the app. Unscoped
// apps use the cluster issuer. Scoped apps each have their own issuer, so a
// relying party that trusts one app's issuer rejects tokens minted for any
// other app.
func (s *IssuanceService) getOIDCTokenIssuer(ctx context.Context, app types.Application) (string, error) {
	if app.GetScope() == "" {
		issuer, err := oidc.IssuerForCluster(ctx, s.cache)
		return issuer, trace.Wrap(err)
	}
	issuerPath, err := scopedapp.OIDCAppTokenIssuerPath(app.GetScope(), app.GetName())
	if err != nil {
		return "", trace.Wrap(err)
	}
	issuer, err := oidc.IssuerForCluster(ctx, s.cache, issuerPath)
	return issuer, trace.Wrap(err)
}

// getOIDCTokenAudience verifies the app may have OIDC tokens issued for it and
// returns the audience to be used. This check ensures that the audience of the
// JWT token always starts with specific schema so it won't collide with
// consumers like AWS/Azure OIDC integrations.
// Scoped MCP apps receive an audience in the form of <scope>/<app-name>.
func getOIDCTokenAudience(app types.Application) (string, error) {
	switch types.GetMCPServerTransportType(app.GetURI()) {
	case types.MCPTransportHTTP, types.MCPTransportSSE:
		if app.GetScope() == "" {
			return app.GetURI(), nil
		}
		return scopedapp.AppTokenAudience(app.GetScope(), app.GetName()), nil
	}
	return "", trace.BadParameter("application protocol does not support OIDC token signing")
}

func (s *IssuanceService) getOIDCTokenExpires(reqTTL *durationpb.Duration, identityExpires time.Time) (time.Time, error) {
	if reqTTL == nil {
		return time.Time{}, trace.BadParameter("ttl is required")
	}

	ttl := reqTTL.AsDuration()
	if ttl <= 0 {
		return time.Time{}, trace.BadParameter("ttl must be positive")
	}
	// App service should request a small TTL like 10 minutes. Cap it to one
	// hour just in case.
	if ttl > time.Hour {
		return time.Time{}, trace.BadParameter("ttl exceeds maximum lifetime of one hour")
	}
	// Clamp to the app session cert expiry so the token never outlives it.
	expires := s.clock.Now().Add(ttl)
	if expires.After(identityExpires) {
		expires = identityExpires
	}
	return expires, nil
}

func (s *IssuanceService) signJWT(ctx context.Context, caType types.CertAuthType, params jwt.SignParams) (string, error) {
	clusterName, err := s.cache.GetClusterName(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	ca, err := s.cache.GetCertAuthority(ctx, types.CertAuthID{
		Type:        caType,
		ClusterName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return "", trace.Wrap(err)
	}
	signer, err := s.keyStore.GetJWTSigner(ctx, ca)
	if err != nil {
		return "", trace.Wrap(err)
	}
	key, err := services.GetJWTSigner(signer, ca.GetClusterName(), s.clock)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return key.Sign(params)
}
