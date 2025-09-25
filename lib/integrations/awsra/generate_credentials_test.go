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
	"maps"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/integrations/awsra/createsession"
	"github.com/gravitational/teleport/lib/service/servicecfg"
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

	cache := &mockCache{
		domainName: "cluster-name",
		ca:         ca,
	}

	keyStoreManager, err := keystore.NewManager(t.Context(), &servicecfg.KeystoreConfig{}, &keystore.Options{
		ClusterName:          &types.ClusterNameV2{Metadata: types.Metadata{Name: "cluster-name"}},
		AuthPreferenceGetter: cache,
	})
	require.NoError(t, err)

	req := GenerateCredentialsRequest{
		Clock:                 clock,
		TrustAnchorARN:        "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/12345678-1234-1234-1234-123456789012",
		ProfileARN:            "arn:aws:rolesanywhere:us-east-1:123456789012:profile/12345678-1234-1234-1234-123456789012",
		RoleARN:               "arn:aws:iam::123456789012:role/teleport-role",
		SubjectCommonName:     "test-common-name",
		DurationSeconds:       nil,
		AcceptRoleSessionName: true,
		KeyStoreManager:       keyStoreManager,
		Cache:                 cache,
		CreateSession:         mockCreateSessionAPI,
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
	domainName   string
	ca           types.CertAuthority
	appServers   []types.AppServer
	integrations map[string]types.Integration
}

func (m *mockCache) GetAuthPreference(context.Context) (types.AuthPreference, error) {
	return types.DefaultAuthPreference(), nil
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

func (m *mockCache) UpsertApplicationServer(ctx context.Context, server types.AppServer) (*types.KeepAlive, error) {
	if m.appServers == nil {
		m.appServers = []types.AppServer{}
	}

	// Ensure the public address is a valid URL.
	appURL := "https://" + server.GetApp().GetPublicAddr()
	if _, err := url.Parse(appURL); err != nil {
		return nil, trace.BadParameter("invalid public address %q for app server %q: %v", server.GetApp().GetPublicAddr(), server.GetName(), err)
	}

	m.appServers = append(m.appServers, server)
	return nil, nil
}

func (m *mockCache) GetProxies() ([]types.Server, error) {
	return []types.Server{&types.ServerV2{
		Spec: types.ServerSpecV2{
			PublicAddrs: []string{"proxy.example.com"},
		},
	}}, nil
}

func (m *mockCache) ListIntegrations(ctx context.Context, pageSize int, nextKey string) ([]types.Integration, string, error) {
	if m.integrations == nil {
		m.integrations = map[string]types.Integration{}
	}
	return slices.Collect(maps.Values(m.integrations)), "", nil
}

func (m *mockCache) UpdateIntegration(ctx context.Context, integration types.Integration) (types.Integration, error) {
	if m.integrations == nil {
		m.integrations = map[string]types.Integration{}
	}
	if _, exists := m.integrations[integration.GetName()]; !exists {
		return nil, trace.NotFound("integration %q not found", integration.GetName())
	}

	m.integrations[integration.GetName()] = integration
	return integration, nil
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

func TestEncodeCredentialProcessFormat(t *testing.T) {
	credentials := Credentials{
		Version:         1,
		AccessKeyID:     "mock-access-key-id",
		SecretAccessKey: "mock-secret-access-key",
		SessionToken:    "mock-session-token",
		Expiration:      time.Date(2030, 6, 24, 0, 0, 0, 0, time.UTC),
	}
	encoded, err := credentials.EncodeCredentialProcessFormat()
	require.NoError(t, err)

	expected := `{"Version":1,"AccessKeyId":"mock-access-key-id","SecretAccessKey":"mock-secret-access-key","SessionToken":"mock-session-token","Expiration":"2030-06-24T00:00:00Z"}`
	require.JSONEq(t, expected, encoded)
}

func TestRoleSessionNameFromSubject(t *testing.T) {
	for _, tt := range []struct {
		name     string
		subject  string
		expected string
	}{
		{
			name:     "valid subject",
			subject:  "my-service-name",
			expected: "my-service-name",
		},
		{
			name:     "using email",
			subject:  "user@example.com",
			expected: "user@example.com",
		},
		{
			name:     "using email with plus sign",
			subject:  "user+tag@example.com",
			expected: "user_tag@example.com",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := roleSessionNameFromSubject(t.Context(), tt.subject)
			require.Equal(t, tt.expected, result)
		})
	}
}
