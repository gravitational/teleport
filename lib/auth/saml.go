/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
// implemeneted in auth.Server and provide no connector-specific logic.
type SAMLService interface {
	// CreateSAMLAuthRequest creates SAML AuthnRequest
	CreateSAMLAuthRequest(ctx context.Context, req types.SAMLAuthRequest) (*types.SAMLAuthRequest, error)
	// ValidateSAMLResponse validates SAML auth response
	ValidateSAMLResponse(ctx context.Context, samlResponse, connectorID, clientIP string) (*authclient.SAMLAuthResponse, error)
}

// UpsertSAMLConnector creates or updates a SAML connector.
func (a *Server) UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) error {
	// Validate the SAML connector here, because even though Services.UpsertSAMLConnector
	// also validates, it does not have a RoleGetter to use to validate the roles, so
	// has to pass `nil` for the second argument.
	if err := services.ValidateSAMLConnector(connector, a); err != nil {
		return trace.Wrap(err)
	}

	// If someone is applying a SAML Connector obtained with `tctl get` without secrets, the signing key pair is
	// not empty (cert is set) but the private key is missing. Such a SAML resource is invalid and not usable.
	if connector.GetSigningKeyPair().PrivateKey == "" {
		err := services.FillSAMLSigningKeyFromExisting(ctx, connector, a.Services)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if err := a.Services.UpsertSAMLConnector(ctx, connector); err != nil {
		return trace.Wrap(err)
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

	return nil
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
