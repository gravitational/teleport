package idp

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/crewjam/saml"
	"github.com/jonboulle/clockwork"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/stretchr/testify/assert"

	idpsaml "github.com/gravitational/teleport/e/lib/idp/saml"
	"github.com/gravitational/teleport/e/lib/idp/saml/attribute"
)

func idpServer(t *testing.T, urlIn string) *Server {
	baseURL, err := url.Parse(urlIn)
	assert.NoError(t, err)
	s := &Server{
		IDP: &saml.IdentityProvider{
			Key:            mustParsePrivateKey(t),
			Certificate:    mustParseCert(t),
			SSOURL:         *baseURL.JoinPath("sso"),
			LogoutURL:      *baseURL.JoinPath("logout"),
			MetadataURL:    *baseURL.JoinPath("metadata"),
			AssertionMaker: &saml.DefaultAssertionMaker{},
		},
	}
	s.IDP.SessionProvider = s
	s.IDP.ServiceProviderProvider = s
	return s
}

type Server struct {
	IDP *saml.IdentityProvider
	sp  *saml.ServiceProvider
}

func (s *Server) GetServiceProvider(_ *http.Request, _ string) (*saml.EntityDescriptor, error) {
	return s.sp.Metadata(), nil
}

func (s *Server) GetSession(_ http.ResponseWriter, _ *http.Request, _ *saml.IdpAuthnRequest) *saml.Session {
	return nil
}

func mustGenerateRequest(t *testing.T, id string, user string, groups []string, clock clockwork.Clock) string {
	idpServer := idpServer(t, "teleport.example.com")

	key := mustParsePrivateKey(t)
	rsaKey, ok := key.(*rsa.PrivateKey)
	assert.True(t, ok)
	idpServer.sp = &saml.ServiceProvider{
		Key:               rsaKey,
		Certificate:       mustParseCert(t),
		MetadataURL:       mustParseURL(t, "https://teleport.example.com:3080/v1/webapi/saml/acs/test"),
		AcsURL:            mustParseURL(t, "https://sp.example.com/saml2/acs/test"),
		IDPMetadata:       idpServer.IDP.Metadata(),
		AllowIDPInitiated: true,
	}

	authnReq := saml.AuthnRequest{
		ID:           id,
		Version:      "2.0",
		IssueInstant: clock.Now(),
		Issuer: &saml.Issuer{
			Value: "test",
		},
	}
	reqBuffer, err := xml.Marshal(authnReq)
	assert.NoError(t, err)

	ctx := context.Background()
	testReq := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	req := &saml.IdpAuthnRequest{
		IDP:                     idpServer.IDP,
		SPSSODescriptor:         &saml.SPSSODescriptor{},
		HTTPRequest:             testReq,
		RequestBuffer:           reqBuffer,
		ServiceProviderMetadata: idpServer.IDP.Metadata(),
		Now:                     clock.Now(),
	}

	err = req.Validate()
	assert.NoError(t, err)

	session := &saml.Session{
		UserName: user,
		Groups:   groups,
		NameID:   user,
		CustomAttributes: idpsaml.SamlMappableAttributeToCustomAttribute(attribute.SAMLMappableUserSpec{
			Traits: map[string][]string{
				"groups":    groups,
				"firstname": {user},
			},
			Username: user,
		}),
	}
	err = idpServer.IDP.AssertionMaker.MakeAssertion(req, session)
	assert.NoError(t, err)

	req.Assertion.Conditions.NotBefore = clock.Now().Add(-1 * 24 * time.Hour)
	req.Assertion.Conditions.NotOnOrAfter = clock.Now().Add(time.Hour * 24)

	err = req.MakeResponse()
	assert.NoError(t, err)

	responseDoc := etree.NewDocument()
	responseDoc.SetRoot(req.ResponseEl)
	buff, err := responseDoc.WriteToBytes()
	assert.NoError(t, err)
	certStore := &dsig.MemoryX509CertificateStore{
		Roots: []*x509.Certificate{
			idpServer.IDP.Certificate,
		},
	}
	validationCtx := dsig.NewDefaultValidationContext(certStore)
	validationCtx.Clock = dsig.NewFakeClock(clockwork.NewFakeClockAt(idpServer.IDP.Certificate.NotBefore))
	_, err = validationCtx.Validate(req.ResponseEl)
	assert.NoError(t, err)

	return string(buff)
}

func mustParseURL(t *testing.T, s string) url.URL {
	rv, err := url.Parse(s)
	assert.NoError(t, err)
	assert.NotNil(t, rv)
	return *rv
}

func extractURL(htmlPayload string) string {
	var HTML struct {
		XMLName xml.Name `xml:"html"`
		Head    struct {
			Title string `xml:"title"`
			Meta  []struct {
				HTTPEquiv string `xml:"http-equiv,attr"`
				Content   string `xml:"content,attr"`
			} `xml:"meta"`
		} `xml:"head"`
	}

	if err := xml.Unmarshal([]byte(htmlPayload), &HTML); err != nil {
		panic(err)
	}
	s := strings.TrimPrefix(strings.Split(HTML.Head.Meta[1].Content, ";")[1], "URL='")
	s = strings.TrimSuffix(s, "'")
	return s
}
