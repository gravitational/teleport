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
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/coreos/go-oidc/oauth2"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"

	"github.com/jonboulle/clockwork"
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
