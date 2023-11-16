/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package saml

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
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

type testServices struct {
	samlIdP        *Service //nolint:revive // Because we want this to be IdP.
	clusterService services.ClusterConfiguration
	caService      services.Trust
	spService      services.SAMLIdPServiceProviders
	userService    *local.IdentityService
	accessService  *local.AccessService
	eventService   *local.EventsService
	client         *testClient
	emitter        *eventstest.ChannelEmitter
}

type testClient struct {
	services.ClusterConfiguration
	services.Trust
	services.SAMLIdPServiceProviders
	services.UsersService
	services.SAMLIdPSession
	services.RoleGetter
	samlidppb.SAMLIdPServiceServer
	types.Events
	services.Access

	// signingCtx is a context that can be injected into the signing service.
	signingCtx     context.Context
	signingService *SigningService
}

func (t testClient) GetRole(ctx context.Context, name string) (types.Role, error) {
	return t.Access.GetRole(ctx, name)
}

func (t testClient) SAMLIdPClient() samlidppb.SAMLIdPServiceClient {
	return t
}

func (t testClient) GetDomainName(ctx context.Context) (string, error) {
	return "test-cluster", nil
}

func (t testClient) CreateSAMLIdPSession(ctx context.Context, req types.CreateSAMLIdPSessionRequest) (types.WebSession, error) {
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

// ProcessSAMLIdPRequest is a mock SAML IdP response processor for testing.
//
//nolint:revive // Because we want this to be IdP.
func (t testClient) ProcessSAMLIdPRequest(ctx context.Context, req *samlidppb.ProcessSAMLIdPRequestRequest, _ ...grpc.CallOption) (*samlidppb.ProcessSAMLIdPRequestResponse, error) {
	if t.signingCtx != nil {
		ctx = t.signingCtx
	}
	return t.signingService.ProcessSAMLIdPRequest(ctx, req)
}

func (t *testClient) ValidateMFAAuthResponse(ctx context.Context, resp *proto.MFAAuthenticateResponse, user string, passwordless bool) (*types.MFADevice, string, error) {
	return nil, "", nil
}

func samlTestService(ctx context.Context, t *testing.T, clock clockwork.Clock) testServices {
	return samlTestServiceWithURL(ctx, t, clock, "https://test.url:443")
}

func samlTestServiceWithURL(ctx context.Context, t *testing.T, clock clockwork.Clock, baseURL string) testServices {
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

	client := &testClient{
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
	ca := createCA(t)
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

	//nolint:revive // Because we want this to be IdP.
	samlIdP, err := New(ctx, Config{
		Log:         logrus.NewEntry(logrus.New()),
		Clock:       clock,
		Client:      client,
		AccessPoint: client,
		Authorizer:  authorizer,
		BaseURL:     baseURL,
		Emitter:     emitter,
	})
	require.NoError(t, err)

	keyStore, err := keystore.NewManager(ctx, keystore.Config{
		Software: keystore.SoftwareConfig{
			RSAKeyPairSource: native.GenerateKeyPair,
		},
	})
	require.NoError(t, err)
	signingService, err := NewSigningService(&SigningServiceConfig{
		Client:     client,
		KeyStore:   keyStore,
		Authorizer: authorizer,
	})
	require.NoError(t, err)
	client.signingService = signingService

	return testServices{
		samlIdP:        samlIdP,
		clusterService: clusterService,
		caService:      caService,
		spService:      spService,
		userService:    userService,
		accessService:  accessService,
		eventService:   eventService,
		client:         client,
		emitter:        emitter,
	}
}

func withRole(ctx context.Context, role types.SystemRole) context.Context {
	identity := auth.TestBuiltin(role)
	return authz.ContextWithUser(ctx, identity.I)
}

func newTestEntityDescriptor(entityID string) string {
	return fmt.Sprintf(testEntityDescriptor, entityID)
}

func createCA(t *testing.T) types.CertAuthority {
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
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
      <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sptest.iamshowcase.com/acs" index="0" isDefault="true"/>
   </md:SPSSODescriptor>
</md:EntityDescriptor>
`
