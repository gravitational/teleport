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
	"github.com/zitadel/oidc/v3/pkg/client"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/integration/credentials"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	liboidc "github.com/gravitational/teleport/lib/oidc"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// oauthProxyAuthRequestTTL is the duration to store an in-flight request.
	// This must be long enough for the authorizer to complete authorization and consent.
	oauthProxyAuthRequestTTL = 5 * time.Minute
	// discoverTimeout is the timeout for an OIDC discovery request.
	// The request times out, rather than retries, because some of the endpoints to be discovered are optional,
	// and we don't want to make the user wait indefinitely if they can't be discovered.
	discoverTimeout = 15 * time.Second
	// exchangeTimeout is the timeout for an OAuth exchange request.
	// The request times out, rather than retries, because authorization code is single use and requests are not idempotent.
	exchangeTimeout = 15 * time.Second
)

// oauthProxyHostIDNamespace is a UUID that represents the name space to be used for generating UUIDs for OAuth Proxy integration app servers.
var oauthProxyHostIDNamespace = uuid.NewSHA1(uuid.Nil, []byte("oauth-proxy"))

// OAuthProxyUserCredentialsStore is used to store the credential from the OAuth Authorization grant exchange.
// TODO(nixpig): Implement this interface for the double-encrypted user credentials store.
// See: RFD 0340 https://github.com/gravitational/rfd/blob/727e7f3ee5f92b630f64ca8cbc01cd40e90ea603/rfd/0340-user-encrypted-secret-storage.md
type OAuthProxyUserCredentialsStore interface {
	// StoreOAuthProxyCredential persists the credential to the backend.
	StoreOAuthProxyCredential(ctx context.Context, integration, user string, tok *oauth2.Token) error
}

// OAuthProxyServiceConfig holds configuration options for the OAuth Proxy Integration gRPC service.
type OAuthProxyServiceConfig struct {
	Authorizer            authz.Authorizer
	Clock                 clockwork.Clock
	Logger                *slog.Logger
	Backend               Backend
	Cache                 Cache
	ProxyPublicAddrGetter func(context.Context) string
	UserCredentialsStore  OAuthProxyUserCredentialsStore
	Client                *http.Client
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
	case cfg.UserCredentialsStore == nil:
		// TODO(nixpig): Require an actual credentials store and fail on omission, once implemented.
		// See: RFD 0340 https://github.com/gravitational/rfd/blob/727e7f3ee5f92b630f64ca8cbc01cd40e90ea603/rfd/0340-user-encrypted-secret-storage.md
		// return trace.BadParameter("credential store is required")
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

	authorizer            authz.Authorizer
	logger                *slog.Logger
	clock                 clockwork.Clock
	backend               Backend
	cache                 Cache
	proxyPublicAddrGetter func(context.Context) string
	userCredentialsStore  OAuthProxyUserCredentialsStore
	client                *http.Client
}

// NewOAuthProxyService returns a new OAuthProxyService.
func NewOAuthProxyService(cfg *OAuthProxyServiceConfig) (*OAuthProxyService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &OAuthProxyService{
		logger:                cfg.Logger,
		authorizer:            cfg.Authorizer,
		clock:                 cfg.Clock,
		backend:               cfg.Backend,
		cache:                 cfg.Cache,
		proxyPublicAddrGetter: cfg.ProxyPublicAddrGetter,
		userCredentialsStore:  cfg.UserCredentialsStore,
		client: &http.Client{
			Transport:     liboidc.NewOIDCRoundTripper(cfg.Client.Transport),
			CheckRedirect: cfg.Client.CheckRedirect,
			Jar:           cfg.Client.Jar,
			Timeout:       cfg.Client.Timeout,
		},
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

	ig, err := s.backend.GetIntegration(ctx, req.GetName())
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

	ig, err := s.backend.GetIntegration(ctx, req.GetName())
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

	authReq, err := s.backend.ConsumeOAuthProxyAuthRequest(ctx, authzCtx.User.GetName(), ig.GetName(), req.GetState())
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to consume request", "integration", ig.GetName(), "error", err)
		return nil, trace.NotFound("valid request not found")
	}

	exchangeCtx := context.WithValue(ctx, oauth2.HTTPClient, s.client)
	exchangeCtx, cancel := context.WithTimeout(exchangeCtx, exchangeTimeout)
	defer cancel()

	tok, err := cfg.Exchange(exchangeCtx, req.GetAuthorizationCode(), oauth2.VerifierOption(authReq.CodeVerifier))
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to complete token exchange", "integration", ig.GetName(), "error", err)
		return nil, trace.Errorf("failed to complete token exchange")
	}

	if tok.AccessToken == "" {
		s.logger.ErrorContext(ctx, "Missing access token on provider response", "integration", ig.GetName())
		return nil, trace.BadParameter("invalid response")
	}

	// TODO(nixpig): Remove nil check once credentials store is implemented.
	// See: RFD 0340 https://github.com/gravitational/rfd/blob/727e7f3ee5f92b630f64ca8cbc01cd40e90ea603/rfd/0340-user-encrypted-secret-storage.md
	if s.userCredentialsStore == nil {
		return nil, trace.NotImplemented("credential storage not implemented")
	}
	if err := s.userCredentialsStore.StoreOAuthProxyCredential(ctx, authReq.Integration, authReq.User, tok); err != nil {
		return nil, trace.Wrap(err)
	}

	return integrationpb.CompleteOAuthProxyAuthorizationResponse_builder{
		Expires: timestamppb.New(tok.Expiry),
	}.Build(), nil
}

// oauthConfig builds the OAuth config for the authorization request.
func (s *OAuthProxyService) oauthConfig(ctx context.Context, ig types.Integration) (*oauth2.Config, error) {
	redirectURL, err := s.redirectURI(ctx, ig.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	spec := ig.GetOAuthProxyIntegrationSpec()
	authorizationEndpoint := spec.AuthorizationEndpoint
	tokenEndpoint := spec.TokenEndpoint

	shouldDiscover := authorizationEndpoint == "" || tokenEndpoint == ""

	if shouldDiscover {
		discoverCtx, cancel := context.WithTimeout(ctx, discoverTimeout)
		defer cancel()

		dc, err := client.Discover(discoverCtx, spec.IssuerUrl, s.client)
		if err != nil {
			s.logger.WarnContext(ctx, "Failed to call discovery endpoint", "issuer_url", spec.IssuerUrl, "error", err)
		} else {
			if authorizationEndpoint == "" && dc.AuthorizationEndpoint != "" {
				if err := validateHTTPSEndpoint(dc.AuthorizationEndpoint); err != nil {
					s.logger.ErrorContext(ctx, "Discovered authorization_endpoint is invalid", "authorization_endpoint", dc.AuthorizationEndpoint, "error", err)
					return nil, trace.BadParameter("discovered authorization_endpoint is invalid")
				}
				authorizationEndpoint = dc.AuthorizationEndpoint
			}
			if tokenEndpoint == "" && dc.TokenEndpoint != "" {
				if err := validateHTTPSEndpoint(dc.TokenEndpoint); err != nil {
					s.logger.ErrorContext(ctx, "Discovered token_endpoint is invalid", "token_endpoint", dc.TokenEndpoint, "error", err)
					return nil, trace.BadParameter("discovered token_endpoint is invalid")
				}
				tokenEndpoint = dc.TokenEndpoint
			}
		}
	}

	switch {
	case authorizationEndpoint == "":
		return nil, trace.BadParameter("authorization_endpoint is required")
	case tokenEndpoint == "":
		return nil, trace.BadParameter("token_endpoint is required")
	}

	clientID, clientSecret, err := s.oauthProxyClientSecret(ctx, ig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       spec.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:   authorizationEndpoint,
			TokenURL:  tokenEndpoint,
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

	if err := validateHTTPSEndpoint(publicProxyAddr); err != nil {
		return "", trace.Wrap(err)
	}

	return fmt.Sprintf("%s/web/integrations/oauthproxy/%s/callback", publicProxyAddr, url.PathEscape(integration)), nil
}

// oauthProxyClientSecret returns the client ID and client secret for an OAuth Proxy integration from its static credentials ref.
func (s *OAuthProxyService) oauthProxyClientSecret(ctx context.Context, ig types.Integration) (string, string, error) {
	creds := ig.GetCredentials()
	if creds == nil || creds.GetStaticCredentialsRef() == nil {
		return "", "", trace.NotFound("integration %q has no credentials", ig.GetName())
	}

	cred, err := credentials.GetByPurpose(ctx, creds.GetStaticCredentialsRef(), credentials.PurposeOAuthProxyClient, s.cache)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	clientID, clientSecret := cred.GetOAuthClientSecret()

	return clientID, clientSecret, nil
}

// validateHTTPSEndpoint ensures the given endpoint is a valid HTTPS URL.
func validateHTTPSEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	switch {
	case err != nil:
		return trace.Wrap(err)
	case u.Scheme != "https" || u.Host == "":
		return trace.Errorf("not a valid HTTPS URL")
	}

	return nil
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
