package web

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

var (
	csrfToken  = "2ebcb768d0090ea4368e42880c970b61865c326172a4a2343b645cf5d7f20992"
	csrfCookie = &http.Cookie{Name: csrf.CookieName, Value: csrfToken}
)

func TestGithubWithoutRoleMapping(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const connectorMappedRole = "access"
	const accessListRole = "admin"

	tests := []struct {
		name                    string
		teamsToRoles            []types.TeamRolesMapping
		hasConnectorMappedRoles bool
		hasAccessListRoles      bool
		validSession            bool
		expectedRedirectURL     string
	}{
		{
			name:                    "mapped teams to roles without access list roles",
			teamsToRoles:            []types.TeamRolesMapping{{Organization: "octocats", Team: "devs", Roles: []string{connectorMappedRole}}},
			hasConnectorMappedRoles: true,
			validSession:            true,
			expectedRedirectURL:     "/after",
		},
		{
			name:                    "mapped teams to roles with access list roles",
			teamsToRoles:            []types.TeamRolesMapping{{Organization: "octocats", Team: "devs", Roles: []string{connectorMappedRole}}},
			hasConnectorMappedRoles: true,
			hasAccessListRoles:      true,
			validSession:            true,
			expectedRedirectURL:     "/after",
		},
		{
			name:         "fail to map teams to roles without access list roles",
			teamsToRoles: []types.TeamRolesMapping{{Organization: "octocats", Team: "unknown", Roles: []string{"unknown"}}},
		},
		{
			name:                "fail to map teams to roles with access list roles",
			teamsToRoles:        []types.TeamRolesMapping{{Organization: "octocats", Team: "unknown", Roles: []string{"unknown"}}},
			hasAccessListRoles:  true,
			validSession:        true,
			expectedRedirectURL: "/after",
		},
		{
			name:         "no teams to roles without access list roles",
			teamsToRoles: nil,
		},
		{
			name:                "no teams to roles with access list roles",
			teamsToRoles:        nil,
			validSession:        true,
			hasAccessListRoles:  true,
			expectedRedirectURL: "/after",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			env := newWebPack(t, 1)

			env.server.Auth().GithubUserAndTeamsOverride = githubUserAndTeamsOverride

			mustCreateRole(t, ctx, env, connectorMappedRole)

			if tc.hasAccessListRoles {
				mustCreateRole(t, ctx, env, accessListRole)
				mustSetupAccessList(t, ctx, env, "alice", accessListRole)
			}

			connector, err := types.NewGithubConnector("github", types.GithubConnectorSpecV3{
				ClientID:     "12345",
				ClientSecret: "678910",
				RedirectURL:  "https://proxy.example.com/v1/webapi/github/callback",
				Display:      "Github",
				TeamsToRoles: tc.teamsToRoles,
			})
			require.NoError(t, err)
			mustCreateGithubSSOConnector(t, ctx, env, connector)

			clt := env.proxies[0].newClientNoRedirects(t)

			loginURL, err := url.Parse(clt.Endpoint("webapi", "github", "login", "web") + `?connector_id=` + connector.GetName() + `&redirect_url=http://localhost/after`)
			require.NoError(t, err)

			loginResp := mustSendRequest(t, clt, csrfCookie, loginURL.String())

			locationURL, err := url.Parse(loginResp.Headers().Get("Location"))
			require.NoError(t, err)

			state := locationURL.Query().Get("state")
			callbackURL, err := url.Parse(clt.Endpoint("webapi", "github", "callback") + `?state=` + state + `&code=success`)
			require.NoError(t, err)

			callbackResp := mustSendRequest(t, clt, csrfCookie, callbackURL.String())

			webSessions, err := env.server.Auth().WebSessions().List(ctx)
			require.NoError(t, err)

			// If we don't expect a valid session, verify there is none and return without further assertions.
			if !tc.validSession {
				require.Empty(t, webSessions)
				return
			}
			require.Len(t, webSessions, 1)

			userIdentity := mustGetUserIdentityFromWebSession(t, webSessions[0])

			if tc.hasConnectorMappedRoles {
				require.Contains(t, userIdentity.Groups, connectorMappedRole)
			} else {
				require.NotContains(t, userIdentity.Groups, connectorMappedRole)
			}

			if tc.hasAccessListRoles {
				require.Contains(t, userIdentity.Groups, accessListRole)
			} else {
				require.NotContains(t, userIdentity.Groups, accessListRole)
			}

			require.NotEmpty(t, callbackResp.Headers().Get("Set-Cookie"))
			require.Contains(t, string(callbackResp.Bytes()), tc.expectedRedirectURL)
		})
	}
}

// githubUserAndTeamsOverride overrides the user and teams mapping on the Github user response.
func githubUserAndTeamsOverride() (*auth.GithubUserResponse, []auth.GithubTeamResponse, error) {
	return &auth.GithubUserResponse{
			Login: "alice",
		}, []auth.GithubTeamResponse{{
			Name: "devs",
			Slug: "devs",
			Org:  auth.GithubOrgResponse{Login: "octocats"},
		}}, nil
}

// mustCreateGithubSSOConnector creates the given connector in the backend.
// If the connector has teams_to_roles mapping, the auth server upsert is used.
// If the connector has no teams_to_roles mapping, the connector is put to the backend directly,
// bypassing the teams_to_roles validation.
// This is necessary because a connector without teams_to_roles is valid for reads and SSO,
// but not valid for writes.
// See RFD: https://github.com/gravitational/rfd/blob/main/rfd/0248-sso-connector-without-role-mapping.md
// TODO(nixpig): Post phase 2 (v19.0.x) remove this condition and go through the normal auth server upsert path for both.
func mustCreateGithubSSOConnector(t *testing.T, ctx context.Context, env *webPack, connector types.GithubConnector) {
	t.Helper()

	if len(connector.GetTeamsToRoles()) > 0 {
		_, err := env.server.Auth().UpsertGithubConnector(ctx, connector)
		require.NoError(t, err)

		return
	}

	value, err := utils.FastMarshal(connector)
	require.NoError(t, err)

	githubConnectorBackendKey := backend.NewKey("web", "connectors", "github", "connectors", connector.GetName())
	_, err = env.server.AuthServer.Backend.Put(ctx, backend.Item{
		Key:   githubConnectorBackendKey,
		Value: value,
	})
	require.NoError(t, err)
}

// mustSendRequest uses the client to send a request to the baseURL with the csrfCookie, and returns the response.
func mustSendRequest(t *testing.T, clt *TestWebClient, csrfCookie *http.Cookie, baseURL string) *roundtrip.Response {
	t.Helper()

	req, err := http.NewRequest("GET", baseURL, nil)
	require.NoError(t, err)

	req.AddCookie(csrfCookie)

	resp, err := clt.Client.RoundTrip(func() (*http.Response, error) {
		return clt.Client.HTTPClient().Do(req)
	})

	require.NoError(t, err)
	return resp
}

// mustGetUserIdentityFromWebSession gets the user identity from the web session.
func mustGetUserIdentityFromWebSession(t *testing.T, webSess types.WebSession) *tlsca.Identity {
	t.Helper()

	cert, err := tlsca.ParseCertificatePEM(webSess.GetTLSCert())
	require.NoError(t, err)

	userIdentity, err := tlsca.FromSubject(cert.Subject, time.Now())
	require.NoError(t, err)

	return userIdentity
}

// mustSetupAccessList creates an Access List that grants the given role,
// and adds the username as an Access List Member.
func mustSetupAccessList(t *testing.T, ctx context.Context, env *webPack, username, role string) {
	t.Helper()

	clock := env.server.Auth().GetClock()

	accessList, err := accesslist.NewAccessList(
		header.Metadata{Name: "accesslist"},
		accesslist.Spec{
			Title:  "simple",
			Audit:  accesslist.Audit{NextAuditDate: clock.Now().AddDate(1, 0, 0)},
			Grants: accesslist.Grants{Roles: []string{role}},
			Owners: []accesslist.Owner{{Name: role}},
		},
	)
	require.NoError(t, err)

	_, err = env.server.Auth().UpsertAccessList(ctx, accessList)
	require.NoError(t, err)

	accessListMember, err := accesslist.NewAccessListMember(
		header.Metadata{Name: username},
		accesslist.AccessListMemberSpec{
			AccessList: accessList.GetName(),
			Name:       username,
			Joined:     clock.Now(),
			AddedBy:    role,
		},
	)
	require.NoError(t, err)

	_, err = env.server.Auth().UpsertAccessListMember(ctx, accessListMember)
	require.NoError(t, err)
}

// mustCreateRole creates a role with an empty spec.
func mustCreateRole(t *testing.T, ctx context.Context, env *webPack, roleName string) {
	t.Helper()

	role, err := types.NewRole(roleName, types.RoleSpecV6{})
	require.NoError(t, err)

	_, err = env.server.Auth().CreateRole(ctx, role)
	require.NoError(t, err)
}
