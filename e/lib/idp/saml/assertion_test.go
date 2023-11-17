package saml

import (
	"context"
	"crypto/x509"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestMakeAssertion(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClockAt(time.Now())
	svcs := samlTestService(ctx, t, clock)
	svcs.client.signingCtx = withRole(ctx, types.RoleProxy)

	idp, err := svcs.samlIdP.createIdP(ctx)
	require.NoError(t, err)

	// The assertion maker will use the ServiceProviderProvider to ensure
	// that associated entity IDs are present, so we need to add a service
	// provider into the backend for testing.
	sp1, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp1",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newTestEntityDescriptor("sp1"),
			EntityID:         "sp1",
		},
	)
	require.NoError(t, err)
	require.NoError(t, svcs.spService.CreateSAMLIdPServiceProvider(ctx, sp1))

	ed, err := samlsp.ParseMetadata([]byte(sp1.GetEntityDescriptor()))
	require.NoError(t, err)

	testReq := httptest.NewRequest("GET", "/", nil).WithContext(ctx)

	// Create a valid AuthnRequest.
	authnReq := saml.AuthnRequest{
		ID:           "auth-id",
		Version:      "2.0",
		IssueInstant: clock.Now(),
		Issuer: &saml.Issuer{
			Value: "sp1",
		},
	}

	// Create a valid IdpAuthnRequest.

	reqBuffer, err := xml.Marshal(authnReq)
	require.NoError(t, err)
	req := &saml.IdpAuthnRequest{
		IDP:                     &idp,
		SPSSODescriptor:         &saml.SPSSODescriptor{},
		HTTPRequest:             testReq,
		RequestBuffer:           reqBuffer,
		ServiceProviderMetadata: ed,
		Now:                     clock.Now(),
	}
	session := &saml.Session{
		UserName: "test-user",
		Groups:   []string{"group1", "group2"},
	}

	// req.Validate mutates the original request.
	require.NoError(t, req.Validate())
	require.NoError(t, svcs.samlIdP.MakeAssertion(req, session))

	// Create the expected request. We'll copy a few bits of the validated request, as needed,
	// as it's been altered by the above function calls.
	expectedAuthnReq := req.Request
	expectedAuthnReq.Issuer = &saml.Issuer{
		XMLName: xml.Name{
			Space: "urn:oasis:names:tc:SAML:2.0:assertion",
			Local: "Issuer",
		},
		Value: "sp1",
	}

	// There are here so that we can pass in *bools to objects in the expected IdpAuthnRequest.
	falseBool := false
	trueBool := true

	expectedReq := &saml.IdpAuthnRequest{
		IDP: &idp,
		SPSSODescriptor: &saml.SPSSODescriptor{
			XMLName: xml.Name{
				Space: "urn:oasis:names:tc:SAML:2.0:metadata",
				Local: "SPSSODescriptor",
			},
			SSODescriptor: saml.SSODescriptor{
				RoleDescriptor: saml.RoleDescriptor{
					ProtocolSupportEnumeration: "urn:oasis:names:tc:SAML:2.0:protocol",
				},
				NameIDFormats: []saml.NameIDFormat{
					"urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified",
					"urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
				},
			},
			AuthnRequestsSigned:  &falseBool,
			WantAssertionsSigned: &trueBool,
			AssertionConsumerServices: []saml.IndexedEndpoint{
				{
					Binding:   "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
					Location:  "https://sptest.iamshowcase.com/acs",
					IsDefault: &trueBool,
				},
			},
		},
		HTTPRequest:   testReq,
		RequestBuffer: reqBuffer,
		Request:       expectedAuthnReq,
		ACSEndpoint: &saml.IndexedEndpoint{
			Binding:   "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
			Location:  "https://sptest.iamshowcase.com/acs",
			IsDefault: &trueBool,
		},
		ServiceProviderMetadata: ed,
		Assertion: testAssertion(clock, req.Assertion.Conditions, testReq.RemoteAddr, "auth-id", "https://sptest.iamshowcase.com/acs",
			attribute("uid", "urn:oid:0.9.2342.19200300.100.1.1", uriNameFormat, "test-user"),
			attribute("eduPersonAffiliation", "urn:oid:1.3.6.1.4.1.5923.1.1.1.1", uriNameFormat, "group1", "group2"),
		),
		Now: clock.Now(),
	}

	// Ignore the HTTP request, identity provider, etree elements, and assertion IDs here.
	require.Empty(t, cmp.Diff(expectedReq, req,
		cmpopts.IgnoreTypes(&saml.IdentityProvider{}, &http.Request{}, &etree.Element{}),
		cmpopts.IgnoreFields(saml.Assertion{}, "ID")))

	// Validate the signature of the resposne.
	certStore := &dsig.MemoryX509CertificateStore{
		Roots: []*x509.Certificate{
			idp.Certificate,
		},
	}
	validationCtx := dsig.NewDefaultValidationContext(certStore)
	validationCtx.Clock = dsig.NewFakeClock(clockwork.NewFakeClockAt(idp.Certificate.NotBefore))
	_, err = validationCtx.Validate(req.ResponseEl)
	require.NoError(t, err)
}

// testAssertion creates a test assertion with the given inputs.
func testAssertion(clock clockwork.Clock, conditions *saml.Conditions, address, inResponseTo, recipient string,
	attributes ...saml.Attribute) *saml.Assertion {
	return &saml.Assertion{
		IssueInstant: clock.Now(),
		Version:      "2.0",
		Issuer: saml.Issuer{
			Format: "urn:oasis:names:tc:SAML:2.0:nameid-format:entity",
			Value:  "https://test.url/enterprise/saml-idp/metadata",
		},
		Subject: &saml.Subject{
			NameID: &saml.NameID{
				NameQualifier:   "https://test.url/enterprise/saml-idp/metadata",
				SPNameQualifier: "sp1",
				Format:          "urn:oasis:names:tc:SAML:2.0:nameid-format:transient",
			},
			SubjectConfirmations: []saml.SubjectConfirmation{
				{
					Method: "urn:oasis:names:tc:SAML:2.0:cm:bearer",
					SubjectConfirmationData: &saml.SubjectConfirmationData{
						Address:      address,
						InResponseTo: inResponseTo,
						NotOnOrAfter: clock.Now().Add(saml.MaxIssueDelay),
						Recipient:    recipient,
					},
				},
			},
		},
		Conditions: &saml.Conditions{
			NotBefore:    conditions.NotBefore,
			NotOnOrAfter: conditions.NotOnOrAfter,
			AudienceRestrictions: []saml.AudienceRestriction{
				{
					Audience: saml.Audience{
						Value: "sp1",
					},
				},
			},
		},
		AuthnStatements: []saml.AuthnStatement{
			{
				SubjectLocality: &saml.SubjectLocality{
					Address: address,
				},
				AuthnContext: saml.AuthnContext{
					AuthnContextClassRef: &saml.AuthnContextClassRef{
						Value: authnContextClassRef,
					},
				},
			},
		},
		AttributeStatements: []saml.AttributeStatement{
			{
				Attributes: attributes,
			},
		},
	}
}
