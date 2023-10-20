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

package web

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
)

func TestCreateNode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	env := newWebPack(t, 1)
	clusterName := env.server.ClusterName()

	makeCreateNodeRequest := func(fn func(*createNodeRequest)) createNodeRequest {
		s := createNodeRequest{
			Name:     uuid.NewString(),
			SubKind:  types.SubKindOpenSSHEICENode,
			Hostname: "myhostname",
			Addr:     "172.31.1.1:22",
			Labels:   []ui.Label{},
			AWSInfo: &ui.AWSMetadata{
				AccountID:   "123456789012",
				InstanceID:  "i-123",
				Region:      "us-east-1",
				VPCID:       "vpc-abcd",
				SubnetID:    "subnet-123",
				Integration: "myintegration",
			},
		}
		fn(&s)
		return s
	}

	t.Run("user without rbac access to nodes", func(t *testing.T) {
		username := "user-without-node-access"
		roleUpsertNode, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
			Allow: types.RoleConditions{
				Logins: []string{"osuser"},
				NodeLabels: types.Labels{
					types.Wildcard: {types.Wildcard},
				},
			},
		})
		require.NoError(t, err)
		pack := env.proxies[0].authPack(t, username, []types.Role{roleUpsertNode})

		createNodeEndpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "nodes")
		// Create node must return access denied.
		req := makeCreateNodeRequest(func(cnr *createNodeRequest) {})
		_, err = pack.clt.PostJSON(ctx, createNodeEndpoint, req)
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("user with rbac access to nodes", func(t *testing.T) {
		username := "someuser"
		roleUpsertNode, err := types.NewRole(services.RoleNameForUser(username), types.RoleSpecV6{
			Allow: types.RoleConditions{
				Logins: []string{"osuser"},
				Rules: []types.Rule{
					types.NewRule(types.KindNode,
						[]string{types.VerbCreate, types.VerbUpdate}),
				},
				NodeLabels: types.Labels{
					types.Wildcard: {types.Wildcard},
				},
			},
		})
		require.NoError(t, err)
		pack := env.proxies[0].authPack(t, username, []types.Role{roleUpsertNode})

		createNodeEndpoint := pack.clt.Endpoint("webapi", "sites", clusterName, "nodes")

		for _, tt := range []struct {
			name           string
			req            createNodeRequest
			expectedStatus int
			errAssert      require.ErrorAssertionFunc
		}{
			{
				name:           "valid",
				req:            makeCreateNodeRequest(func(cnr *createNodeRequest) {}),
				expectedStatus: http.StatusOK,
				errAssert:      require.NoError,
			},
			{
				name: "empty name",
				req: makeCreateNodeRequest(func(cnr *createNodeRequest) {
					cnr.Name = ""
				}),
				expectedStatus: http.StatusBadRequest,
				errAssert: func(tt require.TestingT, err error, i ...interface{}) {
					require.ErrorContains(tt, err, "missing node name")
				},
			},
			{
				name: "missing aws account id",
				req: makeCreateNodeRequest(func(cnr *createNodeRequest) {
					cnr.AWSInfo.AccountID = ""
				}),
				expectedStatus: http.StatusBadRequest,
				errAssert: func(tt require.TestingT, err error, i ...interface{}) {
					require.ErrorContains(tt, err, `missing AWS Account ID (required for "openssh-ec2-ice" SubKind)`)
				},
			},
			{
				name: "invalid subkind",
				req: makeCreateNodeRequest(func(cnr *createNodeRequest) {
					cnr.SubKind = types.SubKindOpenSSHNode
				}),
				expectedStatus: http.StatusBadRequest,
				errAssert: func(tt require.TestingT, err error, i ...interface{}) {
					require.ErrorContains(tt, err, `invalid subkind "openssh", only "openssh-ec2-ice" is supported`)
				},
			},
		} {
			// Create node
			resp, err := pack.clt.PostJSON(ctx, createNodeEndpoint, tt.req)
			tt.errAssert(t, err)

			require.Equal(t, resp.Code(), tt.expectedStatus, "invalid status code received")

			if err != nil {
				continue
			}

			// Ensure node exists
			node, err := env.proxies[0].client.GetNode(ctx, "default", tt.req.Name)
			require.NoError(t, err)

			require.Equal(t, node.GetName(), tt.req.Name)
			require.Equal(t, node.GetAWSInfo(), &types.AWSInfo{
				AccountID:   tt.req.AWSInfo.AccountID,
				InstanceID:  tt.req.AWSInfo.InstanceID,
				Region:      tt.req.AWSInfo.Region,
				VPCID:       tt.req.AWSInfo.VPCID,
				Integration: tt.req.AWSInfo.Integration,
				SubnetID:    tt.req.AWSInfo.SubnetID,
			})
		}
	})

}
