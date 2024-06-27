package web

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/gravitational/roundtrip"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

var (
	csrfToken  = "2ebcb768d0090ea4368e42880c970b61865c326172a4a2343b645cf5d7f20992"
	csrfCookie = &http.Cookie{Name: csrf.CookieName, Value: csrfToken}
)

func TestSAML(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{SAML: true},
	})

	tests := []struct {
		name                string
		rawConnector        string
		validSession        bool
		expectedRedirectURL string
	}{
		{
			name:                "success",
			rawConnector:        fixtures.SAMLOktaConnectorV2,
			validSession:        true,
			expectedRedirectURL: "/after",
		},
		{
			name:                "fail to map claims to roles",
			rawConnector:        strings.ReplaceAll(fixtures.SAMLOktaConnectorV2, "Everyone", "No-one"),
			validSession:        false,
			expectedRedirectURL: client.LoginFailedUnauthorizedRedirectURL,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s := newWebSuite(t)
			input := tc.rawConnector

			connector := prepareSSOConnectorSetup(t, input, ctx, s)

			clt := s.clientNoRedirects()
			resp := initSSOLogin(t, clt, connector, csrfCookie)
			// we got a redirect
			id := mustExtractSAMLRequestID(t, resp)
			getAuthRequestAndSwapID(t, ctx, s, id, csrfToken)
			// now respond with pre-recorded request to the POST url
			resp = mustSendSAMLResponse(t, clt, csrfCookie)

			if tc.validSession {
				// we have got valid session
				require.NotEmpty(t, resp.Headers().Get("Set-Cookie"))
			}
			require.Contains(t, string(resp.Bytes()), tc.expectedRedirectURL)
		})
	}
}

func TestSAMLNoEphemeralUser(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{SAML: true},
	})
	ctx := context.Background()
	s := newWebSuite(t)
	input := fixtures.SAMLOktaConnectorV2

	oktaUserTraits := map[string][]string{"okta/org": {"dev"}}
	mustCreateOktaPermanentUser(t, ctx, s, oktaUserTraits, "ops@gravitational.io")

	connector := prepareSSOConnectorSetup(t, input, ctx, s)

	clt := s.clientNoRedirects()
	resp := initSSOLogin(t, clt, connector, csrfCookie)
	id := mustExtractSAMLRequestID(t, resp)
	getAuthRequestAndSwapID(t, ctx, s, id, csrfToken)
	mustSendSAMLResponse(t, clt, csrfCookie)

	t.Run("web sessions should contains role evaluated based on attribute mappings ", func(t *testing.T) {
		webSessions, err := s.testAuthServer.AuthServer.AuthServer.WebSessions().List(ctx)
		require.NoError(t, err)
		require.Len(t, webSessions, 1)

		userIdentity := mustGetUserIdentityFromWebSession(t, webSessions[0])
		require.Equal(t, []string{"admin"}, userIdentity.Groups)

		// Check that the user has the correct traits
		// propagated from permanent SAML user created by Okta service during user sync
		// and traits from the SAML assertion.
		want := wrappers.Traits(oktaUserTraits)
		want["groups"] = []string{"Everyone"}
		require.Equal(t, want, userIdentity.Traits)
	})
}

func mustCreateOktaPermanentUser(t *testing.T, ctx context.Context, s *webSuite, traits map[string][]string, userName string) {
	newUser, err := types.NewUser(userName)
	require.NoError(t, err)
	newUser.SetStaticLabels(map[string]string{
		types.OriginLabel: types.OriginOkta,
	})
	newUser.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{
			Name: teleport.UserSystem,
		},
		Connector: &types.ConnectorRef{
			ID:       "okta",
			Type:     constants.SAML,
			Identity: "oktaUserID",
		},
	})
	newUser.SetTraits(traits)
	_, err = s.testAuthServer.Auth().CreateUser(ctx, newUser)
	require.NoError(t, err)
}

func mustGetUserIdentityFromWebSession(t *testing.T, webSess types.WebSession) *tlsca.Identity {
	cert, err := tlsca.ParseCertificatePEM(webSess.GetTLSCert())
	require.NoError(t, err)
	userIdentity, err := tlsca.FromSubject(cert.Subject, time.Now())
	require.NoError(t, err)
	return userIdentity
}

func prepareSSOConnectorSetup(t *testing.T, input string, ctx context.Context, s *webSuite) types.SAMLConnector {
	connector := mustUnmarshalSAMLConnector(t, input)
	mustCreateRole(t, ctx, s, connector.GetAttributesToRoles()[0].Roles[0])
	_, err := s.testAuthServer.Auth().CreateSAMLConnector(ctx, connector)
	require.NoError(t, err)
	s.testAuthServer.Auth().SetClock(clockwork.NewFakeClockAt(time.Date(2017, 5, 10, 18, 53, 0, 0, time.UTC)))
	return connector
}

func getAuthRequestAndSwapID(t *testing.T, ctx context.Context, s *webSuite, id string, csrfToken string) {
	authRequest, err := s.testAuthServer.Auth().GetSAMLAuthRequest(context.Background(), id)
	require.NoError(t, err)
	// now swap the request id to the hardcoded one in fixtures
	authRequest.ID = fixtures.SAMLOktaAuthRequestID
	authRequest.CSRFToken = csrfToken
	err = s.testAuthServer.Auth().Services.CreateSAMLAuthRequest(ctx, *authRequest, backend.Forever)
	require.NoError(t, err)
}

func mustCreateRole(t *testing.T, ctx context.Context, s *webSuite, name string) {
	_, err := auth.CreateRole(ctx, s.testAuthServer.Auth(), name, types.RoleSpecV6{
		Options: types.RoleOptions{
			MaxSessionTTL: types.NewDuration(apidefaults.MaxCertDuration),
		},
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			Namespaces: []string{apidefaults.Namespace},
			Rules: []types.Rule{
				types.NewRule(types.Wildcard, services.RW()),
			},
			Logins: []string{s.user},
		},
	})
	require.NoError(t, err)
}

func mustSendSAMLResponse(t *testing.T, clt *client.WebClient, csrfCookie *http.Cookie) *roundtrip.Response {
	in := &bytes.Buffer{}
	fw, err := flate.NewWriter(in, flate.DefaultCompression)
	require.NoError(t, err)
	_, err = fw.Write([]byte(fixtures.SAMLOktaAuthnResponseXML))
	require.NoError(t, err)
	err = fw.Close()
	require.NoError(t, err)
	encodedResponse := base64.StdEncoding.EncodeToString(in.Bytes())
	require.NotNil(t, encodedResponse)

	// now send the response to the server to exchange it for auth session
	form := url.Values{}
	form.Add("SAMLResponse", encodedResponse)
	req, err := http.NewRequest("POST", clt.Endpoint("webapi", "saml", "acs"), strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(csrfCookie)
	require.NoError(t, err)
	resp, err := clt.Client.RoundTrip(func() (*http.Response, error) {
		return clt.Client.HTTPClient().Do(req)
	})
	require.NoError(t, err)

	// This route uses a meta redirect, so expect redirect URL in body instead of location header.
	require.Equal(t, http.StatusOK, resp.Code(), "Response: %v", string(resp.Bytes()))

	return resp
}

func mustExtractSAMLRequestID(t *testing.T, re *roundtrip.Response) string {
	urlPattern := regexp.MustCompile(`URL='([^']*)'`)
	locationURL := urlPattern.FindStringSubmatch(string(re.Bytes()))[1]
	u, err := url.Parse(locationURL)
	require.NoError(t, err)
	require.Equal(t, fixtures.SAMLOktaSSO, u.Scheme+"://"+u.Host+u.Path)
	data, err := base64.StdEncoding.DecodeString(u.Query().Get("SAMLRequest"))
	require.NoError(t, err)
	buf, err := io.ReadAll(flate.NewReader(bytes.NewReader(data)))
	require.NoError(t, err)
	doc := etree.NewDocument()
	err = doc.ReadFromBytes(buf)
	require.NoError(t, err)
	id := doc.Root().SelectAttr("ID")
	require.NotNil(t, id)
	return id.Value
}

func initSSOLogin(t *testing.T, clt *client.WebClient, connector types.SAMLConnector, csrfCookie *http.Cookie) *roundtrip.Response {
	baseURL, err := url.Parse(clt.Endpoint("webapi", "saml", "sso") + `?connector_id=` + connector.GetName() + `&redirect_url=http://localhost/after`)
	require.NoError(t, err)
	req, err := http.NewRequest("GET", baseURL.String(), nil)
	require.NoError(t, err)
	req.AddCookie(csrfCookie)
	re, err := clt.Client.RoundTrip(func() (*http.Response, error) {
		return clt.Client.HTTPClient().Do(req)
	})
	require.NoError(t, err)
	return re
}

func mustUnmarshalSAMLConnector(t *testing.T, input string) types.SAMLConnector {
	decoder := kyaml.NewYAMLOrJSONDecoder(strings.NewReader(input), defaults.LookaheadBufSize)
	var raw services.UnknownResource
	err := decoder.Decode(&raw)
	require.NoError(t, err)

	connector, err := services.UnmarshalSAMLConnector(raw.Raw)
	require.NoError(t, err)
	return connector
}
