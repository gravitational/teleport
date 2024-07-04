/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package awsoidc

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

type mockIntegrationsTokenGenerator struct {
	proxies         []types.Server
	integrations    map[string]types.Integration
	tokenCallsCount int
}

// GetIntegration returns the specified integration resources.
func (m *mockIntegrationsTokenGenerator) GetIntegration(ctx context.Context, name string) (types.Integration, error) {
	if ig, found := m.integrations[name]; found {
		return ig, nil
	}

	return nil, trace.NotFound("integration not found")
}

// GetProxies returns a list of registered proxies.
func (m *mockIntegrationsTokenGenerator) GetProxies() ([]types.Server, error) {
	return m.proxies, nil
}

// GenerateAWSOIDCToken generates a token to be used to execute an AWS OIDC Integration action.
func (m *mockIntegrationsTokenGenerator) GenerateAWSOIDCToken(ctx context.Context, integration string) (string, error) {
	m.tokenCallsCount++
	return uuid.NewString(), nil
}

func TestNewSessionV1(t *testing.T) {
	ctx := context.Background()

	dummyIntegration, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "myawsintegration"},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:sts::123456789012:role/TestRole",
		},
	)
	require.NoError(t, err)

	dummyProxy, err := types.NewServer(
		"proxy-123", types.KindProxy,
		types.ServerSpecV2{
			PublicAddrs: []string{"https://localhost:3080/"},
		},
	)
	require.NoError(t, err)

	for _, tt := range []struct {
		name             string
		region           string
		integration      string
		tokenFetchCount  int
		expectedErr      require.ErrorAssertionFunc
		sessionValidator func(*testing.T, *session.Session)
	}{
		{
			name:        "valid",
			region:      "us-dummy-1",
			integration: "myawsintegration",
			expectedErr: require.NoError,
			sessionValidator: func(t *testing.T, s *session.Session) {
				require.Equal(t, aws.String("us-dummy-1"), s.Config.Region)
			},
		},
		{
			name:        "valid with empty region",
			region:      "",
			integration: "myawsintegration",
			expectedErr: require.NoError,
			sessionValidator: func(t *testing.T, s *session.Session) {
				require.Equal(t, aws.String(""), s.Config.Region)
			},
		},
		{
			name:        "not found error when integration is missing",
			region:      "us-dummy-1",
			integration: "not-found",
			expectedErr: notFoundCheck,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mockTokenGenertor := &mockIntegrationsTokenGenerator{
				proxies: []types.Server{dummyProxy},
				integrations: map[string]types.Integration{
					dummyIntegration.GetName(): dummyIntegration,
				},
			}
			awsSessionOut, err := NewSessionV1(ctx, mockTokenGenertor, tt.region, tt.integration)

			tt.expectedErr(t, err)
			if tt.sessionValidator != nil {
				tt.sessionValidator(t, awsSessionOut)
			}
			require.Zero(t, tt.tokenFetchCount)
		})
	}

}
