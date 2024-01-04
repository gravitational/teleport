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

package unifiedresources

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
)

func TestUnifiedResourcesList(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cluster := &clusters.Cluster{URI: uri.NewClusterURI("foo"), ProfileName: "foo"}

	node, err := types.NewServer("testNode", types.KindNode, types.ServerSpecV2{})
	require.NoError(t, err)

	database, err := types.NewDatabaseServerV3(types.Metadata{
		Name: "testDb",
	}, types.DatabaseServerSpecV3{
		Hostname: "localhost",
		HostID:   uuid.New().String(), Database: &types.DatabaseV3{
			Spec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres, URI: "localhost:5432",
			},
			Metadata: types.Metadata{Name: "testDb"}}})
	require.NoError(t, err)

	kube, err := types.NewKubernetesServerV3(types.Metadata{
		Name: "testKube",
	}, types.KubernetesServerSpecV3{
		HostID: uuid.New().String(),
		Cluster: &types.KubernetesClusterV3{
			Metadata: types.Metadata{
				Name: "testKube",
			},
			Spec: types.KubernetesClusterSpecV3{},
		},
	})
	require.NoError(t, err)

	mockedResources := []*proto.PaginatedResource{
		{Resource: &proto.PaginatedResource_Node{Node: node.(*types.ServerV2)}},
		{Resource: &proto.PaginatedResource_DatabaseServer{DatabaseServer: database}},
		{Resource: &proto.PaginatedResource_KubernetesServer{KubernetesServer: kube}},
	}
	mockedNextKey := "nextKey"

	mockedClient := &mockClient{
		paginatedResources: mockedResources,
		nextKey:            mockedNextKey,
	}

	response, err := List(ctx, cluster, mockedClient, &proto.ListUnifiedResourcesRequest{})
	require.NoError(t, err)
	require.Equal(t, UnifiedResource{Server: &clusters.Server{
		URI:    uri.NewClusterURI(cluster.ProfileName).AppendServer(node.GetName()),
		Server: node,
	}}, response.Resources[0])
	require.Equal(t, UnifiedResource{Database: &clusters.Database{
		URI:      uri.NewClusterURI(cluster.ProfileName).AppendDB(database.GetName()),
		Database: database.GetDatabase(),
	}}, response.Resources[1])
	require.Equal(t, UnifiedResource{Kube: &clusters.Kube{
		URI:               uri.NewClusterURI(cluster.ProfileName).AppendKube(kube.GetCluster().GetName()),
		KubernetesCluster: kube.GetCluster(),
	}}, response.Resources[2])
	require.Equal(t, mockedNextKey, response.NextKey)
}

type mockClient struct {
	paginatedResources []*proto.PaginatedResource
	nextKey            string
}

func (m *mockClient) ListUnifiedResources(ctx context.Context, req *proto.ListUnifiedResourcesRequest) (*proto.ListUnifiedResourcesResponse, error) {
	return &proto.ListUnifiedResourcesResponse{
		Resources: m.paginatedResources,
		NextKey:   m.nextKey,
	}, nil
}
