/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package awsra

import (
	"context"
	"crypto/x509/pkix"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/integrations/awsra/createsession"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestGenerateCredentials(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mockCreateSessionAPI := func(ctx context.Context, req createsession.CreateSessionRequest) (*createsession.CreateSessionResponse, error) {
		return &createsession.CreateSessionResponse{
			Version:         1,
			AccessKeyID:     "mock-access-key-id",
			SecretAccessKey: "mock-secret-access-key",
			SessionToken:    "mock-session-token",
			Expiration:      clock.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}, nil
	}

	ca := newCertAuthority(t, types.AWSRACA, "cluster-name")

	req := GenerateCredentialsRequest{
		Clock:                 clock,
		TrustAnchorARN:        "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/12345678-1234-1234-1234-123456789012",
		ProfileARN:            "arn:aws:rolesanywhere:us-east-1:123456789012:profile/12345678-1234-1234-1234-123456789012",
		RoleARN:               "arn:aws:iam::123456789012:role/teleport-role",
		SubjectCommonName:     "test-common-name",
		DurationSeconds:       nil,
		AcceptRoleSessionName: true,
		KeyStoreManager:       keystore.NewSoftwareKeystoreForTests(t),
		Cache: &mockCache{
			domainName: "cluster-name",
			ca:         ca,
		},
		CreateSession: mockCreateSessionAPI,
	}

	credentials, err := GenerateCredentials(ctx, req)
	require.NoError(t, err)

	// Validate the returned credentials
	require.Equal(t, 1, credentials.Version)
	require.Equal(t, "mock-access-key-id", credentials.AccessKeyID)
	require.Equal(t, "mock-secret-access-key", credentials.SecretAccessKey)
	require.Equal(t, "mock-session-token", credentials.SessionToken)
	require.NotEmpty(t, credentials.Expiration)
}

type mockCache struct {
	domainName string
	ca         types.CertAuthority
}

// GetClusterName returns local auth domain of the current auth server
func (m *mockCache) GetClusterName(_ context.Context) (types.ClusterName, error) {
	return &types.ClusterNameV2{
		Spec: types.ClusterNameSpecV2{
			ClusterName: m.domainName,
		},
	}, nil
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (m *mockCache) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadSigningKeys bool) (types.CertAuthority, error) {
	return m.ca, nil
}

func newCertAuthority(t *testing.T, caType types.CertAuthType, domain string) types.CertAuthority {
	t.Helper()

	key, cert, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: domain}, nil, time.Minute)
	require.NoError(t, err)

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        caType,
		ClusterName: domain,
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{{
				Key:  key,
				Cert: cert,
			}},
		},
	})
	require.NoError(t, err)

	return ca
}
