// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	kube, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: "testKube",
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)

	mockedResources := []*proto.PaginatedResource{
		{Resource: &proto.PaginatedResource_Node{Node: node.(*types.ServerV2)}},
		{Resource: &proto.PaginatedResource_DatabaseServer{DatabaseServer: database}},
		{Resource: &proto.PaginatedResource_KubeCluster{KubeCluster: kube}},
	}
	mockedNextKey := "nextKey"

	mockedClient := &mockClient{
		paginatedResources: mockedResources,
		nextKey:            mockedNextKey,
	}

	response, err := List(ctx, cluster, mockedClient, &proto.ListUnifiedResourcesRequest{})
	require.NoError(t, err)
	require.Equal(t, CombinedResource{Server: &clusters.Server{
		URI:    uri.NewClusterURI(cluster.ProfileName).AppendServer(node.GetName()),
		Server: node,
	}}, response.Resources[0])
	require.Equal(t, CombinedResource{Database: &clusters.Database{
		URI:      uri.NewClusterURI(cluster.ProfileName).AppendDB(database.GetName()),
		Database: database.GetDatabase(),
	}}, response.Resources[1])
	require.Equal(t, CombinedResource{Kube: &clusters.Kube{
		URI:               uri.NewClusterURI(cluster.ProfileName).AppendKube(kube.GetName()),
		KubernetesCluster: kube,
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
