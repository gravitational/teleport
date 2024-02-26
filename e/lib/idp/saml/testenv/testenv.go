package testenv

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
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
}

// TClient is a test environment client for SAMl IdP.
type TClient struct {
	services.ClusterConfiguration
	services.Trust
	services.SAMLIdPServiceProviders
	services.UsersService
	services.SAMLIdPSession
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

// CreateSAMLIdPSession creates SAML IdP session from WebSession.
func (t TClient) CreateSAMLIdPSession(ctx context.Context, req types.CreateSAMLIdPSessionRequest) (types.WebSession, error) {
	session, err := types.NewWebSession(req.SessionID, types.KindSAMLIdPSession,
		types.WebSessionSpecV2{
			User:        req.Username,
			Expires:     req.SAMLSession.ExpireTime,
			SAMLSession: req.SAMLSession,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := t.UpsertSAMLIdPSession(ctx, session); err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// NewTEnv creates new SAML IdP test environment.
func NewTEnv(ctx context.Context, t *testing.T, clock clockwork.Clock) TEnv {
	return NewTEnvWithURL(ctx, t, clock, BASEURL)
}

// NewTEnvWithURL creates new SAML IdP test environment with baseURL.
func NewTEnvWithURL(ctx context.Context, t *testing.T, clock clockwork.Clock, baseURL string) TEnv {
	backend, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)

	// Establish local providers for interaction with the backend.
	clusterService, err := local.NewClusterConfigurationService(backend)
	require.NoError(t, err)
	caService := local.NewCAService(backend)
	spService, err := local.NewSAMLIdPServiceProviderService(backend)
	require.NoError(t, err)
	userService := local.NewIdentityService(backend)
	accessService := local.NewAccessService(backend)
	eventService := local.NewEventsService(backend)

	// Set up default singletons
	require.NoError(t, clusterService.SetAuthPreference(ctx, types.DefaultAuthPreference()))
	require.NoError(t, clusterService.SetClusterAuditConfig(ctx, types.DefaultClusterAuditConfig()))
	require.NoError(t, clusterService.SetClusterNetworkingConfig(ctx, types.DefaultClusterNetworkingConfig()))
	require.NoError(t, clusterService.SetSessionRecordingConfig(ctx, types.DefaultSessionRecordingConfig()))

	client := &TClient{
		ClusterConfiguration:    clusterService,
		Trust:                   caService,
		SAMLIdPServiceProviders: spService,
		UsersService:            userService,
		SAMLIdPSession:          userService,
		RoleGetter:              accessService,
		Events:                  eventService,
		Access:                  accessService,
	}

	require.NoError(t, clusterService.SetAuthPreference(ctx, types.DefaultAuthPreference()))

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

	emitter := eventstest.NewChannelEmitter(1)

	keyStore, err := keystore.NewManager(ctx, keystore.Config{
		Software: keystore.SoftwareConfig{
			RSAKeyPairSource: native.GenerateKeyPair,
		},
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
		KeyStore:       keyStore,
	}
}

// WithRole returns context with role.
func WithRole(ctx context.Context, role types.SystemRole) context.Context {
	identity := auth.TestBuiltin(role)
	return authz.ContextWithUser(ctx, identity.I)
}

// NewTestEntityDescriptor creates new entity descriptor with provided entityID.
func NewTestEntityDescriptor(entityID string) string {
	return fmt.Sprintf(testEntityDescriptor, entityID)
}

// CreateCA creates a new CA with preset "test-cluster" value as cluster name.
func CreateCA(t *testing.T) types.CertAuthority {
	key, cert, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: "test-cluster"}, nil, time.Hour)
	require.NoError(t, err)

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.SAMLIDPCA,
		ClusterName: "test-cluster",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{{
				Cert: cert,
				Key:  key,
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
      <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sptest.iamshowcase.com/acs" index="0" isDefault="true"/>
   </md:SPSSODescriptor>
</md:EntityDescriptor>
`
