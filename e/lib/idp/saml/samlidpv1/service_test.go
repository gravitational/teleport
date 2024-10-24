package samlidpv1

import (
	"context"
	"crypto/x509"
	"encoding/xml"
	"testing"

	"github.com/beevik/etree"
	"github.com/crewjam/saml"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/e/lib/idp/saml/testenv"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// tEnv is a TEnv for samlidpv1
type tEnv struct {
	AccessService  *local.AccessService
	UserService    *local.IdentityService
	SamlIDPService *SAMLIdPService
}

// newTEnv creates new tEnv with testenv.BASEURL as a base URL value.
func newTEnv(t *testing.T, clock clockwork.Clock) *tEnv {
	ctx := context.Background()
	env := testenv.NewTEnvWithURL(ctx, t, clock, testenv.BASEURL)

	samlIdPService, err := NewSAMLIdPService(SAMLIdPServiceConfig{
		Client:           env.Client,
		KeyStore:         env.KeyStore,
		Authorizer:       env.Authorizer,
		MFAAuthenticator: &fakeMFAAuthenticator{},
		Logger:           utils.NewSlogLoggerForTests(),
	})
	require.NoError(t, err)

	return &tEnv{
		AccessService:  env.AccessService,
		UserService:    env.UserService,
		SamlIDPService: samlIdPService,
	}
}

type fakeMFAAuthenticator struct {
	validCodes map[string]string // map of users to valid tokens
}

func (a *fakeMFAAuthenticator) ValidateMFAAuthResponse(ctx context.Context, resp *proto.MFAAuthenticateResponse, user string, requiredExtensions *mfav1.ChallengeExtensions) (*authz.MFAAuthData, error) {
	validCode, ok := a.validCodes[user]
	if !ok || resp.GetTOTP().GetCode() != validCode {
		return nil, trace.AccessDenied("invalid MFA")
	}
	return nil, nil
}

func TestProcessSAMLIdPRequest(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	env := newTEnv(t, clock)

	// Add a fake mfa authenticator for the requesting user.
	userName := "username"
	validTOTPCode := "valid"
	env.SamlIDPService.mfaAuthenticator = &fakeMFAAuthenticator{
		validCodes: map[string]string{
			userName: validTOTPCode,
		},
	}

	assertion := &saml.Assertion{
		ID:           "dummy-id",
		IssueInstant: clock.Now(),
		Version:      "2.0",
		Subject: &saml.Subject{
			NameID: &saml.NameID{
				Value: userName,
			},
		},
		Issuer: saml.Issuer{
			Format: "urn:oasis:names:tc:SAML:2.0:nameid-format:entity",
			Value:  "my-entity-id",
		},
	}

	ssoDescriptorBytes, err := xml.Marshal(saml.SPSSODescriptor{})
	require.NoError(t, err)

	doc := etree.NewDocument()
	doc.SetRoot(assertion.Element())

	assertionBytes, err := doc.WriteToBytes()
	require.NoError(t, err)

	// Used to validate successful assertion responses.
	validateResp := func(t *testing.T, resp *samlidppb.ProcessSAMLIdPRequestResponse) {
		respDoc := etree.NewDocument()
		require.NoError(t, respDoc.ReadFromBytes(resp.Response))

		cas, err := env.SamlIDPService.client.GetCertAuthorities(ctx, types.SAMLIDPCA, false)
		require.NoError(t, err)
		ca := cas[0]

		caKeySet := ca.GetActiveKeys()
		rawCert := caKeySet.TLS[0].Cert
		require.NotEmpty(t, rawCert)

		cert, err := tlsca.ParseCertificatePEM(rawCert)
		require.NoError(t, err)

		certStore := &dsig.MemoryX509CertificateStore{
			Roots: []*x509.Certificate{
				cert,
			},
		}

		dsigClock := dsig.NewFakeClock(clockwork.NewFakeClockAt(cert.NotBefore))
		validationCtx := dsig.NewDefaultValidationContext(certStore)
		validationCtx.Clock = dsigClock
		_, err = validationCtx.Validate(respDoc.Root())
		require.NoError(t, err)
	}

	req := &samlidppb.ProcessSAMLIdPRequestRequest{
		Assertion:                    assertionBytes,
		Destination:                  "http://destination",
		RequestId:                    "request-id",
		RequestTime:                  timestamppb.New(assertion.IssueInstant),
		MetadataUrl:                  "https://metadata",
		SignatureMethod:              dsig.RSASHA256SignatureMethod,
		ServiceProviderSsoDescriptor: ssoDescriptorBytes,
	}

	// Admin shouldn't have access
	_, err = env.SamlIDPService.ProcessSAMLIdPRequest(withRole(ctx, types.RoleAdmin), req)
	require.True(t, trace.IsAccessDenied(err))

	// Proxy should have access
	resp, err := env.SamlIDPService.ProcessSAMLIdPRequest(withRole(ctx, types.RoleProxy), req)
	require.NoError(t, err)
	validateResp(t, resp)

	// If an invalid MFA response is provided, it should fail.
	req.MfaResponse = &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{
				Code: "invalid",
			},
		},
	}
	_, err = env.SamlIDPService.ProcessSAMLIdPRequest(withRole(ctx, types.RoleProxy), req)
	require.True(t, trace.IsAccessDenied(err))

	// If a valid MFA response is provided, it should succeed.
	req.MfaResponse = &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{
				Code: validTOTPCode,
			},
		},
	}
	resp, err = env.SamlIDPService.ProcessSAMLIdPRequest(withRole(ctx, types.RoleProxy), req)
	require.NoError(t, err)
	validateResp(t, resp)
}

func TestAttributeMappingCommand(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	env := newTEnv(t, clock)
	setupUsers(t, env)

	req := &samlidppb.TestSAMLIdPAttributeMappingRequest{
		ServiceProvider: &types.SAMLIdPServiceProviderV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "testapp",
				},
			},
			Spec: types.SAMLIdPServiceProviderSpecV1{
				EntityDescriptor: "",
				AttributeMapping: []*types.SAMLAttributeMapping{
					{
						Name:  "username",
						Value: "user.metadata.name",
					},
					{
						Name:  "firstname",
						Value: "strings.upper(user.spec.traits.firstname)",
					},
				},
			},
		},
		Users: []*types.UserV2{
			{
				Kind:    types.KindUser,
				Version: types.V2,
				Metadata: types.Metadata{
					Name: "testuser",
				},
				Spec: types.UserSpecV2{

					Traits: map[string][]string{
						"firstname": {"first"},
					},
				},
			},
		},
	}

	expectedResp := &samlidppb.TestSAMLIdPAttributeMappingResponse{
		MappedAttributes: []*samlidppb.MappedAttribute{
			{
				Username: "testuser",
				MappedValues: map[string]*wrappers.StringValues{
					"username": {
						Values: []string{"testuser"},
					},
					"firstname": {
						Values: []string{"FIRST"},
					},
				},
			},
		},
	}

	userWithListVerbContext := getUserContext(ctx, "userWithListVerb")
	_, err := env.SamlIDPService.TestSAMLIdPAttributeMapping(userWithListVerbContext, req)
	require.ErrorContains(t, err, "access denied")

	userWithCreateVerbContext := getUserContext(ctx, "userWithCreateVerb")
	resp, err := env.SamlIDPService.TestSAMLIdPAttributeMapping(userWithCreateVerbContext, req)
	require.NoError(t, err)

	require.Equal(t, expectedResp, resp)
}

func getUserContext(ctx context.Context, username string) context.Context {
	return authz.ContextWithUser(ctx, authz.LocalUser{
		Username: username,
		Identity: tlsca.Identity{
			Username: username,
		},
	})
}

func setupUsers(t *testing.T, tEnv *tEnv) {
	// user with role that allows create verb on KindSAMLIdPServiceProvider
	userWithCreateVerb, err := types.NewUser("userWithCreateVerb")
	require.NoError(t, err)
	samlCreateRole, err := types.NewRole("samlcreate", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindSAMLIdPServiceProvider},
					Verbs:     []string{types.VerbList, types.VerbRead, types.VerbUpdate, types.VerbCreate, types.VerbDelete},
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = tEnv.AccessService.CreateRole(context.Background(), samlCreateRole)
	require.NoError(t, err)
	userWithCreateVerb.AddRole(samlCreateRole.GetName())
	_, err = tEnv.UserService.CreateUser(context.Background(), userWithCreateVerb)
	require.NoError(t, err)

	// user with role that allows list verb on KindSAMLIdPServiceProvider
	userWithListVerb, err := types.NewUser("userWithListVerb")
	require.NoError(t, err)
	samlListRole, err := types.NewRole("samllist", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.KindSAMLIdPServiceProvider},
					Verbs:     []string{types.VerbList, types.VerbRead},
				},
			},
		},
	})
	require.NoError(t, err)
	_, err = tEnv.AccessService.CreateRole(context.Background(), samlListRole)
	require.NoError(t, err)
	userWithListVerb.AddRole(samlListRole.GetName())
	_, err = tEnv.UserService.CreateUser(context.Background(), userWithListVerb)
	require.NoError(t, err)
}

func withRole(ctx context.Context, role types.SystemRole) context.Context {
	identity := auth.TestBuiltin(role)
	return authz.ContextWithUser(ctx, identity.I)
}
