/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	componentfeaturesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/componentfeatures/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/componentfeatures"
)

type fakeSupportClient struct {
	presence []types.Server
	nodes    []*types.ServerV2
}

func (f fakeSupportClient) ListAuthServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
	return f.presence, "", nil
}

func (f fakeSupportClient) ListProxyServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
	return f.presence, "", nil
}

func (f fakeSupportClient) GetAuthServers() ([]types.Server, error) {
	return f.presence, nil
}

func (f fakeSupportClient) GetProxies() ([]types.Server, error) {
	return f.presence, nil
}

func (f fakeSupportClient) ListUnifiedResources(ctx context.Context, req *proto.ListUnifiedResourcesRequest) (*proto.ListUnifiedResourcesResponse, error) {
	resp := &proto.ListUnifiedResourcesResponse{}
	for _, n := range f.nodes {
		resp.Resources = append(resp.Resources, &proto.PaginatedResource{
			Resource: &proto.PaginatedResource_Node{Node: n},
		})
	}
	return resp, nil
}

func supportServer(t *testing.T, name string, features *componentfeaturesv1.ComponentFeatures) *types.ServerV2 {
	t.Helper()
	srv, err := types.NewServer(name, types.KindNode, types.ServerSpecV2{
		Version: "19.0.0",
	})
	require.NoError(t, err)
	s := srv.(*types.ServerV2)
	s.Spec.ComponentFeatures = features
	return s
}

func TestVerifyConstraintSupport(t *testing.T) {
	t.Parallel()

	rcFeatures := componentfeatures.New(componentfeatures.FeatureResourceConstraintsV1)
	constrained := []types.ResourceAccessID{sshRAID("main", "web-1", "root")}
	leafConstrained := []types.ResourceAccessID{sshRAID("leaf", "web-1", "root")}
	unconstrained := []types.ResourceAccessID{
		{Id: types.ResourceID{ClusterName: "main", Kind: types.KindNode, Name: "web-1"}},
	}

	tests := []struct {
		name         string
		presence     *componentfeaturesv1.ComponentFeatures
		leafPresence *componentfeaturesv1.ComponentFeatures
		node         *componentfeaturesv1.ComponentFeatures
		raids        []types.ResourceAccessID
		skipResource bool
		wantErr      string
	}{
		{
			name:     "all components support",
			presence: rcFeatures,
			node:     rcFeatures,
			raids:    constrained,
		},
		{
			name:     "unconstrained skips the check",
			presence: nil,
			node:     nil,
			raids:    unconstrained,
		},
		{
			name:     "old auth or proxy rejected",
			presence: componentfeatures.New(),
			node:     rcFeatures,
			raids:    constrained,
			wantErr:  "Auth or Proxy servers do not support",
		},
		{
			name:     "old agent rejected",
			presence: rcFeatures,
			node:     nil,
			raids:    constrained,
			wantErr:  "does not support the requested constraints",
		},
		{
			name:         "nil cluster client skips resource check",
			presence:     rcFeatures,
			node:         nil,
			raids:        constrained,
			skipResource: true,
		},
		{
			name:         "leaf resource with full support",
			presence:     rcFeatures,
			leafPresence: rcFeatures,
			node:         rcFeatures,
			raids:        leafConstrained,
		},
		{
			name:         "old leaf auth or proxy rejected",
			presence:     rcFeatures,
			leafPresence: componentfeatures.New(),
			node:         rcFeatures,
			raids:        leafConstrained,
			wantErr:      `cluster "leaf"'s Auth or Proxy servers do not support`,
		},
		{
			name:         "old leaf agent rejected",
			presence:     rcFeatures,
			leafPresence: rcFeatures,
			node:         nil,
			raids:        leafConstrained,
			wantErr:      "does not support the requested constraints",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rootClt := fakeSupportClient{
				presence: []types.Server{supportServer(t, "presence", tt.presence)},
			}
			getClusterClient := func(ctx context.Context, name string) (ClusterSupportClient, error) {
				if tt.skipResource {
					return nil, nil
				}
				presence := tt.presence
				if name != "main" {
					require.Equal(t, "leaf", name)
					presence = tt.leafPresence
				}
				return fakeSupportClient{
					presence: []types.Server{supportServer(t, "presence", presence)},
					nodes:    []*types.ServerV2{supportServer(t, "web-1", tt.node)},
				}, nil
			}
			err := VerifyConstraintSupport(context.Background(), slog.Default(), "main", rootClt, getClusterClient, tt.raids)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
