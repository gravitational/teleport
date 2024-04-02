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
	"encoding/json"
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
)

type OIDCService interface {
	CreateOIDCAuthRequest(ctx context.Context, req types.OIDCAuthRequest) (*types.OIDCAuthRequest, error)
	ValidateOIDCAuthCallback(ctx context.Context, q url.Values) (*OIDCAuthResponse, error)
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
		log.WithError(err).Warn("Failed to emit OIDC connector create event.")
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
		log.WithError(err).Warn("Failed to emit OIDC connector update event.")
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
		log.WithError(err).Warn("Failed to emit OIDC connector create event.")
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
		log.WithError(err).Warn("Failed to emit OIDC connector delete event.")
	}
	return nil
}

func (a *Server) CreateOIDCAuthRequest(ctx context.Context, req types.OIDCAuthRequest) (*types.OIDCAuthRequest, error) {
	if a.oidcAuthService == nil {
		return nil, errOIDCNotImplemented
	}

	rq, err := a.oidcAuthService.CreateOIDCAuthRequest(ctx, req)
	return rq, trace.Wrap(err)
}

func (a *Server) ValidateOIDCAuthCallback(ctx context.Context, q url.Values) (*OIDCAuthResponse, error) {
	if a.oidcAuthService == nil {
		return nil, errOIDCNotImplemented
	}

	resp, err := a.oidcAuthService.ValidateOIDCAuthCallback(ctx, q)
	return resp, trace.Wrap(err)
}

// OIDCAuthResponse is returned when auth server validated callback parameters
// returned from OIDC provider
type OIDCAuthResponse struct {
	// Username is authenticated teleport username
	Username string `json:"username"`
	// Identity contains validated OIDC identity
	Identity types.ExternalIdentity `json:"identity"`
	// Web session will be generated by auth server if requested in OIDCAuthRequest
	Session types.WebSession `json:"session,omitempty"`
	// Cert will be generated by certificate authority
	Cert []byte `json:"cert,omitempty"`
	// TLSCert is PEM encoded TLS certificate
	TLSCert []byte `json:"tls_cert,omitempty"`
	// Req is original oidc auth request
	Req OIDCAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []types.CertAuthority `json:"host_signers"`
}

// OIDCAuthRequest is an OIDC auth request that supports standard json marshaling.
type OIDCAuthRequest struct {
	// ConnectorID is ID of OIDC connector this request uses
	ConnectorID string `json:"connector_id"`
	// CSRFToken is associated with user web session token
	CSRFToken string `json:"csrf_token"`
	// PublicKey is an optional public key, users want these
	// keys to be signed by auth servers user CA in case
	// of successful auth
	PublicKey []byte `json:"public_key"`
	// CreateWebSession indicates if user wants to generate a web
	// session after successful authentication
	CreateWebSession bool `json:"create_web_session"`
	// ClientRedirectURL is a URL client wants to be redirected
	// after successful authentication
	ClientRedirectURL string `json:"client_redirect_url"`
}

// ValidateOIDCAuthCallbackReq is the request made by the proxy to validate
// and activate a login via OIDC.
type ValidateOIDCAuthCallbackReq struct {
	Query url.Values `json:"query"`
}

// OIDCAuthRawResponse is returned when auth server validated callback parameters
// returned from OIDC provider
type OIDCAuthRawResponse struct {
	// Username is authenticated teleport username
	Username string `json:"username"`
	// Identity contains validated OIDC identity
	Identity types.ExternalIdentity `json:"identity"`
	// Web session will be generated by auth server if requested in OIDCAuthRequest
	Session json.RawMessage `json:"session,omitempty"`
	// Cert will be generated by certificate authority
	Cert []byte `json:"cert,omitempty"`
	// TLSCert is PEM encoded TLS certificate
	TLSCert []byte `json:"tls_cert,omitempty"`
	// Req is original oidc auth request
	Req OIDCAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []json.RawMessage `json:"host_signers"`
}
