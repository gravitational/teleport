package web

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/crewjam/saml"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/gravitational/teleport/api/types"
	samlidp "github.com/gravitational/teleport/e/lib/idp/saml"
	"github.com/gravitational/teleport/e/lib/idp/saml/testenv"
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

	handlerHasPath(t, h, http.MethodPost, "/enterprise/cloud/survey")

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
	authnRequest := setupSAMLSP(t, s, saml.HTTPRedirectBinding)
	authnMessage := makeAuthnMessage(t, authnRequest, http.MethodGet)

	// HTTP-Redirect binding without session.
	client := s.client(t)
	client.HTTPClient().CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	endpoint := client.Endpoint("enterprise", "saml-idp", "sso")
	resp, err := client.Get(s.ctx, endpoint, authnMessage)
	require.NoError(t, err)
	require.Equal(t, http.StatusSeeOther, resp.Code())
	redirectLocation := resp.Headers().Get("Location")
	redirectURL := ssoRedirectURL(t, redirectLocation)
	require.Equal(t, redirectURL.Query(), authnMessage)

	// HTTP-Redirect binding with session and redirectURL.
	client = s.newAuthWebPack(t, "user").clt
	respForValidSession, err := client.HTTPClient().Get(redirectURL.String())
	require.NoError(t, err)
	defer respForValidSession.Body.Close()
	// We are just testing for a response StatusOK to ensure the
	// original GET request safely survived the redirection.
	require.Equal(t, http.StatusOK, respForValidSession.StatusCode)
}

func TestWithSAMLAuthHTTPPOSTBinding(t *testing.T) {
	s := newWebSuite(t)
	authnRequest := setupSAMLSP(t, s, saml.HTTPRedirectBinding)
	authnMessage := makeAuthnMessage(t, authnRequest, http.MethodPost)

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
	require.Equal(t, http.MethodPost, redirectURL.Query().Get("Method"))
	require.Equal(t, authnMessage.Get(samlidp.SAMLRequest.String()), redirectURL.Query().Get(samlidp.SAMLRequest.String()))
	require.Equal(t, authnMessage.Get(samlidp.RelayState.String()), redirectURL.Query().Get(samlidp.RelayState.String()))

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

func setupSAMLSP(t *testing.T, s *webSuite, binding string) saml.AuthnRequest {
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

	clock := clockwork.NewRealClock()
	return saml.AuthnRequest{
		ID:           "auth-id",
		Version:      "2.0",
		IssueInstant: clock.Now(),
		Issuer: &saml.Issuer{
			Value: "https://sp1",
		},
		Destination:     fmt.Sprintf("%s/enterprise/saml-idp/sso", s.webServer.URL),
		ProtocolBinding: binding,
	}
}

func makeAuthnMessage(t *testing.T, authnRequest saml.AuthnRequest, httpMethod string) url.Values {
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
