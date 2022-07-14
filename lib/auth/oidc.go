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
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/go-cmp/cmp"
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

// getOIDCConnectorAndClient returns the associated oidc connector
// and client for the given oidc auth request.
func (a *Server) getOIDCConnectorAndClient(ctx context.Context, request types.OIDCAuthRequest) (types.OIDCConnector, *oidc.Client, error) {
	// stateless test flow
	if request.SSOTestFlow {
		if request.ConnectorSpec == nil {
			return nil, nil, trace.BadParameter("ConnectorSpec cannot be nil when SSOTestFlow is true")
		}

		if request.ConnectorID == "" {
			return nil, nil, trace.BadParameter("ConnectorID cannot be empty")
		}

		connector, err := types.NewOIDCConnector(request.ConnectorID, *request.ConnectorSpec)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		// we don't want to cache the client. construct it directly.
		client, err := newOIDCClient(ctx, connector, request.ProxyAddress)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if err := client.waitFirstSync(defaults.WebHeadersTimeout); err != nil {
			return nil, nil, trace.Wrap(err)
		}

		// close this request-scoped oidc client after 10 minutes
		go func() {
			ticker := a.GetClock().NewTicker(defaults.OIDCAuthRequestTTL)
			defer ticker.Stop()
			select {
			case <-ticker.Chan():
				client.syncCancel()
			case <-client.syncCtx.Done():
			}
		}()

		return connector, client.client, nil
	}

	// regular execution flow
	connector, err := a.Identity.GetOIDCConnector(ctx, request.ConnectorID, true)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	client, err := a.getCachedOIDCClient(ctx, connector, request.ProxyAddress)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Wait for the client to successfully sync after getting it from the cache.
	// We do this after caching the client to prevent locking the server during
	// the initial sync period.
	if err := client.waitFirstSync(defaults.WebHeadersTimeout); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return connector, client.client, nil
}

// getCachedOIDCClient gets a cached oidc client for
// the given OIDC connector and redirectURL preference.
func (a *Server) getCachedOIDCClient(ctx context.Context, conn types.OIDCConnector, proxyAddr string) (*oidcClient, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	// Each connector and proxy combination has a distinct client,
	// so we use a composite key to capture all combinations.
	clientMapKey := conn.GetName() + "_" + proxyAddr

	cachedClient, ok := a.oidcClients[clientMapKey]
	if ok {
		if !cachedClient.needsRefresh(conn) && cachedClient.syncCtx.Err() == nil {
			return cachedClient, nil
		}
		// Cached client needs to be refreshed or is no longer syncing.
		cachedClient.syncCancel()
		delete(a.oidcClients, clientMapKey)
	}

	// Create a new oidc client and add it to the cache.
	client, err := newOIDCClient(ctx, conn, proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a.oidcClients[clientMapKey] = client
	return client, nil
}

func newOIDCClient(ctx context.Context, conn types.OIDCConnector, proxyAddr string) (*oidcClient, error) {
	redirectURL, err := services.GetRedirectURL(conn, proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := oidcConfig(conn, redirectURL)
	client, err := oidc.NewClient(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oidcClient := &oidcClient{client: client, connector: conn, firstSync: make(chan struct{})}
	oidcClient.startSync(ctx)
	return oidcClient, nil
}

func oidcConfig(conn types.OIDCConnector, redirectURL string) oidc.ClientConfig {
	return oidc.ClientConfig{
		RedirectURL: redirectURL,
		Credentials: oidc.ClientCredentials{
			ID:     conn.GetClientID(),
			Secret: conn.GetClientSecret(),
		},
		// open id notifies provider that we are using OIDC scopes
		Scope: apiutils.Deduplicate(append([]string{"openid", "email"}, conn.GetScope()...)),
	}
}

// needsRefresh returns whether the client's connector and the
// given connector have the same values for fields relevant to
// generating and syncing an oidc.Client.
func (c *oidcClient) needsRefresh(conn types.OIDCConnector) bool {
	return !cmp.Equal(conn.GetRedirectURLs(), c.connector.GetRedirectURLs()) ||
		conn.GetClientID() != c.connector.GetClientID() ||
		conn.GetClientSecret() != c.connector.GetClientSecret() ||
		!cmp.Equal(conn.GetScope(), c.connector.GetScope()) ||
		conn.GetIssuerURL() != c.connector.GetIssuerURL()
}

// startSync starts a goroutine to sync the client with its provider
// config until the given ctx is closed or the sync is canceled.
func (c *oidcClient) startSync(ctx context.Context) {
	// SyncProviderConfig doesn't take a context for cancellation, instead it
	// returns a channel that has to be closed to stop the sync. To ensure that the
	// sync is eventually stopped, we "wrap" the stop channel with a cancel context.
	c.syncCtx, c.syncCancel = context.WithCancel(ctx)
	go func() {
		stop := c.client.SyncProviderConfig(c.connector.GetIssuerURL())
		close(c.firstSync)
		<-c.syncCtx.Done()
		close(stop)
	}()
}

// waitFirstSync waits for the client to start syncing successfully, or
// returns an error if syncing fails or fails to succeed within 10 seconds.
// This prevents waiting on clients with faulty provider configurations.
func (c *oidcClient) waitFirstSync(timeout time.Duration) error {
	timeoutTimer := time.NewTimer(timeout)

	select {
	case <-c.firstSync:
	case <-c.syncCtx.Done():
	case <-timeoutTimer.C:
		// cancel sync so that it gets removed from the cache
		c.syncCancel()
		return trace.ConnectionProblem(nil, "timed out syncing oidc connector %v, ensure URL %q is valid and accessible and check configuration",
			c.connector.GetName(), c.connector.GetIssuerURL())
	}

	// stop and flush timer
	if !timeoutTimer.Stop() {
		<-timeoutTimer.C
	}

	// return the syncing error if there is one
	return trace.Wrap(c.syncCtx.Err())
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

func (a *Server) CreateOIDCAuthRequest(ctx context.Context, req types.OIDCAuthRequest) (*types.OIDCAuthRequest, error) {
	// ensure prompt removal of OIDC client in test flows. does nothing in regular flows.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	connector, client, err := a.getOIDCConnectorAndClient(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	oauthClient, err := client.OAuthClient()
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

	err = a.Identity.CreateOIDCAuthRequest(ctx, req, defaults.OIDCAuthRequestTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

// ValidateOIDCAuthCallback is called by the proxy to check OIDC query parameters
// returned by OIDC Provider, if everything checks out, auth server
// will respond with OIDCAuthResponse, otherwise it will return error
func (a *Server) ValidateOIDCAuthCallback(ctx context.Context, q url.Values) (*OIDCAuthResponse, error) {
	event := &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
		},
		Method: events.LoginMethodOIDC,
	}

	diagCtx := a.newSSODiagContext(types.KindOIDC)

	auth, err := a.validateOIDCAuthCallback(ctx, diagCtx, q)
	diagCtx.info.Error = trace.UserMessage(err)

	diagCtx.writeToBackend(ctx)

	claims := diagCtx.info.OIDCClaims
	if claims != nil {
		attributes, err := apievents.EncodeMap(claims)
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
			log.WithError(err).Warn("Failed to emit OIDC login failed event.")
		}

		return nil, trace.Wrap(err)
	}

	event.Code = events.UserSSOLoginCode
	if diagCtx.info.TestFlow {
		event.Code = events.UserSSOTestFlowLoginCode
	}
	event.User = auth.Username
	event.Status.Success = true

	if err := a.emitter.EmitAuditEvent(a.closeCtx, event); err != nil {
		log.WithError(err).Warn("Failed to emit OIDC login event.")
	}

	return auth, nil
}

func checkEmailVerifiedClaim(claims jose.Claims) error {
	claimName := "email_verified"
	unverifiedErr := trace.AccessDenied("email not verified by OIDC provider")

	emailVerified, hasEmailVerifiedClaim, _ := claims.StringClaim(claimName)
	if hasEmailVerifiedClaim && emailVerified == "false" {
		return unverifiedErr
	}

	data, ok := claims[claimName]
	if !ok {
		return nil
	}

	emailVerifiedBool, ok := data.(bool)
	if !ok {
		return trace.BadParameter("unable to parse oidc claim: %q, must be a string or bool", claimName)
	}

	if !emailVerifiedBool {
		return unverifiedErr
	}

	return nil
}

func (a *Server) validateOIDCAuthCallback(ctx context.Context, diagCtx *ssoDiagContext, q url.Values) (*OIDCAuthResponse, error) {
	if errParam := q.Get("error"); errParam != "" {
		// try to find request so the error gets logged against it.
		state := q.Get("state")
		if state != "" {
			diagCtx.requestID = state
			req, err := a.Identity.GetOIDCAuthRequest(ctx, state)
			if err == nil {
				diagCtx.info.TestFlow = req.SSOTestFlow
			}
		}

		// optional parameter: error_description
		errDesc := q.Get("error_description")
		return nil, trace.OAuth2(oauth2.ErrorInvalidRequest, errParam, q).AddUserMessage("OIDC provider returned error: %v [%v]", errDesc, errParam)
	}

	code := q.Get("code")
	if code == "" {
		return nil, trace.OAuth2(
			oauth2.ErrorInvalidRequest, "code query param must be set", q).AddUserMessage("Invalid parameters received from OIDC provider.")
	}

	stateToken := q.Get("state")
	if stateToken == "" {
		return nil, trace.OAuth2(
			oauth2.ErrorInvalidRequest, "missing state query param", q).AddUserMessage("Invalid parameters received from OIDC provider.")
	}
	diagCtx.requestID = stateToken

	req, err := a.Identity.GetOIDCAuthRequest(ctx, stateToken)
	if err != nil {
		return nil, trace.Wrap(err, "Failed to get OIDC Auth Request.")
	}
	diagCtx.info.TestFlow = req.SSOTestFlow

	// ensure prompt removal of OIDC client in test flows. does nothing in regular flows.
	ctxC, cancel := context.WithCancel(ctx)
	defer cancel()

	connector, client, err := a.getOIDCConnectorAndClient(ctxC, *req)
	if err != nil {
		return nil, trace.Wrap(err, "Failed to get OIDC connector and client.")
	}

	// extract claims from both the id token and the userinfo endpoint and merge them
	claims, err := a.getClaims(client, connector, code)
	if err != nil {
		// different error message for Google Workspace as likely cause is different.
		if isGoogleWorkspaceConnector(connector) {
			return nil, trace.Wrap(err, "Failed to extract OIDC claims. Check your Google Workspace plan and enabled APIs. See: https://goteleport.com/docs/enterprise/sso/google-workspace/#ensure-your-google-workspace-plan-is-correct")
		}

		return nil, trace.Wrap(err, "Failed to extract OIDC claims. This may indicate need to set 'provider' flag in connector definition. See: https://goteleport.com/docs/enterprise/sso/#provider-specific-workarounds")
	}
	diagCtx.info.OIDCClaims = types.OIDCClaims(claims)

	log.Debugf("OIDC claims: %v.", claims)
	if err := checkEmailVerifiedClaim(claims); err != nil {
		return nil, trace.Wrap(err, "OIDC provider did not verify email.")
	}

	// if we are sending acr values, make sure we also validate them
	acrValue := connector.GetACR()
	if acrValue != "" {
		err := a.validateACRValues(acrValue, connector.GetProvider(), claims)
		if err != nil {
			return nil, trace.Wrap(err, "OIDC ACR validation failure.")
		}
		log.Debugf("OIDC ACR values %q successfully validated.", acrValue)
	}

	ident, err := oidc.IdentityFromClaims(claims)
	if err != nil {
		return nil, trace.OAuth2(
			oauth2.ErrorUnsupportedResponseType, "unable to convert claims to identity", q)
	}
	diagCtx.info.OIDCIdentity = &types.OIDCIdentity{
		ID:        ident.ID,
		Name:      ident.Name,
		Email:     ident.Email,
		ExpiresAt: ident.ExpiresAt,
	}
	log.Debugf("OIDC user %q expires at: %v.", ident.Email, ident.ExpiresAt)

	if len(connector.GetClaimsToRoles()) == 0 {
		return nil, trace.BadParameter("no claims to roles mapping, check connector documentation").
			AddUserMessage("Claims-to-roles mapping is empty, SSO user will never have any roles.")
	}
	log.Debugf("Applying %v OIDC claims to roles mappings.", len(connector.GetClaimsToRoles()))
	diagCtx.info.OIDCClaimsToRoles = connector.GetClaimsToRoles()

	// Calculate (figure out name, roles, traits, session TTL) of user and
	// create the user in the backend.
	params, err := a.calculateOIDCUser(diagCtx, connector, claims, ident, req)
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

	user, err := a.createOIDCUser(params, req.SSOTestFlow)
	if err != nil {
		return nil, trace.Wrap(err, "Failed to create user from provided parameters.")
	}

	// Auth was successful, return session, certificate, etc. to caller.
	auth := &OIDCAuthResponse{
		Req: *req,
		Identity: types.ExternalIdentity{
			ConnectorID: params.connectorName,
			Username:    params.username,
		},
		Username: user.GetName(),
	}

	// In test flow skip signing and creating web sessions.
	if req.SSOTestFlow {
		diagCtx.info.Success = true
		return auth, nil
	}

	if !req.CheckUser {
		return auth, nil
	}

	// If the request is coming from a browser, create a web session.
	if req.CreateWebSession {
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
	if len(req.PublicKey) != 0 {
		sshCert, tlsCert, err := a.createSessionCert(user, params.sessionTTL, req.PublicKey, req.Compatibility, req.RouteToCluster, req.KubernetesCluster)
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

	return auth, nil
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
	Req types.OIDCAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []types.CertAuthority `json:"host_signers"`
}

func (a *Server) calculateOIDCUser(diagCtx *ssoDiagContext, connector types.OIDCConnector, claims jose.Claims, ident *oidc.Identity, request *types.OIDCAuthRequest) (*createUserParams, error) {
	var err error

	p := createUserParams{
		connectorName: connector.GetName(),
		username:      ident.Email,
	}

	p.traits = services.OIDCClaimsToTraits(claims)

	diagCtx.info.OIDCTraitsFromClaims = p.traits
	diagCtx.info.OIDCConnectorTraitMapping = connector.GetTraitMappings()

	var warnings []string
	warnings, p.roles = services.TraitsToRoles(connector.GetTraitMappings(), p.traits)
	if len(p.roles) == 0 {
		if len(warnings) != 0 {
			log.WithField("connector", connector).Warnf("No roles mapped from claims. Warnings: %q", warnings)
			diagCtx.info.OIDCClaimsToRolesWarnings = &types.SSOWarnings{
				Message:  "No roles mapped for the user",
				Warnings: warnings,
			}
		} else {
			log.WithField("connector", connector).Warnf("No roles mapped from claims.")
			diagCtx.info.OIDCClaimsToRolesWarnings = &types.SSOWarnings{
				Message: "No roles mapped for the user. The mappings may contain typos.",
			}
		}
		return nil, trace.AccessDenied("No roles mapped from claims. The mappings may contain typos.")
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

func (a *Server) createOIDCUser(p *createUserParams, dryRun bool) (types.User, error) {
	expires := a.GetClock().Now().UTC().Add(p.sessionTTL)

	log.Debugf("Generating dynamic OIDC identity %v/%v with roles: %v. Dry run: %v.", p.connectorName, p.username, p.roles, dryRun)
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
		body, err := io.ReadAll(resp.Body)
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
	return a.getClaimsFun(a.closeCtx, oidcClient, connector, code)
}

// getClaims implements Server.getClaims, but allows that code path to be overridden for testing.
func getClaims(closeCtx context.Context, oidcClient *oidc.Client, connector types.OIDCConnector, code string) (jose.Claims, error) {
	oac, err := getOAuthClient(oidcClient, connector)

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
		claims, err = addGoogleWorkspaceClaims(closeCtx, connector, claims)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return claims, nil
}

// getOAuthClient returns a Oauth2 client from the oidc.Client.  If the connector is set as a Ping provider sets the Client Secret Post auth method
func getOAuthClient(oidcClient *oidc.Client, connector types.OIDCConnector) (*oauth2.Client, error) {
	oac, err := oidcClient.OAuthClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// For OIDC, Ping and Okta will throw an error when the
	// default client secret basic method is used.
	// See: https://github.com/gravitational/teleport/issues/8374
	switch connector.GetProvider() {
	case teleport.Ping, teleport.Okta:
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
