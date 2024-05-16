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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
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
var ErrSAMLRequiresEnterprise = trace.AccessDenied("SAML is only available in Teleport Enterprise")

// SAMLService are the methods that the auth server delegates to a plugin for
// implementing the SAML connector. These are the core functions of SAML
// authentication - the connector CRUD operations and Get methods are
// implemented in auth.Server and provide no connector-specific logic.
type SAMLService interface {
	// CreateSAMLAuthRequest creates SAML AuthnRequest
	CreateSAMLAuthRequest(ctx context.Context, req types.SAMLAuthRequest) (*types.SAMLAuthRequest, error)
	// ValidateSAMLResponse validates SAML auth response
	ValidateSAMLResponse(ctx context.Context, samlResponse, connectorID, clientIP string) (*authclient.SAMLAuthResponse, error)
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
func (a *Server) ValidateSAMLResponse(ctx context.Context, samlResponse, connectorID, clientIP string) (*authclient.SAMLAuthResponse, error) {
	if a.samlAuthService == nil {
		return nil, trace.Wrap(ErrSAMLRequiresEnterprise)
	}

	resp, err := a.samlAuthService.ValidateSAMLResponse(ctx, samlResponse, connectorID, clientIP)
	return resp, trace.Wrap(err)
}
