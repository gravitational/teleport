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
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/oauth2"

	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/credentialencryption"
	"github.com/gravitational/teleport/lib/auth/integration/credentials"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/tlsca"
)

// refreshExchangeTimeout is the timeout for the OAuth refresh exchange.
const refreshExchangeTimeout = 30 * time.Second

// oauthProxyHandler handles user external credentials for OAuth Proxy integrations.
type oauthProxyHandler struct {
	cfg *ServiceConfig
}

// CheckRoute checks the user certificate app route matches the integration.
// The app, app server, and integration all share a common name,
// so the RouteToApp.Name must match the integration, and the app served must belong to the integration.
func (h *oauthProxyHandler) CheckRoute(ctx context.Context, identity *tlsca.Identity, resourceName string) error {
	switch {
	case identity == nil:
		return trace.BadParameter("identity is required")
	case resourceName == "":
		return trace.BadParameter("resource name is required")
	}

	ig, err := h.getOAuthProxyIntegration(ctx, resourceName)
	if err != nil {
		return trace.Wrap(err)
	}

	appServer, err := h.getOAuthProxyAppServer(ctx, resourceName)
	if err != nil {
		return trace.Wrap(err)
	}

	if identity.RouteToApp.Name != ig.GetName() {
		return trace.AccessDenied("user external credential does not belong to the app of the user certificate")
	}

	if err := validateApp(appServer.GetApp(), ig); err != nil {
		return trace.AccessDenied("%s", err)
	}

	return nil
}

// CheckOwnership checks the decrypted credential is an integration credential.
// Unlike the Git server handler, it does not verify with the authorization server that the Teleport user's token belongs to the upstream user of the credential,
// because the identity of the user in Teleport may be different to the identity of the user at the upstream provider,
// e.g. user is authenticated to Teleport using Okta as alice@goteleport.com but upstream provider being proxied is Azure where user is alice@goteleport.onmicrosoft.com.
// TODO(nixpig): Optionally resolve and call introspection endpoint with token to verify ownership.
func (h *oauthProxyHandler) CheckOwnership(ctx context.Context, identity *tlsca.Identity, decrypted *credentialencryption.Payload) error {
	switch {
	case identity == nil:
		return trace.BadParameter("identity is required")
	case decrypted == nil:
		return trace.BadParameter("decrypted payload is required")
	case decrypted.ResourceKind != types.KindIntegration:
		return trace.BadParameter("credential is not for integration")
	}

	return nil
}

// CheckAccess checks the user's role grants access to use the OAuth Proxy integration.
func (h *oauthProxyHandler) CheckAccess(ctx context.Context, authCtx *authz.Context, resourceName string) error {
	if authCtx == nil {
		return trace.BadParameter("auth context is required")
	}

	ig, err := h.getOAuthProxyIntegration(ctx, resourceName)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(authCtx.CheckAccessToResource(ig, types.VerbUse))
}

// RefreshOAuthToken exchanges the user's refresh token for a new access token with the authorization server.
func (h *oauthProxyHandler) RefreshOAuthToken(ctx context.Context, username, resourceName string, refreshToken []byte) (*oauth2.Token, error) {
	switch {
	case username == "":
		return nil, trace.BadParameter("username is required")
	case resourceName == "":
		return nil, trace.BadParameter("resource name is required")
	case len(refreshToken) == 0:
		return nil, trace.BadParameter("refresh token is required")
	}

	ig, err := h.getOAuthProxyIntegration(ctx, resourceName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := h.oauthConfig(ctx, ig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(nixpig): Use ssrf-safe client (next PR in stack) before exposing on service.
	exchangeCtx := context.WithValue(ctx, oauth2.HTTPClient, h.cfg.httpClient)
	exchangeCtx, cancel := context.WithTimeout(exchangeCtx, refreshExchangeTimeout)
	defer cancel()

	tok, err := cfg.TokenSource(exchangeCtx, &oauth2.Token{
		RefreshToken: string(refreshToken),
	}).Token()
	h.emitRefreshEvent(ctx, username, ig, err)
	if err != nil {
		return nil, trace.AccessDenied("authorization no longer valid")
	}

	return tok, nil
}

// emitRefreshEvent emits the OAuth token refresh event.
func (h *oauthProxyHandler) emitRefreshEvent(ctx context.Context, username string, ig types.Integration, refreshErr error) {
	igMeta := apievents.IntegrationMetadata{
		SubKind: types.IntegrationSubKindOAuthProxy,
		OAuthProxy: &apievents.OAuthProxyIntegrationMetadata{
			IssuerUrl:   ig.GetOAuthProxyIntegrationSpec().IssuerUrl,
			UpstreamUrl: ig.GetOAuthProxyIntegrationSpec().UpstreamUrl,
			Scopes:      ig.GetOAuthProxyIntegrationSpec().Scopes,
		},
	}

	code := events.OAuthProxyRefreshCode
	status := apievents.Status{Success: true}

	if refreshErr != nil {
		code = events.OAuthProxyRefreshFailureCode
		status = apievents.Status{
			Success: false,
			Error:   refreshErr.Error(),
		}
	}

	event := &apievents.OAuthProxyRefresh{
		Metadata: apievents.Metadata{
			Type: events.OAuthProxyRefreshEvent,
			Code: code,
		},
		UserMetadata: apievents.UserMetadata{User: username},
		ResourceMetadata: apievents.ResourceMetadata{
			Name:    ig.GetName(),
			Expires: ig.Expiry(),
		},
		IntegrationMetadata: igMeta,
		ConnectionMetadata:  authz.ConnectionMetadata(ctx),
		Status:              status,
	}

	if err := h.cfg.Emitter.EmitAuditEvent(ctx, event); err != nil {
		h.cfg.Logger.WarnContext(ctx, "Failed to emit OAuth Proxy integration refresh audit event", "integration", ig.GetName(), "error", err)
	}
}

// getOAuthProxyIntegration gets the integration and validates it's an OAuth Proxy integration.
func (h *oauthProxyHandler) getOAuthProxyIntegration(ctx context.Context, integration string) (types.Integration, error) {
	ig, err := h.cfg.Cache.GetIntegration(ctx, integrationv1.GetIntegrationRequest_builder{
		Name: integration,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !ig.IsOAuthProxy() {
		return nil, trace.BadParameter("integration is not an OAuth Proxy")
	}

	if ig.GetOAuthProxyIntegrationSpec() == nil {
		return nil, trace.BadParameter("integration has no spec")
	}

	return ig, nil
}

// getOAuthProxyAppServer finds the app server for the OAuth Proxy integration.
func (h *oauthProxyHandler) getOAuthProxyAppServer(ctx context.Context, resourceName string) (types.AppServer, error) {
	for as, err := range h.cfg.Cache.RangeApplicationServersWithName(ctx, resourceName) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if as.GetName() == resourceName {
			return as, nil
		}
	}

	return nil, trace.NotFound("no app server found for integration")
}

// oauthConfig builds the OAuth client config for the integration to use in the refresh exchange.
func (h *oauthProxyHandler) oauthConfig(ctx context.Context, ig types.Integration) (*oauth2.Config, error) {
	// TODO(nixpig): Use ssrf-safe client before exposing.
	endpoints, err := credentials.ResolveOAuthProxyEndpoints(ctx, ig, h.cfg.httpClient, credentials.OAuthEndpointKindToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientID, clientSecret, err := credentials.GetOAuthProxyClient(ctx, ig, h.cfg.Cache)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: endpoints.Token,
			AuthURL:  endpoints.Authorize,
		},
	}, nil
}

// validateApp ensures the app belongs to the integration and that the upstream configuration matches.
func validateApp(app types.Application, ig types.Integration) error {
	switch {
	case app == nil:
		return trace.BadParameter("no application")
	case app.Origin() != types.OriginIntegrationOAuthProxy:
		return trace.BadParameter("not an OAuth Proxy application")
	case app.GetIntegration() != ig.GetName():
		return trace.BadParameter("application does not belong to integration")
	case app.GetURI() != ig.GetOAuthProxyIntegrationSpec().UpstreamUrl:
		return trace.BadParameter("upstream on application does not match integration")
	}

	return nil
}
