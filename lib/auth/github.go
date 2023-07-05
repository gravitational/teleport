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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/oauth2"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/loginrule"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// ErrGithubNoTeams results from a github user not belonging to any teams.
var ErrGithubNoTeams = trace.BadParameter("user does not belong to any teams configured in connector; the configuration may have typos.")

// GithubConverter is a thin wrapper around the ClientI interface that
// ensures GitHub auth connectors use the registered implementation.
type GithubConverter struct {
	ClientI
}

// WithGithubConnectorConversions takes a ClientI and returns one that
// ensures returned or passed [types.GithubConnector] interfaces
// use the registered implementation for the following methods:
//
//   - ClientI.GetGithubConnector
//   - ClientI.GetGithubConnectors
//   - ClientI.UpsertGithubConnector
//
// This is function is necessary so that the
// [github.com/gravitational/teleport/api] module does not import
// [github.com/gravitational/teleport/lib/services].
func WithGithubConnectorConversions(c ClientI) ClientI {
	return &GithubConverter{
		ClientI: c,
	}
}

func (g *GithubConverter) GetGithubConnector(ctx context.Context, name string, withSecrets bool) (types.GithubConnector, error) {
	connector, err := g.ClientI.GetGithubConnector(ctx, name, withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector, err = services.InitGithubConnector(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return connector, nil
}

func (g *GithubConverter) GetGithubConnectors(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error) {
	connectors, err := g.ClientI.GetGithubConnectors(ctx, withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for i, connector := range connectors {
		connectors[i], err = services.InitGithubConnector(connector)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return connectors, nil
}

func (g *GithubConverter) UpsertGithubConnector(ctx context.Context, connector types.GithubConnector) error {
	convertedConnector, err := services.ConvertGithubConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	return g.ClientI.UpsertGithubConnector(ctx, convertedConnector)
}

// CreateGithubAuthRequest creates a new request for Github OAuth2 flow
func (a *Server) CreateGithubAuthRequest(ctx context.Context, req types.GithubAuthRequest) (*types.GithubAuthRequest, error) {
	_, client, err := a.getGithubConnectorAndClient(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.StateToken, err = utils.CryptoRandomHex(TokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.RedirectURL = client.AuthCodeURL(req.StateToken, "", "")
	log.WithFields(logrus.Fields{trace.Component: "github"}).Debugf(
		"Redirect URL: %v.", req.RedirectURL)
	req.SetExpiry(a.GetClock().Now().UTC().Add(defaults.GithubAuthRequestTTL))
	err = a.Services.CreateGithubAuthRequest(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

// upsertGithubConnector creates or updates a Github connector.
func (a *Server) upsertGithubConnector(ctx context.Context, connector types.GithubConnector) error {
	if err := checkGithubOrgSSOSupport(ctx, connector, nil, a.githubOrgSSOCache, nil); err != nil {
		return trace.Wrap(err)
	}
	if err := a.UpsertGithubConnector(ctx, connector); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.GithubConnectorCreate{
		Metadata: apievents.Metadata{
			Type: events.GithubConnectorCreatedEvent,
			Code: events.GithubConnectorCreatedCode,
		},
		UserMetadata: ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: connector.GetName(),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit GitHub connector create event.")
	}

	return nil
}

// httpRequester allows a net/http.Client to be mocked for tests.
// TODO(capnspacehook): test without using this interface
type httpRequester interface {
	Do(req *http.Request) (*http.Response, error)
}

// checkGithubOrgSSOSupport returns an error if any of the Github
// organizations specified in this connector use external SSO.
// If userTeams is not nil, only organizations that are both specified
// in conn and in userTeams will be checked. If client is nil a
// net/http.Client will be used.
func checkGithubOrgSSOSupport(ctx context.Context, conn types.GithubConnector, userTeams []teamResponse, orgCache *utils.FnCache, client httpRequester) error {
	version := modules.GetModules().BuildType()
	if version == modules.BuildEnterprise {
		return nil
	}

	orgs := make(map[string]struct{})
	addOrg := func(org string) {
		if len(userTeams) != 0 {
			// Only check organizations that the user is a member of and
			// that are specified in this auth connector
			for _, team := range userTeams {
				if org == team.Org.Login {
					orgs[org] = struct{}{}
				}
			}
		} else {
			orgs[org] = struct{}{}
		}
	}

	// Check each organization only once
	// DELETE IN 12 (zmb3)
	for _, mapping := range conn.GetTeamsToLogins() {
		addOrg(mapping.Organization)
	}
	for _, mapping := range conn.GetTeamsToRoles() {
		addOrg(mapping.Organization)
	}

	if client == nil {
		var err error
		client, err = defaults.HTTPClient()
		if err != nil {
			return trace.Wrap(err)
		}
	}

	for org := range orgs {
		usesSSO, err := utils.FnCacheGet(ctx, orgCache, org, func(ctx context.Context) (bool, error) {
			return orgUsesExternalSSO(ctx, conn.GetEndpointURL(), org, client)
		})
		if err != nil {
			return trace.Wrap(err)
		}

		if usesSSO {
			return trace.AccessDenied(
				"GitHub organization %s uses external SSO, please purchase a Teleport Enterprise license if you want to authenticate with this organization",
				org,
			)
		}
	}

	return nil
}

// orgUsesExternalSSO returns true if the Github organization org
// uses external SSO.
func orgUsesExternalSSO(ctx context.Context, endpointURL, org string, client httpRequester) (bool, error) {
	// A Github organization will have a "sso" page reachable if it
	// supports external SSO. There doesn't seem to be any way to get this
	// information from the Github REST API without being an owner of the
	// Github organization, so check if this exists instead.
	ssoURL := fmt.Sprintf("%s/orgs/%s/sso", endpointURL, url.PathEscape(org))

	const retries = 3
	var resp *http.Response
	for i := 0; i < retries; i++ {
		var err error
		var urlErr *url.Error

		resp, err = makeHTTPGetReq(ctx, ssoURL, client)
		// Drain and close the body regardless of outcome.
		// Errors handled below.
		if resp != nil {
			io.Copy(io.Discard, resp.Body)
			if bodyErr := resp.Body.Close(); bodyErr != nil {
				logrus.WithError(bodyErr).Error("Error closing response body.")
			}
		}
		// Handle makeHTTPGetReq errors.
		if err == nil {
			break
		} else if errors.As(err, &urlErr) && urlErr.Timeout() {
			if i == retries-1 {
				// The connection timed out a couple of times in a row,
				// stop trying and return the error.
				return false, trace.ConnectionProblem(err, "Timed out trying to reach GitHub to check for organization external SSO.")
			}
			// Connection timed out, try to make the request again
			continue
		}
		// Unknown error, don't try making any more requests
		return false, trace.Wrap(err, "Unknown error trying to reach GitHub to check for organization external SSO")
	}

	// "sso" page exists, org uses external SSO
	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	// "sso" page does not exist, org does not use external SSO
	return false, nil
}

func makeHTTPGetReq(ctx context.Context, url string, client httpRequester) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, defaults.HTTPRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client.Do(req)
}

// deleteGithubConnector deletes a Github connector by name.
func (a *Server) deleteGithubConnector(ctx context.Context, connectorName string) error {
	if err := a.DeleteGithubConnector(ctx, connectorName); err != nil {
		return trace.Wrap(err)
	}

	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.GithubConnectorDelete{
		Metadata: apievents.Metadata{
			Type: events.GithubConnectorDeletedEvent,
			Code: events.GithubConnectorDeletedCode,
		},
		UserMetadata: ClientUserMetadata(ctx),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: connectorName,
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit GitHub connector delete event.")
	}

	return nil
}

// GithubAuthResponse represents Github auth callback validation response
type GithubAuthResponse struct {
	// Username is the name of authenticated user
	Username string `json:"username"`
	// Identity is the external identity
	Identity types.ExternalIdentity `json:"identity"`
	// Session is the created web session
	Session types.WebSession `json:"session,omitempty"`
	// Cert is the generated SSH client certificate
	Cert []byte `json:"cert,omitempty"`
	// TLSCert is PEM encoded TLS client certificate
	TLSCert []byte `json:"tls_cert,omitempty"`
	// Req is the original auth request
	Req GithubAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []types.CertAuthority `json:"host_signers"`
}

// GithubAuthRequest is an Github auth request that supports standard json marshaling
type GithubAuthRequest struct {
	// ConnectorID is the name of the connector to use.
	ConnectorID string `json:"connector_id"`
	// CSRFToken is used to protect against CSRF attacks.
	CSRFToken string `json:"csrf_token"`
	// PublicKey is an optional public key to sign in case of successful auth.
	PublicKey []byte `json:"public_key"`
	// CreateWebSession indicates that a user wants to generate a web session
	// after successful authentication.
	CreateWebSession bool `json:"create_web_session"`
	// ClientRedirectURL is the URL where client will be redirected after
	// successful auth.
	ClientRedirectURL string `json:"client_redirect_url"`
}

// GithubAuthRequestFromProto converts the types.GithubAuthRequest to GithubAuthRequest.
func GithubAuthRequestFromProto(req *types.GithubAuthRequest) GithubAuthRequest {
	return GithubAuthRequest{
		ConnectorID:       req.ConnectorID,
		PublicKey:         req.PublicKey,
		CSRFToken:         req.CSRFToken,
		CreateWebSession:  req.CreateWebSession,
		ClientRedirectURL: req.ClientRedirectURL,
	}
}

type githubManager interface {
	validateGithubAuthCallback(ctx context.Context, diagCtx *ssoDiagContext, q url.Values) (*GithubAuthResponse, error)
	newSSODiagContext(authKind string) *ssoDiagContext
}

// ValidateGithubAuthCallback validates Github auth callback redirect
func (a *Server) ValidateGithubAuthCallback(ctx context.Context, q url.Values) (*GithubAuthResponse, error) {
	return validateGithubAuthCallbackHelper(ctx, a, q, a.emitter)
}

func validateGithubAuthCallbackHelper(ctx context.Context, m githubManager, q url.Values, emitter apievents.Emitter) (*GithubAuthResponse, error) {
	event := &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
		},
		Method: events.LoginMethodGithub,
	}

	diagCtx := m.newSSODiagContext(types.KindGithub)

	auth, err := m.validateGithubAuthCallback(ctx, diagCtx, q)
	diagCtx.info.Error = trace.UserMessage(err)

	diagCtx.writeToBackend(ctx)

	claims := diagCtx.info.GithubClaims
	if claims != nil {
		attributes, err := apievents.EncodeMapStrings(claims.OrganizationToTeams)
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

		if err := emitter.EmitAuditEvent(ctx, event); err != nil {
			log.WithError(err).Warn("Failed to emit GitHub login failed event.")
		}
		return nil, trace.Wrap(err)
	}
	event.Code = events.UserSSOLoginCode
	if diagCtx.info.TestFlow {
		event.Code = events.UserSSOTestFlowLoginCode
	}
	event.Status.Success = true
	event.User = auth.Username

	if err := emitter.EmitAuditEvent(ctx, event); err != nil {
		log.WithError(err).Warn("Failed to emit GitHub login event.")
	}

	return auth, nil
}

func (a *Server) getGithubConnectorAndClient(ctx context.Context, request types.GithubAuthRequest) (types.GithubConnector, *oauth2.Client, error) {
	if request.SSOTestFlow {
		if request.ConnectorSpec == nil {
			return nil, nil, trace.BadParameter("ConnectorSpec cannot be nil when SSOTestFlow is true")
		}

		if request.ConnectorID == "" {
			return nil, nil, trace.BadParameter("ConnectorID cannot be empty")
		}

		// stateless test flow
		connector, err := services.NewGithubConnector(request.ConnectorID, *request.ConnectorSpec)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		// construct client directly.
		config := newGithubOAuth2Config(connector)
		client, err := oauth2.NewClient(http.DefaultClient, config)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		return connector, client, nil
	}

	// regular execution flow
	connector, err := a.GetGithubConnector(ctx, request.ConnectorID, true)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	connector, err = services.InitGithubConnector(connector)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	client, err := a.getGithubOAuth2Client(connector)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return connector, client, nil
}

func newGithubOAuth2Config(connector types.GithubConnector) oauth2.Config {
	return oauth2.Config{
		Credentials: oauth2.ClientCredentials{
			ID:     connector.GetClientID(),
			Secret: connector.GetClientSecret(),
		},
		RedirectURL: connector.GetRedirectURL(),
		Scope:       GithubScopes,
		AuthURL:     fmt.Sprintf("%s/%s", connector.GetEndpointURL(), GithubAuthPath),
		TokenURL:    fmt.Sprintf("%s/%s", connector.GetEndpointURL(), GithubTokenPath),
	}
}

func (a *Server) getGithubOAuth2Client(connector types.GithubConnector) (*oauth2.Client, error) {
	config := newGithubOAuth2Config(connector)

	a.lock.Lock()
	defer a.lock.Unlock()

	cachedClient, ok := a.githubClients[connector.GetName()]
	if ok && oauth2ConfigsEqual(cachedClient.config, config) {
		return cachedClient.client, nil
	}

	delete(a.githubClients, connector.GetName())
	client, err := oauth2.NewClient(http.DefaultClient, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	a.githubClients[connector.GetName()] = &githubClient{
		client: client,
		config: config,
	}
	return client, nil
}

// ValidateGithubAuthCallback validates Github auth callback redirect
func (a *Server) validateGithubAuthCallback(ctx context.Context, diagCtx *ssoDiagContext, q url.Values) (*GithubAuthResponse, error) {
	logger := log.WithFields(logrus.Fields{trace.Component: "github"})

	if errParam := q.Get("error"); errParam != "" {
		// try to find request so the error gets logged against it.
		state := q.Get("state")
		if state != "" {
			diagCtx.requestID = state
			req, err := a.Services.GetGithubAuthRequest(ctx, state)
			if err == nil {
				diagCtx.info.TestFlow = req.SSOTestFlow
			}
		}

		// optional parameter: error_description
		errDesc := q.Get("error_description")
		oauthErr := trace.OAuth2(oauth2.ErrorInvalidRequest, errParam, q)
		return nil, trace.WithUserMessage(oauthErr, "Github returned error: %v [%v]", errDesc, errParam)
	}

	code := q.Get("code")
	if code == "" {
		oauthErr := trace.OAuth2(oauth2.ErrorInvalidRequest, "code query param must be set", q)
		return nil, trace.WithUserMessage(oauthErr, "Invalid parameters received from GitHub.")
	}

	stateToken := q.Get("state")
	if stateToken == "" {
		oauthErr := trace.OAuth2(oauth2.ErrorInvalidRequest, "missing state query param", q)
		return nil, trace.WithUserMessage(oauthErr, "Invalid parameters received from GitHub.")
	}
	diagCtx.requestID = stateToken

	req, err := a.Services.GetGithubAuthRequest(ctx, stateToken)
	if err != nil {
		return nil, trace.Wrap(err, "Failed to get OIDC Auth Request.")
	}
	diagCtx.info.TestFlow = req.SSOTestFlow

	connector, client, err := a.getGithubConnectorAndClient(ctx, *req)
	if err != nil {
		return nil, trace.Wrap(err, "Failed to get GitHub connector and client.")
	}
	diagCtx.info.GithubTeamsToLogins = connector.GetTeamsToLogins()
	diagCtx.info.GithubTeamsToRoles = connector.GetTeamsToRoles()
	logger.Debugf("Connector %q teams to logins: %v, roles: %v", connector.GetName(), connector.GetTeamsToLogins(), connector.GetTeamsToRoles())

	// exchange the authorization code received by the callback for an access token
	token, err := client.RequestToken(oauth2.GrantTypeAuthCode, code)
	if err != nil {
		return nil, trace.Wrap(err, "Requesting GitHub OAuth2 token failed.")
	}

	diagCtx.info.GithubTokenInfo = &types.GithubTokenInfo{
		TokenType: token.TokenType,
		Expires:   int64(token.Expires),
		Scope:     token.Scope,
	}

	logger.Debugf("Obtained OAuth2 token: Type=%v Expires=%v Scope=%v.",
		token.TokenType, token.Expires, token.Scope)

	// Get the Github organizations the user is a member of so we don't
	// make unnecessary API requests
	endpointURL, err := url.Parse(connector.GetEndpointURL())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ghClient := &githubAPIClient{
		token:            token.AccessToken,
		authServer:       a,
		endpointHostname: endpointURL.Host,
	}
	userResp, err := ghClient.getUser()
	if err != nil {
		return nil, trace.Wrap(err, "failed to query GitHub user info")
	}
	teamsResp, err := ghClient.getTeams()
	if err != nil {
		return nil, trace.Wrap(err, "failed to query GitHub user teams")
	}
	log.Debugf("Retrieved %v teams for GitHub user %v.", len(teamsResp), userResp.Login)

	// If we are running Teleport OSS, ensure that the Github organization
	// the user is trying to authenticate with is not using external SSO.
	// SSO is a Teleport Enterprise feature and shouldn't be allowed in OSS.
	// This is checked when Github auth connectors get created or updated, but
	// check again here in case the organization enabled external SSO after
	// the auth connector was created.
	if err := checkGithubOrgSSOSupport(ctx, connector, teamsResp, a.githubOrgSSOCache, nil); err != nil {
		return nil, trace.Wrap(err)
	}

	// Github does not support OIDC so user claims have to be populated
	// by making requests to Github API using the access token
	claims, err := populateGithubClaims(userResp, teamsResp)
	if err != nil {
		return nil, trace.Wrap(err, "Failed to query GitHub API for user claims.")
	}
	diagCtx.info.GithubClaims = claims

	// Calculate (figure out name, roles, traits, session TTL) of user and
	// create the user in the backend.
	params, err := a.calculateGithubUser(ctx, connector, claims, req)
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

	user, err := a.createGithubUser(ctx, params, req.SSOTestFlow)
	if err != nil {
		return nil, trace.Wrap(err, "Failed to create user from provided parameters.")
	}

	// Auth was successful, return session, certificate, etc. to caller.
	auth := GithubAuthResponse{
		Req: GithubAuthRequestFromProto(req),
		Identity: types.ExternalIdentity{
			ConnectorID: params.connectorName,
			Username:    params.username,
		},
		Username: user.GetName(),
	}

	// In test flow skip signing and creating web sessions.
	if req.SSOTestFlow {
		diagCtx.info.Success = true
		return &auth, nil
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
		sshCert, tlsCert, err := a.createSessionCert(user, params.sessionTTL, req.PublicKey, req.Compatibility, req.RouteToCluster,
			req.KubernetesCluster, keys.AttestationStatementFromProto(req.AttestationStatement))
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

	return &auth, nil
}

// createUserParams is a set of parameters used to create a user for an
// external identity provider.
type createUserParams struct {
	// connectorName is the name of the connector for the identity provider.
	connectorName string

	// username is the Teleport user name .
	username string

	// kubeGroups is the list of Kubernetes groups this user belongs to.
	kubeGroups []string

	// kubeUsers is the list of Kubernetes users this user belongs to.
	kubeUsers []string

	// roles is the list of roles this user is assigned to.
	roles []string

	// traits is the list of traits for this user.
	traits map[string][]string

	// sessionTTL is how long this session will last.
	sessionTTL time.Duration
}

func (a *Server) calculateGithubUser(ctx context.Context, connector types.GithubConnector, claims *types.GithubClaims, request *types.GithubAuthRequest) (*createUserParams, error) {
	p := createUserParams{
		connectorName: connector.GetName(),
		username:      claims.Username,
	}

	// Calculate logins, kubegroups, roles, and traits.
	p.roles, p.kubeGroups, p.kubeUsers = connector.MapClaims(*claims)
	if len(p.roles) == 0 {
		return nil, trace.Wrap(ErrGithubNoTeams)
	}
	p.traits = map[string][]string{
		constants.TraitLogins:     {p.username},
		constants.TraitKubeGroups: p.kubeGroups,
		constants.TraitKubeUsers:  p.kubeUsers,
		teleport.TraitTeams:       claims.Teams,
	}

	evaluationInput := &loginrule.EvaluationInput{
		Traits: p.traits,
	}
	evaluationOutput, err := a.GetLoginRuleEvaluator().Evaluate(ctx, evaluationInput)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.traits = evaluationOutput.Traits

	// Kube groups and users are ultimately only set in the traits, not any
	// other property of the User. In case the login rules changed the relevant
	// traits values, reset the value on the user params for accurate
	// diagnostics.
	p.kubeGroups = p.traits[constants.TraitKubeGroups]
	p.kubeUsers = p.traits[constants.TraitKubeUsers]

	// Pick smaller for role: session TTL from role or requested TTL.
	roles, err := services.FetchRoles(p.roles, a, p.traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleTTL := roles.AdjustSessionTTL(apidefaults.MaxCertDuration)
	p.sessionTTL = utils.MinTTL(roleTTL, request.CertTTL)

	return &p, nil
}

func (a *Server) createGithubUser(ctx context.Context, p *createUserParams, dryRun bool) (types.User, error) {
	log.WithFields(logrus.Fields{trace.Component: "github"}).Debugf(
		"Generating dynamic GitHub identity %v/%v with roles: %v. Dry run: %v.",
		p.connectorName, p.username, p.roles, dryRun)

	expires := a.GetClock().Now().UTC().Add(p.sessionTTL)

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
			GithubIdentities: []types.ExternalIdentity{{
				ConnectorID: p.connectorName,
				Username:    p.username,
			}},
			CreatedBy: types.CreatedBy{
				User: types.UserRef{Name: teleport.UserSystem},
				Time: a.GetClock().Now().UTC(),
				Connector: &types.ConnectorRef{
					Type:     constants.Github,
					ID:       p.connectorName,
					Identity: p.username,
				},
			},
		},
	}

	if dryRun {
		return user, nil
	}

	existingUser, err := a.Services.GetUser(p.username, false)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if existingUser != nil {
		ref := user.GetCreatedBy().Connector
		if !ref.IsSameProvider(existingUser.GetCreatedBy().Connector) {
			return nil, trace.AlreadyExists("local user %q already exists and is not a GitHub user",
				existingUser.GetName())
		}

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

// populateGithubClaims builds a GithubClaims using queried
// user, organization and teams information.
func populateGithubClaims(user *userResponse, teams []teamResponse) (*types.GithubClaims, error) {
	orgToTeams := make(map[string][]string)
	teamList := make([]string, 0, len(teams))
	for _, team := range teams {
		orgToTeams[team.Org.Login] = append(
			orgToTeams[team.Org.Login], team.Slug)
		teamList = append(teamList, team.Name)
	}
	if len(orgToTeams) == 0 {
		return nil, trace.AccessDenied(
			"list of user teams is empty, did you grant access?")
	}
	claims := &types.GithubClaims{
		Username:            user.Login,
		OrganizationToTeams: orgToTeams,
		Teams:               teamList,
	}
	log.WithFields(logrus.Fields{trace.Component: "github"}).Debugf(
		"Claims: %#v.", claims)
	return claims, nil
}

// githubAPIClient is a tiny wrapper around some of Github APIs
type githubAPIClient struct {
	// token is the access token retrieved during OAuth2 flow
	token string
	// authServer points to the Auth Server.
	authServer *Server
	// endpointHostname is the Github hostname to connect to.
	endpointHostname string
}

// userResponse represents response from "user" API call
type userResponse struct {
	// Login is the username
	Login string `json:"login"`
}

// getEmails retrieves a list of emails for authenticated user
func (c *githubAPIClient) getUser() (*userResponse, error) {
	// Ignore pagination links, we should never get more than a single user here.
	bytes, _, err := c.get("user")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var user userResponse
	err = json.Unmarshal(bytes, &user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &user, nil
}

// teamResponse represents a single team entry in the "teams" API response
type teamResponse struct {
	// Name is the team name
	Name string `json:"name"`
	// Slug is the team ID
	Slug string `json:"slug"`
	// Org describes the organization this team is a part of
	Org orgResponse `json:"organization"`
}

// orgResponse represents a Github organization
type orgResponse struct {
	// Login is the organization ID
	Login string `json:"login"`
}

// getTeams retrieves a list of teams authenticated user belongs to.
func (c *githubAPIClient) getTeams() ([]teamResponse, error) {
	var result []teamResponse

	bytes, nextPage, err := c.get("user/teams")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Extract the first page of results and append them to the full result set.
	var teams []teamResponse
	err = json.Unmarshal(bytes, &teams)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	result = append(result, teams...)

	// If the response returned a next page link, continue following the next
	// page links until all teams have been retrieved.
	var count int
	for nextPage != "" {
		// To prevent this from looping forever, don't fetch more than a set number
		// of pages, print an error when it does happen, and return the results up
		// to that point.
		if count > MaxPages {
			warningMessage := "Truncating list of teams used to populate claims: " +
				"hit maximum number pages that can be fetched from GitHub."

			// Print warning to Teleport logs as well as the Audit Log.
			log.Warnf(warningMessage)
			if err := c.authServer.emitter.EmitAuditEvent(c.authServer.closeCtx, &apievents.UserLogin{
				Metadata: apievents.Metadata{
					Type: events.UserLoginEvent,
					Code: events.UserSSOLoginFailureCode,
				},
				Method: events.LoginMethodGithub,
				Status: apievents.Status{
					Success: false,
					Error:   warningMessage,
				},
			}); err != nil {
				log.WithError(err).Warn("Failed to emit GitHub login failure event.")
			}
			return result, nil
		}

		u, err := url.Parse(nextPage)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		bytes, nextPage, err = c.get(u.RequestURI())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		err = json.Unmarshal(bytes, &teams)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Append this page of teams to full result set.
		result = append(result, teams...)

		count = count + 1
	}

	return result, nil
}

// get makes a GET request to the provided URL using the client's token for auth
func (c *githubAPIClient) get(page string) ([]byte, string, error) {
	request, err := http.NewRequest("GET", formatGithubURL(c.endpointHostname, page), nil)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	request.Header.Set("Authorization", fmt.Sprintf("token %v", c.token))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer response.Body.Close()
	bytes, err := utils.ReadAtMost(response.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	if response.StatusCode != 200 {
		return nil, "", trace.AccessDenied("bad response: %v %v",
			response.StatusCode, string(bytes))
	}

	// Parse web links header to extract any pagination links. This is used to
	// return the next link which can be used in a loop to pull back all data.
	wls := utils.ParseWebLinks(response)

	return bytes, wls.NextPage, nil
}

// formatGithubURL is a helper for formatting github api request URLs.
func formatGithubURL(host string, path string) string {
	return fmt.Sprintf("https://api.%s/%s", host, strings.TrimPrefix(path, "/"))
}

const (
	// GithubAuthPath is the GitHub authorization endpoint
	GithubAuthPath = "login/oauth/authorize"

	// GithubTokenPath is the GitHub token exchange endpoint
	GithubTokenPath = "login/oauth/access_token"

	// MaxPages is the maximum number of pagination links that will be followed.
	MaxPages = 99
)

// GithubScopes is a list of scopes requested during OAuth2 flow
var GithubScopes = []string{
	// read:org grants read-only access to user's team memberships
	"read:org",
}
