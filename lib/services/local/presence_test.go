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

package local

import (
	"context"
	"fmt"
	"maps"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	gproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services/suite"
)

// TestApplicationServersCRUD verifies backend operations on app servers.
func TestApplicationServersCRUD(t *testing.T) {
	t.Parallel()
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

	// Make another app and an app server.
	appB, err := types.NewAppV3(types.Metadata{Name: "b"},
		types.AppSpecV3{URI: "http://localhost:8081"})
	require.NoError(t, err)
	serverB, err := types.NewAppServerV3(types.Metadata{
		Name: appB.GetName(),
	}, types.AppServerSpecV3{
		Hostname: "localhost",
		HostID:   uuid.New().String(),
		App:      appB,
	})
	require.NoError(t, err)

	// No app servers should be registered initially
	out, err := presence.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, out)

	// Create app servers.
	lease, err := presence.UpsertApplicationServer(ctx, serverA)
	require.NoError(t, err)
	require.Equal(t, &types.KeepAlive{}, lease)
	lease, err = presence.UpsertApplicationServer(ctx, serverB)
	require.NoError(t, err)
	require.Equal(t, &types.KeepAlive{}, lease)

	// Make sure all app servers are registered.
	out, err = presence.GetApplicationServers(ctx, serverA.GetNamespace())
	require.NoError(t, err)
	servers := types.AppServers(out)
	require.NoError(t, servers.SortByCustom(types.SortBy{Field: types.ResourceMetadataName}))
	require.Empty(t, cmp.Diff([]types.AppServer{serverA, serverB}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// Delete an app server.
	err = presence.DeleteApplicationServer(ctx, serverA.GetNamespace(), serverA.GetHostID(), serverA.GetName())
	require.NoError(t, err)

	// Expect only one to return.
	out, err = presence.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.AppServer{serverB}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// Upsert server with TTL.
	serverA.SetExpiry(clock.Now().UTC().Add(time.Hour))
	lease, err = presence.UpsertApplicationServer(ctx, serverA)
	require.NoError(t, err)
	require.Equal(t, &types.KeepAlive{
		Type:      types.KeepAlive_APP,
		Name:      serverA.GetName(),
		Namespace: serverA.GetNamespace(),
		HostID:    serverA.GetHostID(),
		Expires:   serverA.Expiry(),
	}, lease)

	// Delete all app servers.
	err = presence.DeleteAllApplicationServers(ctx, serverA.GetNamespace())
	require.NoError(t, err)

	// Expect no servers to return.
	out, err = presence.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, out)
}

func mustCreateDatabase(t *testing.T, name, protocol, uri string) *types.DatabaseV3 {
	database, err := types.NewDatabaseV3(
		types.Metadata{
			Name: name,
		},
		types.DatabaseSpecV3{
			Protocol: protocol,
			URI:      uri,
		},
	)
	require.NoError(t, err)
	return database
}

func TestDatabaseServersCRUD(t *testing.T) {
	t.Parallel()
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
		Database: mustCreateDatabase(t, "foo", defaults.ProtocolPostgres, "localhost:5432"),
		Hostname: "localhost",
		HostID:   uuid.New().String(),
	})
	require.NoError(t, err)

	// Initially expect not to be returned any servers.
	out, err := presence.GetDatabaseServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, out)

	// Upsert server.
	lease, err := presence.UpsertDatabaseServer(ctx, server)
	require.NoError(t, err)
	require.Equal(t, &types.KeepAlive{}, lease)

	// Check again, expect a single server to be found.
	out, err = presence.GetDatabaseServers(ctx, server.GetNamespace())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.DatabaseServer{server}, out, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

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
	require.Empty(t, out)

	// Upsert server with TTL.
	server.SetExpiry(clock.Now().UTC().Add(time.Hour))
	lease, err = presence.UpsertDatabaseServer(ctx, server)
	require.NoError(t, err)
	require.Equal(t, &types.KeepAlive{
		Type:      types.KeepAlive_DATABASE,
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
	require.Empty(t, out)
}

func TestNodeCRUD(t *testing.T) {
	t.Parallel()
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
		require.Empty(t, nodes)

		// Create nodes
		_, err = presence.UpsertNode(ctx, node1)
		require.NoError(t, err)
		_, err = presence.UpsertNode(ctx, node2)
		require.NoError(t, err)
	})

	t.Run("UpdateNode", func(t *testing.T) {
		node1, err = presence.GetNode(ctx, apidefaults.Namespace, node1.GetName())
		require.NoError(t, err)
		node1.SetAddr("1.2.3.4:8080")

		node2, err = presence.GetNode(ctx, apidefaults.Namespace, node2.GetName())
		require.NoError(t, err)

		node1, err = presence.UpdateNode(ctx, node1)
		require.NoError(t, err)
		require.Equal(t, "1.2.3.4:8080", node1.GetAddr())

		rev := node2.GetRevision()
		node2.SetAddr("1.2.3.4:9090")
		node2.SetRevision(node1.GetRevision())

		_, err = presence.UpdateNode(ctx, node2)
		require.True(t, trace.IsCompareFailed(err))
		node2.SetRevision(rev)

		node2, err = presence.UpdateNode(ctx, node2)
		require.NoError(t, err)
		require.Equal(t, "1.2.3.4:9090", node2.GetAddr())
	})

	// Run NodeGetters in nested subtests to allow parallelization.
	t.Run("NodeGetters", func(t *testing.T) {
		t.Run("GetNodes", func(t *testing.T) {
			t.Parallel()
			// Get all nodes, transparently handle limit exceeded errors
			nodes, err := presence.GetNodes(ctx, apidefaults.Namespace)
			require.NoError(t, err)
			require.Len(t, nodes, 2)
			require.Empty(t, cmp.Diff([]types.Server{node1, node2}, nodes,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// GetNodes should fail if namespace isn't provided
			_, err = presence.GetNodes(ctx, "")
			require.True(t, trace.IsBadParameter(err))
		})
		t.Run("GetNode", func(t *testing.T) {
			t.Parallel()
			// Get Node
			node, err := presence.GetNode(ctx, apidefaults.Namespace, "node1")
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(node1, node,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

			// GetNode should fail if node name isn't provided
			_, err = presence.GetNode(ctx, apidefaults.Namespace, "")
			require.True(t, trace.IsBadParameter(err))

			// GetNode should fail if namespace isn't provided
			_, err = presence.GetNode(ctx, "", "node1")
			require.True(t, trace.IsBadParameter(err))
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
		require.Empty(t, nodes)
	})
}

func TestListResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	tests := map[string]struct {
		resourceType           string
		createResourceFunc     func(context.Context, *PresenceService, string, map[string]string) error
		deleteAllResourcesFunc func(context.Context, *PresenceService) error
		expectedType           types.Resource
	}{
		"DatabaseServers": {
			resourceType: types.KindDatabaseServer,
			createResourceFunc: func(ctx context.Context, presence *PresenceService, name string, labels map[string]string) error {
				db := mustCreateDatabase(t, name, defaults.ProtocolPostgres, "localhost:5432")
				db.SetStaticLabels(labels)
				server, err := types.NewDatabaseServerV3(types.Metadata{
					Name:   name,
					Labels: labels,
				}, types.DatabaseServerSpecV3{
					Database: db,
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
			deleteAllResourcesFunc: func(ctx context.Context, presence *PresenceService) error {
				return presence.DeleteAllDatabaseServers(ctx, apidefaults.Namespace)
			},
		},
		"DatabaseServersSameHost": {
			resourceType: types.KindDatabaseServer,
			createResourceFunc: func(ctx context.Context, presence *PresenceService, name string, labels map[string]string) error {
				db := mustCreateDatabase(t, name, defaults.ProtocolPostgres, "localhost:5432")
				db.SetStaticLabels(labels)
				server, err := types.NewDatabaseServerV3(types.Metadata{
					Name:   name,
					Labels: labels,
				}, types.DatabaseServerSpecV3{
					Database: db,
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
			deleteAllResourcesFunc: func(ctx context.Context, presence *PresenceService) error {
				return presence.DeleteAllDatabaseServers(ctx, apidefaults.Namespace)
			},
		},
		"AppServers": {
			resourceType: types.KindAppServer,
			createResourceFunc: func(ctx context.Context, presence *PresenceService, name string, labels map[string]string) error {
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
			deleteAllResourcesFunc: func(ctx context.Context, presence *PresenceService) error {
				return presence.DeleteAllApplicationServers(ctx, apidefaults.Namespace)
			},
		},
		"AppServersSameHost": {
			resourceType: types.KindAppServer,
			createResourceFunc: func(ctx context.Context, presence *PresenceService, name string, labels map[string]string) error {
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
			deleteAllResourcesFunc: func(ctx context.Context, presence *PresenceService) error {
				return presence.DeleteAllApplicationServers(ctx, apidefaults.Namespace)
			},
		},
		"KubeServer": {
			resourceType: types.KindKubeServer,
			createResourceFunc: func(ctx context.Context, presence *PresenceService, name string, labels map[string]string) error {
				kube, err := types.NewKubernetesClusterV3(
					types.Metadata{
						Name:   name,
						Labels: labels,
					},
					types.KubernetesClusterSpecV3{},
				)
				if err != nil {
					return err
				}
				kubeServer, err := types.NewKubernetesServerV3FromCluster(kube, "host", "hostID")
				if err != nil {
					return err
				}

				// Upsert server.
				_, err = presence.UpsertKubernetesServer(ctx, kubeServer)
				return err
			},
			deleteAllResourcesFunc: func(ctx context.Context, presence *PresenceService) error {
				return presence.DeleteAllKubernetesServers(ctx)
			},
		},
		"Node": {
			resourceType: types.KindNode,
			createResourceFunc: func(ctx context.Context, presence *PresenceService, name string, labels map[string]string) error {
				server, err := types.NewServerWithLabels(name, types.KindNode, types.ServerSpecV2{}, labels)
				if err != nil {
					return err
				}

				// Upsert server.
				_, err = presence.UpsertNode(ctx, server)
				return err
			},
			deleteAllResourcesFunc: func(ctx context.Context, presence *PresenceService) error {
				return presence.DeleteAllNodes(ctx, apidefaults.Namespace)
			},
		},
		"NodeWithDynamicLabels": {
			resourceType: types.KindNode,
			createResourceFunc: func(ctx context.Context, presence *PresenceService, name string, labels map[string]string) error {
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
			deleteAllResourcesFunc: func(ctx context.Context, presence *PresenceService) error {
				return presence.DeleteAllNodes(ctx, apidefaults.Namespace)
			},
		},
		"WindowsDesktopService": {
			resourceType: types.KindWindowsDesktopService,
			createResourceFunc: func(ctx context.Context, presence *PresenceService, name string, labels map[string]string) error {
				desktop, err := types.NewWindowsDesktopServiceV3(
					types.Metadata{
						Name:   name,
						Labels: labels,
					},
					types.WindowsDesktopServiceSpecV3{
						Addr:            "localhost:1234",
						TeleportVersion: teleport.Version,
					})
				if err != nil {
					return err
				}

				_, err = presence.UpsertWindowsDesktopService(ctx, desktop)
				return err
			},
			deleteAllResourcesFunc: func(ctx context.Context, presence *PresenceService) error {
				return presence.DeleteAllWindowsDesktopServices(ctx)
			},
		},
		"WindowsDesktop": {
			resourceType: types.KindWindowsDesktop,
			createResourceFunc: func(ctx context.Context, presence *PresenceService, name string, labels map[string]string) error {
				desktopService := NewWindowsDesktopService(presence.Backend)
				desktop, err := types.NewWindowsDesktopV3(name, labels, types.WindowsDesktopSpecV3{
					Addr: "localhost:1234",
				})
				if err != nil {
					return err
				}

				err = desktopService.UpsertWindowsDesktop(ctx, desktop)
				return err
			},
			deleteAllResourcesFunc: func(ctx context.Context, presence *PresenceService) error {
				desktopService := NewWindowsDesktopService(presence.Backend)
				return desktopService.DeleteAllWindowsDesktops(ctx)
			},
		},
		"GitServer": {
			resourceType: types.KindGitServer,
			createResourceFunc: func(ctx context.Context, presence *PresenceService, name string, labels map[string]string) error {
				gitServerService, err := NewGitServerService(presence.Backend)
				if err != nil {
					return trace.Wrap(err)
				}

				gitServer, err := types.NewGitHubServer(types.GitHubServerMetadata{
					Organization: "my-org",
					Integration:  "my-org",
				})
				if err != nil {
					return trace.Wrap(err)
				}
				gitServer.SetName(name)
				newLabels := gitServer.GetLabels()
				maps.Copy(newLabels, labels)
				gitServer.SetStaticLabels(newLabels)

				_, err = gitServerService.UpsertGitServer(ctx, gitServer)
				return trace.Wrap(err)
			},
			deleteAllResourcesFunc: func(ctx context.Context, presence *PresenceService) error {
				gitServerService, err := NewGitServerService(presence.Backend)
				if err != nil {
					return trace.Wrap(err)
				}
				return gitServerService.DeleteAllGitServers(ctx)
			},
		},
	}

	for testName, test := range tests {
		testName := testName
		test := test
		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			backend, err := lite.NewWithConfig(ctx, lite.Config{
				Path:  t.TempDir(),
				Clock: clock,
			})
			require.NoError(t, err)

			presence := NewPresenceService(backend)

			resp, err := presence.ListResources(ctx, proto.ListResourcesRequest{
				Limit:        1,
				ResourceType: test.resourceType,
				StartKey:     "",
			})
			require.NoError(t, err)
			require.Empty(t, resp.Resources)
			require.Empty(t, resp.NextKey)
			require.Empty(t, resp.TotalCount)

			resourcesPerPage := 4
			totalWithLabels := 7
			totalWithoutLabels := 8
			labels := map[string]string{"env": "test"}
			totalResources := totalWithLabels + totalWithoutLabels

			// with labels
			for i := 0; i < totalWithLabels; i++ {
				err = test.createResourceFunc(ctx, presence, fmt.Sprintf("foo-%d", i), labels)
				require.NoError(t, err)
			}

			// without labels
			for i := 0; i < totalWithoutLabels; i++ {
				err = test.createResourceFunc(ctx, presence, fmt.Sprintf("foo-label-%d", i), map[string]string{})
				require.NoError(t, err)
			}

			resultResourcesLen := 0
			require.Eventually(t, func() bool {
				resp, err = presence.ListResources(ctx, proto.ListResourcesRequest{
					Limit:        int32(resourcesPerPage),
					Namespace:    apidefaults.Namespace,
					ResourceType: test.resourceType,
					StartKey:     resp.NextKey,
				})
				require.NoError(t, err)
				require.Empty(t, resp.TotalCount)

				resultResourcesLen += len(resp.Resources)
				if resultResourcesLen == totalResources {
					require.Empty(t, resp.NextKey)
				}
				return resultResourcesLen == totalResources
			}, time.Second, 100*time.Millisecond)

			// list resources only with matching labels
			resultResourcesWithLabelsLen := 0
			require.Eventually(t, func() bool {
				resp, err = presence.ListResources(ctx, proto.ListResourcesRequest{
					Limit:        int32(resourcesPerPage),
					Namespace:    apidefaults.Namespace,
					ResourceType: test.resourceType,
					StartKey:     resp.NextKey,
					Labels:       labels,
				})
				require.NoError(t, err)
				require.Empty(t, resp.TotalCount)

				resultResourcesWithLabelsLen += len(resp.Resources)
				if resultResourcesWithLabelsLen == totalWithLabels {
					require.Empty(t, resp.NextKey)
				}
				return resultResourcesWithLabelsLen == totalWithLabels
			}, time.Second, 100*time.Millisecond)

			// list resources only with matching search keywords
			resultResourcesWithSearchKeywordsLen := 0
			require.Eventually(t, func() bool {
				resp, err = presence.ListResources(ctx, proto.ListResourcesRequest{
					Limit:          int32(resourcesPerPage),
					Namespace:      apidefaults.Namespace,
					ResourceType:   test.resourceType,
					StartKey:       resp.NextKey,
					SearchKeywords: []string{"env", "test"},
				})
				require.NoError(t, err)
				require.Empty(t, resp.TotalCount)

				resultResourcesWithSearchKeywordsLen += len(resp.Resources)
				if resultResourcesWithSearchKeywordsLen == totalWithLabels {
					require.Empty(t, resp.NextKey)
				}
				return resultResourcesWithSearchKeywordsLen == totalWithLabels
			}, time.Second, 100*time.Millisecond)

			// list resources only with matching expression
			resultResourcesWithMatchExprsLen := 0
			require.Eventually(t, func() bool {
				resp, err = presence.ListResources(ctx, proto.ListResourcesRequest{
					Limit:               int32(resourcesPerPage),
					Namespace:           apidefaults.Namespace,
					ResourceType:        test.resourceType,
					StartKey:            resp.NextKey,
					PredicateExpression: `labels.env == "test"`,
				})
				require.NoError(t, err)
				require.Empty(t, resp.TotalCount)

				resultResourcesWithMatchExprsLen += len(resp.Resources)
				if resultResourcesWithMatchExprsLen == totalWithLabels {
					require.Empty(t, resp.NextKey)
				}
				return resultResourcesWithMatchExprsLen == totalWithLabels
			}, time.Second, 100*time.Millisecond)

			// delete everything
			err = test.deleteAllResourcesFunc(ctx, presence)
			require.NoError(t, err)

			resp, err = presence.ListResources(ctx, proto.ListResourcesRequest{
				Limit:        1,
				Namespace:    apidefaults.Namespace,
				ResourceType: test.resourceType,
				StartKey:     "",
			})
			require.NoError(t, err)
			require.Empty(t, resp.NextKey)
			require.Empty(t, resp.Resources)
			require.Empty(t, resp.TotalCount)
		})
	}
}

func TestListResources_Helpers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	namespace := apidefaults.Namespace
	bend, err := lite.NewWithConfig(ctx, lite.Config{
		Path:  t.TempDir(),
		Clock: clock,
	})
	require.NoError(t, err)
	presence := NewPresenceService(bend)

	tests := []struct {
		name  string
		fetch func(proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
	}{
		{
			name: "listResources",
			fetch: func(req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
				return presence.listResources(ctx, req)
			},
		},
		{
			name: "listResourcesWithSort",
			fetch: func(req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
				return presence.listResourcesWithSort(ctx, req)
			},
		},
		{
			name: "FakePaginate",
			fetch: func(req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
				nodes, err := presence.GetNodes(ctx, namespace)
				require.NoError(t, err)

				return FakePaginate(types.Servers(nodes).AsResources(), FakePaginateParams{
					ResourceType:   req.ResourceType,
					Limit:          req.Limit,
					Labels:         req.Labels,
					SearchKeywords: req.SearchKeywords,
					StartKey:       req.StartKey,
				})
			},
		},
	}

	t.Run("test fetching when there is 0 upserted nodes", func(t *testing.T) {
		req := proto.ListResourcesRequest{
			ResourceType: types.KindNode,
			Limit:        5,
		}
		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				resp, err := tc.fetch(req)
				require.NoError(t, err)
				require.Empty(t, resp.NextKey)
				require.Empty(t, resp.Resources)
				require.Empty(t, resp.TotalCount)
			})
		}
	})

	// Add some test servers.
	for i := 0; i < 20; i++ {
		server := suite.NewServer(types.KindNode, uuid.New().String(), "127.0.0.1:2022", namespace)
		_, err = presence.UpsertNode(ctx, server)
		require.NoError(t, err)
	}

	// Test servers have been inserted.
	nodes, err := presence.GetNodes(ctx, namespace)
	require.NoError(t, err)
	require.Len(t, nodes, 20)

	t.Run("test invalid limit value", func(t *testing.T) {
		req := proto.ListResourcesRequest{
			ResourceType: types.KindNode,
			Namespace:    namespace,
			Limit:        -1,
		}
		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				_, err := tc.fetch(req)
				require.True(t, trace.IsBadParameter(err))
			})
		}
	})

	t.Run("test retrieving entire list upfront", func(t *testing.T) {
		req := proto.ListResourcesRequest{
			ResourceType: types.KindNode,
			Namespace:    namespace,
			Limit:        int32(len(nodes)),
		}
		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				resp, err := tc.fetch(req)
				require.NoError(t, err)
				require.Empty(t, resp.NextKey)

				fetchedNodes, err := types.ResourcesWithLabels(resp.Resources).AsServers()
				require.NoError(t, err)
				require.Equal(t, nodes, fetchedNodes)
			})
		}
	})

	t.Run("test first, middle, last fetching", func(t *testing.T) {
		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				// First fetch.
				resp, err := tc.fetch(proto.ListResourcesRequest{
					ResourceType: types.KindNode,
					Namespace:    namespace,
					Limit:        10,
				})
				require.NoError(t, err)
				require.Len(t, resp.Resources, 10)

				fetchedNodes, err := types.ResourcesWithLabels(resp.Resources).AsServers()
				require.NoError(t, err)
				require.Equal(t, nodes[:10], fetchedNodes)
				require.Equal(t, backend.GetPaginationKey(nodes[10]), resp.NextKey) // 11th item

				// Middle fetch.
				resp, err = tc.fetch(proto.ListResourcesRequest{
					ResourceType: types.KindNode,
					Namespace:    namespace,
					StartKey:     resp.NextKey,
					Limit:        5,
				})
				require.NoError(t, err)
				require.Len(t, resp.Resources, 5)

				fetchedNodes, err = types.ResourcesWithLabels(resp.Resources).AsServers()
				require.NoError(t, err)
				require.Equal(t, nodes[10:15], fetchedNodes)
				require.Equal(t, backend.GetPaginationKey(nodes[15]), resp.NextKey) // 16th item

				// Last fetch.
				resp, err = tc.fetch(proto.ListResourcesRequest{
					ResourceType: types.KindNode,
					Namespace:    namespace,
					StartKey:     resp.NextKey,
					Limit:        5,
				})
				require.NoError(t, err)
				require.Len(t, resp.Resources, 5)

				fetchedNodes, err = types.ResourcesWithLabels(resp.Resources).AsServers()
				require.NoError(t, err)
				require.Equal(t, nodes[15:20], fetchedNodes)
				require.Empty(t, resp.NextKey)
			})
		}
	})

	t.Run("test one result filter", func(t *testing.T) {
		targetVal := nodes[14].GetName()
		req := proto.ListResourcesRequest{
			ResourceType:   types.KindNode,
			Namespace:      namespace,
			StartKey:       "",
			Limit:          5,
			SearchKeywords: []string{targetVal},
		}
		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				resp, err := tc.fetch(req)
				require.NoError(t, err)
				require.Len(t, resp.Resources, 1)
				require.Equal(t, targetVal, resp.Resources[0].GetName())
				require.Empty(t, resp.NextKey)
			})
		}
	})
}

func TestFakePaginate_TotalCount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	namespace := apidefaults.Namespace
	bend, err := lite.NewWithConfig(ctx, lite.Config{
		Path:  t.TempDir(),
		Clock: clock,
	})
	require.NoError(t, err)
	presence := NewPresenceService(bend)

	// Add some control servers.
	server := suite.NewServer(types.KindNode, "foo-bar", "127.0.0.1:2022", namespace)
	_, err = presence.UpsertNode(ctx, server)
	require.NoError(t, err)

	server = suite.NewServer(types.KindNode, "foo-baz", "127.0.0.1:2022", namespace)
	_, err = presence.UpsertNode(ctx, server)
	require.NoError(t, err)

	server = suite.NewServer(types.KindNode, "foo-qux", "127.0.0.1:2022", namespace)
	_, err = presence.UpsertNode(ctx, server)
	require.NoError(t, err)

	// Add some test servers.
	for i := 0; i < 10; i++ {
		server := suite.NewServer(types.KindNode, uuid.New().String(), "127.0.0.1:2022", namespace)
		_, err = presence.UpsertNode(ctx, server)
		require.NoError(t, err)
	}

	// Test servers have been inserted.
	nodes, err := presence.GetNodes(ctx, namespace)
	require.NoError(t, err)
	require.Len(t, nodes, 13)

	// Convert to resources.
	resources := types.Servers(nodes).AsResources()

	t.Run("total count without filter", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name  string
			limit int
		}{
			{
				name:  "single",
				limit: 1,
			},
			{
				name:  "even",
				limit: 4,
			},
			{
				name:  "odd",
				limit: 5,
			},
			{
				name:  "max",
				limit: len(nodes),
			},
		}

		for _, tc := range tests {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				req := FakePaginateParams{
					ResourceType:   types.KindNode,
					Limit:          int32(tc.limit),
					NeedTotalCount: true,
				}

				// First fetch.
				resp, err := FakePaginate(resources, req)
				require.NoError(t, err)
				require.Len(t, resp.Resources, tc.limit)
				require.Equal(t, resources[0:tc.limit], resp.Resources)
				require.Len(t, nodes, resp.TotalCount)

				// Next fetch should return same amount of totals.
				if tc.limit != len(nodes) {
					require.NotEmpty(t, resp.NextKey)

					req.StartKey = resp.NextKey
					resp, err = FakePaginate(resources, req)
					require.NoError(t, err)
					require.Len(t, resp.Resources, tc.limit)
					require.Equal(t, resources[tc.limit:tc.limit*2], resp.Resources)
					require.Len(t, nodes, resp.TotalCount)
				} else {
					require.Empty(t, resp.NextKey)
					require.Equal(t, resources, resp.Resources)
					require.Len(t, nodes, resp.TotalCount)
				}
			})
		}
	})

	t.Run("total count with no match", func(t *testing.T) {
		t.Parallel()
		req := FakePaginateParams{
			ResourceType:   types.KindNode,
			Limit:          5,
			NeedTotalCount: true,
			SearchKeywords: []string{"not-found"},
		}
		resp, err := FakePaginate(resources, req)
		require.NoError(t, err)
		require.Empty(t, resp.Resources)
		require.Empty(t, resp.NextKey)
		require.Empty(t, resp.TotalCount)
	})

	t.Run("total count with all matches", func(t *testing.T) {
		t.Parallel()
		req := FakePaginateParams{
			ResourceType:   types.KindNode,
			Limit:          5,
			NeedTotalCount: true,
			SearchKeywords: []string{"foo"},
		}
		resp, err := FakePaginate(resources, req)
		require.NoError(t, err)
		require.Len(t, resp.Resources, 3)
		require.Empty(t, resp.NextKey)
		require.Equal(t, 3, resp.TotalCount)
	})
}

func TestPresenceService_CancelSemaphoreLease(t *testing.T) {
	t.Parallel()
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

// TestListResources_DuplicateResourceFilterByLabel tests that we can search for a specific label
// among duplicated resources, and once a match is found, excludes duplicated matches from the result.
func TestListResources_DuplicateResourceFilterByLabel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	backend, err := lite.NewWithConfig(ctx, lite.Config{
		Path:  t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	presence := NewPresenceService(backend)

	// Same resource name, but have different labels.
	names := []string{"a", "a", "a", "a"}
	labels := []map[string]string{
		{"env": "prod"},
		{"env": "dev"},
		{"env": "qa"},
		{"env": "dev"},
	}

	tests := []struct {
		name            string
		kind            string
		insertResources func()
		wantNames       []string
	}{
		{
			name: "KindDatabaseServer",
			kind: types.KindDatabaseServer,
			insertResources: func() {
				for i := 0; i < len(names); i++ {
					db, err := types.NewDatabaseServerV3(types.Metadata{
						Name: fmt.Sprintf("name-%v", i),
					}, types.DatabaseServerSpecV3{
						HostID:   "_",
						Hostname: "_",
						Database: &types.DatabaseV3{
							Metadata: types.Metadata{
								Name:   names[i],
								Labels: labels[i],
							},
							Spec: types.DatabaseSpecV3{
								Protocol: "_",
								URI:      "_",
							},
						},
					})
					require.NoError(t, err)
					_, err = presence.UpsertDatabaseServer(ctx, db)
					require.NoError(t, err)
				}
			},
		},
		{
			name: "KindAppServer",
			kind: types.KindAppServer,
			insertResources: func() {
				for i := 0; i < len(names); i++ {
					server, err := types.NewAppServerV3(types.Metadata{
						Name: fmt.Sprintf("name-%v", i),
					}, types.AppServerSpecV3{
						HostID: "_",
						App: &types.AppV3{
							Metadata: types.Metadata{
								Name:   names[i],
								Labels: labels[i],
							},
							Spec: types.AppSpecV3{URI: "_"},
						},
					})
					require.NoError(t, err)
					_, err = presence.UpsertApplicationServer(ctx, server)
					require.NoError(t, err)
				}
			},
		},
		{
			name: "KindKubernetesCluster",
			kind: types.KindKubernetesCluster,
			insertResources: func() {
				for i := 0; i < len(names); i++ {

					kube, err := types.NewKubernetesClusterV3(
						types.Metadata{
							Name:   names[i],
							Labels: labels[i],
						},
						types.KubernetesClusterSpecV3{},
					)
					require.NoError(t, err)
					kubeServer, err := types.NewKubernetesServerV3FromCluster(
						kube,
						fmt.Sprintf("host-%v", i),
						fmt.Sprintf("hostID-%v", i),
					)
					require.NoError(t, err)
					// Upsert server.
					_, err = presence.UpsertKubernetesServer(ctx, kubeServer)
					require.NoError(t, err)

				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.insertResources()

			// Look among the duplicated resource by label
			resp, err := presence.ListResources(ctx, proto.ListResourcesRequest{
				ResourceType:   tc.kind,
				NeedTotalCount: true,
				Limit:          5,
				SearchKeywords: []string{"dev"},
			})
			require.NoError(t, err)
			require.Len(t, resp.Resources, 1)
			require.Equal(t, 1, resp.TotalCount)
			require.Equal(t, map[string]string{"env": "dev"}, resp.Resources[0].GetAllLabels())
		})
	}
}

func TestServerInfoCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bk, err := lite.New(ctx, backend.Params{"path": t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	presence := NewPresenceService(bk)

	serverInfoA, err := types.NewServerInfo(types.Metadata{
		Name: "server1",
		Labels: map[string]string{
			"a": "b",
			"c": "d",
		},
	}, types.ServerInfoSpecV1{})
	require.NoError(t, err)
	serverInfoA.SetSubKind(types.SubKindCloudInfo)

	serverInfoB, err := types.NewServerInfo(types.Metadata{
		Name: "server2",
	}, types.ServerInfoSpecV1{})
	require.NoError(t, err)
	serverInfoB.SetSubKind(types.SubKindCloudInfo)

	// No infos present initially.
	out, err := stream.Collect(presence.GetServerInfos(ctx))
	require.NoError(t, err)
	require.Empty(t, out)

	// Create infos.
	require.NoError(t, presence.UpsertServerInfo(ctx, serverInfoA))
	require.NoError(t, presence.UpsertServerInfo(ctx, serverInfoB))

	// Get server infos.
	out, err = stream.Collect(presence.GetServerInfos(ctx))
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.ServerInfo{serverInfoA, serverInfoB}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	outInfo, err := presence.GetServerInfo(ctx, serverInfoA.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(serverInfoA, outInfo, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	outInfo, err = presence.GetServerInfo(ctx, serverInfoB.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(serverInfoB, outInfo, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	_, err = presence.GetServerInfo(ctx, "nonexistant")
	require.True(t, trace.IsNotFound(err))

	// Delete a server info.
	require.NoError(t, presence.DeleteServerInfo(ctx, serverInfoA.GetName()))
	out, err = stream.Collect(presence.GetServerInfos(ctx))
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.ServerInfo{serverInfoB}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// Update server info.
	serverInfoB.SetStaticLabels(map[string]string{
		"e": "f",
		"g": "h",
	})
	require.NoError(t, presence.UpsertServerInfo(ctx, serverInfoB))
	out, err = stream.Collect(presence.GetServerInfos(ctx))
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.ServerInfo{serverInfoB}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// Delete all server infos.
	require.NoError(t, presence.DeleteAllServerInfos(ctx))
	out, err = stream.Collect(presence.GetServerInfos(ctx))
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestPresenceService_ListReverseTunnels(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	presenceService := NewPresenceService(bk)

	// With no resources, we should not get an error but we should get an empty
	// token and an empty slice.
	rcs, pageToken, err := presenceService.ListReverseTunnels(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, pageToken)
	require.Empty(t, rcs)

	// Create a few remote clusters
	for i := 0; i < 10; i++ {
		rc, err := types.NewReverseTunnel(fmt.Sprintf("rt-%d", i), []string{"example.com:443"})
		require.NoError(t, err)
		_, err = presenceService.UpsertReverseTunnel(ctx, rc)
		require.NoError(t, err)
	}

	// Check limit behaves
	rcs, pageToken, err = presenceService.ListReverseTunnels(ctx, 1, "")
	require.NoError(t, err)
	require.NotEmpty(t, pageToken)
	require.Len(t, rcs, 1)

	// Iterate through all pages with a low limit to ensure that pageToken
	// behaves correctly.
	rcs = []types.ReverseTunnel{}
	pageToken = ""
	for i := 0; i < 10; i++ {
		var got []types.ReverseTunnel
		got, pageToken, err = presenceService.ListReverseTunnels(ctx, 1, pageToken)
		require.NoError(t, err)
		if i == 9 {
			// For the final page, we should not get a page token
			require.Empty(t, pageToken)
		} else {
			require.NotEmpty(t, pageToken)
		}
		require.Len(t, got, 1)
		rcs = append(rcs, got...)
	}
	require.Len(t, rcs, 10)

	// Check that with a higher limit, we get all resources
	rcs, pageToken, err = presenceService.ListReverseTunnels(ctx, 20, "")
	require.NoError(t, err)
	require.Empty(t, pageToken)
	require.Len(t, rcs, 10)
}

func TestPresenceService_UpsertReverseTunnel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	presenceService := NewPresenceService(bk)

	rt, err := types.NewReverseTunnel("my-tunnel", []string{"example.com:443"})
	require.NoError(t, err)

	// Upsert a reverse tunnel
	got, err := presenceService.UpsertReverseTunnel(ctx, rt)
	require.NoError(t, err)
	// Check that the returned resource is the same as the one we upserted
	require.Empty(t, cmp.Diff(rt, got, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// Do a get to check revision matches
	fetched, err := presenceService.GetReverseTunnel(ctx, rt.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(got, fetched))
}

func TestPresenceService_RelayServer(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	defer bk.Close()

	var p *PresenceService
	require.NotPanics(t, func() {
		p = NewPresenceService(bk)
	})

	_, err = p.UpsertRelayServer(ctx, nil)
	require.ErrorAs(t, err, new(*trace.BadParameterError))

	relayA := &presencev1.RelayServer{
		Kind:    types.KindRelayServer,
		SubKind: "",
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "a",
		},
	}

	upsertedA, err := p.UpsertRelayServer(ctx, gproto.CloneOf(relayA))
	require.NoError(t, err)
	require.NotNil(t, upsertedA.GetMetadata())

	diffOpts := []cmp.Option{
		protocmp.Transform(),
		protocmp.IgnoreFields((*headerv1.Metadata)(nil), "revision"),
	}

	require.Empty(t, cmp.Diff(relayA, upsertedA, diffOpts...))

	gottenA, err := p.GetRelayServer(ctx, "a")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(relayA, gottenA, diffOpts...))

	_, err = p.GetRelayServer(ctx, "b")
	require.ErrorAs(t, err, new(*trace.NotFoundError))

	err = p.DeleteRelayServer(ctx, "a")
	require.NoError(t, err)

	_, err = p.GetRelayServer(ctx, "a")
	require.ErrorAs(t, err, new(*trace.NotFoundError))
	err = p.DeleteRelayServer(ctx, "a")
	require.ErrorAs(t, err, new(*trace.NotFoundError))

	relayB := &presencev1.RelayServer{
		Kind:    types.KindRelayServer,
		SubKind: "",
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "b",
		},
	}

	_, err = p.UpsertRelayServer(ctx, gproto.CloneOf(relayA))
	require.NoError(t, err)
	_, err = p.UpsertRelayServer(ctx, gproto.CloneOf(relayB))
	require.NoError(t, err)

	listedRelays, nextPageToken, err := p.ListRelayServers(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, nextPageToken)
	require.Len(t, listedRelays, 2)
	require.Empty(t, cmp.Diff(relayA, listedRelays[0], diffOpts...))
	require.Empty(t, cmp.Diff(relayB, listedRelays[1], diffOpts...))

	shortList, nextPageToken, err := p.ListRelayServers(ctx, 1, "")
	require.NoError(t, err)
	require.Equal(t, "b", nextPageToken)
	require.Len(t, shortList, 1)
	require.Empty(t, cmp.Diff(relayA, shortList[0], diffOpts...))

	shortList, nextPageToken, err = p.ListRelayServers(ctx, 1, "b")
	require.NoError(t, err)
	require.Empty(t, nextPageToken)
	require.Len(t, shortList, 1)
	require.Empty(t, cmp.Diff(relayB, shortList[0], diffOpts...))
}
