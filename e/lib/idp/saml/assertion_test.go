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
	"github.com/gravitational/teleport/e/lib/idp/saml/attribute"
	"github.com/gravitational/teleport/e/lib/idp/saml/testenv"
	"github.com/gravitational/teleport/lib/authz"
)

func TestMakeAssertion(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClockAt(time.Now())
	env := newTEnv(ctx, t, clock)
	env.testServices.Client.SigningCtx = testenv.WithRole(ctx, types.RoleProxy)

	idp, err := env.samlIdPService.createIdP(ctx)
	require.NoError(t, err)

	// The assertion maker will use the ServiceProviderProvider to ensure
	// that associated entity IDs are present, so we need to add a service
	// provider into the backend for testing.
	sp1, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp1",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: testenv.NewTestEntityDescriptor("sp1", "https://sp1.com/acs"),
			EntityID:         "sp1",
			AttributeMapping: []*types.SAMLAttributeMapping{
				{
					Name:  "customUId",
					Value: "strings.upper(uid)",
				},
				{
					Name:  "firstname",
					Value: "user.spec.traits.firstname",
				},
				{
					Name:  "username",
					Value: "user.metadata.name",
				},
				{
					Name:  "roles",
					Value: "user.spec.roles",
				},
				{
					Name:  "roles2",
					Value: `eduPersonAffiliation.add("superadmin")`,
				},
			},
		},
	)
	require.NoError(t, err)
	require.NoError(t, env.testServices.SPService.CreateSAMLIdPServiceProvider(ctx, sp1))

	ed, err := samlsp.ParseMetadata([]byte(sp1.GetEntityDescriptor()))
	require.NoError(t, err)

	user := setupUser(t, env.testServices, clock.Now().Add(time.Hour))
	testReq := httptest.NewRequest("GET", "/", nil).WithContext(authz.ContextWithUser(ctx, user))

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
		CustomAttributes: SamlMappableAttributeToCustomAttribute(attribute.SAMLMappableUserSpec{
			Traits: map[string][]string{
				"groups":    {"g1", "g2"},
				"firstname": {"userf"},
			},
			Username: "test-user",
			Roles:    []string{"r1", "r2"},
		}),
	}

	// req.Validate mutates the original request.
	require.NoError(t, req.Validate())
	require.NoError(t, env.samlIdPService.MakeAssertion(req, session))

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
					"urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
				},
			},
			AuthnRequestsSigned:  &falseBool,
			WantAssertionsSigned: &trueBool,
			AssertionConsumerServices: []saml.IndexedEndpoint{
				{
					Binding:   "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
					Location:  "https://sp1.com/acs",
					IsDefault: &trueBool,
				},
			},
		},
		HTTPRequest:   testReq,
		RequestBuffer: reqBuffer,
		Request:       expectedAuthnReq,
		ACSEndpoint: &saml.IndexedEndpoint{
			Binding:   "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
			Location:  "https://sp1.com/acs",
			IsDefault: &trueBool,
		},
		ServiceProviderMetadata: ed,
		Assertion: testAssertion(clock, req.Assertion.Conditions, testReq.RemoteAddr, "auth-id", "https://sp1.com/acs",
			attribute.New("uid", "urn:oid:0.9.2342.19200300.100.1.1", types.SAMLURINameFormat, "test-user"),
			attribute.New("eduPersonAffiliation", "urn:oid:1.3.6.1.4.1.5923.1.1.1.1", types.SAMLURINameFormat, "group1", "group2"),
			attribute.New("customUId", "customUId", types.SAMLUnspecifiedNameFormat, "TEST-USER"),
			attribute.New("firstname", "firstname", types.SAMLUnspecifiedNameFormat, "userf"),
			attribute.New("username", "username", types.SAMLUnspecifiedNameFormat, "test-user"),
			attribute.New("roles", "roles", types.SAMLUnspecifiedNameFormat, "r1", "r2"),
			attribute.New("roles2", "roles2", types.SAMLUnspecifiedNameFormat, "r1", "r2", "superadmin"),
		),
		Now: clock.Now(),
	}

	// Ignore the HTTP request, identity provider, etree elements, and assertion IDs here.
	require.Empty(t, cmp.Diff(expectedReq, req,
		cmpopts.SortSlices(func(a, b saml.AttributeValue) bool {
			return a.Value < b.Value
		}),
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
				Format:          "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
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
						Value: types.SAMLAuthnContextPublicKeyX509ClassRef,
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
