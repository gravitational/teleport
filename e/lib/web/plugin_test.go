package web

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/gravitational/teleport/api/types"
	samlidp "github.com/gravitational/teleport/e/lib/idp/saml"
	"github.com/gravitational/teleport/e/lib/idp/saml/testenv"
	"github.com/gravitational/teleport/e/lib/okta/common"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/web"
)

func TestRegisterProxyWebHandlers(t *testing.T) {
	h := newWebSuite(t).webPlugin.h

	handlerHasPath(t, h, http.MethodGet, "/enterprise/devices")

	handlerHasPath(t, h, http.MethodGet, "/enterprise/authconnectors")
	handlerHasPath(t, h, http.MethodPost, "/enterprise/saml")
	handlerHasPath(t, h, http.MethodPut, "/enterprise/saml/:name")
	handlerHasPath(t, h, http.MethodDelete, "/enterprise/saml/:name")

	handlerHasPath(t, h, http.MethodPost, "/enterprise/oidc")
	handlerHasPath(t, h, http.MethodPut, "/enterprise/oidc/:name")
	handlerHasPath(t, h, http.MethodDelete, "/enterprise/oidc/:name")

	handlerHasPath(t, h, http.MethodPost, "/webapi/saml/acs")
	handlerHasPath(t, h, http.MethodGet, "/webapi/saml/sso")
	handlerHasPath(t, h, http.MethodPost, "/webapi/saml/login/console")

	handlerHasPath(t, h, http.MethodGet, "/webapi/oidc/login/web")
	handlerHasPath(t, h, http.MethodGet, "/webapi/oidc/callback")
	handlerHasPath(t, h, http.MethodPost, "/webapi/oidc/login/console")

	handlerHasPath(t, h, http.MethodGet, "/enterprise/license/status")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/license")

	handlerHasPath(t, h, http.MethodPost, "/enterprise/accessrequest")
	handlerHasPath(t, h, http.MethodPut, "/enterprise/accessrequest")
	handlerHasPath(t, h, http.MethodDelete, "/enterprise/accessrequest/:requestId")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/accessrequest/:requestId")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/accessrequest")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/resourcerequestroles")

	handlerHasPath(t, h, http.MethodGet, "/enterprise/releases")

	handlerHasPath(t, h, http.MethodPost, "/enterprise/plugin")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/plugin")
	handlerHasPath(t, h, http.MethodDelete, "/enterprise/plugin/:name")

	handlerHasPath(t, h, http.MethodGet, "/enterprise/plugins/types")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/plugins/callback/:type")

	handlerHasPath(t, h, http.MethodGet, "/enterprise/cloud/billing")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/cloud/billing-summary")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/cloud/nonbillable-summary")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/cloud/upgradewindowstart")
	handlerHasPath(t, h, http.MethodPost, "/enterprise/cloud/upgradewindowstart")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/sites/:site/upgradewindowstart")
	handlerHasPath(t, h, http.MethodPost, "/enterprise/sites/:site/upgradewindowstart")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/cloud/survey/company")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/sites/:site/contact")
	handlerHasPath(t, h, http.MethodPost, "/enterprise/sites/:site/contact")
	handlerHasPath(t, h, http.MethodDelete, "/enterprise/sites/:site/contact")

	handlerHasPath(t, h, http.MethodPost, "/enterprise/cloud/recovery/start")
	handlerHasPath(t, h, http.MethodPost, "/enterprise/cloud/recovery/verify")
	handlerHasPath(t, h, http.MethodPost, "/enterprise/cloud/recovery/newcredentials")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/cloud/recovery/token/:token")
	handlerHasPath(t, h, http.MethodPost, "/enterprise/cloud/recovery/codes")
	handlerHasPath(t, h, http.MethodGet, "/enterprise/cloud/recovery/codes")

	handlerHasPath(t, h, http.MethodGet, fmt.Sprintf("%s/*unused", samlidp.IdPRoute))
	handlerHasPath(t, h, http.MethodPost, fmt.Sprintf("%s/*unused", samlidp.IdPRoute))
}

// handlerHasPath asserts that the handler has the given path with the given method.
func handlerHasPath(t *testing.T, h *web.Handler, method, path string) {
	handle, _, _ := h.Lookup(method, path)
	require.NotNil(t, handle, "method: %s, path: %s not found", method, path)
}

func TestWithSAMLAuthHTTPRedirectBinding(t *testing.T) {
	s := newWebSuite(t)
	client := s.client(t)
	client.HTTPClient().CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	endpoint := client.Endpoint("enterprise", "saml-idp", "sso")
	authenticatedClt := s.newAuthWebPack(t, "user").clt
	setupSAMLSP(t, s)
	t.Run("http-redirect binding without session", func(t *testing.T) {
		authnMessage := makeAuthnMessage(t, s.webServer.URL, http.MethodGet, saml.HTTPRedirectBinding)

		resp, err := client.Get(s.ctx, endpoint, authnMessage)
		require.NoError(t, err)
		require.Equal(t, http.StatusSeeOther, resp.Code())
		redirectLocation := resp.Headers().Get("Location")
		redirectURL := ssoRedirectURL(t, redirectLocation)
		originalQuery, err := rebuildSAMLRequest(redirectURL.Query())
		require.NoError(t, err)
		require.Equal(t, originalQuery, authnMessage)

		// Retry with a valid session to test redirected URL is correct.
		respForValidSession, err := authenticatedClt.HTTPClient().Get(redirectURL.String())
		require.NoError(t, err)
		defer respForValidSession.Body.Close()
		// We are just testing for a response StatusOK to ensure the
		// original GET request safely survived the redirection.
		require.Equal(t, http.StatusOK, respForValidSession.StatusCode)
	})

	t.Run("http-redirect binding with session", func(t *testing.T) {
		authnMessage := makeAuthnMessage(t, s.webServer.URL, http.MethodGet, saml.HTTPRedirectBinding)

		resp, err := authenticatedClt.Get(s.ctx, endpoint, authnMessage)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())
	})

	t.Run("http-post binding with session and request in URL", func(t *testing.T) {
		authnMessage := makeAuthnMessage(t, s.webServer.URL, http.MethodGet, saml.HTTPPostBinding)

		resp, err := authenticatedClt.Get(s.ctx, endpoint, authnMessage)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code())
	})
}

func TestWithSAMLAuthHTTPPOSTBinding(t *testing.T) {
	s := newWebSuite(t)
	setupSAMLSP(t, s)
	authnMessage := makeAuthnMessage(t, s.webServer.URL, http.MethodPost, saml.HTTPPostBinding)

	// HTTP-POST binding without session.
	client := s.client(t)
	client.HTTPClient().CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	endpoint := client.Endpoint("enterprise", "saml-idp", "sso")
	resp, err := client.PostForm(s.ctx, endpoint, authnMessage)
	require.NoError(t, err)
	require.Equal(t, http.StatusSeeOther, resp.Code())
	// The values from POST form should be available in the redirect url.
	redirectLocation := resp.Headers().Get("Location")
	redirectURL := ssoRedirectURL(t, redirectLocation)
	originalQuery, err := rebuildSAMLRequest(redirectURL.Query())
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, originalQuery.Get("Method"))
	require.Equal(t, authnMessage.Get(samlidp.SAMLRequest.String()), originalQuery.Get(samlidp.SAMLRequest.String()))
	require.Equal(t, authnMessage.Get(samlidp.RelayState.String()), originalQuery.Get(samlidp.RelayState.String()))

	// Requesting the redirectURL with session and a query param Method=POST should
	// respond with an HTML POST form.
	client = s.newAuthWebPack(t, "user").clt
	respForPOSTMethodQueryParam, err := client.HTTPClient().Get(redirectURL.String())
	require.NoError(t, err)
	defer respForPOSTMethodQueryParam.Body.Close()
	require.Equal(t, http.StatusOK, respForPOSTMethodQueryParam.StatusCode)
	htmlDoc, err := html.Parse(respForPOSTMethodQueryParam.Body)
	require.NoError(t, err)
	inputNode := testenv.FindNode(htmlDoc, "input")
	require.NotNil(t, inputNode)
	require.Equal(t, authnMessage, formInputValuesToURLValues(inputNode))

	// Forward the POST form data with session.
	// In a live SSO request, the POST form will be auto-submitted by the browser.
	resp, err = client.PostForm(s.ctx, endpoint, formInputValuesToURLValues(inputNode))
	require.NoError(t, err)
	// We are just testing for a response StatusOK to ensure the
	// original POST request safely survived the redirection.
	require.Equal(t, http.StatusOK, resp.Code())

	// verify Webauthn message is passed to the form.
	const webauthnData = "test_webauthn_data"
	newQuery := redirectURL.Query()
	newQuery.Set("Webauthn", webauthnData)
	redirectURL.RawQuery = newQuery.Encode()
	respForPOSTMethodQueryParam, err = client.HTTPClient().Get(redirectURL.String())
	require.NoError(t, err)
	defer respForPOSTMethodQueryParam.Body.Close()
	require.Equal(t, http.StatusOK, respForPOSTMethodQueryParam.StatusCode)
	htmlDoc, err = html.Parse(respForPOSTMethodQueryParam.Body)
	require.NoError(t, err)
	formNode := testenv.FindNode(htmlDoc, "form")
	formActionURl, err := url.Parse(formNode.Attr[1].Val)
	require.NoError(t, err)
	require.Equal(t, webauthnData, formActionURl.Query().Get("Webauthn"))
}

func formInputValuesToURLValues(inputNode *html.Node) url.Values {
	return url.Values{
		inputNode.Attr[1].Val:                         []string{inputNode.Attr[2].Val},
		inputNode.NextSibling.NextSibling.Attr[1].Val: []string{inputNode.NextSibling.NextSibling.Attr[2].Val},
	}
}

func ssoRedirectURL(t *testing.T, location string) *url.URL {
	parsedLocation, err := url.Parse(location)
	require.NoError(t, err)

	redirectURL := parsedLocation.Query().Get("redirect_uri")
	require.NotEmpty(t, redirectURL)

	ssoRequestQuery, err := url.ParseRequestURI(redirectURL)
	require.NoError(t, err)
	return ssoRequestQuery
}

func setupSAMLSP(t *testing.T, s *webSuite) {
	sp1, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "shortcut-name",
		},
		types.SAMLIdPServiceProviderSpecV1{
			ACSURL:     "https://sp1/acs",
			EntityID:   "https://sp1",
			RelayState: "test-relay-state",
		},
	)
	require.NoError(t, err)

	authClient := s.newAdminAuthClient(s.ctx, t)
	require.NoError(t, authClient.CreateSAMLIdPServiceProvider(s.ctx, sp1))
}

func makeAuthnMessage(t *testing.T, host, httpMethod, binding string) url.Values {
	clock := clockwork.NewRealClock()
	authnRequest := saml.AuthnRequest{
		ID:           "auth-id",
		Version:      "2.0",
		IssueInstant: clock.Now(),
		Issuer: &saml.Issuer{
			Value: "https://sp1",
		},
		Destination:     fmt.Sprintf("%s/enterprise/saml-idp/sso", host),
		ProtocolBinding: binding,
	}

	var buf bytes.Buffer
	require.NoError(t, xml.NewEncoder(&buf).Encode(authnRequest))

	var encodedRequest string
	if httpMethod == http.MethodGet {
		var compressedBuf bytes.Buffer
		flateWriter, err := flate.NewWriter(&compressedBuf, flate.DefaultCompression)
		flateWriter.Write(buf.Bytes())
		require.NoError(t, flateWriter.Close())

		encodedRequest = base64.StdEncoding.EncodeToString(compressedBuf.Bytes())
		require.NoError(t, err)
	} else {
		encodedRequest = base64.StdEncoding.EncodeToString(buf.Bytes())
	}

	return url.Values{
		samlidp.SAMLRequest.String(): []string{encodedRequest},
		samlidp.RelayState.String():  []string{"test_relay_state"},
	}
}

func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

func box[T any](v T) *T {
	result := new(T)
	*result = v
	return result
}

func TestPluginOktaStatusDetails(t *testing.T) {
	const (
		oktaOrg     = "https://example.okta.org"
		oktaAppID   = "00abc123abc123"
		oktaAppName = "test_app_unique_name"
	)

	t0 := time.Now()

	// GIVEN a cluster containing an Okta plugin with attached details in its
	// state...
	ctx := testContext(t)
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")

	testPlugin := &types.PluginV1{
		SubKind: types.PluginSubkindAccess,
		Metadata: types.Metadata{
			Labels: map[string]string{
				plugins.HostedPluginLabel: "true",
			},
			Name: types.PluginTypeOkta,
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{
				Okta: &types.PluginOktaSettings{
					OrgUrl: oktaOrg,
					SyncSettings: &types.PluginOktaSyncSettings{
						SsoConnectorId:  common.OktaSSOConnectorName,
						AppId:           oktaAppID,
						AppName:         oktaAppName,
						SyncUsers:       true,
						GroupFilters:    []string{"^Group.*"},
						AppFilters:      []string{"^App.*"},
						DefaultOwners:   []string{"^admin"},
						SyncAccessLists: true,
					},
				},
			},
		},
		Credentials: &types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
				StaticCredentialsRef: &types.PluginStaticCredentialsRef{
					Labels: map[string]string{
						"plugin": "okta",
					},
				},
			},
		},
		Status: types.PluginStatusV1{
			Code: types.PluginStatusCode_RUNNING,
			Details: &types.PluginStatusV1_Okta{
				Okta: &types.PluginOktaStatusV1{
					UsersSyncDetails: &types.PluginOktaStatusDetailsUsersSync{
						Enabled:        true,
						StatusCode:     types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_SUCCESS,
						LastFailed:     box(t0.Add(-2 * time.Hour)),
						LastSuccessful: box(t0.Add(-1 * time.Hour)),
						NumUsersSynced: 42,
					},
					AccessListsSyncDetails: &types.PluginOktaStatusDetailsAccessListsSync{
						Enabled:         true,
						StatusCode:      types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_SUCCESS,
						GroupFilters:    []string{"^Group.*"},
						AppFilters:      []string{"^App.*"},
						LastFailed:      box(t0.Add(-3 * time.Hour)),
						LastSuccessful:  box(t0.Add(-4 * time.Hour)),
						NumAppsSynced:   84,
						NumGroupsSynced: 168,
					},
					AppGroupSyncDetails: &types.PluginOktaStatusDetailsAppGroupSync{
						StatusCode:      types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_SUCCESS,
						LastFailed:      box(t0.Add(-5 * time.Hour)),
						LastSuccessful:  box(t0.Add(-6 * time.Hour)),
						NumAppsSynced:   336,
						NumGroupsSynced: 672,
					},
					ScimDetails: &types.PluginOktaStatusDetailsSCIM{
						Enabled: true,
					},
					SsoDetails: &types.PluginOktaStatusDetailsSSO{
						Enabled: true,
						AppId:   oktaAppID,
						AppName: oktaAppName,
					},
				},
			},
		},
	}

	t.Log("Installing plugin")
	require.NoError(t, s.authPlugin.PluginsService().CreatePlugin(ctx, testPlugin))

	// WHEN I query for the plugin status
	statusURL := webPack.clt.Endpoint("enterprise", "plugin", "okta")
	resp, err := webPack.clt.Get(ctx, statusURL, url.Values{})

	// EXPECT the query to succeed
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())

	// EXPECT the body to be valid JSON, describing the current status of an
	// Okta plugin
	var actual ui.Plugin
	require.NoError(t, json.Unmarshal(resp.Bytes(), &actual))

	expected := ui.Plugin{
		Name:    types.PluginTypeOkta,
		Type:    types.PluginTypeOkta,
		Details: "Okta applications and groups will be synced to Teleport",
		Spec: &ui.OktaPluginSpec{
			OktaOrgURL:              oktaOrg,
			OktaAppID:               oktaAppID,
			OktaAppName:             oktaAppName,
			DefaultOwners:           []string{"^admin"},
			TeleportSSOConnector:    common.OktaSSOConnectorName,
			EnableAccessListSync:    true,
			EnableUserSync:          true,
			AssignDefaultRoles:      true,
			EnableAppGroupSync:      true,
			EnableBidirectionalSync: true,
		},
		StatusCode: types.PluginStatusCode_RUNNING,
		Status: &ui.PluginStatusV1{
			Code:    types.PluginStatusCode_RUNNING,
			Details: &ui.PluginDetails{},
		},
	}

	// perform some basic coherence checks
	require.NotNil(t, actual.Spec)
	require.IsType(t, (*ui.OktaPluginSpec)(nil), actual.Spec)
	require.NotNil(t, actual.Status.Details.Okta)

	// time values round-tripped through JSON aren't directly comparable with
	// `require.Equal()` their source values thanks to differences in timezone
	// data, so we'll cut out the problematic details block and test it separately.
	expectedDetails := testPlugin.GetStatus().GetOkta()
	actualDetails := actual.Status.Details.Okta
	actual.Status.Details.Okta = nil

	// EXPECT that the overall structure, SsoDetails and ScimDetails are as
	// we want them to be
	require.Equal(t, expected, actual)
	require.Equal(t, expectedDetails.SsoDetails, actualDetails.SsoDetails)
	require.Equal(t, expectedDetails.ScimDetails, actualDetails.ScimDetails)

	// EXPECT that the Access List Sync details are as expected, including the
	// problematic-to-compare timestamps
	expectedACL := expectedDetails.AccessListsSyncDetails
	actualACL := actualDetails.AccessListsSyncDetails
	require.True(t, actualACL.Enabled)
	require.Equal(t, expectedACL.GroupFilters, actualACL.GroupFilters)
	require.Equal(t, expectedACL.AppFilters, actualACL.AppFilters)
	require.Equal(t, expectedACL.StatusCode, actualACL.StatusCode)
	require.True(t, expectedACL.LastFailed.Equal(*actualACL.LastFailed))
	require.True(t, expectedACL.LastSuccessful.Equal(*actualACL.LastSuccessful))
	require.Equal(t, expectedACL.NumGroupsSynced, actualACL.NumGroupsSynced)
	require.Equal(t, expectedACL.NumAppsSynced, actualACL.NumAppsSynced)

	// EXPECT that the User Sync details are as expected, including the
	// problematic-to-compare timestamps
	expectedUsers := expectedDetails.UsersSyncDetails
	actualUsers := actualDetails.UsersSyncDetails
	require.True(t, actualUsers.Enabled)
	require.Equal(t, expectedUsers.StatusCode, actualUsers.StatusCode)
	require.True(t, expectedUsers.LastFailed.Equal(*actualUsers.LastFailed))
	require.True(t, expectedUsers.LastSuccessful.Equal(*actualUsers.LastSuccessful))
	require.Equal(t, expectedUsers.NumUsersSynced, actualUsers.NumUsersSynced)

	// EXPECT that the Ap & Group Sync details are as expected, including the
	// problematic-to-compare timestamps
	expectedAppGroups := expectedDetails.AppGroupSyncDetails
	actualAppGroups := actualDetails.AppGroupSyncDetails
	require.False(t, actualAppGroups.Enabled)
	require.Equal(t, expectedAppGroups.StatusCode, actualAppGroups.StatusCode)
	require.True(t, expectedAppGroups.LastFailed.Equal(*actualAppGroups.LastFailed))
	require.True(t, expectedAppGroups.LastSuccessful.Equal(*actualAppGroups.LastSuccessful))
	require.Equal(t, expectedAppGroups.NumAppsSynced, actualAppGroups.NumAppsSynced)
	require.Equal(t, expectedAppGroups.NumGroupsSynced, actualAppGroups.NumGroupsSynced)
}
