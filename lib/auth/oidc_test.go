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
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"

	"github.com/coreos/go-oidc/oauth2"
	"github.com/coreos/go-oidc/oidc"
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
	conf := oidcConfig(connector)
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
	// Create OIDC user with 1 minute expiry.
	_, err := s.a.createOIDCUser(&createUserParams{
		connectorName: "oidcService",
		username:      "foo@example.com",
		logins:        []string{"foo"},
		roles:         []string{"admin"},
		sessionTTL:    1 * time.Minute,
	})
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
	s := setUpSuite(t)
	// Create configurable IdP to use in tests.
	idp := newFakeIDP(t, false /* tls */)

	// Create OIDC connector and client.
	connector, err := types.NewOIDCConnector("test-connector", types.OIDCConnectorSpecV3{
		IssuerURL:    idp.s.URL,
		ClientID:     "00000000000000000000000000000000",
		ClientSecret: "0000000000000000000000000000000000000000000000000000000000000000",
	})
	require.NoError(t, err)
	oidcClient, err := s.a.getOrCreateOIDCClient(connector)
	require.NoError(t, err)

	// Verify HTTP endpoints return trace.NotFound.
	_, err = claimsFromUserInfo(oidcClient, idp.s.URL, "")
	fixtures.AssertNotFound(t, err)
}

// TestUserInfoBadStatus asserts that a 4xx response from userinfo results
// in AccessDenied.
func TestUserInfoBadStatus(t *testing.T) {
	// Create configurable IdP to use in tests.
	idp := newFakeIDP(t, true /* tls */)

	// Create OIDC connector and client.
	connector, err := types.NewOIDCConnector("test-connector", types.OIDCConnectorSpecV3{
		IssuerURL:    idp.s.URL,
		ClientID:     "00000000000000000000000000000000",
		ClientSecret: "0000000000000000000000000000000000000000000000000000000000000000",
	})
	require.NoError(t, err)
	oidcClient := createInsecureOIDCClient(t, connector)

	// Verify HTTP endpoints return trace.AccessDenied.
	_, err = claimsFromUserInfo(oidcClient, idp.s.URL, "")
	fixtures.AssertAccessDenied(t, err)
}

// TestPingProvider confirms that the client_secret_post auth
//method was set for a oauthclient.
func TestPingProvider(t *testing.T) {
	s := setUpSuite(t)
	// Create configurable IdP to use in tests.
	idp := newFakeIDP(t, false /* tls */)

	// Create OIDC connector and client.
	connector, err := types.NewOIDCConnector("test-connector", types.OIDCConnectorSpecV3{
		IssuerURL:    idp.s.URL,
		ClientID:     "00000000000000000000000000000000",
		ClientSecret: "0000000000000000000000000000000000000000000000000000000000000000",
		Provider:     teleport.Ping,
	})
	require.NoError(t, err)
	oidcClient, err := s.a.getOrCreateOIDCClient(connector)

	require.NoError(t, err)

	oac, err := s.a.getOAuthClient(oidcClient, connector)

	require.NoError(t, err)

	// authMethod should be client secret post now
	require.Equal(t, oauth2.AuthMethodClientSecretPost, oac.GetAuthMethod())
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
