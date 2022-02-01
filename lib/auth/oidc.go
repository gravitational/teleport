/*
Copyright 2017-2021 Gravitational, Inc.

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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/go-oidc/jose"
	"github.com/coreos/go-oidc/oauth2"
	"github.com/coreos/go-oidc/oidc"
	"github.com/gravitational/trace"
)

func (a *Server) getOrCreateOIDCClient(conn types.OIDCConnector) (*oidc.Client, error) {
	client, err := a.getOIDCClient(conn)
	if err == nil {
		return client, nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	return a.createOIDCClient(conn)
}

func (a *Server) getOIDCClient(conn types.OIDCConnector) (*oidc.Client, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	clientPack, ok := a.oidcClients[conn.GetName()]
	if !ok {
		return nil, trace.NotFound("connector %v is not found", conn.GetName())
	}

	config := oidcConfig(conn)
	if ok && oidcConfigsEqual(clientPack.config, config) {
		return clientPack.client, nil
	}

	delete(a.oidcClients, conn.GetName())
	return nil, trace.NotFound("connector %v has updated the configuration and is invalidated", conn.GetName())

}

func (a *Server) createOIDCClient(conn types.OIDCConnector) (*oidc.Client, error) {
	config := oidcConfig(conn)
	client, err := oidc.NewClient(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaults.WebHeadersTimeout)
	defer cancel()

	go func() {
		defer cancel()
		client.SyncProviderConfig(conn.GetIssuerURL())
	}()

	select {
	case <-ctx.Done():
	case <-a.closeCtx.Done():
		return nil, trace.ConnectionProblem(nil, "auth server is shutting down")
	}

	// Canceled is expected in case if sync provider config finishes faster
	// than the deadline
	if ctx.Err() != nil && ctx.Err() != context.Canceled {
		var err error
		if ctx.Err() == context.DeadlineExceeded {
			err = trace.ConnectionProblem(err,
				"failed to reach out to oidc connector %v, most likely URL %q is not valid or not accessible, check configuration and try to re-create the connector",
				conn.GetName(), conn.GetIssuerURL())
		} else {
			err = trace.ConnectionProblem(err,
				"unknown problem with connector %v, most likely URL %q is not valid or not accessible, check configuration and try to re-create the connector",
				conn.GetName(), conn.GetIssuerURL())
		}
		return nil, err
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	a.oidcClients[conn.GetName()] = &oidcClient{client: client, config: config}

	return client, nil
}

func oidcConfig(conn types.OIDCConnector) oidc.ClientConfig {
	return oidc.ClientConfig{
		RedirectURL: conn.GetRedirectURL(),
		Credentials: oidc.ClientCredentials{
			ID:     conn.GetClientID(),
			Secret: conn.GetClientSecret(),
		},
		// open id notifies provider that we are using OIDC scopes
		Scope: apiutils.Deduplicate(append([]string{"openid", "email"}, conn.GetScope()...)),
	}
}

// UpsertOIDCConnector creates or updates an OIDC connector.
func (a *Server) UpsertOIDCConnector(ctx context.Context, connector types.OIDCConnector) error {
	if err := a.Identity.UpsertOIDCConnector(ctx, connector); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.OIDCConnectorCreate{
		Metadata: apievents.Metadata{
			Type: events.OIDCConnectorCreatedEvent,
			Code: events.OIDCConnectorCreatedCode,
		},
		UserMetadata: ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: connector.GetName(),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit OIDC connector create event.")
	}

	return nil
}

// DeleteOIDCConnector deletes an OIDC connector by name.
func (a *Server) DeleteOIDCConnector(ctx context.Context, connectorName string) error {
	if err := a.Identity.DeleteOIDCConnector(ctx, connectorName); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(ctx, &apievents.OIDCConnectorDelete{
		Metadata: apievents.Metadata{
			Type: events.OIDCConnectorDeletedEvent,
			Code: events.OIDCConnectorDeletedCode,
		},
		UserMetadata: ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: connectorName,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit OIDC connector delete event.")
	}
	return nil
}

func (a *Server) CreateOIDCAuthRequest(req services.OIDCAuthRequest) (*services.OIDCAuthRequest, error) {
	ctx := context.TODO()
	connector, err := a.Identity.GetOIDCConnector(ctx, req.ConnectorID, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	oidcClient, err := a.getOrCreateOIDCClient(connector)
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

	// online indicates that this login should only work online
	req.RedirectURL = oauthClient.AuthCodeURL(req.StateToken, teleport.OIDCAccessTypeOnline, connector.GetPrompt())

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

	log.Debugf("OIDC redirect URL: %v.", req.RedirectURL)

	err = a.Identity.CreateOIDCAuthRequest(req, defaults.OIDCAuthRequestTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

// ValidateOIDCAuthCallback is called by the proxy to check OIDC query parameters
// returned by OIDC Provider, if everything checks out, auth server
// will respond with OIDCAuthResponse, otherwise it will return error
func (a *Server) ValidateOIDCAuthCallback(q url.Values) (*OIDCAuthResponse, error) {
	event := &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
		},
		Method: events.LoginMethodOIDC,
	}

	re, err := a.validateOIDCAuthCallback(q)
	if re != nil && re.claims != nil {
		attributes, err := apievents.EncodeMap(re.claims)
		if err != nil {
			event.Status.UserMessage = fmt.Sprintf("Failed to encode identity attributes: %v", err.Error())
			log.WithError(err).Debug("Failed to encode identity attributes.")
		} else {
			event.IdentityAttributes = attributes
		}
	}

	if err != nil {
		event.Code = events.UserSSOLoginFailureCode
		event.Status.Success = false
		event.Status.Error = trace.Unwrap(err).Error()
		event.Status.UserMessage = err.Error()

		if err := a.emitter.EmitAuditEvent(a.closeCtx, event); err != nil {
			log.WithError(err).Warn("Failed to emit OIDC login failed event.")
		}

		return nil, trace.Wrap(err)
	}
	event.Code = events.UserSSOLoginCode
	event.User = re.auth.Username
	event.Status.Success = true

	if err := a.emitter.EmitAuditEvent(a.closeCtx, event); err != nil {
		log.WithError(err).Warn("Failed to emit OIDC login event.")
	}

	return &re.auth, nil
}

type oidcAuthResponse struct {
	auth   OIDCAuthResponse
	claims jose.Claims
}

func (a *Server) validateOIDCAuthCallback(q url.Values) (*oidcAuthResponse, error) {
	ctx := context.TODO()
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

	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req, err := a.Identity.GetOIDCAuthRequest(stateToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connector, err := a.Identity.GetOIDCConnector(ctx, req.ConnectorID, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oidcClient, err := a.getOrCreateOIDCClient(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// extract claims from both the id token and the userinfo endpoint and merge them
	claims, err := a.getClaims(oidcClient, connector, code)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	re := &oidcAuthResponse{
		claims: claims,
	}

	log.Debugf("OIDC claims: %v.", re.claims)

	// if we are sending acr values, make sure we also validate them
	acrValue := connector.GetACR()
	if acrValue != "" {
		err := a.validateACRValues(acrValue, connector.GetProvider(), claims)
		if err != nil {
			return re, trace.Wrap(err)
		}
		log.Debugf("OIDC ACR values %q successfully validated.", acrValue)
	}

	ident, err := oidc.IdentityFromClaims(claims)
	if err != nil {
		return re, trace.OAuth2(
			oauth2.ErrorUnsupportedResponseType, "unable to convert claims to identity", q)
	}
	log.Debugf("OIDC user %q expires at: %v.", ident.Email, ident.ExpiresAt)

	if len(connector.GetClaimsToRoles()) == 0 {
		return re, trace.BadParameter("no claims to roles mapping, check connector documentation")
	}
	log.Debugf("Applying %v OIDC claims to roles mappings.", len(connector.GetClaimsToRoles()))

	// Calculate (figure out name, roles, traits, session TTL) of user and
	// create the user in the backend.
	params, err := a.calculateOIDCUser(connector, claims, ident, req)
	if err != nil {
		return re, trace.Wrap(err)
	}
	user, err := a.createOIDCUser(params)
	if err != nil {
		return re, trace.Wrap(err)
	}

	// Auth was successful, return session, certificate, etc. to caller.
	re.auth = OIDCAuthResponse{
		Req: *req,
		Identity: types.ExternalIdentity{
			ConnectorID: params.connectorName,
			Username:    params.username,
		},
		Username: user.GetName(),
	}

	if !req.CheckUser {
		return re, nil
	}

	// If the request is coming from a browser, create a web session.
	if req.CreateWebSession {
		session, err := a.createWebSession(context.TODO(), types.NewWebSessionRequest{
			User:       user.GetName(),
			Roles:      user.GetRoles(),
			Traits:     user.GetTraits(),
			SessionTTL: params.sessionTTL,
			LoginTime:  a.clock.Now().UTC(),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		re.auth.Session = session
	}

	// If a public key was provided, sign it and return a certificate.
	if len(req.PublicKey) != 0 {
		sshCert, tlsCert, err := a.createSessionCert(user, params.sessionTTL, req.PublicKey, req.Compatibility, req.RouteToCluster, req.KubernetesCluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		re.auth.Cert = sshCert
		re.auth.TLSCert = tlsCert

		// Return the host CA for this cluster only.
		authority, err := a.GetCertAuthority(types.CertAuthID{
			Type:       types.HostCA,
			DomainName: clusterName.GetClusterName(),
		}, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		re.auth.HostSigners = append(re.auth.HostSigners, authority)
	}

	return re, nil
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
	Req services.OIDCAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []types.CertAuthority `json:"host_signers"`
}

func (a *Server) calculateOIDCUser(connector types.OIDCConnector, claims jose.Claims, ident *oidc.Identity, request *services.OIDCAuthRequest) (*createUserParams, error) {
	var err error

	p := createUserParams{
		connectorName: connector.GetName(),
		username:      ident.Email,
	}

	p.traits = services.OIDCClaimsToTraits(claims)

	var warnings []string
	warnings, p.roles = services.TraitsToRoles(connector.GetTraitMappings(), p.traits)
	if len(p.roles) == 0 {
		if len(warnings) != 0 {
			log.WithField("connector", connector).Warnf("Unable to map attibutes to roles: %q", warnings)
		}
		return nil, trace.AccessDenied("unable to map claims to role for connector: %v", connector.GetName())
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

func (a *Server) createOIDCUser(p *createUserParams) (types.User, error) {
	expires := a.GetClock().Now().UTC().Add(p.sessionTTL)

	log.Debugf("Generating dynamic OIDC identity %v/%v with roles: %v.", p.connectorName, p.username, p.roles)
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
			OIDCIdentities: []types.ExternalIdentity{
				{
					ConnectorID: p.connectorName,
					Username:    p.username,
				},
			},
			CreatedBy: types.CreatedBy{
				User: types.UserRef{Name: teleport.UserSystem},
				Time: a.clock.Now().UTC(),
				Connector: &types.ConnectorRef{
					Type:     constants.OIDC,
					ID:       p.connectorName,
					Identity: p.username,
				},
			},
		},
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
				"email in OIDC identity or remove local user and try again.", existingUser.GetName())
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

	log.Debugf("Extracting OIDC claims from ID token.")

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
	// If the issuer URL is not HTTPS, return the error as trace.NotFound to
	// allow the caller to treat this condition gracefully and extract claims
	// just from the token.
	err := isHTTPS(issuerURL)
	if err != nil {
		return nil, trace.NotFound(err.Error())
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

	// If the userinfo endpoint is not HTTPS, return the error as trace.NotFound to
	// allow the caller to treat this condition gracefully and extract claims
	// just from the token.
	err = isHTTPS(endpoint)
	if err != nil {
		return nil, trace.NotFound(err.Error())
	}
	log.Debugf("Fetching OIDC claims from UserInfo endpoint: %q.", endpoint)

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

	code := resp.StatusCode
	if code < 200 || code > 299 {
		// These are expected userinfo failures.
		if code == http.StatusBadRequest || code == http.StatusUnauthorized ||
			code == http.StatusForbidden || code == http.StatusMethodNotAllowed {
			return nil, trace.AccessDenied("bad status code: %v", code)
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return nil, trace.ReadError(code, body)
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
func (a *Server) getClaims(oidcClient *oidc.Client, connector types.OIDCConnector, code string) (jose.Claims, error) {

	oac, err := a.getOAuthClient(oidcClient, connector)

	if err != nil {
		return nil, trace.Wrap(err)
	}

	t, err := oac.RequestToken(oauth2.GrantTypeAuthCode, code)
	if err != nil {
		if e, ok := err.(*oauth2.Error); ok {
			if e.Type == oauth2.ErrorAccessDenied {
				return nil, trace.Wrap(err, "the client_id and/or client_secret may be incorrect")
			}
		}
		return nil, trace.Wrap(err)
	}

	idTokenClaims, err := claimsFromIDToken(oidcClient, t.IDToken)
	if err != nil {
		log.Debugf("Unable to fetch OIDC ID token claims: %v.", err)
		return nil, trace.Wrap(err, "unable to fetch OIDC ID token claims")
	}
	log.Debugf("OIDC ID Token claims: %v.", idTokenClaims)

	userInfoClaims, err := claimsFromUserInfo(oidcClient, connector.GetIssuerURL(), t.AccessToken)
	if err != nil {
		if trace.IsNotFound(err) {
			log.Debugf("OIDC provider doesn't offer valid UserInfo endpoint. Returning token claims: %v.", idTokenClaims)
			return idTokenClaims, nil
		}
		// This captures 400, 401, 403, and 405.
		if trace.IsAccessDenied(err) {
			log.Debugf("UserInfo endpoint returned an error: %v. Returning token claims: %v.", err, idTokenClaims)
			return idTokenClaims, nil
		}
		log.Debugf("Unable to fetch UserInfo claims: %v.", err)
		return nil, trace.Wrap(err, "unable to fetch UserInfo claims")
	}
	log.Debugf("UserInfo claims: %v.", userInfoClaims)

	// make sure that the subject in the userinfo claim matches the subject in
	// the id token otherwise there is the possibility of a token substitution attack.
	// see section 16.11 of the oidc spec for more details.
	var idsub string
	var uisub string
	var exists bool
	if idsub, exists, err = idTokenClaims.StringClaim("sub"); err != nil || !exists {
		log.Debugf("Unable to extract OIDC sub claim from ID token.")
		return nil, trace.Wrap(err, "unable to extract OIDC sub claim from ID token")
	}
	if uisub, exists, err = userInfoClaims.StringClaim("sub"); err != nil || !exists {
		log.Debugf("Unable to extract OIDC sub claim from UserInfo.")
		return nil, trace.Wrap(err, "unable to extract OIDC sub claim from UserInfo")
	}
	if idsub != uisub {
		log.Debugf("OIDC claim subjects don't match '%v' != '%v'.", idsub, uisub)
		return nil, trace.BadParameter("OIDC claim subjects in UserInfo does not match")
	}

	claims, err := mergeClaims(idTokenClaims, userInfoClaims)
	if err != nil {
		log.Debugf("Unable to merge OIDC claims: %v.", err)
		return nil, trace.Wrap(err, "unable to merge OIDC claims")
	}

	if isGoogleWorkspaceConnector(connector) {
		claims, err = addGoogleWorkspaceClaims(a.closeCtx, connector, claims)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return claims, nil
}

// getOAuthClient returns a Oauth2 client from the oidc.Client.  If the connector is set as a Ping provider sets the Client Secret Post auth method
func (a *Server) getOAuthClient(oidcClient *oidc.Client, connector types.OIDCConnector) (*oauth2.Client, error) {

	oac, err := oidcClient.OAuthClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	//If the default client secret basic is used the Ping OIDC
	// will throw an error of multiple client credentials.  Even if you set in Ping
	// to use Client Secret Post it will return to use client secret basic.
	// Issue https://github.com/gravitational/teleport/issues/8374
	if connector.GetProvider() == teleport.Ping {
		oac.SetAuthMethod(oauth2.AuthMethodClientSecretPost)
	}
	return oac, err
}

// validateACRValues validates that we get an appropriate response for acr values. By default
// we expect the same value we send, but this function also handles Identity Provider specific
// forms of validation.
func (a *Server) validateACRValues(acrValue string, identityProvider string, claims jose.Claims) error {
	switch identityProvider {
	case teleport.NetIQ:
		log.Debugf("Validating OIDC ACR values with '%v' rules.", identityProvider)

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
			log.Debugf("No OIDC ACR match found for '%v' in '%v'.", acrValue, tokenAcrValues)
			return trace.BadParameter("acr claim does not match")
		}
	default:
		log.Debugf("Validating OIDC ACR values with default rules.")

		claimValue, exists, err := claims.StringClaim("acr")
		if !exists {
			return trace.BadParameter("acr claim does not exist")
		}
		if err != nil {
			return trace.Wrap(err)
		}
		if claimValue != acrValue {
			log.Debugf("No OIDC ACR match found '%v' != '%v'.", acrValue, claimValue)
			return trace.BadParameter("acr claim does not match")
		}
	}

	return nil
}
