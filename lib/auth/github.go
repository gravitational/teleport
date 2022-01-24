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
	"net/http"
	"net/url"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/go-oidc/oauth2"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// CreateGithubAuthRequest creates a new request for Github OAuth2 flow
func (a *Server) CreateGithubAuthRequest(req services.GithubAuthRequest) (*services.GithubAuthRequest, error) {
	ctx := context.TODO()
	connector, err := a.Identity.GetGithubConnector(ctx, req.ConnectorID, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err := a.getGithubOAuth2Client(connector)
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
	err = a.Identity.CreateGithubAuthRequest(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

// upsertGithubConnector creates or updates a Github connector.
func (a *Server) upsertGithubConnector(ctx context.Context, connector types.GithubConnector) error {
	if err := a.Identity.UpsertGithubConnector(ctx, connector); err != nil {
		return trace.Wrap(err)
	}
	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.GithubConnectorCreate{
		Metadata: apievents.Metadata{
			Type: events.GithubConnectorCreatedEvent,
			Code: events.GithubConnectorCreatedCode,
		},
		UserMetadata: apievents.UserMetadata{
			User:         ClientUsername(ctx),
			Impersonator: ClientImpersonator(ctx),
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Name: connector.GetName(),
		},
	}); err != nil {
		log.WithError(err).Warn("Failed to emit GitHub connector create event.")
	}

	return nil
}

// deleteGithubConnector deletes a Github connector by name.
func (a *Server) deleteGithubConnector(ctx context.Context, connectorName string) error {
	if err := a.Identity.DeleteGithubConnector(ctx, connectorName); err != nil {
		return trace.Wrap(err)
	}

	if err := a.emitter.EmitAuditEvent(a.closeCtx, &apievents.GithubConnectorDelete{
		Metadata: apievents.Metadata{
			Type: events.GithubConnectorDeletedEvent,
			Code: events.GithubConnectorDeletedCode,
		},
		UserMetadata: apievents.UserMetadata{
			User:         ClientUsername(ctx),
			Impersonator: ClientImpersonator(ctx),
		},
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
	Req services.GithubAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []types.CertAuthority `json:"host_signers"`
}

type githubManager interface {
	validateGithubAuthCallback(q url.Values) (*githubAuthResponse, error)
}

// ValidateGithubAuthCallback validates Github auth callback redirect
func (a *Server) ValidateGithubAuthCallback(q url.Values) (*GithubAuthResponse, error) {
	return validateGithubAuthCallbackHelper(a.closeCtx, a, q, a.emitter)
}

func validateGithubAuthCallbackHelper(ctx context.Context, m githubManager, q url.Values, emitter apievents.Emitter) (*GithubAuthResponse, error) {
	event := &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
		},
		Method: events.LoginMethodGithub,
	}

	re, err := m.validateGithubAuthCallback(q)
	if re != nil && re.claims != nil {
		attributes, err := apievents.EncodeMapStrings(re.claims)
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

		if err := emitter.EmitAuditEvent(ctx, event); err != nil {
			log.WithError(err).Warn("Failed to emit Github login failed event.")
		}
		return nil, trace.Wrap(err)
	}
	event.Code = events.UserSSOLoginCode
	event.Status.Success = true
	event.User = re.auth.Username

	if err := emitter.EmitAuditEvent(ctx, event); err != nil {
		log.WithError(err).Warn("Failed to emit Github login event.")
	}

	return &re.auth, nil
}

type githubAuthResponse struct {
	auth   GithubAuthResponse
	claims map[string][]string
}

// ValidateGithubAuthCallback validates Github auth callback redirect
func (a *Server) validateGithubAuthCallback(q url.Values) (*githubAuthResponse, error) {
	ctx := context.TODO()
	logger := log.WithFields(logrus.Fields{trace.Component: "github"})
	error := q.Get("error")
	if error != "" {
		return nil, trace.OAuth2(oauth2.ErrorInvalidRequest, error, q)
	}
	code := q.Get("code")
	if code == "" {
		return nil, trace.OAuth2(oauth2.ErrorInvalidRequest,
			"code query param must be set", q)
	}
	stateToken := q.Get("state")
	if stateToken == "" {
		return nil, trace.OAuth2(oauth2.ErrorInvalidRequest,
			"missing state query param", q)
	}
	req, err := a.Identity.GetGithubAuthRequest(stateToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector, err := a.Identity.GetGithubConnector(ctx, req.ConnectorID, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(connector.GetTeamsToLogins()) == 0 {
		logger.Warnf("Github connector %q has empty teams_to_logins mapping, cannot populate claims.",
			connector.GetName())
		return nil, trace.BadParameter(
			"connector %q has empty teams_to_logins mapping", connector.GetName())
	}
	client, err := a.getGithubOAuth2Client(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// exchange the authorization code received by the callback for an access token
	token, err := client.RequestToken(oauth2.GrantTypeAuthCode, code)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger.Debugf("Obtained OAuth2 token: Type=%v Expires=%v Scope=%v.",
		token.TokenType, token.Expires, token.Scope)
	// Github does not support OIDC so user claims have to be populated
	// by making requests to Github API using the access token
	claims, err := populateGithubClaims(&githubAPIClient{
		token:      token.AccessToken,
		authServer: a,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	re := &githubAuthResponse{
		claims: claims.OrganizationToTeams,
	}

	// Calculate (figure out name, roles, traits, session TTL) of user and
	// create the user in the backend.
	params, err := a.calculateGithubUser(connector, claims, req)
	if err != nil {
		return re, trace.Wrap(err)
	}
	user, err := a.createGithubUser(params)
	if err != nil {
		return re, trace.Wrap(err)
	}

	// Auth was successful, return session, certificate, etc. to caller.
	re.auth = GithubAuthResponse{
		Req: *req,
		Identity: types.ExternalIdentity{
			ConnectorID: params.connectorName,
			Username:    params.username,
		},
		Username: user.GetName(),
	}
	re.auth.Username = user.GetName()

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

		clusterName, err := a.GetClusterName()
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

// createUserParams is a set of parameters used to create a user for an
// external identity provider.
type createUserParams struct {
	// connectorName is the name of the connector for the identity provider.
	connectorName string

	// username is the Teleport user name .
	username string

	// logins is the list of *nix logins.
	logins []string

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

func (a *Server) calculateGithubUser(connector types.GithubConnector, claims *types.GithubClaims, request *services.GithubAuthRequest) (*createUserParams, error) {
	p := createUserParams{
		connectorName: connector.GetName(),
		username:      claims.Username,
	}

	// Calculate logins, kubegroups, roles, and traits.
	p.logins, p.kubeGroups, p.kubeUsers = connector.MapClaims(*claims)
	if len(p.logins) == 0 {
		return nil, trace.BadParameter(
			"user %q does not belong to any teams configured in %q connector",
			claims.Username, connector.GetName())
	}
	p.roles = p.logins
	p.traits = map[string][]string{
		teleport.TraitLogins:     []string{p.username},
		teleport.TraitKubeGroups: p.kubeGroups,
		teleport.TraitKubeUsers:  p.kubeUsers,
		teleport.TraitTeams:      claims.Teams,
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

func (a *Server) createGithubUser(p *createUserParams) (types.User, error) {

	log.WithFields(logrus.Fields{trace.Component: "github"}).Debugf(
		"Generating dynamic identity %v/%v with logins: %v.",
		p.connectorName, p.username, p.logins)

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

	existingUser, err := a.GetUser(p.username, false)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	ctx := context.TODO()

	if existingUser != nil {
		ref := user.GetCreatedBy().Connector
		if !ref.IsSameProvider(existingUser.GetCreatedBy().Connector) {
			return nil, trace.AlreadyExists("local user %q already exists and is not a Github user",
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

// populateGithubClaims retrieves information about user and its team
// memberships by calling Github API using the access token
func populateGithubClaims(client githubAPIClientI) (*types.GithubClaims, error) {
	// find out the username
	user, err := client.getUser()
	if err != nil {
		return nil, trace.Wrap(err, "failed to query Github user info")
	}
	// build team memberships
	teams, err := client.getTeams()
	if err != nil {
		return nil, trace.Wrap(err, "failed to query Github user teams")
	}
	log.Debugf("Retrieved %v teams for GitHub user %v.", len(teams), user.Login)

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

func (a *Server) getGithubOAuth2Client(connector types.GithubConnector) (*oauth2.Client, error) {
	a.lock.Lock()
	defer a.lock.Unlock()
	config := oauth2.Config{
		Credentials: oauth2.ClientCredentials{
			ID:     connector.GetClientID(),
			Secret: connector.GetClientSecret(),
		},
		RedirectURL: connector.GetRedirectURL(),
		Scope:       GithubScopes,
		AuthURL:     GithubAuthURL,
		TokenURL:    GithubTokenURL,
	}
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

// githubAPIClientI defines an interface for Github API wrapper
// so it can be substituted in tests
type githubAPIClientI interface {
	// getUser returns user information
	getUser() (*userResponse, error)
	// getTeams returns a list of user team memberships
	getTeams() ([]teamResponse, error)
}

// githubAPIClient is a tiny wrapper around some of Github APIs
type githubAPIClient struct {
	// token is the access token retrieved during OAuth2 flow
	token string
	// authServer points to the Auth Server.
	authServer *Server
}

// userResponse represents response from "user" API call
type userResponse struct {
	// Login is the username
	Login string `json:"login"`
}

// getEmails retrieves a list of emails for authenticated user
func (c *githubAPIClient) getUser() (*userResponse, error) {
	// Ignore pagination links, we should never get more than a single user here.
	bytes, _, err := c.get("/user")
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

	bytes, nextPage, err := c.get("/user/teams")
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
func (c *githubAPIClient) get(url string) ([]byte, string, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%v%v", GithubAPIURL, url), nil)
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

const (
	// GithubAuthURL is the Github authorization endpoint
	GithubAuthURL = "https://github.com/login/oauth/authorize"

	// GithubTokenURL is the Github token exchange endpoint
	GithubTokenURL = "https://github.com/login/oauth/access_token"

	// GithubAPIURL is the Github base API URL
	GithubAPIURL = "https://api.github.com"

	// MaxPages is the maximum number of pagination links that will be followed.
	MaxPages = 99
)

var (
	// GithubScopes is a list of scopes requested during OAuth2 flow
	GithubScopes = []string{
		// read:org grants read-only access to user's team memberships
		"read:org",
	}
)
