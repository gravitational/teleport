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
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"gopkg.in/check.v1"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gravitational/trace"
)

type PresenceSuite struct {
	bk backend.Backend
}

var _ = check.Suite(&PresenceSuite{})

func (s *PresenceSuite) SetUpTest(c *check.C) {
	var err error

	s.bk, err = lite.New(context.TODO(), backend.Params{"path": c.MkDir()})
	c.Assert(err, check.IsNil)
}

func (s *PresenceSuite) TearDownTest(c *check.C) {
	c.Assert(s.bk.Close(), check.IsNil)
}

func (s *PresenceSuite) TestTrustedClusterCRUD(c *check.C) {
	ctx := context.Background()
	presenceBackend := NewPresenceService(s.bk)

	tc, err := types.NewTrustedCluster("foo", types.TrustedClusterSpecV2{
		Enabled:              true,
		Roles:                []string{"bar", "baz"},
		Token:                "qux",
		ProxyAddress:         "quux",
		ReverseTunnelAddress: "quuz",
	})
	c.Assert(err, check.IsNil)

	// we just insert this one for get all
	stc, err := types.NewTrustedCluster("bar", types.TrustedClusterSpecV2{
		Enabled:              false,
		Roles:                []string{"baz", "aux"},
		Token:                "quux",
		ProxyAddress:         "quuz",
		ReverseTunnelAddress: "corge",
	})
	c.Assert(err, check.IsNil)

	// create trusted clusters
	_, err = presenceBackend.UpsertTrustedCluster(ctx, tc)
	c.Assert(err, check.IsNil)
	_, err = presenceBackend.UpsertTrustedCluster(ctx, stc)
	c.Assert(err, check.IsNil)

	// get trusted cluster make sure it's correct
	gotTC, err := presenceBackend.GetTrustedCluster(ctx, "foo")
	c.Assert(err, check.IsNil)
	c.Assert(gotTC.GetName(), check.Equals, "foo")
	c.Assert(gotTC.GetEnabled(), check.Equals, true)
	c.Assert(gotTC.GetRoles(), check.DeepEquals, []string{"bar", "baz"})
	c.Assert(gotTC.GetToken(), check.Equals, "qux")
	c.Assert(gotTC.GetProxyAddress(), check.Equals, "quux")
	c.Assert(gotTC.GetReverseTunnelAddress(), check.Equals, "quuz")

	// get all clusters
	allTC, err := presenceBackend.GetTrustedClusters(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(allTC, check.HasLen, 2)

	// delete cluster
	err = presenceBackend.DeleteTrustedCluster(ctx, "foo")
	c.Assert(err, check.IsNil)

	// make sure it's really gone
	_, err = presenceBackend.GetTrustedCluster(ctx, "foo")
	c.Assert(err, check.NotNil)
	c.Assert(trace.IsNotFound(err), check.Equals, true)
}

// TestApplicationServersCRUD verifies backend operations on app servers.
func TestApplicationServersCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	backend, err := lite.NewWithConfig(ctx, lite.Config{
		Path:  t.TempDir(),
		Clock: clock,
	})
	require.NoError(t, err)

	presence := NewPresenceService(backend)

	// Make an app and an app server.
	appA, err := types.NewAppV3(types.Metadata{Name: "a"},
		types.AppSpecV3{URI: "http://localhost:8080"})
	require.NoError(t, err)
	serverA, err := types.NewAppServerV3(types.Metadata{
		Name: appA.GetName(),
	}, types.AppServerSpecV3{
		Hostname: "localhost",
		HostID:   uuid.New().String(),
		App:      appA,
	})
	require.NoError(t, err)

	// Make a legacy app server.
	appBLegacy := &types.App{Name: "b", URI: "http://localhost:8081"}
	appB, err := types.NewAppV3FromLegacyApp(appBLegacy)
	require.NoError(t, err)
	serverBLegacy, err := types.NewServer(uuid.New().String(), types.KindAppServer,
		types.ServerSpecV2{
			Hostname: "localhost",
			Apps:     []*types.App{appBLegacy},
		})
	require.NoError(t, err)
	serverB, err := types.NewAppServerV3(types.Metadata{
		Name: appBLegacy.Name,
	}, types.AppServerSpecV3{
		Hostname: "localhost",
		HostID:   serverBLegacy.GetName(),
		App:      appB,
	})
	require.NoError(t, err)

	// No app servers should be registered initially
	out, err := presence.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))

	// Create app server.
	lease, err := presence.UpsertApplicationServer(ctx, serverA)
	require.NoError(t, err)
	require.Equal(t, &types.KeepAlive{}, lease)

	// Create legacy app server.
	lease, err = presence.UpsertAppServer(ctx, serverBLegacy)
	require.NoError(t, err)
	require.Equal(t, &types.KeepAlive{}, lease)

	// Make sure all app servers are registered.
	out, err = presence.GetApplicationServers(ctx, serverA.GetNamespace())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.AppServer{serverA, serverB}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Delete app server.
	err = presence.DeleteApplicationServer(ctx, serverA.GetNamespace(), serverA.GetHostID(), serverA.GetName())
	require.NoError(t, err)

	// Expect only the legacy one to be returned.
	out, err = presence.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.AppServer{serverB}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Upsert server with TTL.
	serverA.SetExpiry(clock.Now().UTC().Add(time.Hour))
	lease, err = presence.UpsertApplicationServer(ctx, serverA)
	require.NoError(t, err)
	require.Equal(t, &types.KeepAlive{
		Type:      types.KeepAlive_APP,
		LeaseID:   lease.LeaseID,
		Name:      serverA.GetName(),
		Namespace: serverA.GetNamespace(),
		HostID:    serverA.GetHostID(),
		Expires:   serverA.Expiry(),
	}, lease)

	// Delete all app servers.
	err = presence.DeleteAllApplicationServers(ctx, serverA.GetNamespace())
	require.NoError(t, err)

	// Expect only legacy one to be returned.
	out, err = presence.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.AppServer{serverB}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))
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
	server, err := types.NewDatabaseServerV3(types.Metadata{
		Name: "foo",
	}, types.DatabaseServerSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		Hostname: "localhost",
		HostID:   uuid.New().String(),
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

func TestListResources(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	backend, err := lite.NewWithConfig(ctx, lite.Config{
		Path:  t.TempDir(),
		Clock: clock,
	})
	require.NoError(t, err)

	presence := NewPresenceService(backend)

	tests := map[string]struct {
		resourceType           string
		createResourceFunc     func(context.Context, string, map[string]string) error
		deleteAllResourcesFunc func(context.Context) error
		expectedType           types.Resource
	}{
		"DatabaseServers": {
			resourceType: types.KindDatabaseServer,
			createResourceFunc: func(ctx context.Context, name string, labels map[string]string) error {
				server, err := types.NewDatabaseServerV3(types.Metadata{
					Name:   name,
					Labels: labels,
				}, types.DatabaseServerSpecV3{
					Protocol: defaults.ProtocolPostgres,
					URI:      "localhost:5432",
					Hostname: "localhost",
					HostID:   uuid.New().String(),
				})
				if err != nil {
					return err
				}

				// Upsert server.
				_, err = presence.UpsertDatabaseServer(ctx, server)
				return err
			},
			deleteAllResourcesFunc: func(ctx context.Context) error {
				return presence.DeleteAllDatabaseServers(ctx, apidefaults.Namespace)
			},
		},
		"DatabaseServersSameHost": {
			resourceType: types.KindDatabaseServer,
			createResourceFunc: func(ctx context.Context, name string, labels map[string]string) error {
				server, err := types.NewDatabaseServerV3(types.Metadata{
					Name:   name,
					Labels: labels,
				}, types.DatabaseServerSpecV3{
					Protocol: defaults.ProtocolPostgres,
					URI:      "localhost:5432",
					Hostname: "localhost",
					HostID:   "some-host",
				})
				if err != nil {
					return err
				}

				// Upsert server.
				_, err = presence.UpsertDatabaseServer(ctx, server)
				return err
			},
			deleteAllResourcesFunc: func(ctx context.Context) error {
				return presence.DeleteAllDatabaseServers(ctx, apidefaults.Namespace)
			},
		},
		"AppServers": {
			resourceType: types.KindAppServer,
			createResourceFunc: func(ctx context.Context, name string, labels map[string]string) error {
				app, err := types.NewAppV3(types.Metadata{
					Name:   name,
					Labels: labels,
				}, types.AppSpecV3{
					URI: "localhost",
				})
				if err != nil {
					return err
				}

				server, err := types.NewAppServerV3(types.Metadata{
					Name:   name,
					Labels: labels,
				}, types.AppServerSpecV3{
					Hostname: "localhost",
					HostID:   uuid.New().String(),
					App:      app,
				})
				if err != nil {
					return err
				}

				// Upsert server.
				_, err = presence.UpsertApplicationServer(ctx, server)
				return err
			},
			deleteAllResourcesFunc: func(ctx context.Context) error {
				return presence.DeleteAllApplicationServers(ctx, apidefaults.Namespace)
			},
		},
		"AppServersSameHost": {
			resourceType: types.KindAppServer,
			createResourceFunc: func(ctx context.Context, name string, labels map[string]string) error {
				app, err := types.NewAppV3(types.Metadata{
					Name:   name,
					Labels: labels,
				}, types.AppSpecV3{
					URI: "localhost",
				})
				if err != nil {
					return err
				}

				server, err := types.NewAppServerV3(types.Metadata{
					Name:   name,
					Labels: labels,
				}, types.AppServerSpecV3{
					Hostname: "localhost",
					HostID:   "some-host",
					App:      app,
				})
				if err != nil {
					return err
				}

				// Upsert server.
				_, err = presence.UpsertApplicationServer(ctx, server)
				return err
			},
			deleteAllResourcesFunc: func(ctx context.Context) error {
				return presence.DeleteAllApplicationServers(ctx, apidefaults.Namespace)
			},
		},
		"KubeService": {
			resourceType: types.KindKubeService,
			createResourceFunc: func(ctx context.Context, name string, labels map[string]string) error {
				server, err := types.NewServerWithLabels(name, types.KindKubeService, types.ServerSpecV2{
					KubernetesClusters: []*types.KubernetesCluster{
						{Name: name, StaticLabels: labels},
					},
				}, labels)
				if err != nil {
					return err
				}

				// Upsert server.
				return presence.UpsertKubeService(ctx, server)
			},
			deleteAllResourcesFunc: func(ctx context.Context) error {
				return presence.DeleteAllKubeServices(ctx)
			},
		},
		"Node": {
			resourceType: types.KindNode,
			createResourceFunc: func(ctx context.Context, name string, labels map[string]string) error {
				server, err := types.NewServerWithLabels(name, types.KindNode, types.ServerSpecV2{}, labels)
				if err != nil {
					return err
				}

				// Upsert server.
				_, err = presence.UpsertNode(ctx, server)
				return err
			},
			deleteAllResourcesFunc: func(ctx context.Context) error {
				return presence.DeleteAllNodes(ctx, apidefaults.Namespace)
			},
		},
		"NodeWithDynamicLabels": {
			resourceType: types.KindNode,
			createResourceFunc: func(ctx context.Context, name string, labels map[string]string) error {
				dynamicLabels := make(map[string]types.CommandLabelV2)
				for name, value := range labels {
					dynamicLabels[name] = types.CommandLabelV2{
						Period:  types.NewDuration(time.Second),
						Command: []string{name},
						Result:  value,
					}
				}

				server, err := types.NewServer(name, types.KindNode, types.ServerSpecV2{
					CmdLabels: dynamicLabels,
				})
				if err != nil {
					return err
				}

				// Upsert server.
				_, err = presence.UpsertNode(ctx, server)
				return err
			},
			deleteAllResourcesFunc: func(ctx context.Context) error {
				return presence.DeleteAllNodes(ctx, apidefaults.Namespace)
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			resources, nextKey, err := presence.ListResources(ctx, proto.ListResourcesRequest{
				Limit:        1,
				Namespace:    apidefaults.Namespace,
				ResourceType: test.resourceType,
				StartKey:     "",
			})
			require.NoError(t, err)
			require.Empty(t, resources)
			require.Empty(t, nextKey)

			resourcesPerPage := 4
			totalWithLabels := 7
			totalWithoutLabels := 8
			labels := map[string]string{"env": "test"}
			totalResources := totalWithLabels + totalWithoutLabels

			// with labels
			for i := 0; i < totalWithLabels; i++ {
				err = test.createResourceFunc(ctx, fmt.Sprintf("foo-%d", i), labels)
				require.NoError(t, err)
			}

			// without labels
			for i := 0; i < totalWithoutLabels; i++ {
				err = test.createResourceFunc(ctx, fmt.Sprintf("foo-label-%d", i), map[string]string{})
				require.NoError(t, err)
			}

			var resultResources []types.ResourceWithLabels
			require.Eventually(t, func() bool {
				resources, nextKey, err = presence.ListResources(ctx, proto.ListResourcesRequest{
					Limit:        int32(resourcesPerPage),
					Namespace:    apidefaults.Namespace,
					ResourceType: test.resourceType,
					StartKey:     nextKey,
				})
				require.NoError(t, err)

				resultResources = append(resultResources, resources...)
				return len(resultResources) == totalResources
			}, time.Second, 100*time.Millisecond)
			require.Empty(t, nextKey)

			// list resources only with matching labels
			var resultResourcesWithLabels []types.ResourceWithLabels
			require.Eventually(t, func() bool {
				resources, nextKey, err = presence.ListResources(ctx, proto.ListResourcesRequest{
					Limit:        int32(resourcesPerPage),
					Namespace:    apidefaults.Namespace,
					ResourceType: test.resourceType,
					StartKey:     nextKey,
					Labels:       labels,
				})
				require.NoError(t, err)

				resultResourcesWithLabels = append(resultResourcesWithLabels, resources...)
				return len(resultResourcesWithLabels) == totalWithLabels
			}, time.Second, 100*time.Millisecond)
			require.Empty(t, nextKey)

			// delete everything
			err = test.deleteAllResourcesFunc(ctx)
			require.NoError(t, err)

			resources, nextKey, err = presence.ListResources(ctx, proto.ListResourcesRequest{
				Limit:        1,
				Namespace:    apidefaults.Namespace,
				ResourceType: test.resourceType,
				StartKey:     "",
			})
			require.NoError(t, err)
			require.Empty(t, nextKey)
			require.Empty(t, resources)
		})
	}

}
