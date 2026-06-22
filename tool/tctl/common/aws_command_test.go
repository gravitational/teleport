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

package common

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
)

func TestAWSCommandTestOIDC(t *testing.T) {
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
			cmd := &AWSCommand{
				stdout: &out,
				testOIDCArgs: awsOIDCTestArgs{
					integration: "my-integration",
					format:      tc.format,
				},
			}

			err := cmd.TestOIDC(context.Background(), fakeAWSOIDCClient{
				service: fakeAWSOIDCServiceClient{
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

func TestAWSCommandTestOIDCPropagatesErrors(t *testing.T) {
	t.Parallel()

	cmd := &AWSCommand{
		testOIDCArgs: awsOIDCTestArgs{
			integration: "my-integration",
		},
	}

	err := cmd.TestOIDC(context.Background(), fakeAWSOIDCClient{
		service: fakeAWSOIDCServiceClient{
			pingFn: func(context.Context, *integrationv1.PingRequest) (*integrationv1.PingResponse, error) {
				return nil, trace.AccessDenied("denied")
			},
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

type fakeAWSOIDCServiceClient struct {
	pingFn func(context.Context, *integrationv1.PingRequest) (*integrationv1.PingResponse, error)
}

func (f fakeAWSOIDCServiceClient) Ping(ctx context.Context, req *integrationv1.PingRequest, _ ...grpc.CallOption) (*integrationv1.PingResponse, error) {
	return f.pingFn(ctx, req)
}

type fakeAWSOIDCClient struct {
	service awsOIDCPinger
}

func (f fakeAWSOIDCClient) IntegrationAWSOIDCClient() awsOIDCPinger {
	return f.service
}
