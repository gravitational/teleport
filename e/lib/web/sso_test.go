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
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

var (
	csrfToken  = "2ebcb768d0090ea4368e42880c970b61865c326172a4a2343b645cf5d7f20992"
	csrfCookie = &http.Cookie{Name: csrf.CookieName, Value: csrfToken}
)

func TestSAML(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                string
		rawConnector        string
		validSession        bool
		expectedRedirectURL string
		hasAccessListRoles  bool
	}{
		{
			name:                "mapped claims to roles without access list roles",
			rawConnector:        fixtures.SAMLOktaConnectorV2,
			validSession:        true,
			expectedRedirectURL: "/after",
		},
		{
			name:                "mapped claims to roles with access list roles",
			rawConnector:        fixtures.SAMLOktaConnectorV2,
			validSession:        true,
			hasAccessListRoles:  true,
			expectedRedirectURL: "/after",
		},
		{
			name:                "fail to map claims to roles without access list roles",
			rawConnector:        strings.ReplaceAll(fixtures.SAMLOktaConnectorV2, "Everyone", "No-one"),
			validSession:        false,
			expectedRedirectURL: sso.LoginFailedUnauthorizedRedirectURL,
		},
		{
			name:                "fail to map claims to roles with access list roles",
			rawConnector:        strings.ReplaceAll(fixtures.SAMLOktaConnectorV2, "Everyone", "No-one"),
			validSession:        true,
			hasAccessListRoles:  true,
			expectedRedirectURL: "/after",
		},
		{
			name:                "no claims to roles without access list roles",
			rawConnector:        fixtures.SAMLOktaConnectorV2WithoutRoleMapping,
			validSession:        false,
			expectedRedirectURL: sso.LoginFailedUnauthorizedRedirectURL,
		},
		{
			name:                "no claims to roles with access list roles",
			rawConnector:        fixtures.SAMLOktaConnectorV2WithoutRoleMapping,
			validSession:        true,
			hasAccessListRoles:  true,
			expectedRedirectURL: "/after",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			s := newWebSuite(t,
				withClock(clockwork.NewFakeClockAt(time.Date(2017, 5, 10, 18, 53, 0, 0, time.UTC))),
				withModules(&modulestest.Modules{
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.SAML: {Enabled: true},
						},
					},
				}),
			)
			input := tc.rawConnector

			if tc.hasAccessListRoles {
				mustCreateRole(t, ctx, s, "access")
				mustSetupAccessList(t, ctx, s, "ops@gravitational.io", "access")
			}

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
	t.Parallel()
	ctx := t.Context()

	tests := []struct {
		name                    string
		rawConnector            string
		hasAccessListRoles      bool
		hasConnectorMappedRoles bool
	}{
		{
			name:                    "mapped claims to roles without access list roles",
			rawConnector:            fixtures.SAMLOktaConnectorV2,
			hasConnectorMappedRoles: true,
		},
		{
			name:                    "mapped claims to roles with access list roles",
			rawConnector:            fixtures.SAMLOktaConnectorV2,
			hasAccessListRoles:      true,
			hasConnectorMappedRoles: true,
		},
		{
			name:         "fail to map claims to roles without access list roles",
			rawConnector: strings.ReplaceAll(fixtures.SAMLOktaConnectorV2, "Everyone", "No-one"),
		},
		{
			name:               "fail to map claims to roles with access list roles",
			rawConnector:       strings.ReplaceAll(fixtures.SAMLOktaConnectorV2, "Everyone", "No-one"),
			hasAccessListRoles: true,
		},
		{
			name:         "no claims to roles without access list roles",
			rawConnector: fixtures.SAMLOktaConnectorV2WithoutRoleMapping,
		},
		{
			name:               "no claims to roles with access list roles",
			rawConnector:       fixtures.SAMLOktaConnectorV2WithoutRoleMapping,
			hasAccessListRoles: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newWebSuite(t,
				withClock(clockwork.NewFakeClockAt(time.Date(2017, 5, 10, 18, 53, 0, 0, time.UTC))),
				withModules(&modulestest.Modules{
					TestFeatures: modules.Features{
						Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
							entitlements.SAML: {Enabled: true},
						},
					},
				}),
			)

			const username = "ops@gravitational.io"
			const connectorMappedRole = "admin"
			const accessListRole = "access"

			oktaUserTraits := map[string][]string{"okta/org": {"dev"}}
			mustCreateOktaPermanentUser(t, ctx, s, oktaUserTraits, username)

			if tc.hasAccessListRoles {
				mustCreateRole(t, ctx, s, accessListRole)
				mustSetupAccessList(t, ctx, s, username, accessListRole)
			}

			connector := prepareSSOConnectorSetup(t, tc.rawConnector, ctx, s)

			clt := s.clientNoRedirects()
			resp := initSSOLogin(t, clt, connector, csrfCookie)
			id := mustExtractSAMLRequestID(t, resp)
			getAuthRequestAndSwapID(t, ctx, s, id, csrfToken)
			mustSendSAMLResponse(t, clt, csrfCookie)

			webSessions, err := s.testAuthServer.AuthServer.AuthServer.WebSessions().List(ctx)
			require.NoError(t, err)

			// If user has no roles from any source, assert no web session created
			// and return from test with no further assertions.
			if !tc.hasAccessListRoles && !tc.hasConnectorMappedRoles {
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

			// Check that the user has the correct traits
			// propagated from permanent SAML user created by Okta service during user sync
			// and traits from the SAML assertion.
			want := wrappers.Traits(oktaUserTraits)
			want["groups"] = []string{"Everyone"}
			require.Equal(t, want, userIdentity.Traits)
		})
	}
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
	attributesToRoles := connector.GetAttributesToRoles()

	// If attributes_to_roles is present, then it's fine to go through the normal write path.
	// In the case there's no attributes_to_roles, go straight through the backend.
	// This is to satisfy current constraint that a connector without attributes_to_roles
	// is valid to be stored and read (phase 1), but not to write (phase 2).
	// See RFD: https://github.com/gravitational/rfd/blob/main/rfd/0248-sso-connector-without-role-mapping.md
	// TODO(nixpig): Post phase 2 (v19.0.x) remove this condition and go through the normal
	// write path for both.
	if len(attributesToRoles) > 0 {
		for _, attribute := range connector.GetAttributesToRoles() {
			mustCreateRole(t, ctx, s, attribute.Roles[0])
		}
		_, err := s.testAuthServer.Auth().CreateSAMLConnector(ctx, connector)
		require.NoError(t, err)
	} else {
		mustCreateBackendConnector(t, ctx, connector, s)
	}

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
	_, err := authtest.CreateRole(ctx, s.testAuthServer.Auth(), name, types.RoleSpecV6{
		Options: types.RoleOptions{
			MaxSessionTTL: types.NewDuration(apidefaults.MaxCertDuration),
		},
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
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
	require.NoError(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(csrfCookie)
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
	return mustExtractSAMLRequestIDFromURL(t, locationURL, fixtures.SAMLOktaSSO)
}

func mustExtractSAMLRequestIDFromURL(t *testing.T, redirectURL, expectedHostPath string) string {
	u, err := url.Parse(redirectURL)
	require.NoError(t, err)
	require.Equal(t, expectedHostPath, u.Scheme+"://"+u.Host+u.Path)
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

// mustSetupAccessList an Access List that grants the role and adds the username as an Access List Member.
func mustSetupAccessList(t *testing.T, ctx context.Context, s *webSuite, username, role string) {
	clock := s.testAuthServer.Auth().GetClock()

	mustCreateRole(t, ctx, s, role)

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

	_, err = s.testAuthServer.Auth().UpsertAccessList(ctx, accessList)
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

	_, err = s.testAuthServer.Auth().UpsertAccessListMember(ctx, accessListMember)
	require.NoError(t, err)
}

// mustCreateBackendConnector creates a SAML connector directly in the backend.
// This avoids going through the write path validations and is needed to create
// a connector without role-mapping fields, which is valid for read paths and SSO.
func mustCreateBackendConnector(t *testing.T, ctx context.Context, connector types.SAMLConnector, s *webSuite) {
	value, err := utils.FastMarshal(connector)
	require.NoError(t, err)

	samlConnectorBackendKey := backend.NewKey("web", "connectors", "saml", "connectors", connector.GetName())
	_, err = s.testAuthServer.AuthServer.Backend.Put(ctx, backend.Item{
		Key:   samlConnectorBackendKey,
		Value: value,
	})
	require.NoError(t, err)
}
