/*
Copyright 2017 Gravitational, Inc.

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

package local

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gravitational/trace"
)

func TestTrustedClusterCRUD(t *testing.T) {
	ctx := context.Background()

	bk, err := lite.New(ctx, backend.Params{"path": t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	presenceBackend := NewPresenceService(bk)

	tc, err := types.NewTrustedCluster("foo", types.TrustedClusterSpecV2{
		Enabled:              true,
		Roles:                []string{"bar", "baz"},
		Token:                "qux",
		ProxyAddress:         "quux",
		ReverseTunnelAddress: "quuz",
	})
	require.NoError(t, err)

	// we just insert this one for get all
	stc, err := types.NewTrustedCluster("bar", types.TrustedClusterSpecV2{
		Enabled:              false,
		Roles:                []string{"baz", "aux"},
		Token:                "quux",
		ProxyAddress:         "quuz",
		ReverseTunnelAddress: "corge",
	})
	require.NoError(t, err)

	// create trusted clusters
	_, err = presenceBackend.UpsertTrustedCluster(ctx, tc)
	require.NoError(t, err)
	_, err = presenceBackend.UpsertTrustedCluster(ctx, stc)
	require.NoError(t, err)

	// get trusted cluster make sure it's correct
	gotTC, err := presenceBackend.GetTrustedCluster(ctx, "foo")
	require.NoError(t, err)
	require.Equal(t, "foo", gotTC.GetName())
	require.True(t, gotTC.GetEnabled())
	require.EqualValues(t, []string{"bar", "baz"}, gotTC.GetRoles())
	require.Equal(t, "qux", gotTC.GetToken())
	require.Equal(t, "quux", gotTC.GetProxyAddress())
	require.Equal(t, "quuz", gotTC.GetReverseTunnelAddress())

	// get all clusters
	allTC, err := presenceBackend.GetTrustedClusters(ctx)
	require.NoError(t, err)
	require.Len(t, allTC, 2)

	// delete cluster
	err = presenceBackend.DeleteTrustedCluster(ctx, "foo")
	require.NoError(t, err)

	// make sure it's really gone
	_, err = presenceBackend.GetTrustedCluster(ctx, "foo")
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))
}

func TestDatabaseServersCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	backend, err := lite.NewWithConfig(ctx, lite.Config{
		Path:  t.TempDir(),
		Clock: clock,
	})
	require.NoError(t, err)

	presence := NewPresenceService(backend)

	// Create a database server.
	server, err := types.NewDatabaseServerV3("foo", nil,
		types.DatabaseServerSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      "localhost:5432",
			Hostname: "localhost",
			HostID:   uuid.New(),
		})
	require.NoError(t, err)

	// Initially expect not to be returned any servers.
	out, err := presence.GetDatabaseServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))

	// Upsert server.
	lease, err := presence.UpsertDatabaseServer(ctx, server)
	require.NoError(t, err)
	require.Equal(t, &types.KeepAlive{}, lease)

	// Check again, expect a single server to be found.
	out, err = presence.GetDatabaseServers(ctx, server.GetNamespace())
	require.NoError(t, err)
	server.SetResourceID(out[0].GetResourceID())
	require.EqualValues(t, []types.DatabaseServer{server}, out)

	// Make sure can't delete with empty namespace or host ID or name.
	err = presence.DeleteDatabaseServer(ctx, server.GetNamespace(), server.GetHostID(), "")
	require.Error(t, err)
	require.IsType(t, trace.BadParameter(""), err)
	err = presence.DeleteDatabaseServer(ctx, server.GetNamespace(), "", server.GetName())
	require.Error(t, err)
	require.IsType(t, trace.BadParameter(""), err)
	err = presence.DeleteDatabaseServer(ctx, "", server.GetHostID(), server.GetName())
	require.Error(t, err)
	require.IsType(t, trace.BadParameter(""), err)

	// Remove the server.
	err = presence.DeleteDatabaseServer(ctx, server.GetNamespace(), server.GetHostID(), server.GetName())
	require.NoError(t, err)

	// Now expect no servers to be returned.
	out, err = presence.GetDatabaseServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))

	// Upsert server with TTL.
	server.SetExpiry(clock.Now().UTC().Add(time.Hour))
	lease, err = presence.UpsertDatabaseServer(ctx, server)
	require.NoError(t, err)
	require.Equal(t, &types.KeepAlive{
		Type:      types.KeepAlive_DATABASE,
		LeaseID:   lease.LeaseID,
		Name:      server.GetName(),
		Namespace: server.GetNamespace(),
		HostID:    server.GetHostID(),
		Expires:   server.Expiry(),
	}, lease)

	// Make sure can't delete all with empty namespace.
	err = presence.DeleteAllDatabaseServers(ctx, "")
	require.Error(t, err)
	require.IsType(t, trace.BadParameter(""), err)

	// Delete all.
	err = presence.DeleteAllDatabaseServers(ctx, server.GetNamespace())
	require.NoError(t, err)

	// Now expect no servers to be returned.
	out, err = presence.GetDatabaseServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))
}

func TestNodeCRUD(t *testing.T) {
	ctx := context.Background()
	lite, err := lite.NewWithConfig(ctx, lite.Config{Path: t.TempDir()})
	require.NoError(t, err)

	presence := NewPresenceService(lite)

	node1, err := types.NewServerWithLabels("node1", types.KindNode, types.ServerSpecV2{}, nil)
	require.NoError(t, err)

	node2, err := types.NewServerWithLabels("node2", types.KindNode, types.ServerSpecV2{}, nil)
	require.NoError(t, err)

	t.Run("CreateNode", func(t *testing.T) {
		// Initially expect no nodes to be returned.
		nodes, err := presence.GetNodes(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		require.Equal(t, 0, len(nodes))

		// Create nodes
		_, err = presence.UpsertNode(ctx, node1)
		require.NoError(t, err)
		_, err = presence.UpsertNode(ctx, node2)
		require.NoError(t, err)
	})

	// Run NodeGetters in nested subtests to allow parallelization.
	t.Run("NodeGetters", func(t *testing.T) {
		t.Run("List Nodes", func(t *testing.T) {
			t.Parallel()
			// list nodes one at a time, last page should be empty
			nodes, nextKey, err := presence.ListNodes(ctx, proto.ListNodesRequest{
				Namespace: apidefaults.Namespace,
				Limit:     1,
			})
			require.NoError(t, err)
			require.EqualValues(t, 1, len(nodes))
			require.Empty(t, cmp.Diff([]types.Server{node1}, nodes,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))
			require.EqualValues(t, backend.NextPaginationKey(node1), nextKey)

			nodes, nextKey, err = presence.ListNodes(ctx, proto.ListNodesRequest{
				Namespace: apidefaults.Namespace,
				Limit:     1,
				StartKey:  nextKey,
			})
			require.NoError(t, err)
			require.EqualValues(t, 1, len(nodes))
			require.Empty(t, cmp.Diff([]types.Server{node2}, nodes,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))
			require.EqualValues(t, backend.NextPaginationKey(node2), nextKey)

			nodes, nextKey, err = presence.ListNodes(ctx, proto.ListNodesRequest{
				Namespace: apidefaults.Namespace,
				Limit:     1,
				StartKey:  nextKey,
			})
			require.NoError(t, err)
			require.EqualValues(t, 0, len(nodes))
			require.EqualValues(t, "", nextKey)

			// ListNodes should fail if namespace isn't provided
			_, _, err = presence.ListNodes(ctx, proto.ListNodesRequest{
				Limit: 1,
			})
			require.IsType(t, &trace.BadParameterError{}, err.(*trace.TraceErr).OrigError())

			// ListNodes should fail if limit is nonpositive
			_, _, err = presence.ListNodes(ctx, proto.ListNodesRequest{
				Namespace: apidefaults.Namespace,
			})
			require.IsType(t, &trace.BadParameterError{}, err.(*trace.TraceErr).OrigError())

			_, _, err = presence.ListNodes(ctx, proto.ListNodesRequest{
				Namespace: apidefaults.Namespace,
				Limit:     -1,
			})
			require.IsType(t, &trace.BadParameterError{}, err.(*trace.TraceErr).OrigError())
		})
		t.Run("GetNodes", func(t *testing.T) {
			t.Parallel()
			// Get all nodes, transparently handle limit exceeded errors
			nodes, err := presence.GetNodes(ctx, apidefaults.Namespace)
			require.NoError(t, err)
			require.EqualValues(t, len(nodes), 2)
			require.Empty(t, cmp.Diff([]types.Server{node1, node2}, nodes,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))

			// GetNodes should fail if namespace isn't provided
			_, err = presence.GetNodes(ctx, "")
			require.IsType(t, &trace.BadParameterError{}, err.(*trace.TraceErr).OrigError())
		})
		t.Run("GetNode", func(t *testing.T) {
			t.Parallel()
			// Get Node
			node, err := presence.GetNode(ctx, apidefaults.Namespace, "node1")
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(node1, node,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))

			// GetNode should fail if node name isn't provided
			_, err = presence.GetNode(ctx, apidefaults.Namespace, "")
			require.IsType(t, &trace.BadParameterError{}, err.(*trace.TraceErr).OrigError())

			// GetNode should fail if namespace isn't provided
			_, err = presence.GetNode(ctx, "", "node1")
			require.IsType(t, &trace.BadParameterError{}, err.(*trace.TraceErr).OrigError())
		})
	})

	t.Run("DeleteNode", func(t *testing.T) {
		// Delete node.
		err = presence.DeleteNode(ctx, apidefaults.Namespace, node1.GetName())
		require.NoError(t, err)

		// Expect node not found
		_, err := presence.GetNode(ctx, apidefaults.Namespace, "node1")
		require.IsType(t, trace.NotFound(""), err)
	})

	t.Run("DeleteAllNodes", func(t *testing.T) {
		// Delete nodes
		err = presence.DeleteAllNodes(ctx, apidefaults.Namespace)
		require.NoError(t, err)

		// Now expect no nodes to be returned.
		nodes, err := presence.GetNodes(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		require.Equal(t, 0, len(nodes))
	})
}

func TestPresenceService_CancelSemaphoreLease(t *testing.T) {
	ctx := context.Background()
	bk, err := lite.New(ctx, backend.Params{"path": t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })
	presence := NewPresenceService(bk)

	maxLeases := 5
	leases := make([]*types.SemaphoreLease, maxLeases)

	// Acquire max number of leases
	request := types.AcquireSemaphoreRequest{
		SemaphoreKind: "test",
		SemaphoreName: "test",
		MaxLeases:     int64(maxLeases),
		Expires:       time.Now().Add(time.Hour),
		Holder:        "test",
	}
	for i := range leases {
		lease, err := presence.AcquireSemaphore(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, lease)

		leases[i] = lease
	}

	// Validate a semaphore exists with the correct number of leases
	semaphores, err := presence.GetSemaphores(ctx, types.SemaphoreFilter{
		SemaphoreKind: "test",
		SemaphoreName: "test",
	})
	require.NoError(t, err)
	require.Len(t, semaphores, 1)
	require.Len(t, semaphores[0].LeaseRefs(), maxLeases)

	// Cancel the leases concurrently and ensure that all
	// cancellations are honored
	errCh := make(chan error, maxLeases)
	for _, l := range leases {
		l := l
		go func() {
			errCh <- presence.CancelSemaphoreLease(ctx, *l)
		}()
	}

	for i := 0; i < maxLeases; i++ {
		err := <-errCh
		require.NoError(t, err)
	}

	// Validate the semaphore still exists but all leases were removed
	semaphores, err = presence.GetSemaphores(ctx, types.SemaphoreFilter{
		SemaphoreKind: "test",
		SemaphoreName: "test",
	})
	require.NoError(t, err)
	require.Len(t, semaphores, 1)
	require.Empty(t, semaphores[0].LeaseRefs())
}
