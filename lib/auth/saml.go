package auth

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"io/ioutil"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/beevik/etree"
	"github.com/gravitational/trace"
	saml2 "github.com/russellhaering/gosaml2"
)

func (s *AuthServer) UpsertSAMLConnector(connector services.SAMLConnector) error {
	return s.Identity.UpsertSAMLConnector(connector)
}

func (s *AuthServer) DeleteSAMLConnector(connectorName string) error {
	return s.Identity.DeleteSAMLConnector(connectorName)
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

func (a *AuthServer) createSAMLUser(connector services.SAMLConnector, assertionInfo saml2.AssertionInfo, expiresAt time.Time) error {
	roles, err := a.buildSAMLRoles(connector, assertionInfo)
	if err != nil {
		return trace.Wrap(err)
	}

	traits := assertionsToTraitMap(assertionInfo)

	log.Debugf("[SAML] Generating dynamic identity %v/%v with roles: %v", connector.GetName(), assertionInfo.NameID, roles)
	user, err := services.GetUserMarshaler().GenerateUser(&services.UserV2{
		Kind:    services.KindUser,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      assertionInfo.NameID,
			Namespace: defaults.Namespace,
		},
		Spec: services.UserSpecV2{
			Roles:          roles,
			Traits:         traits,
			Expires:        expiresAt,
			SAMLIdentities: []services.ExternalIdentity{{ConnectorID: connector.GetName(), Username: assertionInfo.NameID}},
			CreatedBy: services.CreatedBy{
				User: services.UserRef{Name: "system"},
				Time: time.Now().UTC(),
				Connector: &services.ConnectorRef{
					Type:     teleport.ConnectorSAML,
					ID:       connector.GetName(),
					Identity: assertionInfo.NameID,
				},
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// check if a user exists already
	existingUser, err := a.GetUser(assertionInfo.NameID)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}

	// check if exisiting user is a non-saml user, if so, return an error
	if existingUser != nil {
		connectorRef := existingUser.GetCreatedBy().Connector
		if connectorRef == nil || connectorRef.Type != teleport.ConnectorSAML || connectorRef.ID != connector.GetName() {
			return trace.AlreadyExists("user %q already exists and is not SAML user, remove local user and try again.",
				existingUser.GetName())
		}
	}

	// no non-saml user exists, create or update the exisiting saml user
	err = a.UpsertUser(user)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
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
		log.Errorf("[SAML] Teleport does not support initiating login from an identity provider, login must be initiated from either the Teleport Web UI or CLI.")
		return "", trace.BadParameter("identity provider initiated flows are not supported")
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
		a.EmitAuditEvent(events.UserLoginEvent, events.EventFields{
			events.LoginMethod:        events.LoginMethodSAML,
			events.AuthAttemptSuccess: false,
			events.AuthAttemptErr:     err.Error(),
		})
	} else {
		a.EmitAuditEvent(events.UserLoginEvent, events.EventFields{
			events.EventUser:          re.Username,
			events.AuthAttemptSuccess: true,
			events.LoginMethod:        events.LoginMethodSAML,
		})
	}
	return re, err
}

func (a *AuthServer) validateSAMLResponse(samlResponse string) (*SAMLAuthResponse, error) {
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
		log.Warningf("SAML error: %v", err)
		return nil, trace.AccessDenied("bad SAML response")
	}

	if assertionInfo.WarningInfo.InvalidTime {
		log.Warningf("SAML error, invalid time")
		return nil, trace.AccessDenied("bad SAML response")
	}

	if assertionInfo.WarningInfo.NotInAudience {
		log.Warningf("SAML error, not in audience")
		return nil, trace.AccessDenied("bad SAML response")
	}

	log.Debugf("[SAML] Obtained Assertions for %q", assertionInfo.NameID)
	for key, val := range assertionInfo.Values {
		var vals []string
		for _, vv := range val.Values {
			vals = append(vals, vv.Value)
		}
		log.Debugf("[SAML]   Assertion: %q: %q", key, vals)
	}
	log.Debugf("[SAML] Assertion Warnings: %+v", assertionInfo.WarningInfo)

	log.Debugf("[SAML] Applying %v claims to roles mappings", len(connector.GetAttributesToRoles()))
	if len(connector.GetAttributesToRoles()) == 0 {
		return nil, trace.BadParameter("SAML does not support binding to local users")
	}
	// TODO(klizhentas) use SessionNotOnOrAfter to calculate expiration time
	expiresAt := a.clock.Now().Add(defaults.CertDuration)
	if err := a.createSAMLUser(connector, *assertionInfo, expiresAt); err != nil {
		return nil, trace.Wrap(err)
	}

	identity := services.ExternalIdentity{
		ConnectorID: request.ConnectorID,
		Username:    assertionInfo.NameID,
	}
	user, err := a.Identity.GetUserBySAMLIdentity(identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response := &SAMLAuthResponse{
		Req:      *request,
		Identity: identity,
		Username: user.GetName(),
	}

	var roles services.RoleSet
	roles, err = services.FetchRoles(user.GetRoles(), a.Access, user.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessionTTL := roles.AdjustSessionTTL(utils.ToTTL(a.clock, expiresAt))
	bearerTokenTTL := utils.MinTTL(BearerTokenTTL, sessionTTL)

	if request.CreateWebSession {
		sess, err := a.NewWebSession(user.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// session will expire based on identity TTL and allowed session TTL
		sess.SetExpiryTime(a.clock.Now().UTC().Add(sessionTTL))
		// bearer token will expire based on the expected session renewal
		sess.SetBearerTokenExpiryTime(a.clock.Now().UTC().Add(bearerTokenTTL))
		if err := a.UpsertWebSession(user.GetName(), sess); err != nil {
			return nil, trace.Wrap(err)
		}
		response.Session = sess
	}

	if len(request.PublicKey) != 0 {
		certTTL := utils.MinTTL(sessionTTL, request.CertTTL)
		certs, err := a.generateUserCert(certRequest{
			user:          user,
			roles:         roles,
			ttl:           certTTL,
			publicKey:     request.PublicKey,
			compatibility: request.Compatibility,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.Cert = certs.ssh
		response.TLSCert = certs.tls

		authorities, err := a.GetCertAuthorities(services.HostCA, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, authority := range authorities {
			response.HostSigners = append(response.HostSigners, authority)
		}
	}
	return response, nil
}
