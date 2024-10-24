package auth

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/coreos/go-oidc/jose"
	"github.com/coreos/go-oidc/oidc"
	"github.com/crewjam/saml"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
)

// FakeOIDCIdP is a configurable OIDC IdP that can be used to mock responses in
// tests. At the moment it creates an HTTP server and only responds to the
// "/.well-known/openid-configuration" endpoint.
type FakeOIDCIdP struct {
	S *httptest.Server
}

// NewFakeOIDCIdP creates a new instance of a configurable IdP.
func NewFakeOIDCIdP(t *testing.T, tls bool) *FakeOIDCIdP {
	var s FakeOIDCIdP

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", s.configurationHandler)
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	if tls {
		s.S = httptest.NewTLSServer(mux)
	} else {
		s.S = httptest.NewServer(mux)
	}
	t.Cleanup(s.S.Close)

	return &s
}

// configurationHandler returns OpenID configuration.
func (s *FakeOIDCIdP) configurationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
	"issuer": "%[1]v",
	"authorization_endpoint": "%[1]v/authz",
	"token_endpoint": "%[1]v/token",
	"jwks_uri": "%[1]v/jwks",
	"userinfo_endpoint": "%[1]v/userinfo",
	"subject_types_supported": ["public"],
	"id_token_signing_alg_values_supported": ["HS256", "RS256"]
}`, s.S.URL)
}

// SetStaticOIDCTestClaims sets the OIDCAuthService on [srv] to always use the
// static [claims] when looking up OIDC claims for any user.
func SetStaticOIDCTestClaims(t *testing.T, srv *auth.Server, claims map[string]any) {
	oas, err := NewOIDCAuthService(&OIDCAuthServiceConfig{
		Auth:    srv,
		License: ValidLicense{},
	})
	require.NoError(t, err)
	oas.getClaimsFun = func(_ context.Context, _ *oidc.Client, _ types.OIDCConnector, _ string) (jose.Claims, error) {
		return claims, nil
	}
	srv.SetOIDCService(oas)
}

// FakeSAMLIdP is a fully-functional SAML IdP that can be used to serve SSO
// requests and respond with signed assertions in tests.
type FakeSAMLIdP struct {
	SSOURL      url.URL
	MetadataURL url.URL
	CertPEM     string

	idp *saml.IdentityProvider
}

// NewFakeSAMLIdP returns a functional fake SAML IdP.
func NewFakeSAMLIdP(t *testing.T, clock clockwork.Clock) *FakeSAMLIdP {
	cn := "test-sso.example.com"

	idpKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	idpCertPEM, err := tlsca.GenerateSelfSignedCAWithConfig(tlsca.GenerateCAConfig{
		Signer: idpKey,
		Entity: pkix.Name{
			CommonName:   cn,
			Organization: []string{"example"},
		},
		TTL:   defaults.CATTL,
		Clock: clock,
	})
	require.NoError(t, err)

	idpCert, err := tlsca.ParseCertificatePEM(idpCertPEM)
	require.NoError(t, err)

	f := &FakeSAMLIdP{
		SSOURL: url.URL{
			Scheme: "https",
			Host:   cn,
			Path:   "sso",
		},
		MetadataURL: url.URL{
			Scheme: "https",
			Host:   cn,
			Path:   "metadata",
		},
		CertPEM: string(idpCertPEM),
	}

	f.idp = &saml.IdentityProvider{
		SSOURL:                  f.SSOURL,
		MetadataURL:             f.MetadataURL,
		Signer:                  idpKey,
		SignatureMethod:         dsig.ECDSASHA256SignatureMethod,
		Certificate:             idpCert,
		ServiceProviderProvider: f,
		SessionProvider:         f,
		ResponseWriter:          f,
		Logger:                  stdlog.Default(),
	}

	return f
}

// ServerSSO is how tests should interact with the IdP. ServeSSO will handle a
// SAML redirect URL and respond with a signed response.
func (f *FakeSAMLIdP) ServeSSO(url string) (string, error) {
	w := httptest.NewRecorder()
	f.idp.ServeSSO(w, httptest.NewRequest("GET", url, nil))
	if w.Code != http.StatusOK {
		return "", trace.Wrap(trace.ReadError(w.Code, w.Body.Bytes()), "serving SAML SSO request")
	}
	return w.Body.String(), nil
}

// GetServiceProvider implements [saml.ServiceProviderProvider] and returns a
// valid entity descriptor for every serviceProviderID it's given.
func (f *FakeSAMLIdP) GetServiceProvider(r *http.Request, serviceProviderID string) (*saml.EntityDescriptor, error) {
	return &saml.EntityDescriptor{
		EntityID: serviceProviderID,
		SPSSODescriptors: []saml.SPSSODescriptor{{
			AssertionConsumerServices: []saml.IndexedEndpoint{{
				Location: serviceProviderID,
				Binding:  saml.HTTPPostBinding,
			}},
		}},
	}, nil
}

// GetSession implements [saml.SessionProvider] and always returns a valid
// session for a user named "alice".
func (f *FakeSAMLIdP) GetSession(w http.ResponseWriter, r *http.Request, req *saml.IdpAuthnRequest) *saml.Session {
	return &saml.Session{
		NameID:    "alice",
		UserEmail: "alice@example.com",
		CustomAttributes: []saml.Attribute{{
			Name:   "groups",
			Values: []saml.AttributeValue{{Value: "devs"}},
		}},
	}
}

// Write implements [saml.ResponseWriter] and writes the SAML response to an
// internal channel that is consumed by [f.serveSSO].
func (f *FakeSAMLIdP) Write(w http.ResponseWriter, req *saml.IdpAuthnRequest) error {
	responseForm, err := req.PostBinding()
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := io.Copy(w, strings.NewReader(responseForm.SAMLResponse)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
