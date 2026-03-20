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
	"github.com/gravitational/teleport/lib/utils/aws"
)

func TestUnifiedResourcesList(t *testing.T) {
	ctx := t.Context()

	cluster := &clusters.Cluster{URI: uri.NewClusterURI("foo"), ProfileName: "foo"}

	node, err := types.NewServer("testNode", types.KindNode, types.ServerSpecV2{})
	require.NoError(t, err)

	database, err := types.NewDatabaseServerV3(types.Metadata{
		Name: "testDb",
	}, types.DatabaseServerSpecV3{
		Hostname: "localhost",
		HostID:   uuid.New().String(),
		Database: &types.DatabaseV3{
			Spec: types.DatabaseSpecV3{
				Protocol:  defaults.ProtocolPostgres,
				URI:       "localhost:5432",
				AdminUser: &types.DatabaseAdminUser{Name: "teleport-admin"},
			},
			Metadata: types.Metadata{Name: "testDb"},
		},
	})
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

	app, err := types.NewAppServerV3(types.Metadata{
		Name: "testApp",
	}, types.AppServerSpecV3{
		HostID: uuid.New().String(),
		App: &types.AppV3{
			Metadata: types.Metadata{
				Name: "testApp",
			},
			Spec: types.AppSpecV3{
				URI: "https://test.app",
			},
		},
	})
	require.NoError(t, err)

	samlSP, err := types.NewSAMLIdPServiceProvider(types.Metadata{
		Name: "testApp",
	}, types.SAMLIdPServiceProviderSpecV1{
		ACSURL:   "https://test.example",
		EntityID: "123",
	})
	require.NoError(t, err)

	windowsDesktop, err := types.NewWindowsDesktopV3("testWindowsDesktop", nil,
		types.WindowsDesktopSpecV3{
			Addr:  "127.0.0.1:3389",
			NonAD: true,
		})
	require.NoError(t, err)

	mcp, err := types.NewAppServerV3(types.Metadata{
		Name: "test-mcp",
	}, types.AppServerSpecV3{
		HostID: uuid.New().String(),
		App: &types.AppV3{
			Metadata: types.Metadata{
				Name: "test-mcp",
			},
			Spec: types.AppSpecV3{
				MCP: &types.MCP{
					Command:       "test",
					RunAsHostUser: "test",
				},
			},
		},
	})
	require.NoError(t, err)

	leafDatabase, err := types.NewDatabaseServerV3(types.Metadata{
		Name: "leafDb",
	}, types.DatabaseServerSpecV3{
		Hostname: "localhost",
		HostID:   uuid.New().String(),
		Database: &types.DatabaseV3{
			Spec: types.DatabaseSpecV3{
				Protocol:  defaults.ProtocolPostgres,
				URI:       "localhost:5432",
				AdminUser: &types.DatabaseAdminUser{Name: "teleport-admin"},
			},
			Metadata: types.Metadata{Name: "leafDb"},
		},
	})
	require.NoError(t, err)

	leafCluster := &clusters.Cluster{URI: uri.NewClusterURI("foo").AppendLeafCluster("leaf"), ProfileName: "foo", Name: "leaf"}

	mockedResources := []*proto.PaginatedResource{
		{Resource: &proto.PaginatedResource_Node{Node: node.(*types.ServerV2)}, Logins: []string{"ec2-user"}},
		{Resource: &proto.PaginatedResource_DatabaseServer{DatabaseServer: database}},
		{Resource: &proto.PaginatedResource_KubernetesServer{KubernetesServer: kube}},
		{Resource: &proto.PaginatedResource_AppServer{AppServer: app}},
		{Resource: &proto.PaginatedResource_SAMLIdPServiceProvider{SAMLIdPServiceProvider: samlSP.(*types.SAMLIdPServiceProviderV1)}},
		{Resource: &proto.PaginatedResource_WindowsDesktop{WindowsDesktop: windowsDesktop}},
		{Resource: &proto.PaginatedResource_AppServer{AppServer: mcp}},
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
		Logins: nil, // because the cluster has no SSH logins
	}}, response.Resources[0])

	require.Equal(t, UnifiedResource{Database: &clusters.Database{
		URI:      uri.NewClusterURI(cluster.ProfileName).AppendDB(database.GetName()),
		Database: database.GetDatabase(),
		AutoUserProvisioning: &clusters.AutoUserProvisioning{
			DatabaseRoles: []string{},
		},
		DatabaseUsers: []string{"testUser"},
	}}, response.Resources[1])

	require.Equal(t, UnifiedResource{Kube: &clusters.Kube{
		URI:               uri.NewClusterURI(cluster.ProfileName).AppendKube(kube.GetCluster().GetName()),
		KubernetesCluster: kube.GetCluster(),
	}}, response.Resources[2])

	require.Equal(t, UnifiedResource{App: &clusters.App{
		URI: uri.NewClusterURI(cluster.ProfileName).AppendApp(app.GetApp().GetName()),
		// FQDN looks weird because we cannot mock cluster.status.ProxyHost in tests.
		FQDN:     "testApp.",
		AWSRoles: aws.Roles{},
		App:      app.GetApp(),
	}}, response.Resources[3])

	require.Equal(t, UnifiedResource{SAMLIdPServiceProvider: &clusters.SAMLIdPServiceProvider{
		URI:      uri.NewClusterURI(cluster.ProfileName).AppendApp(samlSP.GetName()),
		Provider: samlSP,
	}}, response.Resources[4])

	require.Equal(t, UnifiedResource{WindowsDesktop: &clusters.WindowsDesktop{
		URI:            uri.NewClusterURI(cluster.ProfileName).AppendWindowsDesktop(windowsDesktop.GetName()),
		WindowsDesktop: windowsDesktop,
	}}, response.Resources[5])

	require.Equal(t, UnifiedResource{App: &clusters.App{
		FQDN:     "test-mcp.",
		URI:      uri.NewClusterURI(cluster.ProfileName).AppendApp(mcp.GetName()),
		AWSRoles: aws.Roles{},
		App:      mcp.GetApp(),
	}}, response.Resources[6])

	require.Equal(t, mockedNextKey, response.NextKey)

	leafResponse, err := List(ctx, leafCluster, &mockClient{
		paginatedResources: []*proto.PaginatedResource{
			{Resource: &proto.PaginatedResource_DatabaseServer{DatabaseServer: leafDatabase}},
		},
	}, &proto.ListUnifiedResourcesRequest{})
	require.NoError(t, err)
	require.Len(t, leafResponse.Resources, 1)
	require.Equal(t, UnifiedResource{Database: &clusters.Database{
		URI:      leafCluster.URI.AppendDB(leafDatabase.GetName()),
		Database: leafDatabase.GetDatabase(),
		AutoUserProvisioning: &clusters.AutoUserProvisioning{
			DatabaseRoles: []string{},
		},
		DatabaseUsers: []string{"testUser"},
	}}, leafResponse.Resources[0])
}

func TestUnifiedResourcesListWildcardDatabaseUsers(t *testing.T) {
	ctx := t.Context()

	cluster := &clusters.Cluster{URI: uri.NewClusterURI("foo"), ProfileName: "foo"}

	database, err := types.NewDatabaseServerV3(types.Metadata{
		Name: "testDb",
	}, types.DatabaseServerSpecV3{
		Hostname: "localhost",
		HostID:   uuid.New().String(),
		Database: &types.DatabaseV3{
			Spec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
			},
			Metadata: types.Metadata{Name: "testDb"},
		},
	})
	require.NoError(t, err)

	wildcardRole, err := types.NewRole("wildcard-db-user", types.RoleSpecV6{
		Allow: types.RoleConditions{
			DatabaseLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			DatabaseUsers:  []string{types.Wildcard},
		},
	})
	require.NoError(t, err)

	response, err := List(ctx, cluster, &mockClient{
		paginatedResources: []*proto.PaginatedResource{
			{Resource: &proto.PaginatedResource_DatabaseServer{DatabaseServer: database}},
		},
		roles: []types.Role{wildcardRole},
	}, &proto.ListUnifiedResourcesRequest{})
	require.NoError(t, err)
	require.Len(t, response.Resources, 1)

	db := response.Resources[0].Database
	require.NotNil(t, db)
	require.Nil(t, db.AutoUserProvisioning)
	require.True(t, db.WildcardUserAllowed)
}

type mockClient struct {
	paginatedResources []*proto.PaginatedResource
	nextKey            string
	roles              []types.Role
}

func (m *mockClient) ListUnifiedResources(ctx context.Context, req *proto.ListUnifiedResourcesRequest) (*proto.ListUnifiedResourcesResponse, error) {
	return &proto.ListUnifiedResourcesResponse{
		Resources: m.paginatedResources,
		NextKey:   m.nextKey,
	}, nil
}

func (m *mockClient) GetCurrentUserRoles(ctx context.Context) ([]types.Role, error) {
	if m.roles != nil {
		return m.roles, nil
	}
	role, err := types.NewRole("auto-db-user", types.RoleSpecV6{
		Options: types.RoleOptions{
			CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
		},
		Allow: types.RoleConditions{
			DatabaseLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
		},
	})
	if err != nil {
		return nil, err
	}
	return []types.Role{role}, nil
}

func (m *mockClient) GetCurrentUser(ctx context.Context) (types.User, error) {
	return types.NewUser("testUser")
}
