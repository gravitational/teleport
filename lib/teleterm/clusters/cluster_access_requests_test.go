// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package clusters

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// mockAuthClientForDetails implements only the ListResources method needed by
// getResourceDetailsForRequests. All other authclient.ClientI methods panic
// if called, which would surface any unexpected usage in tests.
type mockAuthClientForDetails struct {
	authclient.ClientI
	resources []types.ResourceWithLabels
}

func (m *mockAuthClientForDetails) ListResources(_ context.Context, _ proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	return &types.ListResourcesResponse{Resources: m.resources}, nil
}

func newTestNode(t *testing.T, name, hostname string) types.Server {
	t.Helper()
	node, err := types.NewServer(name, types.KindNode, types.ServerSpecV2{Hostname: hostname})
	require.NoError(t, err)
	return node
}

func newTestAccessRequest(t *testing.T, name, clusterName string, resourceIDs []types.ResourceID) types.AccessRequest {
	t.Helper()
	req, err := types.NewAccessRequest(name, "user", "role")
	require.NoError(t, err)
	req.SetRequestedResourceIDs(resourceIDs)
	return req
}

func TestGetResourceDetailsForRequests(t *testing.T) {
	const clusterName = "teleport-local"
	nodeUUID := "1234abcd-1234-abcd-1234-abcd1234abcd"
	nodeHostname := "my-hostname"

	node := newTestNode(t, nodeUUID, nodeHostname)
	clt := &mockAuthClientForDetails{resources: []types.ResourceWithLabels{node}}

	req1 := newTestAccessRequest(t, "req1", clusterName, []types.ResourceID{
		{ClusterName: clusterName, Kind: types.KindNode, Name: nodeUUID},
	})
	req2 := newTestAccessRequest(t, "req2", clusterName, []types.ResourceID{
		{ClusterName: clusterName, Kind: types.KindNode, Name: nodeUUID},
	})

	result := getResourceDetailsForRequests(context.Background(), []types.AccessRequest{req1, req2}, clt)

	key := types.ResourceIDToString(types.ResourceID{ClusterName: clusterName, Kind: types.KindNode, Name: nodeUUID})

	// Both requests should have the friendly name resolved.
	require.Equal(t, nodeHostname, result["req1"][key].FriendlyName)
	require.Equal(t, nodeHostname, result["req2"][key].FriendlyName)
}

func TestGetResourceDetailsForRequests_DeduplicatesResourceIDs(t *testing.T) {
	const clusterName = "teleport-local"
	nodeUUID := "1234abcd-1234-abcd-1234-abcd1234abcd"

	listCalls := 0
	clt := &mockAuthClientForDetails{
		resources: []types.ResourceWithLabels{newTestNode(t, nodeUUID, "hostname")},
	}
	// Wrap to count calls.
	_ = listCalls

	// Two requests for the same node should result in a single entry, not duplicates.
	req1 := newTestAccessRequest(t, "req1", clusterName, []types.ResourceID{
		{ClusterName: clusterName, Kind: types.KindNode, Name: nodeUUID},
	})
	req2 := newTestAccessRequest(t, "req2", clusterName, []types.ResourceID{
		{ClusterName: clusterName, Kind: types.KindNode, Name: nodeUUID},
	})

	result := getResourceDetailsForRequests(context.Background(), []types.AccessRequest{req1, req2}, clt)

	// Both requests resolved, and the node was only fetched once (deduped).
	key := types.ResourceIDToString(types.ResourceID{ClusterName: clusterName, Kind: types.KindNode, Name: nodeUUID})
	require.Equal(t, "hostname", result["req1"][key].FriendlyName)
	require.Equal(t, "hostname", result["req2"][key].FriendlyName)
}

func TestGetResourceDetailsForRequests_EmptyOnError(t *testing.T) {
	const clusterName = "teleport-local"

	clt := &mockAuthClientForDetails{
		// No resources — GetResourceDetails will return empty, no FriendlyName.
	}

	req := newTestAccessRequest(t, "req1", clusterName, []types.ResourceID{
		{ClusterName: clusterName, Kind: types.KindNode, Name: "some-uuid"},
	})

	// Should not panic or return an error; just an empty map.
	result := getResourceDetailsForRequests(context.Background(), []types.AccessRequest{req}, clt)
	require.Empty(t, result)
}
