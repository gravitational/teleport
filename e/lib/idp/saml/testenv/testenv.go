package testenv

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto/x509/pkix"
	"encoding/base64"
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

	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/teleport/lib/tlsca"
)

// BASEURL is a default listen URL for SAML IdP.
const BASEURL = "https://test.url:443"

// TEnv is a test environment for SAML IdP.
type TEnv struct {
	ClusterService services.ClusterConfiguration
	CAService      services.Trust
	SPService      services.SAMLIdPServiceProviders
	UserService    *local.IdentityService
	AccessService  *local.AccessService
	EventService   *local.EventsService
	Client         *TClient
	Emitter        *eventstest.ChannelEmitter
	Authorizer     authz.Authorizer
	KeyStore       *keystore.Manager
	GenericService *generic.Service[types.SAMLIdPServiceProvider]
}

// TClient is a test environment client for SAMl IdP.
type TClient struct {
	services.ClusterConfiguration
	services.Trust
	services.SAMLIdPServiceProviders
	services.UsersService
	services.RoleGetter
	samlidppb.SAMLIdPServiceServer
	types.Events
	services.Access

	// SigningCtx is a context that can be injected into the signing service.
	SigningCtx context.Context
}

// GetRole returns role by name
func (t TClient) GetRole(ctx context.Context, name string) (types.Role, error) {
	return t.Access.GetRole(ctx, name)
}

// GetDomainName returns "test-cluster" string as domain name
func (t TClient) GetDomainName(ctx context.Context) (string, error) {
	return "test-cluster", nil
}

// NewTEnv creates new SAML IdP test environment.
func NewTEnv(ctx context.Context, t *testing.T, clock clockwork.Clock) TEnv {
	return NewTEnvWithURL(ctx, t, clock, BASEURL)
}

// NewTEnvWithURL creates new SAML IdP test environment with baseURL.
func NewTEnvWithURL(ctx context.Context, t *testing.T, clock clockwork.Clock, baseURL string) TEnv {
	bk, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)

	// Establish local providers for interaction with the backend.
	clusterService, err := local.NewClusterConfigurationService(bk)
	require.NoError(t, err)
	caService := local.NewCAService(bk)
	spService, err := local.NewSAMLIdPServiceProviderService(bk)
	require.NoError(t, err)
	userService, err := local.NewIdentityService(bk)
	require.NoError(t, err)
	accessService := local.NewAccessService(bk)
	eventService := local.NewEventsService(bk)

	// Set up default singletons
	_, err = clusterService.UpsertAuthPreference(ctx, types.DefaultAuthPreference())
	require.NoError(t, err)
	require.NoError(t, clusterService.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig()))
	_, err = clusterService.UpsertClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig())
	require.NoError(t, err)
	_, err = clusterService.UpsertSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig())
	require.NoError(t, err)

	client := &TClient{
		ClusterConfiguration:    clusterService,
		Trust:                   caService,
		SAMLIdPServiceProviders: spService,
		UsersService:            userService,
		RoleGetter:              accessService,
		Events:                  eventService,
		Access:                  accessService,
	}

	// Set up the cluster name and the CA.
	clusterName, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterID:   "test-cluster",
		ClusterName: "test-cluster",
	})
	require.NoError(t, err)
	require.NoError(t, clusterService.SetClusterName(clusterName))

	// Create testing CA.
	ca := CreateCA(t)
	require.NoError(t, caService.CreateCertAuthority(ctx, ca))

	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Client:    eventService,
			Component: "test",
		},
		LockGetter: accessService,
	})
	require.NoError(t, err)
	authorizer, err := authz.NewAuthorizer(authz.AuthorizerOpts{
		ClusterName: "test-cluster",
		AccessPoint: client,
		LockWatcher: lockWatcher,
	})
	require.NoError(t, err)

	emitter := eventstest.NewChannelEmitter(20)
	keyStoreManager, err := keystore.NewManager(t.Context(), &servicecfg.KeystoreConfig{}, &keystore.Options{
		ClusterName:          clusterName,
		AuthPreferenceGetter: clusterService,
	})
	require.NoError(t, err)

	svc, err := generic.NewService(&generic.ServiceConfig[types.SAMLIdPServiceProvider]{
		Backend:       bk,
		PageLimit:     20,
		ResourceKind:  types.KindSAMLIdPServiceProvider,
		BackendPrefix: backend.NewKey("saml_idp_service_provider"),
		MarshalFunc:   services.MarshalSAMLIdPServiceProvider,
		UnmarshalFunc: services.UnmarshalSAMLIdPServiceProvider,
	})
	require.NoError(t, err)

	return TEnv{
		ClusterService: clusterService,
		CAService:      caService,
		SPService:      spService,
		UserService:    userService,
		AccessService:  accessService,
		EventService:   eventService,
		Client:         client,
		Emitter:        emitter,
		Authorizer:     authorizer,
		KeyStore:       keyStoreManager,
		GenericService: svc,
	}
}

// WithRole returns context with role.
func WithRole(ctx context.Context, role types.SystemRole) context.Context {
	identity := auth.TestBuiltin(role)
	return authz.ContextWithUser(ctx, identity.I)
}

// NewTestEntityDescriptor creates new entity descriptor with provided entityID.
func NewTestEntityDescriptor(entityID, acsURL string) string {
	return fmt.Sprintf(testEntityDescriptor, entityID, acsURL)
}

// CreateCA creates a new CA with preset "test-cluster" value as cluster name.
func CreateCA(t *testing.T) types.CertAuthority {
	// SAML IdP only supports RSA CA, and TestRotateCertAuthority requires that
	// this doesn't just use the same key fixture every time, actually generate
	// an RSA key here.
	signer, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA2048)
	require.NoError(t, err)
	keyPEM, err := keys.MarshalPrivateKey(signer)
	require.NoError(t, err)
	cert, err := tlsca.GenerateSelfSignedCAWithSigner(signer, pkix.Name{CommonName: "test-cluster"}, nil, time.Hour)
	require.NoError(t, err)

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.SAMLIDPCA,
		ClusterName: "test-cluster",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{{
				Cert: cert,
				Key:  keyPEM,
			}},
		},
	})
	require.NoError(t, err)

	return ca
}

// A test entity descriptor from https://sptest.iamshowcase.com/testsp_metadata.xml.
const testEntityDescriptor = `<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="%s" validUntil="2025-12-09T09:13:31.006Z">
   <md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
      <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="%s" index="0" isDefault="true"/>
   </md:SPSSODescriptor>
</md:EntityDescriptor>
`

func FindNode(node *html.Node, name string) *html.Node {
	if node.Data == name {
		return node
	}

	childNode := node.FirstChild
	for childNode != nil {
		foundNode := FindNode(childNode, name)
		if foundNode != nil {
			return foundNode
		}
		childNode = childNode.NextSibling
	}

	return nil
}

// MakeAuthnMessage returns url.Values as per SAML HTTP-Redirect binding or HTTP-POST binding format.
func MakeAuthnMessage(t *testing.T, authnRequest saml.AuthnRequest, httpMethod, relayState string) url.Values {
	t.Helper()
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
		"SAMLRequest": []string{encodedRequest},
		"RelayState":  []string{relayState},
	}
}
