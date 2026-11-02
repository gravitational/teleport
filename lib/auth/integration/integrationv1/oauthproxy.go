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
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	usercredpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/integration/credentials"
	"github.com/gravitational/teleport/lib/auth/userexternalcredentials"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	liboidc "github.com/gravitational/teleport/lib/oidc"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// oauthProxyAuthRequestTTL is the duration to store an in-flight request.
	// This must be long enough for the authorizer to complete authorization and consent.
	oauthProxyAuthRequestTTL = 5 * time.Minute
	// exchangeTimeout is the timeout for an OAuth exchange request.
	// The request times out, rather than retries, because authorization code is single use and requests are not idempotent.
	exchangeTimeout = 15 * time.Second
)

// oauthProxyHostIDNamespace is a UUID that represents the name space to be used for generating UUIDs for OAuth Proxy integration app servers.
var oauthProxyHostIDNamespace = uuid.NewSHA1(uuid.Nil, []byte("oauth-proxy"))

// OAuthProxyServiceConfig holds configuration options for the OAuth Proxy Integration gRPC service.
type OAuthProxyServiceConfig struct {
	Authorizer              authz.Authorizer
	Clock                   clockwork.Clock
	Logger                  *slog.Logger
	Backend                 Backend
	Cache                   Cache
	ProxyPublicAddrGetter   func(context.Context) string
	UserExternalCredentials services.UserExternalCredentials
	Encryptor               userexternalcredentials.Encryptor
	Client                  *http.Client
	Emitter                 apievents.Emitter
}

// CheckAndSetDefaults checks the OAuthProxyServiceConfig fields and returns an error if a required param is not provided,
// or sets a default for optional params.
func (cfg *OAuthProxyServiceConfig) CheckAndSetDefaults() error {
	switch {
	case cfg.Authorizer == nil:
		return trace.BadParameter("authorizer is required")
	case cfg.ProxyPublicAddrGetter == nil:
		return trace.BadParameter("proxy public addr getter is required")
	case cfg.Backend == nil:
		return trace.BadParameter("backend is required")
	case cfg.Cache == nil:
		return trace.BadParameter("cache is required")
	case cfg.UserExternalCredentials == nil:
		return trace.BadParameter("user external credentials is required")
	case cfg.Encryptor == nil:
		return trace.BadParameter("encryptor is required")
	case cfg.Emitter == nil:
		return trace.BadParameter("emitter is required")
	}

	if cfg.Client == nil {
		clt, err := defaults.HTTPClient()
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Client = clt
	}

	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, "integrations.oauthproxy.service")
	}

	return nil
}

// OAuthProxyService implements the teleport.integration.v1.OAuthProxyService RPC service.
type OAuthProxyService struct {
	integrationpb.UnimplementedOAuthProxyServiceServer

	authorizer              authz.Authorizer
	logger                  *slog.Logger
	clock                   clockwork.Clock
	backend                 Backend
	cache                   Cache
	proxyPublicAddrGetter   func(context.Context) string
	userExternalCredentials services.UserExternalCredentials
	encryptor               userexternalcredentials.Encryptor
	client                  *http.Client
	emitter                 apievents.Emitter
}

// NewOAuthProxyService returns a new OAuthProxyService.
func NewOAuthProxyService(cfg *OAuthProxyServiceConfig) (*OAuthProxyService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &OAuthProxyService{
		logger:                  cfg.Logger,
		authorizer:              cfg.Authorizer,
		clock:                   cfg.Clock,
		backend:                 cfg.Backend,
		cache:                   cfg.Cache,
		proxyPublicAddrGetter:   cfg.ProxyPublicAddrGetter,
		userExternalCredentials: cfg.UserExternalCredentials,
		encryptor:               cfg.Encryptor,
		client: &http.Client{
			Transport:     liboidc.NewOIDCRoundTripper(cfg.Client.Transport),
			CheckRedirect: cfg.Client.CheckRedirect,
			Jar:           cfg.Client.Jar,
			Timeout:       cfg.Client.Timeout,
		},
		emitter: cfg.Emitter,
	}, nil
}

var _ integrationpb.OAuthProxyServiceServer = (*OAuthProxyService)(nil)

// StartOAuthProxyAuthorization builds the in-flight OAuth Proxy authorization and stores it in the backend.
func (s *OAuthProxyService) StartOAuthProxyAuthorization(ctx context.Context, req *integrationpb.StartOAuthProxyAuthorizationRequest) (*integrationpb.StartOAuthProxyAuthorizationResponse, error) {
	if req.GetName() == "" {
		return nil, trace.BadParameter("name is required")
	}

	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	username := authzCtx.User.GetName()
	if username == "" {
		return nil, trace.BadParameter("username is required")
	}

	ig, err := s.backend.GetIntegration(ctx, integrationpb.GetIntegrationRequest_builder{
		Name: req.GetName(),
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToResource(ig, types.VerbUse); err != nil {
		return nil, trace.Wrap(err)
	}

	if !ig.IsOAuthProxy() {
		return nil, trace.BadParameter("integration is not an OAuth Proxy")
	}

	cfg, err := s.oauthConfig(ctx, ig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stateToken, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	verifier := oauth2.GenerateVerifier()

	opts := []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(verifier)}
	spec := ig.GetOAuthProxyIntegrationSpec()
	opts = slices.Grow(opts, len(spec.ExtraParams))
	for k, v := range spec.ExtraParams {
		// Integration verifies no reserved params in extra_params on save, but let's be doubly sure.
		if slices.Contains(types.ReservedOAuthParams(), k) {
			return nil, trace.BadParameter("extra_params contains reserved OAuth param: %s", k)
		}
		opts = append(opts, oauth2.SetAuthURLParam(k, v))
	}

	if err := s.backend.CreateOAuthProxyAuthRequest(ctx, types.OAuthProxyAuthRequest{
		StateToken:   stateToken,
		Integration:  ig.GetName(),
		CodeVerifier: verifier,
		User:         username,
		Expires:      s.clock.Now().Add(oauthProxyAuthRequestTTL),
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return integrationpb.StartOAuthProxyAuthorizationResponse_builder{
		AuthorizeUrl: cfg.AuthCodeURL(stateToken, opts...),
		State:        stateToken,
	}.Build(), nil
}

// CompleteOAuthProxyAuthorization completes the OAuth exchange using the request stored in the backend,
// then stores the credential using the credential store.
func (s *OAuthProxyService) CompleteOAuthProxyAuthorization(ctx context.Context, req *integrationpb.CompleteOAuthProxyAuthorizationRequest) (*integrationpb.CompleteOAuthProxyAuthorizationResponse, error) {
	if err := validateCompleteOAuthProxyAuthorizationRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}

	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ig, err := s.backend.GetIntegration(ctx, integrationpb.GetIntegrationRequest_builder{
		Name: req.GetName(),
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, grantedScopes, err := s.completeOAuthProxyAuthorization(ctx, authzCtx, ig, req)
	s.emitAuthorizeEvent(ctx, authzCtx, ig, grantedScopes, err)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

func (s *OAuthProxyService) completeOAuthProxyAuthorization(ctx context.Context, authzCtx *authz.Context, ig types.Integration, req *integrationpb.CompleteOAuthProxyAuthorizationRequest) (*integrationpb.CompleteOAuthProxyAuthorizationResponse, []string, error) {
	if err := authzCtx.CheckAccessToResource(ig, types.VerbUse); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if !ig.IsOAuthProxy() {
		return nil, nil, trace.BadParameter("integration is not an OAuth Proxy")
	}

	cfg, err := s.oauthConfig(ctx, ig)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	authReq, err := s.backend.ConsumeOAuthProxyAuthRequest(ctx, authzCtx.User.GetName(), ig.GetName(), req.GetState())
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to consume request", "integration", ig.GetName(), "error", err)
		return nil, nil, trace.NotFound("valid request not found")
	}

	exchangeCtx := context.WithValue(ctx, oauth2.HTTPClient, s.client)
	exchangeCtx, cancel := context.WithTimeout(exchangeCtx, exchangeTimeout)
	defer cancel()

	tok, err := cfg.Exchange(exchangeCtx, req.GetAuthorizationCode(), oauth2.VerifierOption(authReq.CodeVerifier))
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to complete token exchange", "integration", ig.GetName(), "error", err)
		return nil, nil, trace.AccessDenied("failed to complete token exchange")
	}

	if tok.AccessToken == "" {
		s.logger.ErrorContext(ctx, "Missing access token on provider response", "integration", ig.GetName())
		return nil, nil, trace.BadParameter("invalid response")
	}

	scopes := grantedScopes(ig.GetOAuthProxyIntegrationSpec().Scopes, tok)

	if err := s.storeUserCredentials(ctx, authReq.Integration, authReq.User, tok); err != nil {
		return nil, scopes, trace.Wrap(err)
	}

	return integrationpb.CompleteOAuthProxyAuthorizationResponse_builder{
		Expires: timestamppb.New(tok.Expiry),
	}.Build(), scopes, nil
}

// oauthConfig builds the OAuth config for the authorization request.
func (s *OAuthProxyService) oauthConfig(ctx context.Context, ig types.Integration) (*oauth2.Config, error) {
	redirectURL, err := s.redirectURI(ctx, ig.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	endpoints, err := credentials.ResolveOAuthProxyEndpoints(ctx, ig, s.client, credentials.OAuthEndpointKindAuthorize, credentials.OAuthEndpointKindToken)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to resolve endpoints", "error", err)
		return nil, trace.BadParameter("failed to resolve endpoints")
	}

	switch {
	case endpoints.Authorize == "":
		return nil, trace.BadParameter("authorization_endpoint is required")
	case endpoints.Token == "":
		return nil, trace.BadParameter("token_endpoint is required")
	}

	clientID, clientSecret, err := credentials.GetOAuthProxyClient(ctx, ig, s.cache)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       ig.GetOAuthProxyIntegrationSpec().Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:   endpoints.Authorize,
			TokenURL:  endpoints.Token,
			AuthStyle: oauth2.AuthStyleAutoDetect,
		},
	}, nil
}

// redirectURI builds the redirect (callback) URI for an OAuth Proxy integration using the Teleport proxy public address.
func (s *OAuthProxyService) redirectURI(ctx context.Context, integration string) (string, error) {
	publicProxyAddr := s.proxyPublicAddrGetter(ctx)
	if publicProxyAddr == "" {
		return "", trace.BadParameter("proxy public address is required")
	}

	if !strings.HasPrefix(publicProxyAddr, "https://") && !strings.HasPrefix(publicProxyAddr, "http://") {
		publicProxyAddr = "https://" + publicProxyAddr
	}

	if err := credentials.ValidateHTTPSEndpoint(publicProxyAddr); err != nil {
		return "", trace.Wrap(err)
	}

	return fmt.Sprintf("%s/web/integrations/oauthproxy/%s/callback", publicProxyAddr, url.PathEscape(integration)), nil
}

func (s *OAuthProxyService) storeUserCredentials(ctx context.Context, integration, user string, tok *oauth2.Token) error {
	switch {
	case integration == "":
		return trace.BadParameter("integration is required")
	case user == "":
		return trace.BadParameter("user is required")
	case tok == nil:
		return trace.BadParameter("token is required")
	}

	if err := s.requireUserSession(ctx, user); err != nil {
		return trace.Wrap(err)
	}

	authEncrypted, err := userexternalcredentials.MakeAuthEncryptedOAuthCredential(ctx, s.encryptor, user, types.KindIntegration, integration, tok)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(userexternalcredentials.Distribute(ctx, s.userExternalCredentials, user, authEncrypted))
}

func (s *OAuthProxyService) requireUserSession(ctx context.Context, user string) error {
	sessions, _, err := s.userExternalCredentials.ListUserExternalCredentials(ctx, 1, "", usercredpb.EncryptionKeyID_builder{
		User: user,
	}.Build())
	if err != nil {
		return trace.Wrap(err)
	}

	if len(sessions) == 0 {
		return trace.NotFound("no sessions for user %q", user)
	}

	return nil
}

// emitAuthorizeEvent emits the authorize event for an OAuth Proxy authorization against a token provider.
func (s *OAuthProxyService) emitAuthorizeEvent(ctx context.Context, authzCtx *authz.Context, ig types.Integration, grantedScopes []string, authErr error) {
	igMeta, err := getIntegrationMetadata(ig)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to build OAuth Proxy integration metadata for audit event", "integration", ig.GetName(), "error", err)
	}

	code := events.OAuthProxyAuthorizeCode
	status := apievents.Status{Success: true}

	if authErr != nil {
		code = events.OAuthProxyAuthorizeFailureCode
		status = apievents.Status{
			Success: false,
			Error:   authErr.Error(),
		}
	}

	event := &apievents.OAuthProxyAuthorize{
		Metadata: apievents.Metadata{
			Type: events.OAuthProxyAuthorizeEvent,
			Code: code,
		},
		UserMetadata: authzCtx.GetUserMetadata(),
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    ig.GetName(),
			Expires: ig.Expiry(),
		},
		IntegrationMetadata: igMeta,
		ConnectionMetadata:  authz.ConnectionMetadata(ctx),
		Status:              status,
		GrantedScopes:       grantedScopes,
	}

	if err := s.emitter.EmitAuditEvent(ctx, event); err != nil {
		s.logger.WarnContext(ctx, "Failed to emit OAuth Proxy integration authorize audit event", "integration", ig.GetName(), "error", err)
	}
}

// grantedScopes returns the scopes granted by the token, if the scope field is present,
// or the requested scopes if none.
func grantedScopes(requestedScopes []string, tok *oauth2.Token) []string {
	if scopes, ok := tok.Extra("scope").(string); ok && strings.TrimSpace(scopes) != "" {
		return strings.Fields(scopes)
	}

	return requestedScopes
}

func validateCompleteOAuthProxyAuthorizationRequest(req *integrationpb.CompleteOAuthProxyAuthorizationRequest) error {
	switch {
	case req.GetName() == "":
		return trace.BadParameter("name is required")
	case req.GetAuthorizationCode() == "":
		return trace.BadParameter("authorization_code is required")
	case req.GetState() == "":
		return trace.BadParameter("state is required")
	}

	return nil
}

// oauthProxyHostID returns an app server host ID for an OAuth Proxy integration.
func oauthProxyHostID(integration string) string {
	return uuid.NewSHA1(oauthProxyHostIDNamespace, []byte(integration)).String()
}
