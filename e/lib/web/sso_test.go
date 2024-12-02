package web

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	eauth "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/secret"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/web/app"
)

var (
	csrfToken  = "2ebcb768d0090ea4368e42880c970b61865c326172a4a2343b645cf5d7f20992"
	csrfCookie = &http.Cookie{Name: csrf.CookieName, Value: csrfToken}
)

func TestSAML(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.SAML: {Enabled: true},
			},
		},
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

// TestSAMLConsole asserts that the proxy can handle SAML authentication flows
// from console (tsh) clients. The clients may send public keys in the old,
// single-key format, or the new split key format.
func TestSAMLConsole(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.SAML: {Enabled: true},
			},
		},
	})
	ctx := context.Background()

	clock := clockwork.NewFakeClockAt(time.Now())
	s := newWebSuite(t, withClock(clock))

	idp := eauth.NewFakeSAMLIdP(t, clock)

	// The response in the callback will be encrypted with this key.
	secretKey, err := secret.NewKey()
	require.NoError(t, err)

	connector, err := types.NewSAMLConnector("example", types.SAMLConnectorSpecV2{
		SSO:                      idp.SSOURL.String(),
		Issuer:                   idp.MetadataURL.String(),
		Cert:                     idp.CertPEM,
		AssertionConsumerService: "https://teleport.example.com/webapi/saml/acs",
		AttributesToRoles: []types.AttributeMapping{{
			Name:  "groups",
			Value: "devs",
			Roles: []string{"access"},
		}},
	})
	require.NoError(t, err)
	mustCreateRole(t, ctx, s, "access")
	_, err = s.testAuthServer.Auth().CreateSAMLConnector(ctx, connector)
	require.NoError(t, err)
	clt := s.clientNoRedirects()

	sshKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(sshKey.Public())
	require.NoError(t, err)
	sshPubBytes := ssh.MarshalAuthorizedKey(sshPub)

	tlsKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	tlsPubBytes, err := keys.MarshalPublicKey(tlsKey.Public())
	require.NoError(t, err)

	for _, tc := range []struct {
		desc                         string
		pubKey, sshPubKey, tlsPubKey []byte
		expectLoginError             string
		expectSSHSubjectKey          ssh.PublicKey
		expectTLSSubjectKey          crypto.PublicKey
	}{
		{
			desc:             "no keys",
			expectLoginError: "Failed to login",
		},
		{
			desc:                "single key",
			pubKey:              sshPubBytes,
			expectSSHSubjectKey: sshPub,
			expectTLSSubjectKey: sshKey.Public(),
		},
		{
			desc:                "split keys",
			sshPubKey:           sshPubBytes,
			tlsPubKey:           tlsPubBytes,
			expectSSHSubjectKey: sshPub,
			expectTLSSubjectKey: tlsKey.Public(),
		},
		{
			desc:                "only ssh",
			sshPubKey:           sshPubBytes,
			expectSSHSubjectKey: sshPub,
		},
		{
			desc:                "only tls",
			tlsPubKey:           tlsPubBytes,
			expectTLSSubjectKey: tlsKey.Public(),
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {

			// Initiate the SAML SSO login.
			redirectURL, err := initSSOLoginConsole(ctx, clt, "saml", client.SSOLoginConsoleReq{
				RedirectURL: (&url.URL{
					Scheme:   "http",
					Host:     "localhost",
					Path:     "callback",
					RawQuery: url.Values{"secret_key": []string{secretKey.String()}}.Encode(),
				}).String(),
				ConnectorID: connector.GetName(),
				SSOUserPublicKeys: client.SSOUserPublicKeys{
					PublicKey: tc.pubKey,
					SSHPubKey: tc.sshPubKey,
					TLSPubKey: tc.tlsPubKey,
				},
				CertTTL: time.Hour,
			})
			if tc.expectLoginError != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.expectLoginError)
				return
			}
			require.NoError(t, err)

			samlResponse, err := idp.ServeSSO(redirectURL)
			require.NoError(t, err)

			// Send the callback to the proxy to complete the login.
			callbackResp, err := clt.PostForm(ctx, clt.Endpoint("webapi", "saml", "acs"), url.Values{
				"SAMLResponse": {samlResponse},
			})
			require.NoError(t, err)

			// Retrieve the login response from the callback response HTML.
			sshLoginResponse := sshLoginResponseFromCallbackResponse(t, callbackResp.Reader(), secretKey)

			// Make sure the subject key in the issued SSH cert matches the
			// expected key and didn't get accidentally switched.
			if tc.expectSSHSubjectKey != nil {
				sshCert, err := apisshutils.ParseCertificate(sshLoginResponse.Cert)
				require.NoError(t, err)
				require.Equal(t, tc.expectSSHSubjectKey, sshCert.Key)
			} else {
				// No SSH cert should be issued if we didn't ask for one.
				require.Empty(t, sshLoginResponse.Cert)
			}

			// Make sure the subject key in the issued TLS cert matches the
			// expected key and didn't get accidentally switched.
			if tc.expectTLSSubjectKey != nil {
				tlsCert, err := tlsca.ParseCertificatePEM(sshLoginResponse.TLSCert)
				require.NoError(t, err)
				require.Equal(t, tc.expectTLSSubjectKey, tlsCert.PublicKey)
			} else {
				// No TLS cert should be issued if we didn't ask for one.
				require.Empty(t, sshLoginResponse.TLSCert)
			}
		})
	}
}

// TestOIDCConsole asserts that the proxy can handle OIDC authentication flows
// from console (tsh) clients. The clients may send public keys in the old,
// single-key format, or the new split key format.
func TestOIDCConsole(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.OIDC: {Enabled: true},
			},
		},
	})
	ctx := context.Background()

	s := newWebSuite(t)
	clt := s.clientNoRedirects()

	// There is no real OIDC IdP, set static valid claims for a test user.
	eauth.SetStaticOIDCTestClaims(t, s.testAuthServer.Auth(), map[string]any{
		"groups": []string{"devs"},
		"email":  "alice@example.com",
		"sub":    "00001234abcd",
	})

	idp := eauth.NewFakeOIDCIdP(t, false /* tls */)

	connector, err := types.NewOIDCConnector("example", types.OIDCConnectorSpecV3{
		IssuerURL:    idp.S.URL,
		ClientID:     "example-client-id",
		ClientSecret: "example-client-secret",
		RedirectURLs: []string{"https://localhost:3080/v1/webapi/oidc/callback"},
		Display:      "sign in with example.com",
		Scope:        []string{"foo", "bar"},
		ClaimsToRoles: []types.ClaimMapping{
			{
				Claim: "groups",
				Value: "devs",
				Roles: []string{"access"},
			},
		},
	})
	require.NoError(t, err)

	_, err = s.testAuthServer.Auth().CreateOIDCConnector(context.Background(), connector)
	require.NoError(t, err)

	_, err = auth.CreateRole(ctx, s.testAuthServer.Auth(), "access", types.RoleSpecV6{})
	require.NoError(t, err)

	// The response in the callback will be encrypted with this key.
	secretKey, err := secret.NewKey()
	require.NoError(t, err)

	sshKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(sshKey.Public())
	require.NoError(t, err)
	sshPubBytes := ssh.MarshalAuthorizedKey(sshPub)

	tlsKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	tlsPubBytes, err := keys.MarshalPublicKey(tlsKey.Public())
	require.NoError(t, err)

	for _, tc := range []struct {
		desc                         string
		pubKey, sshPubKey, tlsPubKey []byte
		expectLoginError             string
		expectSSHSubjectKey          ssh.PublicKey
		expectTLSSubjectKey          crypto.PublicKey
	}{
		{
			desc:             "no keys",
			expectLoginError: "Failed to login",
		},
		{
			desc:                "single key",
			pubKey:              sshPubBytes,
			expectSSHSubjectKey: sshPub,
			expectTLSSubjectKey: sshKey.Public(),
		},
		{
			desc:                "split keys",
			sshPubKey:           sshPubBytes,
			tlsPubKey:           tlsPubBytes,
			expectSSHSubjectKey: sshPub,
			expectTLSSubjectKey: tlsKey.Public(),
		},
		{
			desc:                "only ssh",
			sshPubKey:           sshPubBytes,
			expectSSHSubjectKey: sshPub,
		},
		{
			desc:                "only tls",
			tlsPubKey:           tlsPubBytes,
			expectTLSSubjectKey: tlsKey.Public(),
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// Initiate the OIDC SSO login.
			redirectURL, err := initSSOLoginConsole(ctx, clt, "oidc", client.SSOLoginConsoleReq{
				RedirectURL: (&url.URL{
					Scheme:   "http",
					Host:     "localhost",
					Path:     "callback",
					RawQuery: url.Values{"secret_key": []string{secretKey.String()}}.Encode(),
				}).String(),
				ConnectorID: connector.GetName(),
				SSOUserPublicKeys: client.SSOUserPublicKeys{
					PublicKey: tc.pubKey,
					SSHPubKey: tc.sshPubKey,
					TLSPubKey: tc.tlsPubKey,
				},
				CertTTL: time.Hour,
			})
			if tc.expectLoginError != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.expectLoginError)
				return
			}
			require.NoError(t, err)
			fmt.Println(redirectURL)

			u, err := url.Parse(redirectURL)
			require.NoError(t, err)
			stateToken := u.Query().Get("state")

			// Send the callback to the proxy to complete the login.
			values := url.Values{
				"code":  []string{"XXX-code"},
				"state": []string{stateToken},
			}
			callbackResp, err := clt.Get(ctx, clt.Endpoint("webapi", "oidc", "callback"), values)
			require.NoError(t, err)

			sshLoginResponse := sshLoginResponseFromCallbackResponse(t, callbackResp.Reader(), secretKey)

			// Make sure the subject key in the issued SSH cert matches the
			// expected key and didn't get accidentally switched.
			if tc.expectSSHSubjectKey != nil {
				sshCert, err := apisshutils.ParseCertificate(sshLoginResponse.Cert)
				require.NoError(t, err)
				require.Equal(t, tc.expectSSHSubjectKey, sshCert.Key)
			} else {
				// No SSH cert should be issued if we didn't ask for one.
				require.Empty(t, sshLoginResponse.Cert)
			}

			// Make sure the subject key in the issued TLS cert matches the
			// expected key and didn't get accidentally switched.
			if tc.expectTLSSubjectKey != nil {
				tlsCert, err := tlsca.ParseCertificatePEM(sshLoginResponse.TLSCert)
				require.NoError(t, err)
				require.Equal(t, tc.expectTLSSubjectKey, tlsCert.PublicKey)
			} else {
				// No TLS cert should be issued if we didn't ask for one.
				require.Empty(t, sshLoginResponse.TLSCert)
			}
		})
	}
}

// The login response we're after with the certs in it is:
// - JSON
// - encrypted with [secretKey]
// - in a query param
// - in a redirect URL
// - in an HTML document
func sshLoginResponseFromCallbackResponse(t *testing.T, responseBody io.Reader, secretKey secret.Key) *authclient.SSHLoginResponse {
	// First pull the URL from the HTML meta redirect.
	redirectURL, err := app.GetURLFromMetaRedirect(responseBody)
	require.NoError(t, err)

	// Then get the encrypted JSON out of the query param.
	u, err := url.Parse(redirectURL)
	require.NoError(t, err)
	require.Contains(t, u.Query(), "response", "redirect query did not contain response")
	ciphertext := u.Query().Get("response")

	// Then unencrypt.
	callbackPlaintext, err := secretKey.Open([]byte(ciphertext))
	require.NoError(t, err, "unencrypting github callback response")

	// Then unmarshal the JSON.
	var sshLoginResponse authclient.SSHLoginResponse
	require.NoError(t, json.Unmarshal(callbackPlaintext, &sshLoginResponse))
	return &sshLoginResponse
}

func TestSAMLNoEphemeralUser(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.SAML: {Enabled: true},
			},
		},
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

func initSSOLoginConsole(ctx context.Context, clt *client.WebClient, kind string, req client.SSOLoginConsoleReq) (string, error) {
	resp, err := clt.PostJSON(ctx, clt.Endpoint("webapi", kind, "login", "console"), req)
	if err != nil {
		return "", trace.Wrap(err)
	}

	var loginResp client.SSOLoginConsoleResponse
	if err := json.Unmarshal(resp.Bytes(), &loginResp); err != nil {
		return "", trace.Wrap(err)
	}
	return loginResp.RedirectURL, nil
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
