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
	"bytes"
	"compress/flate"
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/beevik/etree"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	saml2 "github.com/russellhaering/gosaml2"
)

// UpsertSAMLConnector creates or updates a SAML connector.
func (a *Server) UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) error {
	if err := a.Identity.UpsertSAMLConnector(ctx, connector); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.OIDCConnectorCreate{
		Metadata: apievents.Metadata{
			Type: events.SAMLConnectorCreatedEvent,
			Code: events.SAMLConnectorCreatedCode,
		},
		UserMetadata: ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: connector.GetName(),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit SAML connector create event.")
	}

	return nil
}

// DeleteSAMLConnector deletes a SAML connector by name.
func (a *Server) DeleteSAMLConnector(ctx context.Context, connectorName string) error {
	if err := a.Identity.DeleteSAMLConnector(ctx, connectorName); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.OIDCConnectorDelete{
		Metadata: apievents.Metadata{
			Type: events.SAMLConnectorDeletedEvent,
			Code: events.SAMLConnectorDeletedCode,
		},
		UserMetadata: ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: connectorName,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit SAML connector delete event.")
	}

	return nil
}

func (a *Server) CreateSAMLAuthRequest(req services.SAMLAuthRequest) (*services.SAMLAuthRequest, error) {
	ctx := context.TODO()
	connector, provider, err := a.getConnectorAndProvider(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	doc, err := provider.BuildAuthRequestDocument()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	attr := doc.Root().SelectAttr("ID")
	if attr == nil || attr.Value == "" {
		return nil, trace.BadParameter("missing auth request ID")
	}

	req.ID = attr.Value

	// Workaround for Ping: Ping expects `SigAlg` and `Signature` query
	// parameters when "Enforce Signed Authn Request" is enabled, but gosaml2
	// only provides these parameters when binding == BindingHttpRedirect.
	// Luckily, BuildAuthURLRedirect sets this and is otherwise identical to
	// the standard BuildAuthURLFromDocument.
	if connector.GetProvider() == teleport.Ping {
		req.RedirectURL, err = provider.BuildAuthURLRedirect("", doc)
	} else {
		req.RedirectURL, err = provider.BuildAuthURLFromDocument("", doc)
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = a.Identity.CreateSAMLAuthRequest(req, defaults.SAMLAuthRequestTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

func (a *Server) getConnectorAndProvider(ctx context.Context, req services.SAMLAuthRequest) (types.SAMLConnector, *saml2.SAMLServiceProvider, error) {
	if req.SSOTestFlow {
		// stateless test flow
		connector, err := types.NewSAMLConnector(req.ConnectorID, *req.ConnectorSpec)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		// validate, set defaults for connector
		err = services.ValidateSAMLConnector(connector)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		// we don't want to cache the provider. construct it directly instead of using a.getSAMLProvider()
		provider, err := services.GetSAMLServiceProvider(connector, a.clock)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return connector, provider, nil
	}

	// regular execution flow
	connector, err := a.Identity.GetSAMLConnector(ctx, req.ConnectorID, true)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	provider, err := a.getSAMLProvider(connector)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return connector, provider, nil
}

func (a *Server) getSAMLProvider(conn types.SAMLConnector) (*saml2.SAMLServiceProvider, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	providerPack, ok := a.samlProviders[conn.GetName()]
	if ok && cmp.Equal(providerPack.connector, conn) {
		return providerPack.provider, nil
	}
	delete(a.samlProviders, conn.GetName())

	serviceProvider, err := services.GetSAMLServiceProvider(conn, a.clock)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a.samlProviders[conn.GetName()] = &samlProvider{connector: conn, provider: serviceProvider}

	return serviceProvider, nil
}

func (a *Server) createSSODiagInfo(ctx context.Context, authKind string, id string, infoType types.SSOInfoType, value interface{}) {
	entry, err := types.NewSSODiagnosticInfo(infoType, value)
	if err != nil {
		log.WithError(err).Warn("Failed to serialize SSO diag info.")
	}

	err = a.Identity.CreateSSODiagnosticInfo(ctx, authKind, id, *entry)
	if err != nil {
		log.WithError(err).Warn("Failed to create SSO diag info.")
	}
}

func (a *Server) calculateSAMLUser(ctx context.Context, connector types.SAMLConnector, assertionInfo saml2.AssertionInfo, request *services.SAMLAuthRequest) (*createUserParams, error) {
	var err error

	p := createUserParams{
		connectorName: connector.GetName(),
		username:      assertionInfo.NameID,
	}

	p.traits = services.SAMLAssertionsToTraits(assertionInfo)

	diagInfo := func(infoType types.SSOInfoType, value interface{}) {
		a.createSSODiagInfo(ctx, types.KindSAML, request.ID, infoType, value)
	}

	diagInfo(types.SSOInfoType_SAML_TRAITS_FROM_ASSERTIONS, p.traits)
	diagInfo(types.SSOInfoType_SAML_CONNECTOR_TRAIT_MAPPING, connector.GetTraitMappings())

	var warnings []string
	warnings, p.roles = services.TraitsToRoles(connector.GetTraitMappings(), p.traits)
	if len(p.roles) == 0 {
		type warn struct {
			Message  string   `json:"message"`
			Warnings []string `json:"warnings,omitempty"`
		}

		if len(warnings) != 0 {
			diagInfo(types.SSOInfoType_SAML_ATTRIBUTES_TO_ROLES_WARNINGS, warn{
				Message:  "No roles mapped for the user",
				Warnings: warnings})
			log.WithField("connector", connector).Warnf("Unable to map attibutes to roles: %q", warnings)
		} else {
			diagInfo(types.SSOInfoType_SAML_ATTRIBUTES_TO_ROLES_WARNINGS, warn{Message: "No roles mapped for the user. The mappings may contain typos."})
		}
		return nil, trace.AccessDenied("unable to map attributes to role for connector: %v", connector.GetName())
	}

	// Pick smaller for role: session TTL from role or requested TTL.
	roles, err := services.FetchRoles(p.roles, a.Access, p.traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleTTL := roles.AdjustSessionTTL(apidefaults.MaxCertDuration)
	p.sessionTTL = utils.MinTTL(roleTTL, request.CertTTL)

	return &p, nil
}

func (a *Server) createSAMLUser(p *createUserParams, dryRun bool) (types.User, error) {
	expires := a.GetClock().Now().UTC().Add(p.sessionTTL)

	log.Debugf("Generating dynamic SAML identity %v/%v with roles: %v.", p.connectorName, p.username, p.roles)

	user := &types.UserV2{
		Kind:    types.KindUser,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      p.username,
			Namespace: apidefaults.Namespace,
			Expires:   &expires,
		},
		Spec: types.UserSpecV2{
			Roles:  p.roles,
			Traits: p.traits,
			SAMLIdentities: []types.ExternalIdentity{
				{
					ConnectorID: p.connectorName,
					Username:    p.username,
				},
			},
			CreatedBy: types.CreatedBy{
				User: types.UserRef{
					Name: teleport.UserSystem,
				},
				Time: a.clock.Now().UTC(),
				Connector: &types.ConnectorRef{
					Type:     constants.SAML,
					ID:       p.connectorName,
					Identity: p.username,
				},
			},
		},
	}

	if dryRun {
		return user, nil
	}

	// Get the user to check if it already exists or not.
	existingUser, err := a.Identity.GetUser(p.username, false)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	ctx := context.TODO()

	// Overwrite exisiting user if it was created from an external identity provider.
	if existingUser != nil {
		connectorRef := existingUser.GetCreatedBy().Connector

		// If the exisiting user is a local user, fail and advise how to fix the problem.
		if connectorRef == nil {
			return nil, trace.AlreadyExists("local user with name %q already exists. Either change "+
				"NameID in assertion or remove local user and try again.", existingUser.GetName())
		}

		log.Debugf("Overwriting existing user %q created with %v connector %v.",
			existingUser.GetName(), connectorRef.Type, connectorRef.ID)

		if err := a.UpdateUser(ctx, user); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if err := a.CreateUser(ctx, user); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return user, nil
}

func ParseSAMLInResponseTo(response string) (string, error) {
	raw, _ := base64.StdEncoding.DecodeString(response)

	doc := etree.NewDocument()
	err := doc.ReadFromBytes(raw)
	if err != nil {
		// Attempt to inflate the response in case it happens to be compressed (as with one case at saml.oktadev.com)
		buf, err := io.ReadAll(flate.NewReader(bytes.NewReader(raw)))
		if err != nil {
			return "", trace.Wrap(err)
		}

		doc = etree.NewDocument()
		err = doc.ReadFromBytes(buf)
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	if doc.Root() == nil {
		return "", trace.BadParameter("unable to parse response")
	}

	// teleport only supports sending party initiated flows (Teleport sends an
	// AuthnRequest to the IdP and gets a SAMLResponse from the IdP). identity
	// provider initiated flows (where Teleport gets an unsolicited SAMLResponse
	// from the IdP) are not supported.
	el := doc.Root()
	responseTo := el.SelectAttr("InResponseTo")
	if responseTo == nil {
		message := "teleport does not support initiating login from a SAML identity provider, login must be initiated from either the Teleport Web UI or CLI"
		log.Infof(message)
		return "", trace.NotImplemented(message)
	}
	if responseTo.Value == "" {
		return "", trace.BadParameter("InResponseTo can not be empty")
	}
	return responseTo.Value, nil
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
	Req services.SAMLAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []types.CertAuthority `json:"host_signers"`
}

// ValidateSAMLResponse consumes attribute statements from SAML identity provider
func (a *Server) ValidateSAMLResponse(ctx context.Context, samlResponse string) (*SAMLAuthResponse, error) {
	event := &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
		},
		Method: events.LoginMethodSAML,
	}

	var auxInfo validateSAMLAuxInfo

	auth, err := a.validateSAMLResponse(ctx, samlResponse, &auxInfo)

	if auxInfo.attributeStatements != nil {
		attributes, err := apievents.EncodeMapStrings(auxInfo.attributeStatements)
		if err != nil {
			event.Status.UserMessage = fmt.Sprintf("Failed to encode identity attributes: %v", err.Error())
			log.WithError(err).Debug("Failed to encode identity attributes.")
		} else {
			event.IdentityAttributes = attributes
		}
	}

	if err != nil {
		event.Code = events.UserSSOLoginFailureCode
		if auxInfo.ssoTestFlow {
			event.Code = events.UserSSOTestFlowLoginFailureCode
		}
		event.Status.Success = false
		event.Status.Error = trace.Unwrap(err).Error()
		event.Status.UserMessage = err.Error()
		if err := a.emitter.EmitAuditEvent(a.closeCtx, event); err != nil {
			log.WithError(err).Warn("Failed to emit SAML login failed event.")
		}
		return nil, trace.Wrap(err)
	}

	event.Status.Success = true
	event.User = auth.Username
	event.Code = events.UserSSOLoginCode
	if auxInfo.ssoTestFlow {
		event.Code = events.UserSSOTestFlowLoginCode
	}

	if err := a.emitter.EmitAuditEvent(a.closeCtx, event); err != nil {
		log.WithError(err).Warn("Failed to emit SAML login event.")
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}
	return auth, nil
}

type validateSAMLAuxInfo struct {
	attributeStatements map[string][]string
	ssoTestFlow         bool
}

func (a *Server) validateSAMLResponse(ctx context.Context, samlResponse string, auxInfo *validateSAMLAuxInfo) (*SAMLAuthResponse, error) {
	requestID, err := ParseSAMLInResponseTo(samlResponse)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	diagInfo := func(infoType types.SSOInfoType, value interface{}) {
		a.createSSODiagInfo(ctx, types.KindSAML, requestID, infoType, value)
	}

	traceErr := func(msg string, errDetails error) {
		type errInfo struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}

		diagInfo(types.SSOInfoType_ERROR, errInfo{Message: msg, Error: errDetails.Error()})
	}

	request, err := a.Identity.GetSAMLAuthRequest(ctx, requestID)
	if err != nil {
		traceErr("Failed to get SAML Auth Request", err)
		return nil, trace.Wrap(err)
	}

	auxInfo.ssoTestFlow = request.SSOTestFlow

	connector, provider, err := a.getConnectorAndProvider(ctx, *request)
	if err != nil {
		traceErr("Failed to get SAML connector and provider", err)
		return nil, trace.Wrap(err)
	}

	assertionInfo, err := provider.RetrieveAssertionInfo(samlResponse)
	if err != nil {
		err = trace.AccessDenied(
			"received response with incorrect or missing attribute statements, please check the identity provider configuration to make sure that mappings for claims/attribute statements are set up correctly. <See: https://goteleport.com/teleport/docs/enterprise/sso/ssh-sso/>, failed to retrieve SAML assertion info from response: %v.", err)
		traceErr("Failed to retrieve assertion info. This may indicate IdP configuration error.", err)
		return nil, trace.Wrap(err)
	}

	diagInfo(types.SSOInfoType_SAML_ASSERTION_INFO, assertionInfo)

	if assertionInfo.WarningInfo.InvalidTime {
		err = trace.AccessDenied("invalid time in SAML assertion info")
		traceErr("SAML assertion info contained warning: invalid time.", err)
		return nil, trace.Wrap(err)
	}

	if assertionInfo.WarningInfo.NotInAudience {
		err = trace.AccessDenied("no audience in SAML assertion info")
		traceErr("SAML: not in expected audience. Check auth connector audience field and IdP configuration for typos and other errors.", err)
		return nil, trace.Wrap(err)
	}

	log.Debugf("Obtained SAML assertions for %q.", assertionInfo.NameID)

	attributeStatements := map[string][]string{}

	for key, val := range assertionInfo.Values {
		var vals []string
		for _, vv := range val.Values {
			vals = append(vals, vv.Value)
		}
		log.Debugf("SAML assertion: %q: %q.", key, vals)
		attributeStatements[key] = vals
	}

	auxInfo.attributeStatements = attributeStatements

	diagInfo(types.SSOInfoType_SAML_ATTRIBUTE_STATEMENTS, attributeStatements)

	log.Debugf("SAML assertion warnings: %+v.", assertionInfo.WarningInfo)

	diagInfo(types.SSOInfoType_SAML_ATTRIBUTES_TO_ROLES, connector.GetAttributesToRoles())

	if len(connector.GetAttributesToRoles()) == 0 {
		err = trace.BadParameter("no attributes to roles mapping, check connector documentation")
		traceErr("Attributes-to-roles mapping is empty, SSO user will never have any roles.", err)
		return nil, trace.Wrap(err)
	}

	log.Debugf("Applying %v SAML attribute to roles mappings.", len(connector.GetAttributesToRoles()))

	// Calculate (figure out name, roles, traits, session TTL) of user and
	// create the user in the backend.
	params, err := a.calculateSAMLUser(ctx, connector, *assertionInfo, request)
	if err != nil {
		traceErr("Failed to calculate user attributes.", err)
		return nil, trace.Wrap(err)
	}

	diagInfo(types.SSOInfoType_CREATE_USER_PARAMS, params)

	user, err := a.createSAMLUser(params, request.SSOTestFlow)
	if err != nil {
		traceErr("Failed to create user from provided parameters.", err)
		return nil, trace.Wrap(err)
	}

	// Auth was successful, return session, certificate, etc. to caller.
	auth := &SAMLAuthResponse{
		Req: *request,
		Identity: types.ExternalIdentity{
			ConnectorID: params.connectorName,
			Username:    params.username,
		},
		Username: user.GetName(),
	}

	// In test flow skip signing and creating web sessions.
	if request.SSOTestFlow {
		diagInfo(types.SSOInfoType_SUCCESS, "test flow")
		return auth, nil
	}

	// If the request is coming from a browser, create a web session.
	if request.CreateWebSession {
		session, err := a.createWebSession(ctx, types.NewWebSessionRequest{
			User:       user.GetName(),
			Roles:      user.GetRoles(),
			Traits:     user.GetTraits(),
			SessionTTL: params.sessionTTL,
			LoginTime:  a.clock.Now().UTC(),
		})

		if err != nil {
			traceErr("Failed to create web session.", err)
			return nil, trace.Wrap(err)
		}

		auth.Session = session
	}

	// If a public key was provided, sign it and return a certificate.
	if len(request.PublicKey) != 0 {
		sshCert, tlsCert, err := a.createSessionCert(user, params.sessionTTL, request.PublicKey, request.Compatibility, request.RouteToCluster, request.KubernetesCluster)
		if err != nil {
			traceErr("Failed to create session certificate.", err)
			return nil, trace.Wrap(err)
		}
		clusterName, err := a.GetClusterName()
		if err != nil {
			traceErr("Failed to obtain cluster name.", err)
			return nil, trace.Wrap(err)
		}
		auth.Cert = sshCert
		auth.TLSCert = tlsCert

		// Return the host CA for this cluster only.
		authority, err := a.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: clusterName.GetClusterName(),
		}, false)
		if err != nil {
			traceErr("Failed to obtain cluster's host CA.", err)
			return nil, trace.Wrap(err)
		}
		auth.HostSigners = append(auth.HostSigners, authority)
	}

	diagInfo(types.SSOInfoType_SUCCESS, "full flow")
	return auth, nil
}
