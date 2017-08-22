package saml2

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/stretchr/testify/require"
)

func TestRedirect(t *testing.T) {
	r := &http.Request{URL: &url.URL{Path: "/"}}
	w := httptest.NewRecorder()

	spURL := "https://sp.test"

	sp := SAMLServiceProvider{
		AssertionConsumerServiceURL: spURL,
		AudienceURI:                 spURL,
		IdentityProviderIssuer:      spURL,
		IdentityProviderSSOURL:      "https://idp.test/saml/sso",
		SignAuthnRequests:           false,
	}

	require.NoError(t, sp.AuthRedirect(w, r, "foobar"))
	require.Len(t, w.HeaderMap, 1, "wrong number of headers was set")
	require.Equal(t, http.StatusFound, w.Code, "wrong http status was set")

	u, err := url.Parse(w.HeaderMap.Get("Location"))
	require.NoError(t, err, "invalid url used for redirect")

	require.Equal(t, "idp.test", u.Host)
	require.Equal(t, "https", u.Scheme)
	require.Equal(t, "foobar", u.Query().Get("RelayState"))

	bs, err := base64.StdEncoding.DecodeString(u.Query().Get("SAMLRequest"))
	require.NoError(t, err, "error base64 decoding SAMLRequest query param")

	fr := flate.NewReader(bytes.NewReader(bs))

	req := AuthNRequest{}
	require.NoError(t, xml.NewDecoder(fr).Decode(&req), "Error reading/decoding from flate-compressed URL")

	iss, err := url.Parse(req.Issuer)
	require.NoError(t, err, "error parsing request issuer URL")

	require.Equal(t, "sp.test", iss.Host)
	require.WithinDuration(t, time.Now(), req.IssueInstant, time.Second, "IssueInstant was not within the expected time frame")

	dst, err := url.Parse(req.Destination)
	require.NoError(t, err, "error parsing request destination")
	require.Equal(t, "https", dst.Scheme)
	require.Equal(t, "idp.test", dst.Host)

	//Require that the destination is the same as the redirected URL, except params
	require.Equal(t, fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path), dst.String())
}

func TestRequestedAuthnContextOmitted(t *testing.T) {
	spURL := "https://sp.test"
	sp := SAMLServiceProvider{
		AssertionConsumerServiceURL: spURL,
		AudienceURI:                 spURL,
		IdentityProviderIssuer:      spURL,
		IdentityProviderSSOURL:      "https://idp.test/saml/sso",
		SignAuthnRequests:           false,
	}

	request, err := sp.BuildAuthRequest()
	require.NoError(t, err)

	doc := etree.NewDocument()
	err = doc.ReadFromString(request)
	require.NoError(t, err)

	el := doc.FindElement("./AuthnRequest/RequestedAuthnContext")
	require.Nil(t, el)
}

func TestRequestedAuthnContextIncluded(t *testing.T) {
	spURL := "https://sp.test"
	sp := SAMLServiceProvider{
		AssertionConsumerServiceURL: spURL,
		AudienceURI:                 spURL,
		IdentityProviderIssuer:      spURL,
		IdentityProviderSSOURL:      "https://idp.test/saml/sso",
		RequestedAuthnContext: &RequestedAuthnContext{
			Comparison: AuthnPolicyMatchExact,
			Contexts: []string{
				AuthnContextPasswordProtectedTransport,
			},
		},
		SignAuthnRequests: false,
	}

	request, err := sp.BuildAuthRequest()
	require.NoError(t, err)

	doc := etree.NewDocument()
	err = doc.ReadFromString(request)
	require.NoError(t, err)

	el := doc.FindElement("./AuthnRequest/RequestedAuthnContext")
	require.Equal(t, el.SelectAttrValue("Comparison", ""), "exact")
	require.Len(t, el.ChildElements(), 1)
	el = el.ChildElements()[0]
	require.Equal(t, el.Tag, "AuthnContextClassRef")
	require.Equal(t, el.Text(), AuthnContextPasswordProtectedTransport)
}
