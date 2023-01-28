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

	"github.com/beevik/etree"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	saml2 "github.com/russellhaering/gosaml2"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/loginrule"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// ErrSAMLNoRoles results from not mapping any roles from SAML claims.
var ErrSAMLNoRoles = trace.AccessDenied("No roles mapped from claims. The mappings may contain typos.")

// UpsertSAMLConnector creates or updates a SAML connector.
func (a *Server) UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) error {
	if err := services.ValidateSAMLConnector(connector, a); err != nil {
		return trace.Wrap(err)
	}
	if err := a.Services.UpsertSAMLConnector(ctx, connector); err != nil {
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
	if err := a.Services.DeleteSAMLConnector(ctx, connectorName); err != nil {
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

func (a *Server) CreateSAMLAuthRequest(ctx context.Context, req types.SAMLAuthRequest) (*types.SAMLAuthRequest, error) {
	connector, provider, err := a.getSAMLConnectorAndProvider(ctx, req)
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

	err = a.Services.CreateSAMLAuthRequest(ctx, req, defaults.SAMLAuthRequestTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

func (a *Server) getSAMLConnectorAndProviderByID(ctx context.Context, connectorID string) (types.SAMLConnector, *saml2.SAMLServiceProvider, error) {
	connector, err := a.Identity.GetSAMLConnector(ctx, connectorID, true)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	provider, err := a.getSAMLProvider(connector)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return connector, provider, nil
}

func (a *Server) getSAMLConnectorAndProvider(ctx context.Context, req types.SAMLAuthRequest) (types.SAMLConnector, *saml2.SAMLServiceProvider, error) {
	if req.SSOTestFlow {
		if req.ConnectorSpec == nil {
			return nil, nil, trace.BadParameter("ConnectorSpec cannot be nil when SSOTestFlow is true")
		}

		if req.ConnectorID == "" {
			return nil, nil, trace.BadParameter("ConnectorID cannot be empty")
		}

		// stateless test flow
		connector, err := types.NewSAMLConnector(req.ConnectorID, *req.ConnectorSpec)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		// validate, set defaults for connector
		err = services.ValidateSAMLConnector(connector, a)
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
	return a.getSAMLConnectorAndProviderByID(ctx, req.ConnectorID)
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

func (a *Server) calculateSAMLUser(ctx context.Context, diagCtx *ssoDiagContext, connector types.SAMLConnector, assertionInfo saml2.AssertionInfo, request *types.SAMLAuthRequest) (*createUserParams, error) {
	p := createUserParams{
		connectorName: connector.GetName(),
		username:      assertionInfo.NameID,
	}

	p.traits = services.SAMLAssertionsToTraits(assertionInfo)

	evaluationOutput, err := a.GetLoginRuleEvaluator().Evaluate(ctx, &loginrule.EvaluationInput{
		Traits: p.traits,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.traits = evaluationOutput.Traits

	diagCtx.info.SAMLTraitsFromAssertions = p.traits
	diagCtx.info.SAMLConnectorTraitMapping = connector.GetTraitMappings()

	var warnings []string
	warnings, p.roles = services.TraitsToRoles(connector.GetTraitMappings(), p.traits)
	if len(p.roles) == 0 {
		if len(warnings) != 0 {
			log.WithField("connector", connector).Warnf("No roles mapped from claims. Warnings: %q", warnings)
			diagCtx.info.SAMLAttributesToRolesWarnings = &types.SSOWarnings{
				Message:  "No roles mapped for the user",
				Warnings: warnings,
			}
		} else {
			log.WithField("connector", connector).Warnf("No roles mapped from claims.")
			diagCtx.info.SAMLAttributesToRolesWarnings = &types.SSOWarnings{
				Message: "No roles mapped for the user. The mappings may contain typos.",
			}
		}
		return nil, trace.Wrap(ErrSAMLNoRoles)
	}

	// Pick smaller for role: session TTL from role or requested TTL.
	roles, err := services.FetchRoles(p.roles, a, p.traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleTTL := roles.AdjustSessionTTL(apidefaults.MaxCertDuration)

	if request != nil {
		p.sessionTTL = utils.MinTTL(roleTTL, request.CertTTL)
	} else {
		p.sessionTTL = roleTTL
	}

	return &p, nil
}

func (a *Server) createSAMLUser(ctx context.Context, p *createUserParams, dryRun bool) (types.User, error) {
	expires := a.GetClock().Now().UTC().Add(p.sessionTTL)

	log.Debugf("Generating dynamic SAML identity %v/%v with roles: %v. Dry run: %v.", p.connectorName, p.username, p.roles, dryRun)

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
	existingUser, err := a.Services.GetUser(p.username, false)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

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

	// Try to find the InResponseTo attribute in the SAML response. If we can't find this, return
	// a predictable error message so the caller may choose interpret it as an IdP-initiated payload.
	el := doc.Root()
	responseTo := el.SelectAttr("InResponseTo")
	if responseTo == nil {
		return "", trace.NotFound("missing InResponseTo attribute")
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

// SAMLAuthRequestFromProto converts the types.SAMLAuthRequest to SAMLAuthRequestData.
func SAMLAuthRequestFromProto(req *types.SAMLAuthRequest) SAMLAuthRequest {
	return SAMLAuthRequest{
		ID:                req.ID,
		PublicKey:         req.PublicKey,
		CSRFToken:         req.CSRFToken,
		CreateWebSession:  req.CreateWebSession,
		ClientRedirectURL: req.ClientRedirectURL,
	}
}

// ValidateSAMLResponse consumes attribute statements from SAML identity provider
func (a *Server) ValidateSAMLResponse(ctx context.Context, samlResponse string, connectorID string) (*SAMLAuthResponse, error) {
	event := &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
		},
		Method: events.LoginMethodSAML,
	}

	diagCtx := a.newSSODiagContext(types.KindSAML)

	auth, err := a.validateSAMLResponse(ctx, diagCtx, samlResponse, connectorID)
	diagCtx.info.Error = trace.UserMessage(err)

	diagCtx.writeToBackend(ctx)

	attributeStatements := diagCtx.info.SAMLAttributeStatements
	if attributeStatements != nil {
		attributes, err := apievents.EncodeMapStrings(attributeStatements)
		if err != nil {
			event.Status.UserMessage = fmt.Sprintf("Failed to encode identity attributes: %v", err.Error())
			log.WithError(err).Debug("Failed to encode identity attributes.")
		} else {
			event.IdentityAttributes = attributes
		}
	}

	if err != nil {
		event.Code = events.UserSSOLoginFailureCode
		if diagCtx.info.TestFlow {
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
	if diagCtx.info.TestFlow {
		event.Code = events.UserSSOTestFlowLoginCode
	}

	if err := a.emitter.EmitAuditEvent(a.closeCtx, event); err != nil {
		log.WithError(err).Warn("Failed to emit SAML login event.")
	}

	return auth, nil
}

func (a *Server) checkIDPInitiatedSAML(ctx context.Context, connector types.SAMLConnector, assertion *saml2.AssertionInfo) error {
	if !connector.GetAllowIDPInitiated() {
		return trace.AccessDenied("IdP initiated SAML is not allowed by the connector configuration")
	}

	// Not all IdP's provide these variables, replay mitigation is best effort.
	if assertion.SessionIndex != "" || assertion.SessionNotOnOrAfter == nil {
		return nil
	}

	err := a.unstable.RecognizeSSOAssertion(ctx, connector.GetName(), assertion.SessionIndex, assertion.NameID, *assertion.SessionNotOnOrAfter)
	return trace.Wrap(err)
}

func (a *Server) validateSAMLResponse(ctx context.Context, diagCtx *ssoDiagContext, samlResponse string, connectorID string) (*SAMLAuthResponse, error) {
	idpInitiated := false
	var connector types.SAMLConnector
	var provider *saml2.SAMLServiceProvider
	var request *types.SAMLAuthRequest
	requestID, err := ParseSAMLInResponseTo(samlResponse)
	switch {
	case trace.IsNotFound(err):
		if connectorID == "" {
			return nil, trace.BadParameter("ACS URI did not include a valid SAML connector ID parameter")
		}

		idpInitiated = true
		connector, provider, err = a.getSAMLConnectorAndProviderByID(ctx, connectorID)
		if err != nil {
			return nil, trace.Wrap(err, "Failed to get SAML connector and provider")
		}
	case err != nil:
		return nil, trace.Wrap(err)
	default:
		diagCtx.requestID = requestID
		request, err = a.Identity.GetSAMLAuthRequest(ctx, requestID)
		if err != nil {
			return nil, trace.Wrap(err, "Failed to get SAML Auth Request")
		}

		diagCtx.info.TestFlow = request.SSOTestFlow
		connector, provider, err = a.getSAMLConnectorAndProvider(ctx, *request)
		if err != nil {
			return nil, trace.Wrap(err, "Failed to get SAML connector and provider")
		}
	}

	assertionInfo, err := provider.RetrieveAssertionInfo(samlResponse)
	if err != nil {
		samlErr := trace.AccessDenied("received response with incorrect or missing attribute statements, please check the identity provider configuration to make sure that mappings for claims/attribute statements are set up correctly. <See: https://goteleport.com/teleport/docs/enterprise/sso/ssh-sso/>, failed to retrieve SAML assertion info from response: %v.", err)
		return nil, trace.WithUserMessage(samlErr, "Failed to retrieve assertion info. This may indicate IdP configuration error.")
	}

	if assertionInfo != nil {
		diagCtx.info.SAMLAssertionInfo = (*types.AssertionInfo)(assertionInfo)
	}

	if idpInitiated {
		if err := a.checkIDPInitiatedSAML(ctx, connector, assertionInfo); err != nil {
			if trace.IsAccessDenied(err) {
				log.Warnf("Failed to process IdP-initiated login request. IdP-initiated login is disabled for this connector: %v.", err)
			}

			return nil, trace.Wrap(err)
		}
	}

	if assertionInfo.WarningInfo.InvalidTime {
		samlErr := trace.AccessDenied("invalid time in SAML assertion info")
		return nil, trace.WithUserMessage(samlErr, "SAML assertion info contained warning: invalid time.")
	}

	if assertionInfo.WarningInfo.NotInAudience {
		samlErr := trace.AccessDenied("no audience in SAML assertion info")
		return nil, trace.WithUserMessage(samlErr, "SAML: not in expected audience. Check auth connector audience field and IdP configuration for typos and other errors.")
	}

	log.Debugf("Obtained SAML assertions for %q.", assertionInfo.NameID)
	log.Debugf("SAML assertion warnings: %+v.", assertionInfo.WarningInfo)

	attributeStatements := map[string][]string{}

	for key, val := range assertionInfo.Values {
		var vals []string
		for _, vv := range val.Values {
			vals = append(vals, vv.Value)
		}
		log.Debugf("SAML assertion: %q: %q.", key, vals)
		attributeStatements[key] = vals
	}

	diagCtx.info.SAMLAttributeStatements = attributeStatements
	diagCtx.info.SAMLAttributesToRoles = connector.GetAttributesToRoles()

	if len(connector.GetAttributesToRoles()) == 0 {
		samlErr := trace.BadParameter("no attributes to roles mapping, check connector documentation")
		return nil, trace.WithUserMessage(samlErr, "Attributes-to-roles mapping is empty, SSO user will never have any roles.")
	}

	log.Debugf("Applying %v SAML attribute to roles mappings.", len(connector.GetAttributesToRoles()))

	// Calculate (figure out name, roles, traits, session TTL) of user and
	// create the user in the backend.
	params, err := a.calculateSAMLUser(ctx, diagCtx, connector, *assertionInfo, request)
	if err != nil {
		return nil, trace.Wrap(err, "Failed to calculate user attributes.")
	}

	diagCtx.info.CreateUserParams = &types.CreateUserParams{
		ConnectorName: params.connectorName,
		Username:      params.username,
		KubeGroups:    params.kubeGroups,
		KubeUsers:     params.kubeUsers,
		Roles:         params.roles,
		Traits:        params.traits,
		SessionTTL:    types.Duration(params.sessionTTL),
	}

	user, err := a.createSAMLUser(ctx, params, request != nil && request.SSOTestFlow)
	if err != nil {
		return nil, trace.Wrap(err, "Failed to create user from provided parameters.")
	}

	// Auth was successful, return session, certificate, etc. to caller.
	auth := &SAMLAuthResponse{
		Identity: types.ExternalIdentity{
			ConnectorID: params.connectorName,
			Username:    params.username,
		},
		Username: user.GetName(),
	}

	if request != nil {
		auth.Req = SAMLAuthRequestFromProto(request)
	} else {
		auth.Req = SAMLAuthRequest{
			CreateWebSession: true,
		}
	}

	// In test flow skip signing and creating web sessions.
	if request != nil && request.SSOTestFlow {
		diagCtx.info.Success = true
		return auth, nil
	}

	// If the request is coming from a browser, create a web session.
	if request == nil || request.CreateWebSession {
		session, err := a.createWebSession(ctx, types.NewWebSessionRequest{
			User:       user.GetName(),
			Roles:      user.GetRoles(),
			Traits:     user.GetTraits(),
			SessionTTL: params.sessionTTL,
			LoginTime:  a.clock.Now().UTC(),
		})
		if err != nil {
			return nil, trace.Wrap(err, "Failed to create web session.")
		}

		auth.Session = session
	}

	// If a public key was provided, sign it and return a certificate.
	if request != nil && len(request.PublicKey) != 0 {
		sshCert, tlsCert, err := a.createSessionCert(user, params.sessionTTL, request.PublicKey, request.Compatibility, request.RouteToCluster,
			request.KubernetesCluster, keys.AttestationStatementFromProto(request.AttestationStatement))
		if err != nil {
			return nil, trace.Wrap(err, "Failed to create session certificate.")
		}
		clusterName, err := a.GetClusterName()
		if err != nil {
			return nil, trace.Wrap(err, "Failed to obtain cluster name.")
		}
		auth.Cert = sshCert
		auth.TLSCert = tlsCert

		// Return the host CA for this cluster only.
		authority, err := a.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: clusterName.GetClusterName(),
		}, false)
		if err != nil {
			return nil, trace.Wrap(err, "Failed to obtain cluster's host CA.")
		}
		auth.HostSigners = append(auth.HostSigners, authority)
	}

	diagCtx.info.Success = true
	return auth, nil
}
