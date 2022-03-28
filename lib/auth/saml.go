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

	"github.com/google/go-cmp/cmp"

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
	var connector types.SAMLConnector
	var provider *saml2.SAMLServiceProvider
	var err error

	if req.SSOTestFlow {
		// stateless test flow
		connector, err = types.NewSAMLConnector(req.ConnectorID, *req.ConnectorSpec)
		if err != nil {
			return nil, nil, err
		}

		// validate, set defaults for connector
		err := services.ValidateSAMLConnector(connector)
		if err != nil {
			return nil, nil, err
		}

		// we don't want to cache the provider. construct it directly instead of using a.getSAMLProvider()
		provider, err = services.GetSAMLServiceProvider(connector, a.clock)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	} else {
		// regular execution flow
		connector, err = a.Identity.GetSAMLConnector(ctx, req.ConnectorID, true)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		provider, err = a.getSAMLProvider(connector)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
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

func (a *Server) calculateSAMLUser(connector types.SAMLConnector, assertionInfo saml2.AssertionInfo, request *services.SAMLAuthRequest) (*createUserParams, error) {
	var err error

	p := createUserParams{
		connectorName: connector.GetName(),
		username:      assertionInfo.NameID,
	}

	p.traits = services.SAMLAssertionsToTraits(assertionInfo)

	_ = a.Identity.TraceSAMLDiagnosticInfo(context.Background(), request.ID, DiagInfoSAMLTraitsFromAssertions, p.traits)
	_ = a.Identity.TraceSAMLDiagnosticInfo(context.Background(), request.ID, DiagInfoSAMLConnectorTraitMapping, connector.GetTraitMappings())

	var warnings []string
	warnings, p.roles = services.TraitsToRoles(connector.GetTraitMappings(), p.traits)
	if len(p.roles) == 0 {
		if len(warnings) != 0 {
			_ = a.Identity.TraceSAMLDiagnosticInfo(context.Background(), request.ID, DiagInfoSAMLAttributesToRolesWarnings, "No roles mapped for the user", warnings)
			log.WithField("connector", connector).Warnf("Unable to map attibutes to roles: %q", warnings)
		} else {
			_ = a.Identity.TraceSAMLDiagnosticInfo(context.Background(), request.ID, DiagInfoSAMLAttributesToRolesWarnings, "No roles mapped for the user. The mappings may contain typos.")
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
func (a *Server) ValidateSAMLResponse(samlResponse string) (*SAMLAuthResponse, error) {
	event := &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
		},
		Method: events.LoginMethodSAML,
	}

	re, request, err := a.validateSAMLResponse(samlResponse)

	if re != nil && re.attributeStatements != nil {
		attributes, err := apievents.EncodeMapStrings(re.attributeStatements)
		if err != nil {
			event.Status.UserMessage = fmt.Sprintf("Failed to encode identity attributes: %v", err.Error())
			log.WithError(err).Debug("Failed to encode identity attributes.")
		} else {
			event.IdentityAttributes = attributes
		}
	}

	testFlow := request.SSOTestFlow

	if err != nil {
		if testFlow {
			event.Code = events.UserSSOTestFlowLoginFailureCode
		} else {
			event.Code = events.UserSSOLoginFailureCode
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
	event.User = re.auth.Username
	if testFlow {
		event.Code = events.UserSSOTestFlowLoginCode
	} else {
		event.Code = events.UserSSOLoginCode
	}

	if err := a.emitter.EmitAuditEvent(a.closeCtx, event); err != nil {
		log.WithError(err).Warn("Failed to emit SAML login event.")
	}

	if err != nil {
		return nil, err
	}
	return &re.auth, nil
}

type samlAuthResponse struct {
	auth                SAMLAuthResponse
	attributeStatements map[string][]string
}

const (
	DiagInfoError                         = "common.error"
	DiagInfoResult                        = "common.result"
	DiagInfoCreateUserParams              = "common.createUserParams"
	DiagInfoSAMLAttributesToRoles         = "SAML.attributesToRoles"
	DiagInfoSAMLAttributesToRolesWarnings = "SAML.attributesToRolesWarnings"
	DiagInfoSAMLAssertionInfo             = "SAML.assertionInfo"
	DiagInfoSAMLAttributeStatements       = "SAML.attributeStatements"
	DiagInfoSAMLTraitsFromAssertions      = "SAML.traits"
	DiagInfoSAMLConnectorTraitMapping     = "SAML.connector_traits"
)

func (a *Server) validateSAMLResponse(samlResponse string) (*samlAuthResponse, *services.SAMLAuthRequest, error) {
	ctx := context.TODO()

	requestID, err := ParseSAMLInResponseTo(samlResponse)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	traceErr := func(msg string, errInfo error) {
		_ = a.Identity.TraceSAMLDiagnosticInfo(ctx, requestID, DiagInfoError, msg, errInfo.Error())
	}

	request, err := a.Identity.GetSAMLAuthRequest(requestID)
	if err != nil {
		traceErr("Failed to get SAML Auth Request", err)
		return nil, request, trace.Wrap(err)
	}

	connector, provider, err := a.getConnectorAndProvider(ctx, *request)
	if err != nil {
		traceErr("Failed to get SAML connector and provider", err)
		return nil, request, err
	}

	assertionInfo, err := provider.RetrieveAssertionInfo(samlResponse)
	if err != nil {
		errV := trace.AccessDenied(
			"received response with incorrect or missing attribute statements, please check the identity provider configuration to make sure that mappings for claims/attribute statements are set up correctly. <See: https://goteleport.com/teleport/docs/enterprise/sso/ssh-sso/>, failed to retrieve SAML assertion info from response: %v.", err)
		traceErr("Failed to retrieve assertion info. This may indicate IdP configuration error.", errV)
		return nil, request, errV
	}

	_ = a.Identity.TraceSAMLDiagnosticInfo(ctx, requestID, DiagInfoSAMLAssertionInfo, assertionInfo)

	if assertionInfo.WarningInfo.InvalidTime {
		err = trace.AccessDenied("invalid time in SAML assertion info")
		traceErr("SAML assertion info contained warning: invalid time.", err)
		return nil, request, err
	}

	if assertionInfo.WarningInfo.NotInAudience {
		err = trace.AccessDenied("no audience in SAML assertion info")
		traceErr("SAML: not in expected audience. Check auth connector audience field and IdP configuration for typos and other errors.", err)
		return nil, request, err
	}

	log.Debugf("Obtained SAML assertions for %q.", assertionInfo.NameID)
	re := &samlAuthResponse{
		attributeStatements: make(map[string][]string),
	}
	for key, val := range assertionInfo.Values {
		var vals []string
		for _, vv := range val.Values {
			vals = append(vals, vv.Value)
		}
		log.Debugf("SAML assertion: %q: %q.", key, vals)
		re.attributeStatements[key] = vals
	}

	_ = a.Identity.TraceSAMLDiagnosticInfo(ctx, requestID, DiagInfoSAMLAttributeStatements, re.attributeStatements)

	log.Debugf("SAML assertion warnings: %+v.", assertionInfo.WarningInfo)

	_ = a.Identity.TraceSAMLDiagnosticInfo(ctx, requestID, DiagInfoSAMLAttributesToRoles, connector.GetAttributesToRoles())

	if len(connector.GetAttributesToRoles()) == 0 {
		err = trace.BadParameter("no attributes to roles mapping, check connector documentation")
		traceErr("Attributes-to-roles mapping is empty, SSO user will never have any roles.", err)
		return re, request, err
	}

	log.Debugf("Applying %v SAML attribute to roles mappings.", len(connector.GetAttributesToRoles()))

	// Calculate (figure out name, roles, traits, session TTL) of user and
	// create the user in the backend.
	params, err := a.calculateSAMLUser(connector, *assertionInfo, request)
	if err != nil {
		traceErr("Failed to calculate user attributes.", err)
		return re, request, trace.Wrap(err)
	}

	_ = a.Identity.TraceSAMLDiagnosticInfo(ctx, requestID, DiagInfoCreateUserParams, params.toMap())

	user, err := a.createSAMLUser(params, request.SSOTestFlow)
	if err != nil {
		traceErr("Failed to create user from provided parameters.", err)
		return re, request, trace.Wrap(err)
	}

	// Auth was successful, return session, certificate, etc. to caller.
	re.auth = SAMLAuthResponse{
		Req: *request,
		Identity: types.ExternalIdentity{
			ConnectorID: params.connectorName,
			Username:    params.username,
		},
		Username: user.GetName(),
	}

	// In test flow skip signing and creating web sessions.
	if request.SSOTestFlow {
		_ = a.Identity.TraceSAMLDiagnosticInfo(ctx, requestID, DiagInfoResult, "success")
		return re, request, nil
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
			return re, request, trace.Wrap(err)
		}

		re.auth.Session = session
	}

	// If a public key was provided, sign it and return a certificate.
	if len(request.PublicKey) != 0 {
		sshCert, tlsCert, err := a.createSessionCert(user, params.sessionTTL, request.PublicKey, request.Compatibility, request.RouteToCluster, request.KubernetesCluster)
		if err != nil {
			traceErr("Failed to create session certificate.", err)
			return nil, request, trace.Wrap(err)
		}
		clusterName, err := a.GetClusterName()
		if err != nil {
			traceErr("Failed to obtain cluster name.", err)
			return nil, request, trace.Wrap(err)
		}
		re.auth.Cert = sshCert
		re.auth.TLSCert = tlsCert

		// Return the host CA for this cluster only.
		authority, err := a.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: clusterName.GetClusterName(),
		}, false)
		if err != nil {
			traceErr("Failed to obtain cluster's host CA.", err)
			return nil, request, trace.Wrap(err)
		}
		re.auth.HostSigners = append(re.auth.HostSigners, authority)
	}

	_ = a.Identity.TraceSAMLDiagnosticInfo(ctx, requestID, DiagInfoResult, "success")
	return re, request, nil
}
