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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func stringPtr(s string) *string {
	return &s
}

func TestDeployDBServiceRequest(t *testing.T) {
	isBadParamErrFn := func(tt require.TestingT, err error, i ...interface{}) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	for _, tt := range []struct {
		name            string
		req             DeployDBServiceRequest
		errCheck        require.ErrorAssertionFunc
		reqWithDefaults DeployDBServiceRequest
	}{
		{
			name:     "no fields",
			req:      DeployDBServiceRequest{},
			errCheck: isBadParamErrFn,
		},
		{
			name: "empty list of subnets",
			req: DeployDBServiceRequest{
				TeleportClusterName: "mycluster",
				Region:              "r",
				SubnetIDs:           []string{},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "no discovery group",
			req: DeployDBServiceRequest{
				TeleportClusterName: "mycluster",
				Region:              "r",
				SubnetIDs:           []string{"1"},
				TaskRoleARN:         "",
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "all required fields",
			req: DeployDBServiceRequest{
				TeleportClusterName: "mycluster",
				Region:              "r",
				SubnetIDs:           []string{"1"},
				TaskRoleARN:         "arn",
				DiscoveryGroupName:  stringPtr("discovery-group"),
				ProxyServerHostPort: "host:1234",
				TeleportVersion:     "13.0.3",
			},
			errCheck: require.NoError,
			reqWithDefaults: DeployDBServiceRequest{
				TeleportClusterName: "mycluster",
				Region:              "r",
				SubnetIDs:           []string{"1"},
				TaskRoleARN:         "arn",
				DiscoveryGroupName:  stringPtr("discovery-group"),
				ProxyServerHostPort: "host:1234",
				TeleportVersion:     "13.0.3",
				ClusterName:         stringPtr("mycluster-teleport"),
				ServiceName:         stringPtr("mycluster-teleport-database-service"),
				TaskName:            stringPtr("mycluster-teleport-database-service"),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.CheckAndSetDefaults()
			tt.errCheck(t, err)

			if err != nil {
				return
			}
			require.True(t, cmp.Equal(tt.reqWithDefaults, tt.req), cmp.Diff(tt.reqWithDefaults, tt.req))
		})
	}
}
