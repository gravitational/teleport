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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

// ErrSAMLRequiresEnterprise is the error returned by the SAML methods when not
// using the Enterprise edition of Teleport.
//
// TODO(zmb3): ideally we would wrap ErrRequiresEnterprise here, but
// we can't currently propagate wrapped errors across the gRPC boundary,
// and we want tctl to display a clean user-facing message in this case
var ErrSAMLRequiresEnterprise = &trace.AccessDeniedError{Message: "SAML is only available in Teleport Enterprise"}

// SAMLService are the methods that the auth server delegates to a plugin for
// implementing the SAML connector. These are the core functions of SAML
// authentication - the connector CRUD operations and Get methods are
// implemented in auth.Server and provide no connector-specific logic.
type SAMLService interface {
	// CreateSAMLAuthRequest creates SAML AuthnRequest
	CreateSAMLAuthRequest(ctx context.Context, req types.SAMLAuthRequest) (*types.SAMLAuthRequest, error)
	// ValidateSAMLResponse validates SAML auth response
	ValidateSAMLResponse(ctx context.Context, samlResponse, connectorID, clientIP string) (*SAMLAuthResponse, error)
}

// UpsertSAMLConnector creates or updates a SAML connector.
func (a *Server) UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error) {
	// Validate the SAML connector here, because even though Services.UpsertSAMLConnector
	// also validates, it does not have a RoleGetter to use to validate the roles, so
	// has to pass `nil` for the second argument.
	if err := services.ValidateSAMLConnector(connector, a); err != nil {
		return nil, trace.Wrap(err)
	}
	upserted, err := a.Services.UpsertSAMLConnector(ctx, connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.SAMLConnectorCreate{
		Metadata: apievents.Metadata{
			Type: events.SAMLConnectorCreatedEvent,
			Code: events.SAMLConnectorCreatedCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: connector.GetName(),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit SAML connector create event.")
	}

	return upserted, nil
}

// UpdateSAMLConnector updates an existing SAML connector.
func (a *Server) UpdateSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error) {
	// Validate the SAML connector here, because even though Services.UpsertSAMLConnector
	// also validates, it does not have a RoleGetter to use to validate the roles, so
	// has to pass `nil` for the second argument.
	if err := services.ValidateSAMLConnector(connector, a); err != nil {
		return nil, trace.Wrap(err)
	}
	updated, err := a.Services.UpdateSAMLConnector(ctx, connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.SAMLConnectorUpdate{
		Metadata: apievents.Metadata{
			Type: events.SAMLConnectorUpdatedEvent,
			Code: events.SAMLConnectorUpdatedCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: connector.GetName(),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit SAML connector update event.")
	}

	return updated, nil
}

// CreateSAMLConnector creates a new SAML connector.
func (a *Server) CreateSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error) {
	// Validate the SAML connector here, because even though Services.UpsertSAMLConnector
	// also validates, it does not have a RoleGetter to use to validate the roles, so
	// has to pass `nil` for the second argument.
	if err := services.ValidateSAMLConnector(connector, a); err != nil {
		return nil, trace.Wrap(err)
	}
	created, err := a.Services.CreateSAMLConnector(ctx, connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.SAMLConnectorCreate{
		Metadata: apievents.Metadata{
			Type: events.SAMLConnectorCreatedEvent,
			Code: events.SAMLConnectorCreatedCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: connector.GetName(),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit SAML connector create event.")
	}

	return created, nil
}

// DeleteSAMLConnector deletes a SAML connector.
func (a *Server) DeleteSAMLConnector(ctx context.Context, connectorID string) error {
	if err := a.Services.DeleteSAMLConnector(ctx, connectorID); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.SAMLConnectorDelete{
		Metadata: apievents.Metadata{
			Type: events.SAMLConnectorDeletedEvent,
			Code: events.SAMLConnectorDeletedCode,
		},
		UserMetadata: authz.ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: connectorID,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit SAML connector delete event.")
	}

	return nil
}

// CreateSAMLAuthRequest delegates the method call to the samlAuthService if present,
// or returns a NotImplemented error if not present.
func (a *Server) CreateSAMLAuthRequest(ctx context.Context, req types.SAMLAuthRequest) (*types.SAMLAuthRequest, error) {
	if a.samlAuthService == nil {
		return nil, trace.Wrap(ErrSAMLRequiresEnterprise)
	}

	rq, err := a.samlAuthService.CreateSAMLAuthRequest(ctx, req)
	return rq, trace.Wrap(err)
}

// ValidateSAMLResponse delegates the method call to the samlAuthService if present,
// or returns a NotImplemented error if not present.
func (a *Server) ValidateSAMLResponse(ctx context.Context, samlResponse, connectorID, clientIP string) (*SAMLAuthResponse, error) {
	if a.samlAuthService == nil {
		return nil, trace.Wrap(ErrSAMLRequiresEnterprise)
	}

	resp, err := a.samlAuthService.ValidateSAMLResponse(ctx, samlResponse, connectorID, clientIP)
	return resp, trace.Wrap(err)
}

// SAMLAuthResponse is returned when auth server validated callback parameters
// returned from SAML identity provider
type SAMLAuthResponse struct {
	// Username is an authenticated teleport username
	Username string `json:"username"`
	// Identity contains validated SAML identity
	Identity types.ExternalIdentity `json:"identity"`
	// Web session will be generated by auth server if requested in SAMLAuthRequest
	Session types.WebSession `json:"session,omitempty"`
	// Cert will be generated by certificate authority
	Cert []byte `json:"cert,omitempty"`
	// TLSCert is a PEM encoded TLS certificate
	TLSCert []byte `json:"tls_cert,omitempty"`
	// Req is an original SAML auth request
	Req SAMLAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []types.CertAuthority `json:"host_signers"`
}

// SAMLAuthRequest is a SAML auth request that supports standard json marshaling.
type SAMLAuthRequest struct {
	// ID is a unique request ID.
	ID string `json:"id"`
	// PublicKey is an optional public key, users want these
	// keys to be signed by auth servers user CA in case
	// of successful auth.
	PublicKey []byte `json:"public_key"`
	// CSRFToken is associated with user web session token.
	CSRFToken string `json:"csrf_token"`
	// CreateWebSession indicates if user wants to generate a web
	// session after successful authentication.
	CreateWebSession bool `json:"create_web_session"`
	// ClientRedirectURL is a URL client wants to be redirected
	// after successful authentication.
	ClientRedirectURL string `json:"client_redirect_url"`
}

// ValidateSAMLResponseReq is the request made by the proxy to validate
// and activate a login via SAML.
type ValidateSAMLResponseReq struct {
	// Response is SAML statements coming from the identity provider.
	Response string `json:"response"`
	// ConnectorID is ID of a SAML connector that should be used for this request.
	ConnectorID string `json:"connector_id,omitempty"`
	// ClientIP is IP of the logging in client, used in identity provider initiated login case,
	// when we don't have original client's request with their IP stored.
	ClientIP string `json:"client_ip,omitempty"`
}

// SAMLAuthRawResponse is returned when auth server validated callback parameters
// returned from SAML provider
type SAMLAuthRawResponse struct {
	// Username is authenticated teleport username
	Username string `json:"username"`
	// Identity contains validated OIDC identity
	Identity types.ExternalIdentity `json:"identity"`
	// Web session will be generated by auth server if requested in OIDCAuthRequest
	Session json.RawMessage `json:"session,omitempty"`
	// Cert will be generated by certificate authority
	Cert []byte `json:"cert,omitempty"`
	// Req is original oidc auth request
	Req SAMLAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []json.RawMessage `json:"host_signers"`
	// TLSCert is TLS certificate authority certificate
	TLSCert []byte `json:"tls_cert,omitempty"`
}
