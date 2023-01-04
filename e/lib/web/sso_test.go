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
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/services"
)

func TestSAML(t *testing.T) {
	t.Parallel()

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

			decoder := kyaml.NewYAMLOrJSONDecoder(strings.NewReader(input), defaults.LookaheadBufSize)
			var raw services.UnknownResource
			err := decoder.Decode(&raw)
			require.NoError(t, err)

			connector, err := services.UnmarshalSAMLConnector(raw.Raw)
			require.NoError(t, err)

			role, err := types.NewRoleV3(connector.GetAttributesToRoles()[0].Roles[0], types.RoleSpecV6{
				Options: types.RoleOptions{
					MaxSessionTTL: types.NewDuration(apidefaults.MaxCertDuration),
				},
				Allow: types.RoleConditions{
					NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					Namespaces: []string{apidefaults.Namespace},
					Rules: []types.Rule{
						types.NewRule(types.Wildcard, services.RW()),
					},
				},
			})
			require.NoError(t, err)
			role.SetLogins(types.Allow, []string{s.user})
			err = s.testAuthServer.Auth().UpsertRole(s.ctx, role)
			require.NoError(t, err)

			err = s.testAuthServer.Auth().UpsertSAMLConnector(ctx, connector)
			require.NoError(t, err)
			s.testAuthServer.Auth().SetClock(clockwork.NewFakeClockAt(time.Date(2017, 5, 10, 18, 53, 0, 0, time.UTC)))
			clt := s.clientNoRedirects()

			csrfToken := "2ebcb768d0090ea4368e42880c970b61865c326172a4a2343b645cf5d7f20992"
			csrfCookie := &http.Cookie{Name: csrf.CookieName, Value: csrfToken}

			baseURL, err := url.Parse(clt.Endpoint("webapi", "saml", "sso") + `?connector_id=` + connector.GetName() + `&redirect_url=http://localhost/after`)
			require.NoError(t, err)
			req, err := http.NewRequest("GET", baseURL.String(), nil)
			require.NoError(t, err)
			req.AddCookie(csrfCookie)
			re, err := clt.Client.RoundTrip(func() (*http.Response, error) {
				return clt.Client.HTTPClient().Do(req)
			})
			require.NoError(t, err)

			// we got a redirect
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

			authRequest, err := s.testAuthServer.Auth().GetSAMLAuthRequest(context.Background(), id.Value)
			require.NoError(t, err)

			// now swap the request id to the hardcoded one in fixtures
			authRequest.ID = fixtures.SAMLOktaAuthRequestID
			authRequest.CSRFToken = csrfToken
			err = s.testAuthServer.Auth().Services.CreateSAMLAuthRequest(ctx, *authRequest, backend.Forever)
			require.NoError(t, err)

			// now respond with pre-recorded request to the POST url
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
			req, err = http.NewRequest("POST", clt.Endpoint("webapi", "saml", "acs"), strings.NewReader(form.Encode()))
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(csrfCookie)
			require.NoError(t, err)
			authRe, err := clt.Client.RoundTrip(func() (*http.Response, error) {
				return clt.Client.HTTPClient().Do(req)
			})

			require.NoError(t, err)
			// This route uses a meta redirect, so expect redirect URL in body instead of location header.
			require.Equal(t, http.StatusOK, authRe.Code(), "Response: %v", string(authRe.Bytes()))
			if tc.validSession {
				// we have got valid session
				require.NotEmpty(t, authRe.Headers().Get("Set-Cookie"))
			}
			require.Contains(t, string(authRe.Bytes()), tc.expectedRedirectURL)
		})
	}
}
