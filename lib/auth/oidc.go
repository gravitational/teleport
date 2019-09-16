/*
Copyright 2017 Gravitational, Inc.

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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/go-oidc/jose"
	"github.com/coreos/go-oidc/oauth2"
	"github.com/coreos/go-oidc/oidc"
	"github.com/gravitational/trace"
)

func (s *AuthServer) getOIDCClient(conn services.OIDCConnector) (*oidc.Client, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	config := oidc.ClientConfig{
		RedirectURL: conn.GetRedirectURL(),
		Credentials: oidc.ClientCredentials{
			ID:     conn.GetClientID(),
			Secret: conn.GetClientSecret(),
		},
		// open id notifies provider that we are using OIDC scopes
		Scope: utils.Deduplicate(append([]string{"openid", "email"}, conn.GetScope()...)),
	}

	clientPack, ok := s.oidcClients[conn.GetName()]
	if ok && oidcConfigsEqual(clientPack.config, config) {
		return clientPack.client, nil
	}
	delete(s.oidcClients, conn.GetName())

	client, err := oidc.NewClient(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client.SyncProviderConfig(conn.GetIssuerURL())

	s.oidcClients[conn.GetName()] = &oidcClient{client: client, config: config}

	return client, nil
}

func (s *AuthServer) UpsertOIDCConnector(connector services.OIDCConnector) error {
	return s.Identity.UpsertOIDCConnector(connector)
}

func (s *AuthServer) DeleteOIDCConnector(connectorName string) error {
	return s.Identity.DeleteOIDCConnector(connectorName)
}

func (s *AuthServer) CreateOIDCAuthRequest(req services.OIDCAuthRequest) (*services.OIDCAuthRequest, error) {
	connector, err := s.Identity.GetOIDCConnector(req.ConnectorID, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	oidcClient, err := s.getOIDCClient(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	oauthClient, err := oidcClient.OAuthClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stateToken, err := utils.CryptoRandomHex(TokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req.StateToken = stateToken
	// online is OIDC online scope, "select_account" forces user to always select account
	req.RedirectURL = oauthClient.AuthCodeURL(req.StateToken, "online", "select_account")

	// if the connector has an Authentication Context Class Reference (ACR) value set,
	// update redirect url and add it as a query value.
	acrValue := connector.GetACR()
	if acrValue != "" {
		u, err := url.Parse(req.RedirectURL)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		q := u.Query()
		q.Set("acr_values", acrValue)
		u.RawQuery = q.Encode()
		req.RedirectURL = u.String()
	}

	log.Debugf("[OIDC] Redirect URL: %v", req.RedirectURL)

	err = s.Identity.CreateOIDCAuthRequest(req, defaults.OIDCAuthRequestTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

// ValidateOIDCAuthCallback is called by the proxy to check OIDC query parameters
// returned by OIDC Provider, if everything checks out, auth server
// will respond with OIDCAuthResponse, otherwise it will return error
func (a *AuthServer) ValidateOIDCAuthCallback(q url.Values) (*OIDCAuthResponse, error) {
	re, err := a.validateOIDCAuthCallback(q)
	if err != nil {
		a.EmitAuditEvent(events.UserLoginEvent, events.EventFields{
			events.LoginMethod:        events.LoginMethodOIDC,
			events.AuthAttemptSuccess: false,
			events.AuthAttemptErr:     err.Error(),
		})
	} else {
		a.EmitAuditEvent(events.UserLoginEvent, events.EventFields{
			events.EventUser:          re.Username,
			events.AuthAttemptSuccess: true,
			events.LoginMethod:        events.LoginMethodOIDC,
		})
	}
	return re, err
}

func (a *AuthServer) validateOIDCAuthCallback(q url.Values) (*OIDCAuthResponse, error) {
	if error := q.Get("error"); error != "" {
		return nil, trace.OAuth2(oauth2.ErrorInvalidRequest, error, q)
	}

	code := q.Get("code")
	if code == "" {
		return nil, trace.OAuth2(
			oauth2.ErrorInvalidRequest, "code query param must be set", q)
	}

	stateToken := q.Get("state")
	if stateToken == "" {
		return nil, trace.OAuth2(
			oauth2.ErrorInvalidRequest, "missing state query param", q)
	}

	req, err := a.Identity.GetOIDCAuthRequest(stateToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connector, err := a.Identity.GetOIDCConnector(req.ConnectorID, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oidcClient, err := a.getOIDCClient(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// extract claims from both the id token and the userinfo endpoint and merge them
	claims, err := a.getClaims(oidcClient, connector.GetIssuerURL(), code)
	if err != nil {
		return nil, trace.OAuth2(
			oauth2.ErrorUnsupportedResponseType, "unable to construct claims", q)
	}
	log.Debugf("[OIDC] Claims: %v", claims)

	// if we are sending acr values, make sure we also validate them
	acrValue := connector.GetACR()
	if acrValue != "" {
		err := a.validateACRValues(acrValue, connector.GetProvider(), claims)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		log.Debugf("[OIDC] ACR values %q successfully validated", acrValue)
	}

	ident, err := oidc.IdentityFromClaims(claims)
	if err != nil {
		return nil, trace.OAuth2(
			oauth2.ErrorUnsupportedResponseType, "unable to convert claims to identity", q)
	}
	log.Debugf("[OIDC] %q expires at: %v", ident.Email, ident.ExpiresAt)

	if len(connector.GetClaimsToRoles()) == 0 {
		return nil, trace.BadParameter("no claims to roles mapping, check connector documentation")
	}
	log.Debugf("Applying %v OIDC claims to roles mappings.", len(connector.GetClaimsToRoles()))

	// Calculate (figure out name, roles, traits, session TTL) of user and
	// create the user in the backend.
	params, err := a.calculateOIDCUser(connector, claims, ident, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := a.createOIDCUser(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Auth was successful, return session, certificate, etc. to caller.
	response := &OIDCAuthResponse{
		Req: *req,
		Identity: services.ExternalIdentity{
			ConnectorID: params.connectorName,
			Username:    params.username,
		},
		Username: user.GetName(),
	}

	if !req.CheckUser {
		return response, nil
	}

	// If the request is coming from a browser, create a web session.
	if req.CreateWebSession {
		session, err := a.createWebSession(user, params.sessionTTL)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		response.Session = session
	}

	// If a public key was provided, sign it and return a certificate.
	if len(req.PublicKey) != 0 {
		sshCert, tlsCert, err := a.createSessionCert(user, params.sessionTTL, req.PublicKey, req.Compatibility)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		response.Cert = sshCert
		response.TLSCert = tlsCert

		// Return the host CA for this cluster only.
		authority, err := a.GetCertAuthority(services.CertAuthID{
			Type:       services.HostCA,
			DomainName: a.clusterName.GetClusterName(),
		}, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.HostSigners = append(response.HostSigners, authority)
	}

	return response, nil
}

// OIDCAuthResponse is returned when auth server validated callback parameters
// returned from OIDC provider
type OIDCAuthResponse struct {
	// Username is authenticated teleport username
	Username string `json:"username"`
	// Identity contains validated OIDC identity
	Identity services.ExternalIdentity `json:"identity"`
	// Web session will be generated by auth server if requested in OIDCAuthRequest
	Session services.WebSession `json:"session,omitempty"`
	// Cert will be generated by certificate authority
	Cert []byte `json:"cert,omitempty"`
	// TLSCert is PEM encoded TLS certificate
	TLSCert []byte `json:"tls_cert,omitempty"`
	// Req is original oidc auth request
	Req services.OIDCAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []services.CertAuthority `json:"host_signers"`
}

// buildOIDCRoles takes a connector and claims and returns a slice of roles.
func (a *AuthServer) buildOIDCRoles(connector services.OIDCConnector, claims jose.Claims) ([]string, error) {
	roles := connector.MapClaims(claims)
	if len(roles) == 0 {
		return nil, trace.AccessDenied("unable to map claims to role for connector: %v", connector.GetName())
	}

	return roles, nil
}

// claimsToTraitMap extracts all string claims and creates a map of traits
// that can be used to populate role variables.
func claimsToTraitMap(claims jose.Claims) map[string][]string {
	traits := make(map[string][]string)

	for claimName := range claims {
		claimValue, ok, _ := claims.StringClaim(claimName)
		if ok {
			traits[claimName] = []string{claimValue}
		}
		claimValues, ok, _ := claims.StringsClaim(claimName)
		if ok {
			traits[claimName] = claimValues
		}
	}

	return traits
}

func (a *AuthServer) calculateOIDCUser(connector services.OIDCConnector, claims jose.Claims, ident *oidc.Identity, request *services.OIDCAuthRequest) (*createUserParams, error) {
	var err error

	p := createUserParams{
		connectorName: connector.GetName(),
		username:      ident.Email,
	}

	p.roles, err = a.buildOIDCRoles(connector, claims)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.traits = claimsToTraitMap(claims)

	// Pick smaller for role: session TTL from role or requested TTL.
	roles, err := services.FetchRoles(p.roles, a.Access, p.traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleTTL := roles.AdjustSessionTTL(defaults.MaxCertDuration)
	p.sessionTTL = utils.MinTTL(roleTTL, request.CertTTL)

	return &p, nil
}

func (a *AuthServer) createOIDCUser(p *createUserParams) (services.User, error) {
	expires := a.GetClock().Now().UTC().Add(p.sessionTTL)

	log.Debugf("Generating dynamic OIDC identity %v/%v with roles: %v.", p.connectorName, p.username, p.roles)
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
			OIDCIdentities: []services.ExternalIdentity{
				services.ExternalIdentity{
					ConnectorID: p.connectorName,
					Username:    p.username,
				},
			},
			CreatedBy: services.CreatedBy{
				User: services.UserRef{Name: "system"},
				Time: a.clock.Now().UTC(),
				Connector: &services.ConnectorRef{
					Type:     teleport.ConnectorOIDC,
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
	existingUser, err := a.GetUser(p.username)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}

	// check if existing user is a non-oidc user, if so, return an error
	if existingUser != nil {
		connectorRef := existingUser.GetCreatedBy().Connector

		// If the exisiting user is a local user, fail and advise how to fix the problem.
		if connectorRef == nil {
			return nil, trace.AlreadyExists("local user with name '%v' already exists. Either change "+
				"email in OIDC identity or remove local user and try again.", existingUser.GetName())
		}
	}

	// no non-oidc user exists, create or update the exisiting oidc user
	err = a.UpsertUser(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return user, nil
}

// claimsFromIDToken extracts claims from the ID token.
func claimsFromIDToken(oidcClient *oidc.Client, idToken string) (jose.Claims, error) {
	jwt, err := jose.ParseJWT(idToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = oidcClient.VerifyJWT(jwt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Debugf("[OIDC] Extracting claims from ID token")

	claims, err := jwt.Claims()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return claims, nil
}

// claimsFromUserInfo finds the UserInfo endpoint from the provider config and then extracts claims from it.
//
// Note: We don't request signed JWT responses for UserInfo, instead we force the provider config and
// the issuer to be HTTPS and leave integrity and confidentiality to TLS. Authenticity is taken care of
// during the token exchange.
func claimsFromUserInfo(oidcClient *oidc.Client, issuerURL string, accessToken string) (jose.Claims, error) {
	err := isHTTPS(issuerURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oac, err := oidcClient.OAuthClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	hc := oac.HttpClient()

	// go get the provider config so we can find out where the UserInfo endpoint
	// is. if the provider doesn't offer a UserInfo endpoint return not found.
	pc, err := oidc.FetchProviderConfig(oac.HttpClient(), issuerURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if pc.UserInfoEndpoint == nil {
		return nil, trace.NotFound("UserInfo endpoint not found")
	}

	endpoint := pc.UserInfoEndpoint.String()
	err = isHTTPS(endpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("[OIDC] Fetching claims from UserInfo endpoint: %q", endpoint)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := hc.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, trace.AccessDenied("bad status code: %v", resp.StatusCode)
	}

	var claims jose.Claims
	err = json.NewDecoder(resp.Body).Decode(&claims)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return claims, nil
}

// mergeClaims merges b into a.
func mergeClaims(a jose.Claims, b jose.Claims) (jose.Claims, error) {
	for k, v := range b {
		_, ok := a[k]
		if !ok {
			a[k] = v
		}
	}

	return a, nil
}

// getClaims gets claims from ID token and UserInfo and returns UserInfo claims merged into ID token claims.
func (a *AuthServer) getClaims(oidcClient *oidc.Client, issuerURL string, code string) (jose.Claims, error) {
	var err error

	oac, err := oidcClient.OAuthClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	t, err := oac.RequestToken(oauth2.GrantTypeAuthCode, code)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	idTokenClaims, err := claimsFromIDToken(oidcClient, t.IDToken)
	if err != nil {
		log.Debugf("[OIDC] Unable to fetch ID token claims: %v", err)
		return nil, trace.Wrap(err)
	}
	log.Debugf("[OIDC] ID Token claims: %v", idTokenClaims)

	userInfoClaims, err := claimsFromUserInfo(oidcClient, issuerURL, t.AccessToken)
	if err != nil {
		if trace.IsNotFound(err) {
			log.Debugf("[OIDC] Provider doesn't offer UserInfo endpoint. Returning token claims: %v", idTokenClaims)
			return idTokenClaims, nil
		}
		log.Debugf("[OIDC] Unable to fetch UserInfo claims: %v", err)
		return nil, trace.Wrap(err)
	}
	log.Debugf("[OIDC] UserInfo claims: %v", userInfoClaims)

	// make sure that the subject in the userinfo claim matches the subject in
	// the id token otherwise there is the possibility of a token substitution attack.
	// see section 16.11 of the oidc spec for more details.
	var idsub string
	var uisub string
	var exists bool
	if idsub, exists, err = idTokenClaims.StringClaim("sub"); err != nil || !exists {
		log.Debugf("[OIDC] unable to extract sub from ID token")
		return nil, trace.Wrap(err)
	}
	if uisub, exists, err = userInfoClaims.StringClaim("sub"); err != nil || !exists {
		log.Debugf("[OIDC] unable to extract sub from UserInfo")
		return nil, trace.Wrap(err)
	}
	if idsub != uisub {
		log.Debugf("[OIDC] Claim subjects don't match %q != %q", idsub, uisub)
		return nil, trace.BadParameter("invalid subject in UserInfo")
	}

	claims, err := mergeClaims(idTokenClaims, userInfoClaims)
	if err != nil {
		log.Debugf("[OIDC] Unable to merge claims: %v", err)
		return nil, trace.Wrap(err)
	}

	return claims, nil
}

// validateACRValues validates that we get an appropriate response for acr values. By default
// we expect the same value we send, but this function also handles Identity Provider specific
// forms of validation.
func (a *AuthServer) validateACRValues(acrValue string, identityProvider string, claims jose.Claims) error {
	switch identityProvider {
	case teleport.NetIQ:
		log.Debugf("[OIDC] Validating ACR values with %q rules", identityProvider)

		tokenAcr, ok := claims["acr"]
		if !ok {
			return trace.BadParameter("acr not found in claims")
		}
		tokenAcrMap, ok := tokenAcr.(map[string]interface{})
		if !ok {
			return trace.BadParameter("acr unexpected type: %T", tokenAcr)
		}
		tokenAcrValues, ok := tokenAcrMap["values"]
		if !ok {
			return trace.BadParameter("acr.values not found in claims")
		}
		tokenAcrValuesSlice, ok := tokenAcrValues.([]interface{})
		if !ok {
			return trace.BadParameter("acr.values unexpected type: %T", tokenAcr)
		}

		acrValueMatched := false
		for _, v := range tokenAcrValuesSlice {
			vv, ok := v.(string)
			if !ok {
				continue
			}
			if acrValue == vv {
				acrValueMatched = true
				break
			}
		}
		if !acrValueMatched {
			log.Debugf("[OIDC] No ACR match found for %q in %q", acrValue, tokenAcrValues)
			return trace.BadParameter("acr claim does not match")
		}
	default:
		log.Debugf("[OIDC] Validating ACR values with default rules")

		claimValue, exists, err := claims.StringClaim("acr")
		if !exists {
			return trace.BadParameter("acr claim does not exist")
		}
		if err != nil {
			return trace.Wrap(err)
		}
		if claimValue != acrValue {
			log.Debugf("[OIDC] No ACR match found %q != %q", acrValue, claimValue)
			return trace.BadParameter("acr claim does not match")
		}
	}

	return nil
}
