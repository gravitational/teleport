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
	"github.com/gravitational/teleport/lib/ui"
	webui "github.com/gravitational/teleport/lib/web/ui"
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
			AWSInfo: &webui.AWSMetadata{
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

			require.Equal(t, tt.expectedStatus, resp.Code(), "invalid status code received")

			if err != nil {
				continue
			}

			// Ensure node exists
			node, err := env.proxies[0].client.GetNode(ctx, "default", tt.req.Name)
			require.NoError(t, err)

			require.Equal(t, tt.req.Name, node.GetName())
			require.Equal(t, &types.AWSInfo{
				AccountID:   tt.req.AWSInfo.AccountID,
				InstanceID:  tt.req.AWSInfo.InstanceID,
				Region:      tt.req.AWSInfo.Region,
				VPCID:       tt.req.AWSInfo.VPCID,
				Integration: tt.req.AWSInfo.Integration,
				SubnetID:    tt.req.AWSInfo.SubnetID,
			}, node.GetAWSInfo())
		}
	})

}
