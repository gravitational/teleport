// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package credprovider

import (
	"context"
	"crypto"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

const (
	integrationName = "test-integration1"
	awsRegion       = " eu-central-1"
	testUser        = "test-user"
)

func TestCreateAWSConfigForIntegration(t *testing.T) {
	deps := newDepsMock(t)
	ctx := context.Background()

	t.Run("should auto init credentials during retrieve call", func(t *testing.T) {
		stsClient := &fakeSTSClient{clock: clockwork.NewFakeClock()}
		config, err := CreateAWSConfigForIntegration(ctx, Config{
			Region:                awsRegion,
			IntegrationName:       integrationName,
			IntegrationGetter:     deps,
			AWSOIDCTokenGenerator: deps,
			STSClient:             stsClient,
		})
		require.NoError(t, err)

		creds, err := config.Credentials.Retrieve(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, creds.SecretAccessKey)
	})
}

type depsMock struct {
	*local.CA
	*local.ClusterConfigurationService
	*local.IntegrationsService
	*local.PresenceService
	proxies []types.Server
}

func (d *depsMock) GenerateAWSOIDCToken(ctx context.Context, integration string) (string, error) {
	token, err := awsoidc.GenerateAWSOIDCToken(ctx, d, d, awsoidc.GenerateAWSOIDCTokenRequest{
		Integration: integration,
		Username:    testUser,
		Subject:     types.IntegrationAWSOIDCSubject,
	})
	return token, trace.Wrap(err)
}

func (d *depsMock) GetJWTSigner(ctx context.Context, ca types.CertAuthority) (crypto.Signer, error) {
	ca, err := d.CA.GetCertAuthority(ctx, ca.GetID(), true)
	if err != nil {
		return nil, err
	}
	if len(ca.GetTrustedJWTKeyPairs()) == 0 {
		return nil, trace.BadParameter("no JWT keys found")
	}
	return keys.ParsePrivateKey(ca.GetTrustedJWTKeyPairs()[0].PrivateKey)
}

func (d *depsMock) GetProxies() ([]types.Server, error) {
	return d.proxies, nil
}

func (d *depsMock) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
	return types.NewClusterName(types.ClusterNameSpecV2{ClusterName: "teleport.example.com", ClusterID: "cluster-id"})
}

func newDepsMock(t *testing.T) *depsMock {
	ctx := context.Background()
	var out depsMock
	b, err := memory.New(memory.Config{
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, b.Close())
	})
	out.CA = local.NewCAService(b)
	out.ClusterConfigurationService, err = local.NewClusterConfigurationService(b)
	require.NoError(t, err)
	out.IntegrationsService, err = local.NewIntegrationsService(b)
	require.NoError(t, err)
	out.PresenceService = local.NewPresenceService(b)

	ca := newCertAuthority(t, types.OIDCIdPCA, "teleport.example.com")
	require.NoError(t, err)

	err = out.CA.CreateCertAuthority(ctx, ca)
	require.NoError(t, err)

	oidcIntegration, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: integrationName},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:iam::111111111111:role/test-role",
		},
	)
	require.NoError(t, err)

	_, err = out.IntegrationsService.CreateIntegration(context.Background(), oidcIntegration)
	require.NoError(t, err)

	out.proxies = []types.Server{
		&types.ServerV2{Spec: types.ServerSpecV2{
			PublicAddrs: []string{"teleport.example.com"},
		}},
	}
	return &out
}

func newCertAuthority(t *testing.T, caType types.CertAuthType, domain string) types.CertAuthority {
	t.Helper()
	publicKey, privateKey, err := testauthority.New().GenerateJWT()
	require.NoError(t, err)
	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        caType,
		ClusterName: domain,
		ActiveKeys: types.CAKeySet{
			JWT: []*types.JWTKeyPair{{
				PublicKey:      publicKey,
				PrivateKey:     privateKey,
				PrivateKeyType: types.PrivateKeyType_RAW,
			}},
		},
	})
	require.NoError(t, err)
	return ca
}
