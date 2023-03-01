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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend/memory"
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
}

type testClient struct {
	services.ClusterConfiguration
	services.Trust
	services.SAMLIdPServiceProviders
	services.UsersService
	services.SAMLIdPSession
	services.RoleGetter
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

	client := testClient{
		ClusterConfiguration:    clusterService,
		Trust:                   caService,
		SAMLIdPServiceProviders: spService,
		UsersService:            userService,
		SAMLIdPSession:          userService,
		RoleGetter:              accessService,
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
	require.NoError(t, caService.CreateCertAuthority(ca))

	eventService := local.NewEventsService(backend)
	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Client:    eventService,
			Component: "test",
		},
		LockGetter: accessService,
	})
	require.NoError(t, err)
	authorizer, err := auth.NewAuthorizer(auth.AuthorizerOpts{
		ClusterName: "test-cluster",
		AccessPoint: client,
		LockWatcher: lockWatcher,
	})
	require.NoError(t, err)

	//nolint:revive // Because we want this to be IdP.
	samlIdP, err := New(ctx, Config{
		Log:         logrus.NewEntry(logrus.New()),
		Clock:       clock,
		Client:      client,
		AccessPoint: client,
		Authorizer:  authorizer,
		BaseURL:     baseURL,
	})
	require.NoError(t, err)

	return testServices{
		samlIdP:        samlIdP,
		clusterService: clusterService,
		caService:      caService,
		spService:      spService,
		userService:    userService,
		accessService:  accessService,
		eventService:   eventService,
	}
}

func newTestEntityDescriptor(entityID string) string {
	return fmt.Sprintf(testEntityDescriptor, entityID)
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
