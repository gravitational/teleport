/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package integrations

import (
	"bytes"
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"
	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestIntegrationsCommandTest_InvalidKind(t *testing.T) {
	t.Parallel()

	cmd := &Command{
		testArgs: testArgs{
			integration: "my-integration",
		},
	}

	err := cmd.test(t.Context(), fakeClient{
		integrationsService: fakeIntegrationsService{
			getFn: func(_ context.Context, name string) (types.Integration, error) {
				return types.NewIntegrationAzureOIDC(
					types.Metadata{Name: name},
					&types.AzureOIDCIntegrationSpecV1{
						TenantID: "12345",
						ClientID: "67890",
					},
				)
			},
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "unsupported integration subkind: azure-oidc")
}

func TestIntegrationsCommandTest_AWSOIDC(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		name   string
		format string
		output string
	}{
		{
			name:   "text format",
			format: teleport.Text,
			output: bold("AWS OIDC integration is operational.") + `

Integration Name: my-integration
Account ID: 123456789012
Assumed Role ARN: arn:aws:sts::123456789012:assumed-role/teleport/test
User ID: AROAEXAMPLE:test
`,
		},
		{
			name:   "json format",
			format: teleport.JSON,
			output: `{
    "status": "operational",
    "integration_name": "my-integration",
    "account_id": "123456789012",
    "assumed_role_arn": "arn:aws:sts::123456789012:assumed-role/teleport/test",
    "user_id": "AROAEXAMPLE:test"
}
`,
		},
		{
			name:   "yaml format",
			format: teleport.YAML,
			output: `account_id: "123456789012"
assumed_role_arn: arn:aws:sts::123456789012:assumed-role/teleport/test
integration_name: my-integration
status: operational
user_id: AROAEXAMPLE:test
`,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			cmd := &Command{
				stdout: &out,
				testArgs: testArgs{
					integration: "my-integration",
					format:      tc.format,
				},
			}

			err := cmd.test(t.Context(), fakeClient{
				integrationsService: fakeIntegrationsService{
					getFn: func(_ context.Context, name string) (types.Integration, error) {
						return types.NewIntegrationAWSOIDC(
							types.Metadata{Name: name},
							&types.AWSOIDCIntegrationSpecV1{
								RoleARN: "arn:aws:iam::123456789012:role/TeleportOIDCRole",
							},
						)
					},
				},
				awsOICDService: fakeAWSOIDCService{
					pingFn: func(_ context.Context, req *integrationv1.PingRequest) (*integrationv1.PingResponse, error) {
						require.Equal(t, "my-integration", req.GetIntegration())
						return integrationv1.PingResponse_builder{
							AccountId: "123456789012",
							Arn:       "arn:aws:sts::123456789012:assumed-role/teleport/test",
							UserId:    "AROAEXAMPLE:test",
						}.Build(), nil
					},
				},
			})
			require.NoError(t, err)

			assert.Equal(t, tc.output, out.String())
		})
	}
}

func TestIntegrationsCommandTest_AWSOIDCPropagatesErrors(t *testing.T) {
	t.Parallel()

	cmd := &Command{
		testArgs: testArgs{
			integration: "my-integration",
		},
	}

	err := cmd.test(t.Context(), fakeClient{
		integrationsService: fakeIntegrationsService{
			getFn: func(_ context.Context, name string) (types.Integration, error) {
				return types.NewIntegrationAWSOIDC(
					types.Metadata{Name: name},
					&types.AWSOIDCIntegrationSpecV1{
						RoleARN: "arn:aws:iam::123456789012:role/TeleportOIDCRole",
					},
				)
			},
		},
		awsOICDService: fakeAWSOIDCService{
			pingFn: func(context.Context, *integrationv1.PingRequest) (*integrationv1.PingResponse, error) {
				return nil, trace.AccessDenied("denied")
			},
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

type fakeAWSOIDCService struct {
	pingFn func(context.Context, *integrationv1.PingRequest) (*integrationv1.PingResponse, error)
}

func (f fakeAWSOIDCService) Ping(ctx context.Context, req *integrationv1.PingRequest, _ ...grpc.CallOption) (*integrationv1.PingResponse, error) {
	return f.pingFn(ctx, req)
}

type fakeIntegrationsService struct {
	getFn func(ctx context.Context, name string) (types.Integration, error)
}

func (f fakeIntegrationsService) GetIntegration(ctx context.Context, name string) (types.Integration, error) {
	return f.getFn(ctx, name)
}

type fakeClient struct {
	integrationsService integrationsFetcher
	awsOICDService      awsOIDCPinger
}

func (f fakeClient) IntegrationsClient() integrationsFetcher {
	return f.integrationsService
}

func (f fakeClient) AWSOIDCClient() awsOIDCPinger {
	return f.awsOICDService
}
