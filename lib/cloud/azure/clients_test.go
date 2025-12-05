// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package azure

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

type testAzureOIDCCredentials struct {
	integration types.Integration
}

func (m *testAzureOIDCCredentials) GenerateAzureOIDCToken(ctx context.Context, integration string) (string, error) {
	return "dummy-oidc-token", nil
}

func (m *testAzureOIDCCredentials) GetIntegration(ctx context.Context, name string) (types.Integration, error) {
	if m.integration == nil || m.integration.GetName() != name {
		return nil, trace.NotFound("integration %q not found", name)
	}
	return m.integration, nil
}

func TestWithAzureIntegrationCredentials(t *testing.T) {
	const integrationName = "azure"

	tests := []struct {
		name        string
		integration types.Integration
		wantErr     string
	}{
		{
			name: "valid azure integration",
			integration: &types.IntegrationV1{
				ResourceHeader: types.ResourceHeader{
					Kind:    types.KindIntegration,
					SubKind: types.IntegrationSubKindAzureOIDC,
					Version: types.V1,
					Metadata: types.Metadata{
						Name:      integrationName,
						Namespace: defaults.Namespace,
					},
				},
				Spec: types.IntegrationSpecV1{
					SubKindSpec: &types.IntegrationSpecV1_AzureOIDC{
						AzureOIDC: &types.AzureOIDCIntegrationSpecV1{
							ClientID: "baz-quux",
							TenantID: "foo-bar",
						},
					},
				},
			},
		},
		{
			name:        "integration not found",
			integration: nil,
			wantErr:     `integration "azure" not found`,
		},
		{
			name: "invalid integration type",
			integration: &types.IntegrationV1{
				ResourceHeader: types.ResourceHeader{
					Kind:    types.KindIntegration,
					SubKind: types.IntegrationSubKindAWSOIDC,
					Version: types.V1,
					Metadata: types.Metadata{
						Name:      "azure",
						Namespace: defaults.Namespace,
					},
				},
				Spec: types.IntegrationSpecV1{
					SubKindSpec: &types.IntegrationSpecV1_AWSOIDC{
						AWSOIDC: &types.AWSOIDCIntegrationSpecV1{
							RoleARN: "arn:aws:iam::123456789012:role/teleport",
						},
					},
				},
			},
			wantErr: `expected "azure" to be an "azure-oidc" integration, was "aws-oidc" instead`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := WithIntegrationCredentials(integrationName, &testAzureOIDCCredentials{
				integration: tt.integration,
			})
			clients, err := NewClients(opt)
			require.NoError(t, err)

			credential, err := clients.GetCredential(t.Context())

			if tt.wantErr == "" {
				require.NoError(t, err)
				require.NotNil(t, credential)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
				require.Nil(t, credential)
			}
		})
	}
}
