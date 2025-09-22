/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/gravitational/trace"
	"golang.org/x/oauth2"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type OIDCService interface {
	CreateOIDCAuthRequest(ctx context.Context, req types.OIDCAuthRequest) (*types.OIDCAuthRequest, error)
	CreateOIDCAuthRequestForMFA(ctx context.Context, req types.OIDCAuthRequest) (*types.OIDCAuthRequest, error)
	ValidateOIDCAuthCallback(ctx context.Context, q url.Values) (*authclient.OIDCAuthResponse, error)
}

var errOIDCNotImplemented = &trace.AccessDeniedError{Message: "OIDC is only available in enterprise subscriptions"}

// UpsertOIDCConnector creates or updates an OIDC connector.
func (a *Server) UpsertOIDCConnector(ctx context.Context, connector types.OIDCConnector) (types.OIDCConnector, error) {
	upserted, err := a.Services.UpsertOIDCConnector(ctx, connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.OIDCConnectorCreate{
		Metadata: apievents.Metadata{
			Type: events.OIDCConnectorCreatedEvent,
			Code: events.OIDCConnectorCreatedCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: connector.GetName(),
		},
	}); err != nil {
		a.logger.WarnContext(ctx, "Failed to emit OIDC connector create event", "error", err)
	}

	return upserted, nil
}

// UpdateOIDCConnector updates an existing OIDC connector.
func (a *Server) UpdateOIDCConnector(ctx context.Context, connector types.OIDCConnector) (types.OIDCConnector, error) {
	updated, err := a.Services.UpdateOIDCConnector(ctx, connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.OIDCConnectorUpdate{
		Metadata: apievents.Metadata{
			Type: events.OIDCConnectorUpdatedEvent,
			Code: events.OIDCConnectorUpdatedCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: connector.GetName(),
		},
	}); err != nil {
		a.logger.WarnContext(ctx, "Failed to emit OIDC connector update event", "error", err)
	}

	return updated, nil
}

// CreateOIDCConnector creates a new OIDC connector.
func (a *Server) CreateOIDCConnector(ctx context.Context, connector types.OIDCConnector) (types.OIDCConnector, error) {
	created, err := a.Services.CreateOIDCConnector(ctx, connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.OIDCConnectorCreate{
		Metadata: apievents.Metadata{
			Type: events.OIDCConnectorCreatedEvent,
			Code: events.OIDCConnectorCreatedCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: connector.GetName(),
		},
	}); err != nil {
		a.logger.WarnContext(ctx, "Failed to emit OIDC connector create event", "error", err)
	}

	return created, nil
}

// DeleteOIDCConnector deletes an OIDC connector by name.
func (a *Server) DeleteOIDCConnector(ctx context.Context, connectorName string) error {
	if err := a.Services.DeleteOIDCConnector(ctx, connectorName); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.OIDCConnectorDelete{
		Metadata: apievents.Metadata{
			Type: events.OIDCConnectorDeletedEvent,
			Code: events.OIDCConnectorDeletedCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: connectorName,
		},
	}); err != nil {
		a.logger.WarnContext(ctx, "Failed to emit OIDC connector delete event", "error", err)
	}
	return nil
}

func (a *Server) getOIDCConnector(ctx context.Context, req types.OIDCAuthRequest) (types.OIDCConnector, error) {
	connector, err := a.GetOIDCConnector(ctx, req.ConnectorID, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return connector, nil
}

const oidcAuthPath string = "protocol/openid-connect/auth"
const oidcTokenPath string = "protocol/openid-connect/token"

func newOIDCOAuth2Config(connector types.OIDCConnector) oauth2.Config {
	return oauth2.Config{
		ClientID:     connector.GetClientID(),
		ClientSecret: connector.GetClientSecret(),
		RedirectURL:  connector.GetRedirectURLs()[0],
		Scopes:       OIDCScopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s/%s", connector.GetIssuerURL(), oidcAuthPath),
			TokenURL: fmt.Sprintf("%s/%s", connector.GetIssuerURL(), oidcTokenPath),
		},
	}
}

// OIDCScopes is a list of scopes requested during OAuth2 flow
var OIDCScopes = []string{
	"openid phone offline_access email profile roles address",
}

// CreateOIDCAuthRequest delegates the method call to the oidcAuthService if present,
// or returns a NotImplemented error if not present.
func (a *Server) CreateOIDCAuthRequest(ctx context.Context, req types.OIDCAuthRequest) (*types.OIDCAuthRequest, error) {
	connector, err := a.getOIDCConnector(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !req.CreateWebSession {
		if err := ValidateClientRedirect(req.ClientRedirectURL, req.SSOTestFlow, connector.GetClientRedirectSettings()); err != nil {
			return nil, trace.Wrap(err, InvalidClientRedirectErrorMessage)
		}
	}

	req.StateToken, err = utils.CryptoRandomHex(defaults.TokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := newOIDCOAuth2Config(connector)
	req.RedirectURL = config.AuthCodeURL(req.StateToken)

	err = a.Services.CreateOIDCAuthRequest(ctx, req, defaults.GithubAuthRequestTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

// CreateOIDCAuthRequestForMFA delegates the method call to the oidcAuthService if present,
// or returns a NotImplemented error if not present.
func (a *Server) CreateOIDCAuthRequestForMFA(ctx context.Context, req types.OIDCAuthRequest) (*types.OIDCAuthRequest, error) {
	if a.oidcAuthService == nil {
		return nil, errOIDCNotImplemented
	}

	rq, err := a.oidcAuthService.CreateOIDCAuthRequestForMFA(ctx, req)
	return rq, trace.Wrap(err)
}

// ValidateOIDCAuthCallback validates OIDC auth callback redirect
func (a *Server) ValidateOIDCAuthCallback(ctx context.Context, q url.Values) (*authclient.OIDCAuthResponse, error) {
	diagCtx := NewSSODiagContext(types.KindOIDC, a)
	return validateOIDCAuthCallbackHelper(ctx, a, diagCtx, q, a.emitter, a.logger)
}

type oidcManager interface {
	validateOIDCAuthCallback(ctx context.Context, diagCtx *SSODiagContext, q url.Values) (*authclient.OIDCAuthResponse, error)
}

func validateOIDCAuthCallbackHelper(ctx context.Context, m oidcManager, diagCtx *SSODiagContext, q url.Values, emitter apievents.Emitter, logger *slog.Logger) (*authclient.OIDCAuthResponse, error) {
	event := &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
		},
		Method:             events.LoginMethodOIDC,
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
	}

	auth, err := m.validateOIDCAuthCallback(ctx, diagCtx, q)
	diagCtx.Info.Error = trace.UserMessage(err)
	diagCtx.WriteToBackend(ctx)

	if err != nil {
		event.Code = events.UserSSOLoginFailureCode
		event.Status.Success = false
		event.Status.Error = trace.Unwrap(err).Error()
		event.Status.UserMessage = err.Error()

		if err := emitter.EmitAuditEvent(ctx, event); err != nil {
			logger.WarnContext(ctx, "Failed to emit OIDC login failed event", "error", err)
		}
		return nil, trace.Wrap(err)
	}

	event.Code = events.UserSSOLoginCode
	event.Status.Success = true
	event.User = auth.Username

	if err := emitter.EmitAuditEvent(ctx, event); err != nil {
		logger.WarnContext(ctx, "Failed to emit OIDC login event", "error", err)
	}

	return auth, nil
}

// validateOIDCAuthCallback validates OIDC auth callback redirect
func (a *Server) validateOIDCAuthCallback(ctx context.Context, diagCtx *SSODiagContext, q url.Values) (*authclient.OIDCAuthResponse, error) {
	code := q.Get("code")
	if code == "" {
		return nil, trace.BadParameter("code parameter is required")
	}

	stateToken := q.Get("state")
	if stateToken == "" {
		return nil, trace.BadParameter("state parameter is required")
	}
	diagCtx.RequestID = stateToken

	req, err := a.Services.GetOIDCAuthRequest(ctx, stateToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connector, err := a.getOIDCConnector(ctx, *req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := newOIDCOAuth2Config(connector)
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logger.DebugContext(ctx, "OIDC token received", "token", token)

	// Create user
	username := "oidc-user"
	user, err := a.createOIDCUser(ctx, username, connector.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userState, err := a.GetUserOrLoginState(ctx, user.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Generate CSRF token if empty
	csrfToken := req.CSRFToken
	if csrfToken == "" {
		csrfToken, err = utils.CryptoRandomHex(defaults.TokenLenBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	auth := &authclient.OIDCAuthResponse{
		Username: username,
		Req: authclient.OIDCAuthRequest{
			ConnectorID:       connector.GetName(),
			CreateWebSession:  req.CreateWebSession,
			ClientRedirectURL: req.ClientRedirectURL,
			CSRFToken:         csrfToken,
		},
	}

	// Calculate session TTL from roles (like GitHub does)
	roles, err := services.FetchRoles(userState.GetRoles(), a, userState.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleTTL := roles.AdjustSessionTTL(apidefaults.MaxCertDuration)
	sessionTTL := utils.MinTTL(roleTTL, req.CertTTL)

	// If the request is coming from a browser, create a web session.
	if req.CreateWebSession {
		session, err := a.CreateWebSessionFromReq(ctx, NewWebSessionRequest{
			User:                 userState.GetName(),
			Roles:                userState.GetRoles(),
			Traits:               userState.GetTraits(),
			SessionTTL:           sessionTTL,
			LoginTime:            a.clock.Now().UTC(),
			LoginIP:              req.ClientLoginIP,
			LoginUserAgent:       req.ClientUserAgent,
			AttestWebSession:     true,
			CreateDeviceWebToken: true,
		})
		if err != nil {
			return nil, trace.Wrap(err, "Failed to create web session.")
		}

		auth.Session = session
	}

	return auth, nil
}
func (a *Server) createOIDCUser(ctx context.Context, username, connectorName string) (types.User, error) {
	expires := a.GetClock().Now().UTC().Add(defaults.ActiveSessionTTL)

	user := &types.UserV2{
		Kind:    types.KindUser,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      username,
			Namespace: apidefaults.Namespace,
			Expires:   &expires,
		},
		Spec: types.UserSpecV2{
			Roles: []string{"access"},
			Traits: map[string][]string{
				constants.TraitLogins: {"vscode", "root"},
			},
		},
	}

	existingUser, err := a.Services.GetUser(ctx, username, false)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if existingUser != nil {
		user.SetRevision(existingUser.GetRevision())
		if _, err := a.UpdateUser(ctx, user); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if _, err := a.CreateUser(ctx, user); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return user, nil
}
