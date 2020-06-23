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
	"io/ioutil"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/beevik/etree"
	"github.com/gravitational/trace"
	saml2 "github.com/russellhaering/gosaml2"
)

// UpsertSAMLConnector creates or updates a SAML connector.
func (s *AuthServer) UpsertSAMLConnector(ctx context.Context, connector services.SAMLConnector) error {
	if err := s.Identity.UpsertSAMLConnector(connector); err != nil {
		return trace.Wrap(err)
	}

	if err := s.EmitAuditEvent(events.SAMLConnectorCreated, events.EventFields{
		events.FieldName: connector.GetName(),
		events.EventUser: clientUsername(ctx),
	}); err != nil {
		log.Warnf("Failed to emit SAML connector create event: %v", err)
	}

	return nil
}

// DeleteSAMLConnector deletes a SAML connector by name.
func (s *AuthServer) DeleteSAMLConnector(ctx context.Context, connectorName string) error {
	if err := s.Identity.DeleteSAMLConnector(connectorName); err != nil {
		return trace.Wrap(err)
	}

	if err := s.EmitAuditEvent(events.SAMLConnectorDeleted, events.EventFields{
		events.FieldName: connectorName,
		events.EventUser: clientUsername(ctx),
	}); err != nil {
		log.Warnf("Failed to emit SAML connector delete event: %v", err)
	}

	return nil
}

func (s *AuthServer) CreateSAMLAuthRequest(req services.SAMLAuthRequest) (*services.SAMLAuthRequest, error) {
	connector, err := s.Identity.GetSAMLConnector(req.ConnectorID, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	provider, err := s.getSAMLProvider(connector)
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
	req.RedirectURL, err = provider.BuildAuthURLFromDocument("", doc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.Identity.CreateSAMLAuthRequest(req, defaults.SAMLAuthRequestTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

func (s *AuthServer) getSAMLProvider(conn services.SAMLConnector) (*saml2.SAMLServiceProvider, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	providerPack, ok := s.samlProviders[conn.GetName()]
	if ok && providerPack.connector.Equals(conn) {
		return providerPack.provider, nil
	}
	delete(s.samlProviders, conn.GetName())

	serviceProvider, err := conn.GetServiceProvider(s.clock)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.samlProviders[conn.GetName()] = &samlProvider{connector: conn, provider: serviceProvider}

	return serviceProvider, nil
}

// buildSAMLRoles takes a connector and claims and returns a slice of roles.
func (a *AuthServer) buildSAMLRoles(connector services.SAMLConnector, assertionInfo saml2.AssertionInfo) ([]string, error) {
	roles := connector.MapAttributes(assertionInfo)
	if len(roles) == 0 {
		return nil, trace.AccessDenied("unable to map attributes to role for connector: %v", connector.GetName())
	}

	return roles, nil
}

// assertionsToTraitMap extracts all string assertions and creates a map of traits
// that can be used to populate role variables.
func assertionsToTraitMap(assertionInfo saml2.AssertionInfo) map[string][]string {
	traits := make(map[string][]string)

	for _, assr := range assertionInfo.Values {
		var vals []string
		for _, value := range assr.Values {
			vals = append(vals, value.Value)
		}
		traits[assr.Name] = vals
	}

	return traits
}

func (a *AuthServer) calculateSAMLUser(connector services.SAMLConnector, assertionInfo saml2.AssertionInfo, request *services.SAMLAuthRequest) (*createUserParams, error) {
	var err error

	p := createUserParams{
		connectorName: connector.GetName(),
		username:      assertionInfo.NameID,
	}

	p.roles, err = a.buildSAMLRoles(connector, assertionInfo)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.traits = assertionsToTraitMap(assertionInfo)

	// Pick smaller for role: session TTL from role or requested TTL.
	roles, err := services.FetchRoles(p.roles, a.Access, p.traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleTTL := roles.AdjustSessionTTL(defaults.MaxCertDuration)
	p.sessionTTL = utils.MinTTL(roleTTL, request.CertTTL)

	return &p, nil
}

func (a *AuthServer) createSAMLUser(p *createUserParams) (services.User, error) {
	expires := a.GetClock().Now().UTC().Add(p.sessionTTL)

	log.Debugf("Generating dynamic SAML identity %v/%v with roles: %v.", p.connectorName, p.username, p.roles)

	user, err := services.GetUserMarshaler().GenerateUser(&services.UserV2{
		Kind:    services.KindUser,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      p.username,
			Namespace: defaults.Namespace,
			Expires:   &expires,
		},
		Spec: services.UserSpecV2{
			Roles:  p.roles,
			Traits: p.traits,
			SAMLIdentities: []services.ExternalIdentity{
				{
					ConnectorID: p.connectorName,
					Username:    p.username,
				},
			},
			CreatedBy: services.CreatedBy{
				User: services.UserRef{
					Name: teleport.UserSystem,
				},
				Time: a.clock.Now().UTC(),
				Connector: &services.ConnectorRef{
					Type:     teleport.ConnectorSAML,
					ID:       p.connectorName,
					Identity: p.username,
				},
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
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

func parseSAMLInResponseTo(response string) (string, error) {
	raw, _ := base64.StdEncoding.DecodeString(response)

	doc := etree.NewDocument()
	err := doc.ReadFromBytes(raw)
	if err != nil {
		// Attempt to inflate the response in case it happens to be compressed (as with one case at saml.oktadev.com)
		buf, err := ioutil.ReadAll(flate.NewReader(bytes.NewReader(raw)))
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
	Identity services.ExternalIdentity `json:"identity"`
	// Web session will be generated by auth server if requested in SAMLAuthRequest
	Session services.WebSession `json:"session,omitempty"`
	// Cert will be generated by certificate authority
	Cert []byte `json:"cert,omitempty"`
	// TLSCert is a PEM encoded TLS certificate
	TLSCert []byte `json:"tls_cert,omitempty"`
	// Req is an original SAML auth request
	Req services.SAMLAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []services.CertAuthority `json:"host_signers"`
}

// ValidateSAMLResponse consumes attribute statements from SAML identity provider
func (a *AuthServer) ValidateSAMLResponse(samlResponse string) (*SAMLAuthResponse, error) {
	re, err := a.validateSAMLResponse(samlResponse)
	if err != nil {
		fields := events.EventFields{
			events.LoginMethod:        events.LoginMethodSAML,
			events.AuthAttemptSuccess: false,
			events.AuthAttemptErr:     err.Error(),
		}
		if re != nil && re.attributeStatements != nil {
			fields[events.IdentityAttributes] = re.attributeStatements
		}
		if err := a.EmitAuditEvent(events.UserSSOLoginFailure, fields); err != nil {
			log.Warnf("Failed to emit SAML login failure event: %v", err)
		}
		return nil, trace.Wrap(err)
	}
	fields := events.EventFields{
		events.EventUser:          re.auth.Username,
		events.AuthAttemptSuccess: true,
		events.LoginMethod:        events.LoginMethodSAML,
	}
	if re.attributeStatements != nil {
		fields[events.IdentityAttributes] = re.attributeStatements
	}
	if err := a.EmitAuditEvent(events.UserSSOLogin, fields); err != nil {
		log.Warnf("Failed to emit SAML user login event: %v", err)
	}
	return &re.auth, nil
}

type samlAuthResponse struct {
	auth                SAMLAuthResponse
	attributeStatements map[string][]string
}

func (a *AuthServer) validateSAMLResponse(samlResponse string) (*samlAuthResponse, error) {
	requestID, err := parseSAMLInResponseTo(samlResponse)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	request, err := a.Identity.GetSAMLAuthRequest(requestID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector, err := a.Identity.GetSAMLConnector(request.ConnectorID, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	provider, err := a.getSAMLProvider(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	assertionInfo, err := provider.RetrieveAssertionInfo(samlResponse)
	if err != nil {
		log.Warnf("Received response with incorrect or no claims/attribute statements. Please check the identity provider configuration to make sure that mappings for claims/attribute statements are set up correctly. <See: https://gravitational.com/teleport/docs/enterprise/ssh_sso/>. Failed to retrieve SAML AssertionInfo from response: %v.", err)
		return nil, trace.AccessDenied("bad SAML response")
	}

	if assertionInfo.WarningInfo.InvalidTime {
		log.Warnf("Invalid time in SAML AssertionInfo.")
		return nil, trace.AccessDenied("bad SAML response")
	}

	if assertionInfo.WarningInfo.NotInAudience {
		log.Warnf("No audience in SAML AssertionInfo.")
		return nil, trace.AccessDenied("bad SAML response")
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

	log.Debugf("SAML assertion warnings: %+v.", assertionInfo.WarningInfo)

	if len(connector.GetAttributesToRoles()) == 0 {
		return re, trace.BadParameter("no attributes to roles mapping, check connector documentation")
	}
	log.Debugf("Applying %v SAML attribute to roles mappings.", len(connector.GetAttributesToRoles()))

	// Calculate (figure out name, roles, traits, session TTL) of user and
	// create the user in the backend.
	params, err := a.calculateSAMLUser(connector, *assertionInfo, request)
	if err != nil {
		return re, trace.Wrap(err)
	}
	user, err := a.createSAMLUser(params)
	if err != nil {
		return re, trace.Wrap(err)
	}

	// Auth was successful, return session, certificate, etc. to caller.
	re.auth = SAMLAuthResponse{
		Req: *request,
		Identity: services.ExternalIdentity{
			ConnectorID: params.connectorName,
			Username:    params.username,
		},
		Username: user.GetName(),
	}

	// If the request is coming from a browser, create a web session.
	if request.CreateWebSession {
		session, err := a.createWebSession(user, params.sessionTTL)
		if err != nil {
			return re, trace.Wrap(err)
		}

		re.auth.Session = session
	}

	// If a public key was provided, sign it and return a certificate.
	if len(request.PublicKey) != 0 {
		sshCert, tlsCert, err := a.createSessionCert(user, params.sessionTTL, request.PublicKey, request.Compatibility)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clusterName, err := a.GetClusterName()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		re.auth.Cert = sshCert
		re.auth.TLSCert = tlsCert

		// Return the host CA for this cluster only.
		authority, err := a.GetCertAuthority(services.CertAuthID{
			Type:       services.HostCA,
			DomainName: clusterName.GetClusterName(),
		}, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		re.auth.HostSigners = append(re.auth.HostSigners, authority)
	}

	return re, nil
}
