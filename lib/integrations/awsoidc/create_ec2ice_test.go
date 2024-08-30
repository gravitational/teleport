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
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/integrations/awsoidc/tags"
)

type mockCreateEC2ICEClient struct {
	subnetToName map[string]string
	err          error
}

func (m mockCreateEC2ICEClient) CreateInstanceConnectEndpoint(ctx context.Context, params *ec2.CreateInstanceConnectEndpointInput, optFns ...func(*ec2.Options)) (*ec2.CreateInstanceConnectEndpointOutput, error) {
	if m.err != nil {
		return nil, m.err
	}

	name, ok := m.subnetToName[aws.ToString(params.SubnetId)]
	if !ok {
		return nil, trace.NotFound("subnet not configured")
	}

	return &ec2.CreateInstanceConnectEndpointOutput{
		InstanceConnectEndpoint: &ec2Types.Ec2InstanceConnectEndpoint{
			InstanceConnectEndpointId: &name,
		},
	}, nil
}

func TestCreateEC2ICE_success(t *testing.T) {
	ctx := context.Background()
	mockCreateClient := &mockCreateEC2ICEClient{
		subnetToName: map[string]string{
			"subnet-id123": "eice-123",
		},
	}
	resp, err := CreateEC2ICE(ctx, mockCreateClient, CreateEC2ICERequest{
		Cluster:         "c1",
		IntegrationName: "i1",
		Endpoints: []EC2ICEEndpoint{{
			SubnetID: "subnet-id123",
		}},
	})
	require.NoError(t, err)
	require.Equal(t, "eice-123", resp.Name)
}

func TestCreateEC2ICE_success_multiple(t *testing.T) {
	ctx := context.Background()
	mockCreateClient := &mockCreateEC2ICEClient{
		subnetToName: map[string]string{
			"subnet-id123": "eice-123",
			"subnet-id456": "eice-456",
		},
	}
	resp, err := CreateEC2ICE(ctx, mockCreateClient, CreateEC2ICERequest{
		Cluster:         "c1",
		IntegrationName: "i1",
		Endpoints: []EC2ICEEndpoint{
			{SubnetID: "subnet-id123"},
			{SubnetID: "subnet-id456"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "eice-123,eice-456", resp.Name)
	require.Len(t, resp.CreatedEndpoints, 2)
	require.Equal(t, EC2ICEEndpoint{SubnetID: "subnet-id123", Name: "eice-123"}, resp.CreatedEndpoints[0])
	require.Equal(t, EC2ICEEndpoint{SubnetID: "subnet-id456", Name: "eice-456"}, resp.CreatedEndpoints[1])
}

func TestCreateEC2ICE_error_quota_reached(t *testing.T) {
	ctx := context.Background()
	mockCreateClient := &mockCreateEC2ICEClient{
		err: fmt.Errorf("api error ResourceLimitExceeded: You've reached the quota for the maximum number of Instance Connect Endpoints for this subnet. Delete unused Instance Connect Endpoints, or request a quota increase."),
	}
	_, err := CreateEC2ICE(ctx, mockCreateClient, CreateEC2ICERequest{
		Cluster:         "c1",
		IntegrationName: "i1",
		Endpoints: []EC2ICEEndpoint{{
			SubnetID: "subnet-id123",
		}},
	})
	require.ErrorContains(t, err, "api error ResourceLimitExceeded: You've reached the quota for the maximum number of Instance Connect Endpoints for this subnet. Delete unused Instance Connect Endpoints, or request a quota increase.")
}

func TestCreateEC2ICERequest(t *testing.T) {
	isBadParamErrFn := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	baseReqFn := func() CreateEC2ICERequest {
		return CreateEC2ICERequest{
			Cluster:         "teleport-cluster",
			IntegrationName: "teleportdev",
			Endpoints: []EC2ICEEndpoint{{
				SubnetID:         "subnet-123",
				SecurityGroupIDs: []string{"sg-1", "sg-2"},
			}},
		}
	}

	for _, tt := range []struct {
		name            string
		req             func() CreateEC2ICERequest
		errCheck        require.ErrorAssertionFunc
		reqWithDefaults CreateEC2ICERequest
	}{
		{
			name: "no fields",
			req: func() CreateEC2ICERequest {
				return CreateEC2ICERequest{}
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing teleport cluster name",
			req: func() CreateEC2ICERequest {
				r := baseReqFn()
				r.Cluster = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing integration name",
			req: func() CreateEC2ICERequest {
				r := baseReqFn()
				r.IntegrationName = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing subnet id",
			req: func() CreateEC2ICERequest {
				r := baseReqFn()
				r.Endpoints[0].SubnetID = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing endpoints list",
			req: func() CreateEC2ICERequest {
				r := baseReqFn()
				r.Endpoints = nil
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name:     "fill defaults",
			req:      baseReqFn,
			errCheck: require.NoError,
			reqWithDefaults: CreateEC2ICERequest{
				Cluster:         "teleport-cluster",
				IntegrationName: "teleportdev",
				Endpoints: []EC2ICEEndpoint{{
					SubnetID:         "subnet-123",
					SecurityGroupIDs: []string{"sg-1", "sg-2"},
				}},
				ResourceCreationTags: tags.AWSTags{
					"teleport.dev/origin":      "integration_awsoidc",
					"teleport.dev/cluster":     "teleport-cluster",
					"teleport.dev/integration": "teleportdev",
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.req()
			err := r.CheckAndSetDefaults()
			tt.errCheck(t, err)

			if err != nil {
				return
			}

			require.Empty(t, cmp.Diff(tt.reqWithDefaults, r))
		})
	}
}
