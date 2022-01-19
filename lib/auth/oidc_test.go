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
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	directory "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/cloudidentity/v1"
	"google.golang.org/api/option"
	"gopkg.in/check.v1"
)

type OIDCSuite struct {
	a *Server
	b backend.Backend
	c clockwork.FakeClock
}

var _ = check.Suite(&OIDCSuite{})

func (s *OIDCSuite) SetUpSuite(c *check.C) {
	s.c = clockwork.NewFakeClockAt(time.Now())

	var err error
	s.b, err = lite.NewWithConfig(context.Background(), lite.Config{
		Path:             c.MkDir(),
		PollStreamPeriod: 200 * time.Millisecond,
		Clock:            s.c,
	})
	c.Assert(err, check.IsNil)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	c.Assert(err, check.IsNil)

	authConfig := &InitConfig{
		ClusterName:            clusterName,
		Backend:                s.b,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}
	s.a, err = NewServer(authConfig)
	c.Assert(err, check.IsNil)
}

func (s *OIDCSuite) TestCreateOIDCUser(c *check.C) {
	// Create OIDC user with 1 minute expiry.
	_, err := s.a.createOIDCUser(&createUserParams{
		connectorName: "oidcService",
		username:      "foo@example.com",
		logins:        []string{"foo"},
		roles:         []string{"admin"},
		sessionTTL:    1 * time.Minute,
	})
	c.Assert(err, check.IsNil)

	// Within that 1 minute period the user should still exist.
	_, err = s.a.GetUser("foo@example.com", false)
	c.Assert(err, check.IsNil)

	// Advance time 2 minutes, the user should be gone.
	s.c.Advance(2 * time.Minute)
	_, err = s.a.GetUser("foo@example.com", false)
	c.Assert(err, check.NotNil)
}

// TestUserInfo ensures that an insecure userinfo endpoint returns
// trace.NotFound similar to an invalid userinfo endpoint. For these users,
// all claim information is already within the token and additional claim
// information does not need to be fetched.
func (s *OIDCSuite) TestUserInfo(c *check.C) {
	// Create configurable IdP to use in tests.
	idp := newFakeIDP()
	defer idp.Close()

	// Create OIDC connector and client.
	connector, err := types.NewOIDCConnector("test-connector", types.OIDCConnectorSpecV2{
		IssuerURL:    idp.s.URL,
		ClientID:     "00000000000000000000000000000000",
		ClientSecret: "0000000000000000000000000000000000000000000000000000000000000000",
	})
	c.Assert(err, check.IsNil)
	oidcClient, err := s.a.getOrCreateOIDCClient(connector)
	c.Assert(err, check.IsNil)

	// Verify HTTP endpoints return trace.NotFound.
	_, err = claimsFromUserInfo(oidcClient, idp.s.URL, "")
	fixtures.ExpectNotFound(c, err)
}

// TestPingProvider confirms that the client_secret_post auth
//method was set for a oauthclient.
func (s *OIDCSuite) TestPingProvider(c *check.C) {
	// Create configurable IdP to use in tests.
	idp := newFakeIDP()
	defer idp.Close()

	// Create OIDC connector and client.
	connector, err := types.NewOIDCConnector("test-connector", types.OIDCConnectorSpecV2{
		IssuerURL:    idp.s.URL,
		ClientID:     "00000000000000000000000000000000",
		ClientSecret: "0000000000000000000000000000000000000000000000000000000000000000",
		Provider:     teleport.Ping,
	})
	c.Assert(err, check.IsNil)
	oidcClient, err := s.a.getOrCreateOIDCClient(connector)

	c.Assert(err, check.IsNil)

	oac, err := s.a.getOAuthClient(oidcClient, connector)

	c.Assert(err, check.IsNil)

	// authMethod should be client secret post now
	c.Assert(oac.GetAuthMethod(), check.Equals, oauth2.AuthMethodClientSecretPost)
}

// fakeIDP is a configurable OIDC IdP that can be used to mock responses in
// tests. At the moment it creates an HTTP server and only responds to the
// "/.well-known/openid-configuration" endpoint.
type fakeIDP struct {
	s *httptest.Server
}

// newFakeIDP creates a new instance of a configurable IdP.
func newFakeIDP() *fakeIDP {
	var s fakeIDP
	s.s = httptest.NewServer(http.HandlerFunc(s.configurationHandler))
	return &s
}

// Close will close the underlying server.
func (s *fakeIDP) Close() {
	s.s.Close()
}

// configurationHandler returns OpenID configuration.
func (s *fakeIDP) configurationHandler(w http.ResponseWriter, r *http.Request) {
	resp := fmt.Sprintf(`
{
	"issuer": "%v",
	"authorization_endpoint": "%v",
	"token_endpoint": "%v",
	"jwks_uri": "%v",
	"subject_types_supported": ["public"],
	"id_token_signing_alg_values_supported": ["HS256", "RS256"]
}`, s.s.URL, s.s.URL, s.s.URL, s.s.URL)

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
