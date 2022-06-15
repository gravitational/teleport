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
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"

	"github.com/coreos/go-oidc/jose"
	"github.com/coreos/go-oidc/oauth2"
	"github.com/coreos/go-oidc/oidc"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	directory "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/cloudidentity/v1"
	"google.golang.org/api/option"
)

type OIDCSuite struct {
	a *Server
	b backend.Backend
	c clockwork.FakeClock
}

func setUpSuite(t *testing.T) *OIDCSuite {
	s := OIDCSuite{}
	s.c = clockwork.NewFakeClockAt(time.Now())

	var err error
	s.b, err = lite.NewWithConfig(context.Background(), lite.Config{
		Path:             t.TempDir(),
		PollStreamPeriod: 200 * time.Millisecond,
		Clock:            s.c,
	})
	require.NoError(t, err)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	authConfig := &InitConfig{
		ClusterName:            clusterName,
		Backend:                s.b,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}
	s.a, err = NewServer(authConfig)
	require.NoError(t, err)
	return &s
}

// createInsecureOIDCClient creates an insecure client for testing.
func createInsecureOIDCClient(t *testing.T, connector types.OIDCConnector) *oidc.Client {
	conf := oidcConfig(connector, "")
	conf.HTTPClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	client, err := oidc.NewClient(conf)
	require.NoError(t, err)
	client.SyncProviderConfig(connector.GetIssuerURL())
	return client
}

func TestCreateOIDCUser(t *testing.T) {
	s := setUpSuite(t)

	// Dry-run creation of OIDC user.
	user, err := s.a.createOIDCUser(&createUserParams{
		connectorName: "oidcService",
		username:      "foo@example.com",
		roles:         []string{"admin"},
		sessionTTL:    1 * time.Minute,
	}, true)
	require.NoError(t, err)
	require.Equal(t, "foo@example.com", user.GetName())

	// Dry-run must not create a user.
	_, err = s.a.GetUser("foo@example.com", false)
	require.Error(t, err)

	// Create OIDC user with 1 minute expiry.
	_, err = s.a.createOIDCUser(&createUserParams{
		connectorName: "oidcService",
		username:      "foo@example.com",
		roles:         []string{"admin"},
		sessionTTL:    1 * time.Minute,
	}, false)
	require.NoError(t, err)

	// Within that 1 minute period the user should still exist.
	_, err = s.a.GetUser("foo@example.com", false)
	require.NoError(t, err)

	// Advance time 2 minutes, the user should be gone.
	s.c.Advance(2 * time.Minute)
	_, err = s.a.GetUser("foo@example.com", false)
	require.Error(t, err)
}

// TestUserInfoBlockHTTP ensures that an insecure userinfo endpoint returns
// trace.NotFound similar to an invalid userinfo endpoint. For these users,
// all claim information is already within the token and additional claim
// information does not need to be fetched.
func TestUserInfoBlockHTTP(t *testing.T) {
	ctx := context.Background()
	s := setUpSuite(t)
	// Create configurable IdP to use in tests.
	idp := newFakeIDP(t, false /* tls */)

	// Create OIDC connector and client.
	connector, err := types.NewOIDCConnector("test-connector", types.OIDCConnectorSpecV3{
		IssuerURL:     idp.s.URL,
		ClientID:      "00000000000000000000000000000000",
		ClientSecret:  "0000000000000000000000000000000000000000000000000000000000000000",
		ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
		RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
	})
	require.NoError(t, err)

	oidcClient, err := s.a.getCachedOIDCClient(ctx, connector, "")
	require.NoError(t, err)

	// Verify HTTP endpoints return trace.NotFound.
	_, err = claimsFromUserInfo(oidcClient.client, idp.s.URL, "")
	fixtures.AssertNotFound(t, err)
}

// TestUserInfoBadStatus asserts that a 4xx response from userinfo results
// in AccessDenied.
func TestUserInfoBadStatus(t *testing.T) {
	// Create configurable IdP to use in tests.
	idp := newFakeIDP(t, true /* tls */)

	// Create OIDC connector and client.
	connector, err := types.NewOIDCConnector("test-connector", types.OIDCConnectorSpecV3{
		IssuerURL:     idp.s.URL,
		ClientID:      "00000000000000000000000000000000",
		ClientSecret:  "0000000000000000000000000000000000000000000000000000000000000000",
		ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
		RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
	})
	require.NoError(t, err)
	oidcClient := createInsecureOIDCClient(t, connector)

	// Verify HTTP endpoints return trace.AccessDenied.
	_, err = claimsFromUserInfo(oidcClient, idp.s.URL, "")
	fixtures.AssertAccessDenied(t, err)
}

func TestSSODiagnostic(t *testing.T) {
	ctx := context.Background()
	s := setUpSuite(t)
	// Create configurable IdP to use in tests.
	idp := newFakeIDP(t, false /* tls */)

	// create role referenced in request.
	role, err := types.NewRole("access", types.RoleSpecV5{
		Allow: types.RoleConditions{
			Logins: []string{"dummy"},
		},
	})
	require.NoError(t, err)
	err = s.a.CreateRole(role)
	require.NoError(t, err)

	// connector spec
	spec := types.OIDCConnectorSpecV3{
		IssuerURL:    idp.s.URL,
		ClientID:     "00000000000000000000000000000000",
		ClientSecret: "0000000000000000000000000000000000000000000000000000000000000000",
		Display:      "Test",
		Scope:        []string{"groups"},
		ClaimsToRoles: []types.ClaimMapping{
			{
				Claim: "groups",
				Value: "idp-admin",
				Roles: []string{"access"},
			},
		},
		RedirectURLs: []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
	}

	oidcRequest := types.OIDCAuthRequest{
		ConnectorID:   "-sso-test-okta",
		Type:          constants.OIDC,
		CertTTL:       types.Duration(defaults.OIDCAuthRequestTTL),
		SSOTestFlow:   true,
		ConnectorSpec: &spec,
	}

	request, err := s.a.CreateOIDCAuthRequest(ctx, oidcRequest)
	require.NoError(t, err)
	require.NotNil(t, request)

	values := url.Values{
		"code":  []string{"XXX-code"},
		"state": []string{request.StateToken},
	}

	// override getClaimsFun.
	s.a.getClaimsFun = func(closeCtx context.Context, oidcClient *oidc.Client, connector types.OIDCConnector, code string) (jose.Claims, error) {
		cc := map[string]interface{}{
			"email_verified": true,
			"groups":         []string{"everyone", "idp-admin", "idp-dev"},
			"email":          "superuser@example.com",
			"sub":            "00001234abcd",
			"exp":            1652091713.0,
		}
		return cc, nil
	}

	resp, err := s.a.ValidateOIDCAuthCallback(ctx, values)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, &OIDCAuthResponse{
		Username: "superuser@example.com",
		Identity: types.ExternalIdentity{
			ConnectorID: "-sso-test-okta",
			Username:    "superuser@example.com",
		},
		Req: *request,
	}, resp)

	diagCtx := ssoDiagContext{}

	resp, err = s.a.validateOIDCAuthCallback(ctx, &diagCtx, values)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, &OIDCAuthResponse{
		Username: "superuser@example.com",
		Identity: types.ExternalIdentity{
			ConnectorID: "-sso-test-okta",
			Username:    "superuser@example.com",
		},
		Req: *request,
	}, resp)
	require.Equal(t, types.SSODiagnosticInfo{
		TestFlow: true,
		Success:  true,
		CreateUserParams: &types.CreateUserParams{
			ConnectorName: "-sso-test-okta",
			Username:      "superuser@example.com",
			Logins:        nil,
			KubeGroups:    nil,
			KubeUsers:     nil,
			Roles:         []string{"access"},
			Traits: map[string][]string{
				"email":  {"superuser@example.com"},
				"groups": {"everyone", "idp-admin", "idp-dev"},
				"sub":    {"00001234abcd"},
			},
			SessionTTL: 600000000000,
		},
		OIDCClaimsToRoles: []types.ClaimMapping{
			{
				Claim: "groups",
				Value: "idp-admin",
				Roles: []string{"access"},
			},
		},
		OIDCClaimsToRolesWarnings: nil,
		OIDCClaims: map[string]interface{}{
			"email_verified": true,
			"groups":         []string{"everyone", "idp-admin", "idp-dev"},
			"email":          "superuser@example.com",
			"sub":            "00001234abcd",
			"exp":            1652091713.0,
		},
		OIDCIdentity: &types.OIDCIdentity{
			ID:        "00001234abcd",
			Name:      "",
			Email:     "superuser@example.com",
			ExpiresAt: diagCtx.info.OIDCIdentity.ExpiresAt,
		},
		OIDCTraitsFromClaims: map[string][]string{
			"email":  {"superuser@example.com"},
			"groups": {"everyone", "idp-admin", "idp-dev"},
			"sub":    {"00001234abcd"},
		},
		OIDCConnectorTraitMapping: []types.TraitMapping{
			{
				Trait: "groups",
				Value: "idp-admin",
				Roles: []string{"access"},
			},
		},
	}, diagCtx.info)
}

// TestPingProvider confirms that the client_secret_post auth
// method was set for a oauthclient.
func TestPingProvider(t *testing.T) {
	ctx := context.Background()
	s := setUpSuite(t)
	// Create configurable IdP to use in tests.
	idp := newFakeIDP(t, false /* tls */)

	// Create and upsert oidc connector into identity
	connector, err := types.NewOIDCConnector("test-connector", types.OIDCConnectorSpecV3{
		IssuerURL:     idp.s.URL,
		ClientID:      "00000000000000000000000000000000",
		ClientSecret:  "0000000000000000000000000000000000000000000000000000000000000000",
		Provider:      teleport.Ping,
		ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
		RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
	})
	require.NoError(t, err)
	err = s.a.Identity.UpsertOIDCConnector(ctx, connector)
	require.NoError(t, err)

	for _, req := range []types.OIDCAuthRequest{
		{
			ConnectorID: "test-connector",
		}, {
			SSOTestFlow: true,
			ConnectorID: "test-connector",
			ConnectorSpec: &types.OIDCConnectorSpecV3{
				IssuerURL:     idp.s.URL,
				ClientID:      "00000000000000000000000000000000",
				ClientSecret:  "0000000000000000000000000000000000000000000000000000000000000000",
				Provider:      teleport.Ping,
				ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
				RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
			},
		},
	} {
		t.Run(fmt.Sprintf("Test SSOFlow: %v", req.SSOTestFlow), func(t *testing.T) {
			oidcConnector, oidcClient, err := s.a.getOIDCConnectorAndClient(ctx, req)
			require.NoError(t, err)

			oac, err := getOAuthClient(oidcClient, oidcConnector)
			require.NoError(t, err)

			// authMethod should be client secret post now
			require.Equal(t, oauth2.AuthMethodClientSecretPost, oac.GetAuthMethod())
		})
	}
}

func TestOIDCClientProviderSync(t *testing.T) {
	ctx := context.Background()
	// Create configurable IdP to use in tests.
	idp := newFakeIDP(t, false /* tls */)

	// Create OIDC connector and client.
	connector, err := types.NewOIDCConnector("test-connector", types.OIDCConnectorSpecV3{
		IssuerURL:     idp.s.URL,
		ClientID:      "00000000000000000000000000000000",
		ClientSecret:  "0000000000000000000000000000000000000000000000000000000000000000",
		Provider:      teleport.Ping,
		ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
		RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
	})
	require.NoError(t, err)

	client, err := newOIDCClient(ctx, connector, "proxy.example.com")
	require.NoError(t, err)

	// first sync should complete successfully
	require.NoError(t, client.waitFirstSync(100*time.Millisecond))
	require.NoError(t, client.syncCtx.Err())

	// Create OIDC client with a canceled ctx
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	client, err = newOIDCClient(canceledCtx, connector, "proxy.example.com")
	require.NoError(t, err)

	// provider sync goroutine should end and first sync should fail
	require.ErrorIs(t, client.syncCtx.Err(), context.Canceled)
	err = client.waitFirstSync(100 * time.Millisecond)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)

	// Create OIDC connector and client without an issuer URL for provider syncing
	connectorNoIssuer, err := types.NewOIDCConnector("test-connector", types.OIDCConnectorSpecV3{
		ClientID:      "00000000000000000000000000000000",
		ClientSecret:  "0000000000000000000000000000000000000000000000000000000000000000",
		Provider:      teleport.Ping,
		ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
		RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
	})
	require.NoError(t, err)

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	client, err = newOIDCClient(timeoutCtx, connectorNoIssuer, "proxy.example.com")
	require.NoError(t, err)

	// first sync should fail after the given timeout and cancel the sync goroutine.
	err = client.waitFirstSync(100 * time.Millisecond)
	require.Error(t, err)
	require.True(t, trace.IsConnectionProblem(err))
	require.ErrorIs(t, client.syncCtx.Err(), context.Canceled)
}

func TestOIDCClientCache(t *testing.T) {
	ctx := context.Background()
	s := setUpSuite(t)
	// Create configurable IdP to use in tests.
	idp := newFakeIDP(t, false /* tls */)
	connectorSpec := types.OIDCConnectorSpecV3{
		IssuerURL:     idp.s.URL,
		ClientID:      "00000000000000000000000000000000",
		ClientSecret:  "0000000000000000000000000000000000000000000000000000000000000000",
		Provider:      teleport.Ping,
		ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
		RedirectURLs:  []string{"https://proxy.example.com/v1/webapi/oidc/callback"},
	}
	connector, err := types.NewOIDCConnector("test-connector", connectorSpec)
	require.NoError(t, err)

	// Create and cache a new oidc client
	client, err := s.a.getCachedOIDCClient(ctx, connector, "proxy.example.com")
	require.NoError(t, err)

	// The next call should return the same client (compare memory address)
	cachedClient, err := s.a.getCachedOIDCClient(ctx, connector, "proxy.example.com")
	require.NoError(t, err)
	require.True(t, client == cachedClient)

	// Canceling provider sync on a cached client should cause it to be replaced
	client.syncCancel()
	cachedClient, err = s.a.getCachedOIDCClient(ctx, connector, "proxy.example.com")
	require.NoError(t, err)
	require.False(t, client == cachedClient)

	// Certain changes to the connector should cause the cached client to be refreshed
	originalClient := cachedClient
	for _, tc := range []struct {
		desc            string
		mutateConnector func(types.OIDCConnector)
		expectNoRefresh bool
	}{
		{
			desc: "IssuerURL",
			mutateConnector: func(conn types.OIDCConnector) {
				conn.SetIssuerURL(newFakeIDP(t, false /* tls */).s.URL)
			},
		}, {
			desc: "ClientID",
			mutateConnector: func(conn types.OIDCConnector) {
				conn.SetClientID("11111111111111111111111111111111")
			},
		}, {
			desc: "ClientSecret",
			mutateConnector: func(conn types.OIDCConnector) {
				conn.SetClientSecret("1111111111111111111111111111111111111111111111111111111111111111")
			},
		}, {
			desc: "RedirectURLs",
			mutateConnector: func(conn types.OIDCConnector) {
				conn.SetRedirectURLs([]string{"https://other.example.com/v1/webapi/oidc/callback"})
			},
		}, {
			desc: "Scope",
			mutateConnector: func(conn types.OIDCConnector) {
				conn.SetScope([]string{"groups"})
			},
		}, {
			desc: "Prompt - no refresh",
			mutateConnector: func(conn types.OIDCConnector) {
				conn.SetPrompt("none")
			},
			expectNoRefresh: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			newConnector, err := types.NewOIDCConnector("test-connector", connectorSpec)
			require.NoError(t, err)
			tc.mutateConnector(newConnector)

			client, err = s.a.getCachedOIDCClient(ctx, newConnector, "proxy.example.com")
			require.NoError(t, err)
			require.True(t, (client == originalClient) == tc.expectNoRefresh)

			// reset cached client to the original client for remaining tests
			originalClient, err = s.a.getCachedOIDCClient(ctx, connector, "proxy.example.com")
			require.NoError(t, err)
		})
	}
}

// fakeIDP is a configurable OIDC IdP that can be used to mock responses in
// tests. At the moment it creates an HTTP server and only responds to the
// "/.well-known/openid-configuration" endpoint.
type fakeIDP struct {
	s *httptest.Server
}

// newFakeIDP creates a new instance of a configurable IdP.
func newFakeIDP(t *testing.T, tls bool) *fakeIDP {
	var s fakeIDP

	mux := http.NewServeMux()
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	mux.HandleFunc("/", s.configurationHandler)

	if tls {
		s.s = httptest.NewTLSServer(mux)
	} else {
		s.s = httptest.NewServer(mux)
	}

	t.Cleanup(s.s.Close)
	return &s
}

// configurationHandler returns OpenID configuration.
func (s *fakeIDP) configurationHandler(w http.ResponseWriter, r *http.Request) {
	resp := fmt.Sprintf(`
{
	"issuer": "%v",
	"authorization_endpoint": "%v",
	"token_endpoint": "%v",
	"jwks_uri": "%v",
	"userinfo_endpoint": "%v/userinfo",
	"subject_types_supported": ["public"],
	"id_token_signing_alg_values_supported": ["HS256", "RS256"]
}`, s.s.URL, s.s.URL, s.s.URL, s.s.URL, s.s.URL)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, resp)
}

func TestOIDCGoogle(t *testing.T) {
	directGroups := map[string][]string{
		"alice@foo.example":  {"group1@foo.example", "group2@sub.foo.example", "group3@bar.example"},
		"bob@foo.example":    {"group1@foo.example"},
		"carlos@bar.example": {"group1@foo.example", "group2@sub.foo.example", "group3@bar.example"},
	}

	// group2@sub.foo.example is in group3@bar.example and group3@bar.example is in group4@bar.example
	strictDirectGroups := map[string][]string{
		"alice@foo.example":  {"group1@foo.example", "group2@sub.foo.example"},
		"bob@foo.example":    {"group1@foo.example"},
		"carlos@bar.example": {"group1@foo.example", "group2@sub.foo.example"},
	}
	directIndirectGroups := map[string][]string{
		"alice@foo.example":  {"group3@bar.example"},
		"bob@foo.example":    {},
		"carlos@bar.example": {"group3@bar.example"},
	}
	indirectGroups := map[string][]string{
		"alice@foo.example":  {"group4@bar.example"},
		"bob@foo.example":    {},
		"carlos@bar.example": {"group4@bar.example"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/admin/directory/v1/groups", func(rw http.ResponseWriter, r *http.Request) {
		require.Equal(t, "GET", r.Method)

		email := r.URL.Query().Get("userKey")
		require.NotEmpty(t, email)
		require.Contains(t, directGroups, email)

		domain := r.URL.Query().Get("domain")

		resp := &directory.Groups{}
		for _, groupEmail := range directGroups[email] {
			if domain == "" || strings.HasSuffix(groupEmail, "@"+domain) {
				resp.Groups = append(resp.Groups, &directory.Group{Email: groupEmail})
			}
		}

		require.NoError(t, json.NewEncoder(rw).Encode(resp))
	})
	mux.HandleFunc("/v1/groups/-/memberships:searchTransitiveGroups", func(rw http.ResponseWriter, r *http.Request) {
		require.Equal(t, "GET", r.Method)
		q := r.URL.Query().Get("query")

		// hacky solution but the query parameter of searchTransitiveGroups is also pretty hacky
		prefix := "member_key_id == '"
		suffix := "' && 'cloudidentity.googleapis.com/groups.discussion_forum' in labels"
		require.True(t, strings.HasPrefix(q, prefix))
		require.True(t, strings.HasSuffix(q, suffix))
		email := strings.TrimSuffix(strings.TrimPrefix(q, prefix), suffix)
		require.NotEmpty(t, email)
		require.Contains(t, directGroups, email)

		resp := &cloudidentity.SearchTransitiveGroupsResponse{}

		for relationType, groupEmails := range map[string][]string{
			"DIRECT":              strictDirectGroups[email],
			"DIRECT_AND_INDIRECT": directIndirectGroups[email],
			"INDIRECT":            indirectGroups[email],
		} {
			for _, groupEmail := range groupEmails {
				resp.Memberships = append(resp.Memberships, &cloudidentity.GroupRelation{
					GroupKey: &cloudidentity.EntityKey{
						Id: groupEmail,
					},
					Labels: map[string]string{
						"cloudidentity.googleapis.com/groups.discussion_forum": "",
					},
					RelationType: relationType,
				})
			}
		}

		require.NoError(t, json.NewEncoder(rw).Encode(resp))
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	testOptions := []option.ClientOption{option.WithEndpoint(ts.URL), option.WithoutAuthentication()}

	ctx := context.Background()

	for _, testCase := range []struct {
		email, domain                string
		transitive, direct, filtered []string
	}{
		{"alice@foo.example", "foo.example",
			[]string{"group1@foo.example", "group2@sub.foo.example", "group3@bar.example", "group4@bar.example"},
			[]string{"group1@foo.example", "group2@sub.foo.example", "group3@bar.example"},
			[]string{"group1@foo.example"},
		},
		{"bob@foo.example", "foo.example",
			[]string{"group1@foo.example"},
			[]string{"group1@foo.example"},
			[]string{"group1@foo.example"},
		},
		{"carlos@bar.example", "bar.example",
			[]string{"group1@foo.example", "group2@sub.foo.example", "group3@bar.example", "group4@bar.example"},
			[]string{"group1@foo.example", "group2@sub.foo.example", "group3@bar.example"},
			[]string{"group3@bar.example"},
		},
	} {
		// transitive groups
		groups, err := groupsFromGoogleCloudIdentity(ctx, testCase.email, testOptions...)
		require.NoError(t, err)
		require.ElementsMatch(t, testCase.transitive, groups)

		// direct groups, unfiltered
		groups, err = groupsFromGoogleDirectory(ctx, testCase.email, "", testOptions...)
		require.NoError(t, err)
		require.ElementsMatch(t, testCase.direct, groups)

		// direct groups, filtered by domain
		groups, err = groupsFromGoogleDirectory(ctx, testCase.email, testCase.domain, testOptions...)
		require.NoError(t, err)
		require.ElementsMatch(t, testCase.filtered, groups)
	}
}
