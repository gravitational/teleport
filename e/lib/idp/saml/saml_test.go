package saml

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/idp/saml/samlidpv1"
	"github.com/gravitational/teleport/e/lib/idp/saml/testenv"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// tEnvWithSAMLService is a combined testenv.TEnv (sets auth service dependencies)
// and SAML IdP test environment.
type tEnvWithSAMLService struct {
	testServices   testenv.TEnv
	samlIdPService *Service
}

// tClientWithSAMLIdPV1 is a combined testenv.TClient and samlidpv1 client.
type tClientWithSAMLIdPV1 struct {
	*testenv.TClient
	samlidpv1Service *samlidpv1.SAMLIdPService
}

// SAMLIdPClient implements samlidpv1 client.
func (t tClientWithSAMLIdPV1) SAMLIdPClient() samlidppb.SAMLIdPServiceClient {
	return t
}

// ProcessSAMLIdPRequest samlidpv1 ProcessSAMLIdPRequest.
func (t tClientWithSAMLIdPV1) ProcessSAMLIdPRequest(ctx context.Context, req *samlidppb.ProcessSAMLIdPRequestRequest, _ ...grpc.CallOption) (*samlidppb.ProcessSAMLIdPRequestResponse, error) {
	if t.SigningCtx != nil {
		ctx = t.SigningCtx
	}
	return t.samlidpv1Service.ProcessSAMLIdPRequest(ctx, req)
}

// TestSAMLIdPAttributeMapping implements samlidpv1 TestSAMLIdPAttributeMapping.
func (t tClientWithSAMLIdPV1) TestSAMLIdPAttributeMapping(ctx context.Context, req *samlidppb.TestSAMLIdPAttributeMappingRequest, _ ...grpc.CallOption) (*samlidppb.TestSAMLIdPAttributeMappingResponse, error) {
	return t.samlidpv1Service.TestSAMLIdPAttributeMapping(ctx, req)
}

// newTEnvWithURL creates new SAML IdP test environment with SAML IdP service and samlidpv1 client.
func newTEnvWithURL(ctx context.Context, t *testing.T, clock clockwork.Clock, baseURL string) *tEnvWithSAMLService {
	svcs := testenv.NewTEnvWithURL(ctx, t, clock, baseURL)

	samlidpv1Service, err := samlidpv1.NewSAMLIdPService(samlidpv1.SAMLIdPServiceConfig{
		Client:           svcs.Client,
		KeyStore:         svcs.KeyStore,
		Authorizer:       svcs.Authorizer,
		MFAAuthenticator: &fakeMFAAuthenticator{},
		Logger:           logtest.NewLogger(),
	})
	require.NoError(t, err)

	ntclient := &tClientWithSAMLIdPV1{
		svcs.Client,
		samlidpv1Service,
	}

	fakeRateLimiter := func(fn httplib.HandlerFunc) httprouter.Handle {
		return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
			return fn(w, r, p)
		})
	}

	samlIdPService, err := New(ctx, Config{
		Logger:      logtest.NewLogger(),
		Clock:       clock,
		Client:      ntclient,
		AccessPoint: ntclient,
		Authorizer:  svcs.Authorizer,
		BaseURL:     baseURL,
		Emitter:     svcs.Emitter,
		HighLimiter: fakeRateLimiter,
	})
	require.NoError(t, err)

	return &tEnvWithSAMLService{testServices: svcs, samlIdPService: samlIdPService}
}

type fakeMFAAuthenticator struct{}

func (a *fakeMFAAuthenticator) ValidateMFAAuthResponse(ctx context.Context, resp *proto.MFAAuthenticateResponse, user string, requiredExtensions *mfav1.ChallengeExtensions) (*authz.MFAAuthData, error) {
	// Always succeed
	return nil, nil
}

// newTEnv creates test environemtn with testenv.BASEURL as base URL.
func newTEnv(ctx context.Context, t *testing.T, clock clockwork.Clock) *tEnvWithSAMLService {
	return newTEnvWithURL(ctx, t, clock, testenv.BASEURL)
}

func TestInitIdP(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClockAt(time.Now())

	env := newTEnvWithURL(ctx, t, clock, testenv.BASEURL)
	require.Equal(t, "test.url", env.samlIdPService.metadataURL.Host)

	// A non-standard https port should still be present in the host.
	env = newTEnvWithURL(ctx, t, clock, "https://test.url:12345")
	require.Equal(t, "test.url:12345", env.samlIdPService.metadataURL.Host)
}

func TestRotateCertAuthority(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClockAt(time.Now())

	env := newTEnv(ctx, t, clock)

	cas, err := env.testServices.CAService.GetCertAuthorities(ctx, types.SAMLIDPCA, true)
	require.NoError(t, err)
	require.Len(t, cas, 1)
	ca := cas[0]

	// Verify the current certs against the IdP metadata.
	idp, err := env.samlIdPService.createIdP(ctx)
	require.NoError(t, err)
	certsInIdP(t, ca, &idp)

	// Create a new CA and swap it in.
	newCA := testenv.CreateCA(t)
	require.NoError(t, env.testServices.CAService.CompareAndSwapCertAuthority(newCA, ca))

	// Ensure that the new CA is reflected in the IdP metadata.
	idp, err = env.samlIdPService.createIdP(ctx)
	require.NoError(t, err)
	certsInIdP(t, newCA, &idp)
}

// certsInIdP ensures that the given cert authority is mirrored in the identity provider.
//
//nolint:revive // Because we want this to be IdP.
func certsInIdP(t *testing.T, ca types.CertAuthority, idp *saml.IdentityProvider) {
	tlsKeys := ca.GetTrustedTLSKeyPairs()
	pemCert := tlsKeys[0].Cert
	cert, err := tlsca.ParseCertificatePEM(pemCert)
	require.NoError(t, err)

	require.Equal(t, cert, idp.Certificate)

	for _, kd := range idp.Metadata().IDPSSODescriptors[0].KeyDescriptors {
		// Massage the pem cert string so that it matches the data from the entity descriptor key
		pemCertString := string(pemCert)
		pemCertString = strings.Replace(pemCertString, "-----BEGIN CERTIFICATE-----", "", 1)
		pemCertString = strings.Replace(pemCertString, "-----END CERTIFICATE-----", "", 1)
		pemCertString = strings.ReplaceAll(pemCertString, "\n", "")

		require.Equal(t, pemCertString, kd.KeyInfo.X509Data.X509Certificates[0].Data)
	}
}
