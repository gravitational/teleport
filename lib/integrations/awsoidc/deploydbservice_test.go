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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
)

func stringPtr(s string) *string {
	return &s
}

func TestDeployDBServiceRequest(t *testing.T) {
	isBadParamErrFn := func(tt require.TestingT, err error, i ...interface{}) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	baseReqFn := func() DeployDBServiceRequest {
		return DeployDBServiceRequest{
			TeleportClusterName: "mycluster",
			Region:              "r",
			SubnetIDs:           []string{"1"},
			TaskRoleARN:         "arn",
			DiscoveryGroupName:  stringPtr("discovery-group"),
			ProxyServerHostPort: "host:1234",
			TeleportVersion:     "13.0.3",
			AgentMatcherLabels: types.Labels{
				"env": utils.Strings{"prod"},
				"app": utils.Strings{"xyz"},
			},
		}
	}

	for _, tt := range []struct {
		name            string
		req             func() DeployDBServiceRequest
		errCheck        require.ErrorAssertionFunc
		reqWithDefaults DeployDBServiceRequest
	}{
		{
			name: "no fields",
			req: func() DeployDBServiceRequest {
				return DeployDBServiceRequest{}
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing teleport cluster name",
			req: func() DeployDBServiceRequest {
				r := baseReqFn()
				r.TeleportClusterName = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing region",
			req: func() DeployDBServiceRequest {
				r := baseReqFn()
				r.Region = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "empty list of subnets",
			req: func() DeployDBServiceRequest {
				r := baseReqFn()
				r.SubnetIDs = []string{}
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing task role arn",
			req: func() DeployDBServiceRequest {
				r := baseReqFn()
				r.TaskRoleARN = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing proxy server host port",
			req: func() DeployDBServiceRequest {
				r := baseReqFn()
				r.ProxyServerHostPort = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing teleport version",
			req: func() DeployDBServiceRequest {
				r := baseReqFn()
				r.ProxyServerHostPort = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing agent matcher labels",
			req: func() DeployDBServiceRequest {
				r := baseReqFn()
				r.AgentMatcherLabels = types.Labels{}
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name:     "fill defaults",
			req:      baseReqFn,
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
				AgentMatcherLabels: types.Labels{
					"env": utils.Strings{"prod"},
					"app": utils.Strings{"xyz"},
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
			require.True(t, cmp.Equal(tt.reqWithDefaults, r), cmp.Diff(tt.reqWithDefaults, r))
		})
	}
}
