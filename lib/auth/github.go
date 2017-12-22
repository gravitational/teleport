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
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/go-oidc/oauth2"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// CreateGithubAuthRequest creates a new request for Github OAuth2 flow
func (s *AuthServer) CreateGithubAuthRequest(req services.GithubAuthRequest) (*services.GithubAuthRequest, error) {
	connector, err := s.Identity.GetGithubConnector(req.ConnectorID, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err := s.getGithubOAuth2Client(connector)
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
	err = s.Identity.CreateGithubAuthRequest(req, defaults.GithubAuthRequestTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

// GithubAuthResponse represents Github auth callback validation response
type GithubAuthResponse struct {
	// Username is the name of authenticated user
	Username string `json:"username"`
	// Identity is the external identity
	Identity services.ExternalIdentity `json:"identity"`
	// Session is the created web session
	Session services.WebSession `json:"session,omitempty"`
	// Cert is the generated SSH client certificate
	Cert []byte `json:"cert,omitempty"`
	// TLSCert is PEM encoded TLS client certificate
	TLSCert []byte `json:"tls_cert,omitempty"`
	// Req is the original auth request
	Req services.GithubAuthRequest `json:"req"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy, used in console login
	HostSigners []services.CertAuthority `json:"host_signers"`
}

// ValidateGithubAuthCallback validates Github auth callback redirect
func (s *AuthServer) ValidateGithubAuthCallback(q url.Values) (*GithubAuthResponse, error) {
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
	req, err := s.Identity.GetGithubAuthRequest(stateToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector, err := s.Identity.GetGithubConnector(req.ConnectorID, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(connector.GetTeamsToLogins()) == 0 {
		logger.Warnf("Github connector %q has empty teams_to_logins mapping, cannot populate claims.",
			connector.GetName())
		return nil, trace.BadParameter(
			"connector %q has empty teams_to_logins mapping", connector.GetName())
	}
	client, err := s.getGithubOAuth2Client(connector)
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
	claims, err := populateGithubClaims(&githubAPIClient{token: token.AccessToken})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = s.createGithubUser(connector, *claims)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response := &GithubAuthResponse{
		Identity: services.ExternalIdentity{
			ConnectorID: connector.GetName(),
			Username:    claims.Username,
		},
		Req: *req,
	}
	user, err := s.Identity.GetUserByGithubIdentity(response.Identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response.Username = user.GetName()
	roles, err := services.FetchRoles(user.GetRoles(), s.Access, user.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if req.CreateWebSession {
		session, err := s.NewWebSession(user.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		sessionTTL := roles.AdjustSessionTTL(defaults.OAuth2TTL)
		bearerTTL := utils.MinTTL(BearerTokenTTL, sessionTTL)
		session.SetExpiryTime(s.clock.Now().UTC().Add(sessionTTL))
		session.SetBearerTokenExpiryTime(s.clock.Now().UTC().Add(bearerTTL))
		err = s.UpsertWebSession(user.GetName(), session)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.Session = session
	}
	if len(req.PublicKey) != 0 {
		certTTL := utils.MinTTL(defaults.OAuth2TTL, req.CertTTL)
		certs, err := s.generateUserCert(certRequest{
			user:          user,
			roles:         roles,
			ttl:           certTTL,
			publicKey:     req.PublicKey,
			compatibility: req.Compatibility,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.Cert = certs.ssh
		response.TLSCert = certs.tls

		authorities, err := s.GetCertAuthorities(services.HostCA, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, authority := range authorities {
			response.HostSigners = append(response.HostSigners, authority)
		}
	}
	s.EmitAuditEvent(events.UserLoginEvent, events.EventFields{
		events.EventUser:   user.GetName(),
		events.LoginMethod: events.LoginMethodGithub,
	})
	return response, nil
}

func (s *AuthServer) createGithubUser(connector services.GithubConnector, claims services.GithubClaims) error {
	logins := connector.MapClaims(claims)
	if len(logins) == 0 {
		return trace.BadParameter(
			"user %q does not belong to any teams configured in %q connector",
			claims.Username, connector.GetName())
	}
	log.WithFields(logrus.Fields{trace.Component: "github"}).Debugf(
		"Generating dynamic identity %v/%v with logins: %v.",
		connector.GetName(), claims.Username, logins)
	user, err := services.GetUserMarshaler().GenerateUser(&services.UserV2{
		Kind:    services.KindUser,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      claims.Username,
			Namespace: defaults.Namespace,
		},
		Spec: services.UserSpecV2{
			Roles:   modules.GetModules().RolesFromLogins(logins),
			Traits:  modules.GetModules().TraitsFromLogins(logins),
			Expires: s.clock.Now().UTC().Add(defaults.OAuth2TTL),
			GithubIdentities: []services.ExternalIdentity{{
				ConnectorID: connector.GetName(),
				Username:    claims.Username,
			}},
			CreatedBy: services.CreatedBy{
				User: services.UserRef{Name: "system"},
				Time: time.Now().UTC(),
				Connector: &services.ConnectorRef{
					Type:     teleport.ConnectorGithub,
					ID:       connector.GetName(),
					Identity: claims.Username,
				},
			},
		},
	})
	existingUser, err := s.GetUser(claims.Username)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if existingUser != nil {
		ref := user.GetCreatedBy().Connector
		if !ref.IsSameProvider(existingUser.GetCreatedBy().Connector) {
			return trace.AlreadyExists("user %q already exists and is not Github user",
				existingUser.GetName())
		}
	}
	err = s.UpsertUser(user)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// populateGithubClaims retrieves information about user and its team
// memberships by calling Github API using the access token
func populateGithubClaims(client githubAPIClientI) (*services.GithubClaims, error) {
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
	orgToTeams := make(map[string][]string)
	for _, team := range teams {
		orgToTeams[team.Org.Login] = append(
			orgToTeams[team.Org.Login], team.Slug)
	}
	if len(orgToTeams) == 0 {
		return nil, trace.AccessDenied(
			"list of user teams is empty, did you grant access?")
	}
	claims := &services.GithubClaims{
		Username:            user.Login,
		OrganizationToTeams: orgToTeams,
	}
	log.WithFields(logrus.Fields{trace.Component: "github"}).Debugf(
		"Claims: %#v.", claims)
	return claims, nil
}

func (s *AuthServer) getGithubOAuth2Client(connector services.GithubConnector) (*oauth2.Client, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
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
	cachedClient, ok := s.githubClients[connector.GetName()]
	if ok && oauth2ConfigsEqual(cachedClient.config, config) {
		return cachedClient.client, nil
	}
	delete(s.githubClients, connector.GetName())
	client, err := oauth2.NewClient(http.DefaultClient, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.githubClients[connector.GetName()] = &githubClient{
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
}

// userResponse represents response from "user" API call
type userResponse struct {
	// Login is the username
	Login string `json:"login"`
}

// getEmails retrieves a list of emails for authenticated user
func (c *githubAPIClient) getUser() (*userResponse, error) {
	bytes, err := c.get("/user")
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

// getTeams retrieves a list of teams authenticated user belongs to
func (c *githubAPIClient) getTeams() ([]teamResponse, error) {
	bytes, err := c.get("/user/teams")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var teams []teamResponse
	err = json.Unmarshal(bytes, &teams)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return teams, nil
}

// get makes a GET request to the provided URL using the client's token for auth
func (c *githubAPIClient) get(url string) ([]byte, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%v%v", GithubAPIURL, url), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	request.Header.Set("Authorization", fmt.Sprintf("token %v", c.token))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer response.Body.Close()
	bytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if response.StatusCode != 200 {
		return nil, trace.AccessDenied("bad response: %v %v",
			response.StatusCode, string(bytes))
	}
	return bytes, nil
}

const (
	// GithubAuthURL is the Github authorization endpoint
	GithubAuthURL = "https://github.com/login/oauth/authorize"
	// GithubTokenURL is the Github token exchange endpoint
	GithubTokenURL = "https://github.com/login/oauth/access_token"
	// GithubAPIURL is the Github base API URL
	GithubAPIURL = "https://api.github.com"
)

var (
	// GithubScopes is a list of scopes requested during OAuth2 flow
	GithubScopes = []string{
		// read:org grants read-only access to user's team memberships
		"read:org",
	}
)
