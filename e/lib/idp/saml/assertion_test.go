package saml

import (
	"context"
	"crypto/x509"
	"encoding/xml"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

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
	t.Parallel()
	tests := []struct {
		name             string
		attributeMapping []*types.SAMLAttributeMapping
		traits           map[string][]string
		wantAttributes   []saml.AttributeStatement
	}{
		{
			name:             "no custom mapping keeps defaults",
			attributeMapping: nil,
			wantAttributes: []saml.AttributeStatement{{
				Attributes: []saml.Attribute{
					attribute.New("uid", types.SAMLUIDName, types.SAMLURINameFormat, "test-user"),
					attribute.New("eduPersonAffiliation", types.SAMLEduPersonAffiliationName, types.SAMLURINameFormat, "group1", "group2"),
				},
			}},
		},
		{
			name: "custom mapping adds attributes alongside defaults",
			attributeMapping: []*types.SAMLAttributeMapping{
				{Name: "customUId", Value: "strings.upper(uid)"},
				{Name: "firstname", Value: "user.spec.traits.firstname"},
				{Name: "username", Value: "user.metadata.name"},
				{Name: "roles", Value: "user.spec.roles"},
				{Name: "roles2", Value: `eduPersonAffiliation.add("superadmin")`},
			},
			wantAttributes: []saml.AttributeStatement{{
				Attributes: []saml.Attribute{
					attribute.New("customUId", "customUId", types.SAMLUnspecifiedNameFormat, "TEST-USER"),
					attribute.New("firstname", "firstname", types.SAMLUnspecifiedNameFormat, "userf"),
					attribute.New("username", "username", types.SAMLUnspecifiedNameFormat, "test-user"),
					attribute.New("roles", "roles", types.SAMLUnspecifiedNameFormat, "r1", "r2"),
					attribute.New("roles2", "roles2", types.SAMLUnspecifiedNameFormat, "r1", "r2", "superadmin"),
					attribute.New("uid", types.SAMLUIDName, types.SAMLURINameFormat, "test-user"),
					attribute.New("eduPersonAffiliation", types.SAMLEduPersonAffiliationName, types.SAMLURINameFormat, "group1", "group2"),
				},
			}},
		},
		{
			name: "custom eduPersonAffiliation",
			attributeMapping: []*types.SAMLAttributeMapping{
				{
					Name:       types.SAMLEduPersonAffiliationName,
					NameFormat: types.SAMLURINameFormat,
					Value:      `user.spec.roles`,
				},
			},
			wantAttributes: []saml.AttributeStatement{{
				Attributes: []saml.Attribute{
					attribute.New("uid", types.SAMLUIDName, types.SAMLURINameFormat, "test-user"),
					attribute.New(types.SAMLEduPersonAffiliationName, types.SAMLEduPersonAffiliationName, types.SAMLURINameFormat, "r1", "r2"),
				},
			}},
		},
		{
			name: "custom empty eduPersonAffiliation should remove attribute",
			attributeMapping: []*types.SAMLAttributeMapping{
				{
					Name:       types.SAMLEduPersonAffiliationName,
					NameFormat: types.SAMLURINameFormat,
					Value:      `set()`,
				},
			},
			wantAttributes: []saml.AttributeStatement{{
				Attributes: []saml.Attribute{
					attribute.New("uid", types.SAMLUIDName, types.SAMLURINameFormat, "test-user"),
				},
			}},
		},
	}

	ctx := context.Background()
	clock := clockwork.NewFakeClockAt(time.Now())
	env := newTEnv(ctx, t, clock)
	env.testServices.Client.SigningCtx = testenv.WithRole(ctx, types.RoleProxy)

	idp, err := env.samlIdPService.createIdP(ctx)
	require.NoError(t, err)

	certStore := &dsig.MemoryX509CertificateStore{
		Roots: []*x509.Certificate{idp.Certificate},
	}
	validationCtx := dsig.NewDefaultValidationContext(certStore)
	validationCtx.Clock = dsig.NewFakeClock(clockwork.NewFakeClockAt(idp.Certificate.NotBefore))

	user := setupUser(t, env.testServices, clock.Now().Add(time.Hour))

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spID := fmt.Sprintf("sp%d", i)
			sp, err := types.NewSAMLIdPServiceProvider(
				types.Metadata{Name: spID},
				types.SAMLIdPServiceProviderSpecV1{
					EntityDescriptor: testenv.NewTestEntityDescriptor(spID, "https://"+spID+".com/acs"),
					EntityID:         spID,
					AttributeMapping: tt.attributeMapping,
				},
			)
			require.NoError(t, err)
			require.NoError(t, env.testServices.SPService.CreateSAMLIdPServiceProvider(ctx, sp))

			ed, err := samlsp.ParseMetadata([]byte(sp.GetEntityDescriptor()))
			require.NoError(t, err)

			testReq := httptest.NewRequest("GET", "/", nil).WithContext(authz.ContextWithUser(ctx, user))

			authnReq := saml.AuthnRequest{
				ID:           "auth-id",
				Version:      "2.0",
				IssueInstant: clock.Now(),
				Issuer:       &saml.Issuer{Value: spID},
			}
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

			require.NoError(t, req.Validate())
			require.NoError(t, env.samlIdPService.MakeAssertion(req, session))

			wantAssertion := testAssertion(clock, req.Assertion.Conditions, testReq.RemoteAddr,
				spID, "auth-id", "https://"+spID+".com/acs", tt.wantAttributes[0].Attributes...)
			require.Empty(t, cmp.Diff(wantAssertion, req.Assertion,
				cmpopts.SortSlices(func(a, b saml.Attribute) bool { return a.Name < b.Name }),
				cmpopts.SortSlices(func(a, b saml.AttributeValue) bool { return a.Value < b.Value }),
				cmpopts.IgnoreFields(saml.Assertion{}, "ID", "IssueInstant", "Signature"),
				cmpopts.IgnoreFields(saml.AuthnStatement{}, "AuthnInstant", "SessionIndex"),
			))
			_, err = validationCtx.Validate(req.ResponseEl)
			require.NoError(t, err)
		})
	}
}

// testAssertion creates a test assertion with the given inputs.
func testAssertion(clock clockwork.Clock, conditions *saml.Conditions, address, spEntityID, inResponseTo, recipient string,
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
				SPNameQualifier: spEntityID,
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
						Value: spEntityID,
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
