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

package awsoidc

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type mockCreateEC2ICEClient struct {
	name string
	err  error
}

func (m mockCreateEC2ICEClient) CreateInstanceConnectEndpoint(ctx context.Context, params *ec2.CreateInstanceConnectEndpointInput, optFns ...func(*ec2.Options)) (*ec2.CreateInstanceConnectEndpointOutput, error) {
	if m.err != nil {
		return nil, m.err
	}

	return &ec2.CreateInstanceConnectEndpointOutput{
		InstanceConnectEndpoint: &ec2Types.Ec2InstanceConnectEndpoint{
			InstanceConnectEndpointId: &m.name,
		},
	}, nil
}

func TestCreateEC2ICE_success(t *testing.T) {
	ctx := context.Background()
	mockCreateClient := &mockCreateEC2ICEClient{
		name: "eice-123",
	}
	resp, err := CreateEC2ICE(ctx, mockCreateClient, CreateEC2ICERequest{
		Cluster:         "c1",
		IntegrationName: "i1",
		SubnetID:        "subnet-id123",
	})
	require.NoError(t, err)
	require.Equal(t, "eice-123", resp.Name)
}

func TestCreateEC2ICE_error_quota_reached(t *testing.T) {
	ctx := context.Background()
	mockCreateClient := &mockCreateEC2ICEClient{
		err: fmt.Errorf("api error ResourceLimitExceeded: You've reached the quota for the maximum number of Instance Connect Endpoints for this subnet. Delete unused Instance Connect Endpoints, or request a quota increase."),
	}
	_, err := CreateEC2ICE(ctx, mockCreateClient, CreateEC2ICERequest{
		Cluster:         "c1",
		IntegrationName: "i1",
		SubnetID:        "subnet-id123",
	})
	require.ErrorContains(t, err, "api error ResourceLimitExceeded: You've reached the quota for the maximum number of Instance Connect Endpoints for this subnet. Delete unused Instance Connect Endpoints, or request a quota increase.")
}

func TestCreateEC2ICERequest(t *testing.T) {
	isBadParamErrFn := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	baseReqFn := func() CreateEC2ICERequest {
		return CreateEC2ICERequest{
			Cluster:          "teleport-cluster",
			IntegrationName:  "teleportdev",
			SubnetID:         "subnet-123",
			SecurityGroupIDs: []string{"sg-1", "sg-2"},
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
				r.SubnetID = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name:     "fill defaults",
			req:      baseReqFn,
			errCheck: require.NoError,
			reqWithDefaults: CreateEC2ICERequest{
				Cluster:          "teleport-cluster",
				IntegrationName:  "teleportdev",
				SubnetID:         "subnet-123",
				SecurityGroupIDs: []string{"sg-1", "sg-2"},
				ResourceCreationTags: AWSTags{
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
