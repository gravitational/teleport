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

package accessrequest

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

func newNode(t *testing.T, name, hostname string) types.Server {
	t.Helper()
	node, err := types.NewServer(name, types.KindNode,
		types.ServerSpecV2{
			Hostname: hostname,
		})
	require.NoError(t, err)
	return node
}

func newApp(t *testing.T, name, description, origin string) types.Application {
	t.Helper()
	app, err := types.NewAppV3(types.Metadata{
		Name:        name,
		Description: description,
		Labels: map[string]string{
			types.OriginLabel: origin,
		},
	},
		types.AppSpecV3{
			URI:        "https://some-addr.com",
			PublicAddr: "https://some-addr.com",
		})
	require.NoError(t, err)
	return app
}

func newUserGroup(t *testing.T, name, description, origin string) types.UserGroup {
	t.Helper()
	userGroup, err := types.NewUserGroup(types.Metadata{
		Name:        name,
		Description: description,
		Labels: map[string]string{
			types.OriginLabel: origin,
		},
	}, types.UserGroupSpecV1{})
	require.NoError(t, err)
	return userGroup
}

func newResourceID(clusterName, kind, name string) types.ResourceID {
	return types.ResourceID{
		ClusterName: clusterName,
		Kind:        kind,
		Name:        name,
	}
}

type mockResourceLister struct {
	resources []types.ResourceWithLabels
}

func (m *mockResourceLister) ListResources(ctx context.Context, _ proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	return &types.ListResourcesResponse{
		Resources: m.resources,
	}, nil
}

func TestGetResourceDetails(t *testing.T) {
	clusterName := "cluster"

	presence := &mockResourceLister{
		resources: []types.ResourceWithLabels{
			newNode(t, "node1", "hostname 1"),
			newApp(t, "app1", "friendly app 1", types.OriginDynamic),
			newApp(t, "app2", "friendly app 2", types.OriginDynamic),
			newApp(t, "app3", "friendly app 3", types.OriginOkta),
			newUserGroup(t, "group1", "friendly group 1", types.OriginOkta),
		},
	}
	resourceIDs := []types.ResourceID{
		newResourceID(clusterName, types.KindNode, "node1"),
		newResourceID(clusterName, types.KindApp, "app1"),
		newResourceID(clusterName, types.KindApp, "app2"),
		newResourceID(clusterName, types.KindApp, "app3"),
		newResourceID(clusterName, types.KindUserGroup, "group1"),
	}

	ctx := context.Background()

	details, err := GetResourceDetails(ctx, clusterName, presence, resourceIDs)
	require.NoError(t, err)

	// Check the resource details to see if friendly names properly propagated.

	// Node should be named for its hostname.
	require.Equal(t, "hostname 1", details[types.ResourceIDToString(resourceIDs[0])].FriendlyName)

	// app1 and app2 are expected to be empty because they're not Okta sourced resources.
	require.Empty(t, details[types.ResourceIDToString(resourceIDs[1])].FriendlyName)

	require.Empty(t, details[types.ResourceIDToString(resourceIDs[2])].FriendlyName)

	// This Okta sourced app should have a friendly name.
	require.Equal(t, "friendly app 3", details[types.ResourceIDToString(resourceIDs[3])].FriendlyName)

	// This Okta sourced user group should have a friendly name.
	require.Equal(t, "friendly group 1", details[types.ResourceIDToString(resourceIDs[4])].FriendlyName)
}

func TestGetResourceNames(t *testing.T) {
	clusterName := "cluster"

	presence := &mockResourceLister{
		resources: []types.ResourceWithLabels{
			newNode(t, "node1", "hostname 1"),
			newApp(t, "app1", "friendly app 1", types.OriginDynamic),
			newApp(t, "app2", "friendly app 2", types.OriginDynamic),
			newApp(t, "app3", "friendly app 3", types.OriginOkta),
			newUserGroup(t, "group1", "friendly group 1", types.OriginOkta),
		},
	}
	resourceIDs := []types.ResourceID{
		newResourceID(clusterName, types.KindNode, "node1"),
		newResourceID(clusterName, types.KindApp, "app1"),
		newResourceID(clusterName, types.KindApp, "app2"),
		newResourceID(clusterName, types.KindApp, "app3"),
		newResourceID(clusterName, types.KindUserGroup, "group1"),
	}

	ctx := context.Background()

	req, err := types.NewAccessRequestWithResources(uuid.New().String(), "some-user", []string{}, resourceIDs)
	require.NoError(t, err)
	names, err := GetResourceNames(ctx, presence, req)
	require.NoError(t, err)
	expected := []string{
		"/node/hostname 1",
		"/cluster/app/app1",
		"/cluster/app/app2",
		"/app/friendly app 3",
		"/user_group/friendly group 1",
	}
	require.Equal(t, expected, names)
}
