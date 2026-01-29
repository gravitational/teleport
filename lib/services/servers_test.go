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

package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

// TestServersCompare tests comparing two servers
func TestServersCompare(t *testing.T) {
	t.Parallel()

	t.Run("compare servers", func(t *testing.T) {
		node := &types.ServerV2{
			Kind:    types.KindNode,
			Version: types.V2,
			Metadata: types.Metadata{
				Name:      "node1",
				Namespace: apidefaults.Namespace,
				Labels:    map[string]string{"a": "b"},
			},
			Spec: types.ServerSpecV2{
				Addr:      "localhost:3022",
				CmdLabels: map[string]types.CommandLabelV2{"a": {Period: types.Duration(time.Minute), Command: []string{"ls", "-l"}}},
				Version:   "4.0.0",
			},
		}
		node.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 6, time.UTC))
		// Server is equal to itself
		require.Equal(t, Equal, CompareServers(node, node))

		// Only timestamps are different
		node2 := *node
		node2.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 8, time.UTC))
		require.Equal(t, OnlyTimestampsDifferent, CompareServers(node, &node2))

		// Labels are different
		node2 = *node
		node2.Metadata.Labels = map[string]string{"a": "d"}
		require.Equal(t, Different, CompareServers(node, &node2))

		// Command labels are different
		node2 = *node
		node2.Spec.CmdLabels = map[string]types.CommandLabelV2{"a": {Period: types.Duration(time.Minute), Command: []string{"ls", "-lR"}}}
		require.Equal(t, Different, CompareServers(node, &node2))

		// Address has changed
		node2 = *node
		node2.Spec.Addr = "localhost:3033"
		require.Equal(t, Different, CompareServers(node, &node2))

		// Proxy addr has changed
		node2 = *node
		node2.Spec.PublicAddrs = []string{"localhost:3033"}
		require.Equal(t, Different, CompareServers(node, &node2))

		// Hostname has changed
		node2 = *node
		node2.Spec.Hostname = "luna2"
		require.Equal(t, Different, CompareServers(node, &node2))

		// TeleportVersion has changed
		node2 = *node
		node2.Spec.Version = "5.0.0"
		require.Equal(t, Different, CompareServers(node, &node2))

		// Rotation has changed
		node2 = *node
		node2.Spec.Rotation = types.Rotation{
			State:       types.RotationStateInProgress,
			Phase:       types.RotationPhaseUpdateClients,
			CurrentID:   "1",
			Started:     time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
			GracePeriod: types.Duration(3 * time.Hour),
			LastRotated: time.Date(2017, 2, 3, 4, 5, 6, 7, time.UTC),
			Schedule: types.RotationSchedule{
				UpdateClients: time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
				UpdateServers: time.Date(2018, 3, 4, 7, 6, 7, 8, time.UTC),
				Standby:       time.Date(2018, 3, 4, 5, 6, 13, 8, time.UTC),
			},
		}
		require.Equal(t, Different, CompareServers(node, &node2))

		// Scope has changed
		node2 = *node
		node2.Scope = "test"
		require.Equal(t, Different, CompareServers(node, &node2))
	})

	t.Run("compare application servers", func(t *testing.T) {
		appSrv := &types.AppServerV3{
			Kind:    types.KindAppServer,
			Version: types.V3,
			Metadata: types.Metadata{
				Name:      "app1",
				Namespace: apidefaults.Namespace,
			},
			Spec: types.AppServerSpecV3{
				Hostname: "app.example.com",
				Version:  "4.0.0",
				App: &types.AppV3{
					Metadata: types.Metadata{
						Name:      "app1",
						Namespace: apidefaults.Namespace,
						Labels:    map[string]string{"a": "b"},
					},
					Spec: types.AppSpecV3{
						URI: "http://localhost:8080",
					},
				},
			},
		}
		appSrv.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 6, time.UTC))

		// AppServer is equal to itself
		require.Equal(t, Equal, CompareServers(appSrv, appSrv))

		// Name has changed
		appSrv2 := *appSrv
		appSrv2.Metadata.Name = "app2"
		require.Equal(t, Different, CompareServers(appSrv, &appSrv2))

		// Namespace has changed
		appSrv2 = *appSrv
		appSrv2.Metadata.Namespace = "new-namespace"
		require.Equal(t, Different, CompareServers(appSrv, &appSrv2))

		// TeleportVersion has changed
		appSrv2 = *appSrv
		appSrv2.Spec.Version = "5.0.0"
		require.Equal(t, Different, CompareServers(appSrv, &appSrv2))

		// Rotation has changed
		appSrv2 = *appSrv
		appSrv2.Spec.Rotation = types.Rotation{
			State:       types.RotationStateInProgress,
			Phase:       types.RotationPhaseUpdateClients,
			CurrentID:   "1",
			Started:     time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
			GracePeriod: types.Duration(3 * time.Hour),
			LastRotated: time.Date(2017, 2, 3, 4, 5, 6, 7, time.UTC),
			Schedule: types.RotationSchedule{
				UpdateClients: time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
				UpdateServers: time.Date(2018, 3, 4, 7, 6, 7, 8, time.UTC),
				Standby:       time.Date(2018, 3, 4, 5, 6, 13, 8, time.UTC),
			},
		}
		require.Equal(t, Different, CompareServers(appSrv, &appSrv2))

		// Application definition has changed
		appSrv2 = *appSrv
		appSrv2.Spec.App = &types.AppV3{
			Metadata: types.Metadata{
				Name:      "app1",
				Namespace: apidefaults.Namespace,
			},
			Spec: types.AppSpecV3{
				URI: "http://localhost:9090",
			},
		}
		require.Equal(t, Different, CompareServers(appSrv, &appSrv2))

		// ProxyIDs have changed
		appSrv2 = *appSrv
		appSrv2.Spec.ProxyIDs = []string{"new-proxy-id"}
		require.Equal(t, Different, CompareServers(appSrv, &appSrv2))

		// RelayGroup has changed
		appSrv2 = *appSrv
		appSrv2.Spec.RelayGroup = "new-relay-group"
		require.Equal(t, Different, CompareServers(appSrv, &appSrv2))

		// RelayIDs have changed
		appSrv2 = *appSrv
		appSrv2.Spec.RelayIds = []string{"new-relay-id"}
		require.Equal(t, Different, CompareServers(appSrv, &appSrv2))

		// Scope has changed
		appSrv2 = *appSrv
		appSrv2.Scope = "test"
		require.Equal(t, Different, CompareServers(appSrv, &appSrv2))

		// App Labels are different
		appSrv2 = *appSrv
		appSrv2.Spec.App = appSrv2.Spec.App.Copy()
		appSrv2.Spec.App.Metadata.Labels = map[string]string{"a": "d"}
		require.Equal(t, Different, CompareServers(appSrv, &appSrv2))

		// AppServer labels are different
		appSrv2 = *appSrv
		appSrv2.Metadata.Labels = map[string]string{"b": "c"}
		require.Equal(t, Different, CompareServers(appSrv, &appSrv2))

		// App label is preferred when corresponding AppServer label exists and is different
		appSrv2 = *appSrv
		appSrv2.Metadata.Labels = map[string]string{"a": "d"}
		require.Equal(t, Equal, CompareServers(appSrv, &appSrv2))

		// Only timestamps are different
		appSrv2 = *appSrv
		appSrv2.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 8, time.UTC))
		require.Equal(t, OnlyTimestampsDifferent, CompareServers(appSrv, &appSrv2))
	})

	t.Run("compare database services", func(t *testing.T) {
		service := &types.DatabaseServiceV1{
			ResourceHeader: types.ResourceHeader{
				Kind: types.KindDatabaseService,
				Metadata: types.Metadata{
					Name:   "dbServiceT01",
					Labels: map[string]string{"env": "stg"},
				},
			},
			Spec: types.DatabaseServiceSpecV1{
				ResourceMatchers: []*types.DatabaseResourceMatcher{
					{Labels: &types.Labels{"env": []string{"stg"}}},
				},
			},
		}
		service.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 6, time.UTC))

		// DatabaseService is equal to itself
		require.Equal(t, Equal, CompareServers(service, service))

		// Name is different
		service2 := *service
		service2.Metadata.Name = "dbServiceT02"
		require.Equal(t, Different, CompareServers(service, &service2))

		// Namespace is different
		service2 = *service
		service2.Metadata.Namespace = "new-namespace"
		require.Equal(t, Different, CompareServers(service, &service2))

		// Resource Matcher has changed
		service2 = *service
		service2.Spec.ResourceMatchers = []*types.DatabaseResourceMatcher{
			{Labels: &types.Labels{"env": []string{"stg", "qa"}}},
		}
		require.Equal(t, Different, CompareServers(service, &service2))

		// Labels are different
		service2 = *service
		service2.Metadata.Labels = map[string]string{"a": "d"}
		require.Equal(t, Different, CompareServers(service, &service2))

		// Only timestamps are different
		service2 = *service
		service2.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 8, time.UTC))
		require.Equal(t, OnlyTimestampsDifferent, CompareServers(service, &service2))
	})

	t.Run("compare Kubernetes servers", func(t *testing.T) {
		kubeSrv := &types.KubernetesServerV3{
			Kind:    types.KindKubeServer,
			Version: types.V3,
			Metadata: types.Metadata{
				Name:      "kube1",
				Namespace: apidefaults.Namespace,
				Labels:    map[string]string{"a": "b"},
			},
			Spec: types.KubernetesServerSpecV3{
				Hostname: "kube.example.com",
				Version:  "4.0.0",
				Cluster: &types.KubernetesClusterV3{
					Kind: types.KindKubernetesCluster,
				},
			},
		}
		kubeSrv.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 6, time.UTC))

		// KubernetesServer is equal to itself
		require.Equal(t, Equal, CompareServers(kubeSrv, kubeSrv))

		// Name is different
		kubeSrv2 := *kubeSrv
		kubeSrv2.Metadata.Name = "kube2"
		require.Equal(t, Different, CompareServers(kubeSrv, &kubeSrv2))

		// Namespace is differentß
		kubeSrv2 = *kubeSrv
		kubeSrv2.Metadata.Namespace = "new-namespace"
		require.Equal(t, Different, CompareServers(kubeSrv, &kubeSrv2))

		// TeleportVersion has changed
		kubeSrv2 = *kubeSrv
		kubeSrv2.Spec.Version = "5.0.0"
		require.Equal(t, Different, CompareServers(kubeSrv, &kubeSrv2))

		// Rotation has changed
		kubeSrv2 = *kubeSrv
		kubeSrv2.Spec.Rotation = types.Rotation{
			State:       types.RotationStateInProgress,
			Phase:       types.RotationPhaseUpdateClients,
			CurrentID:   "1",
			Started:     time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
			GracePeriod: types.Duration(3 * time.Hour),
			LastRotated: time.Date(2017, 2, 3, 4, 5, 6, 7, time.UTC),
			Schedule: types.RotationSchedule{
				UpdateClients: time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
				UpdateServers: time.Date(2018, 3, 4, 7, 6, 7, 8, time.UTC),
				Standby:       time.Date(2018, 3, 4, 5, 6, 13, 8, time.UTC),
			},
		}
		require.Equal(t, Different, CompareServers(kubeSrv, &kubeSrv2))

		// Kubernetes cluster definition has changed
		kubeSrv2 = *kubeSrv
		kubeSrv2.Spec.Cluster = &types.KubernetesClusterV3{
			Kind: types.KindKubernetesCluster,
			Metadata: types.Metadata{
				Name:      "kube-cluster-2",
				Namespace: apidefaults.Namespace,
			},
		}
		require.Equal(t, Different, CompareServers(kubeSrv, &kubeSrv2))

		// ProxyIDs have changed
		kubeSrv2 = *kubeSrv
		kubeSrv2.Spec.ProxyIDs = []string{"new-proxy-id"}
		require.Equal(t, Different, CompareServers(kubeSrv, &kubeSrv2))

		// RelayGroup has changed
		kubeSrv2 = *kubeSrv
		kubeSrv2.Spec.RelayGroup = "new-relay-group"
		require.Equal(t, Different, CompareServers(kubeSrv, &kubeSrv2))

		// RelayIDs have changed
		kubeSrv2 = *kubeSrv
		kubeSrv2.Spec.RelayIds = []string{"new-relay-id"}
		require.Equal(t, Different, CompareServers(kubeSrv, &kubeSrv2))

		// Scope has changed
		kubeSrv2 = *kubeSrv
		kubeSrv2.Scope = "test"
		require.Equal(t, Different, CompareServers(kubeSrv, &kubeSrv2))

		// Labels are different
		kubeSrv2 = *kubeSrv
		kubeSrv2.Metadata.Labels = map[string]string{"a": "d"}
		require.Equal(t, Different, CompareServers(kubeSrv, &kubeSrv2))

		// Only timestamps are different
		kubeSrv2 = *kubeSrv
		kubeSrv2.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 8, time.UTC))
		require.Equal(t, OnlyTimestampsDifferent, CompareServers(kubeSrv, &kubeSrv2))
	})

	t.Run("compare database servers", func(t *testing.T) {
		dbSrv := &types.DatabaseServerV3{
			Kind:    types.KindDatabaseServer,
			Version: types.V3,
			Metadata: types.Metadata{
				Name:      "db1",
				Namespace: apidefaults.Namespace,
				Labels:    map[string]string{"a": "b"},
			},
			Spec: types.DatabaseServerSpecV3{
				Hostname: "db.example.com",
				Version:  "4.0.0",
				Database: &types.DatabaseV3{
					Kind: types.KindDatabase,
				},
			},
		}
		dbSrv.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 6, time.UTC))

		// DatabaseServer is equal to itself
		require.Equal(t, Equal, CompareServers(dbSrv, dbSrv))

		// Name is different
		dbSrv2 := *dbSrv
		dbSrv2.Metadata.Name = "db2"
		require.Equal(t, Different, CompareServers(dbSrv, &dbSrv2))

		// Namespace is different
		dbSrv2 = *dbSrv
		dbSrv2.Metadata.Namespace = "new-namespace"
		require.Equal(t, Different, CompareServers(dbSrv, &dbSrv2))

		// TeleportVersion has changed
		dbSrv2 = *dbSrv
		dbSrv2.Spec.Version = "5.0.0"
		require.Equal(t, Different, CompareServers(dbSrv, &dbSrv2))

		// Rotation has changed
		dbSrv2 = *dbSrv
		dbSrv2.Spec.Rotation = types.Rotation{
			State:       types.RotationStateInProgress,
			Phase:       types.RotationPhaseUpdateClients,
			CurrentID:   "1",
			Started:     time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
			GracePeriod: types.Duration(3 * time.Hour),
			LastRotated: time.Date(2017, 2, 3, 4, 5, 6, 7, time.UTC),
			Schedule: types.RotationSchedule{
				UpdateClients: time.Date(2018, 3, 4, 5, 6, 7, 8, time.UTC),
				UpdateServers: time.Date(2018, 3, 4, 7, 6, 7, 8, time.UTC),
				Standby:       time.Date(2018, 3, 4, 5, 6, 13, 8, time.UTC),
			},
		}
		require.Equal(t, Different, CompareServers(dbSrv, &dbSrv2))

		// Database definition has changed
		dbSrv2 = *dbSrv
		dbSrv2.Spec.Database = &types.DatabaseV3{
			Kind: types.KindDatabase,
			Metadata: types.Metadata{
				Name:      "db2",
				Namespace: apidefaults.Namespace,
			},
		}
		require.Equal(t, Different, CompareServers(dbSrv, &dbSrv2))

		// ProxyIDs have changed
		dbSrv2 = *dbSrv
		dbSrv2.Spec.ProxyIDs = []string{"new-proxy-id"}
		require.Equal(t, Different, CompareServers(dbSrv, &dbSrv2))

		// RelayGroup has changed
		dbSrv2 = *dbSrv
		dbSrv2.Spec.RelayGroup = "new-relay-group"
		require.Equal(t, Different, CompareServers(dbSrv, &dbSrv2))

		// RelayIDs have changed
		dbSrv2 = *dbSrv
		dbSrv2.Spec.RelayIds = []string{"new-relay-id"}
		require.Equal(t, Different, CompareServers(dbSrv, &dbSrv2))

		// Scope has changed
		dbSrv2 = *dbSrv
		dbSrv2.Scope = "test"
		require.Equal(t, Different, CompareServers(dbSrv, &dbSrv2))

		// Labels are different
		dbSrv2 = *dbSrv
		dbSrv2.Metadata.Labels = map[string]string{"a": "d"}
		require.Equal(t, Different, CompareServers(dbSrv, &dbSrv2))

		// Only timestamps are different
		dbSrv2 = *dbSrv
		dbSrv2.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 8, time.UTC))
		require.Equal(t, OnlyTimestampsDifferent, CompareServers(dbSrv, &dbSrv2))
	})

	t.Run("compare windows desktop services", func(t *testing.T) {
		winSrv := &types.WindowsDesktopServiceV3{
			ResourceHeader: types.ResourceHeader{
				Kind: types.KindWindowsDesktopService,
				Metadata: types.Metadata{
					Name:      "winDesktopService1",
					Namespace: apidefaults.Namespace,
					Labels:    map[string]string{"env": "stg"},
				},
			},
			Spec: types.WindowsDesktopServiceSpecV3{
				Hostname:        "win-desktop.example.com",
				Addr:            "win-desktop.example.com:3389",
				TeleportVersion: "4.0.0",
			},
		}
		winSrv.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 6, time.UTC))

		// WindowsDesktopService is equal to itself
		require.Equal(t, Equal, CompareServers(winSrv, winSrv))

		// Name is different
		winSrv2 := *winSrv
		winSrv2.Metadata.Name = "winDesktopService2"
		require.Equal(t, Different, CompareServers(winSrv, &winSrv2))

		// Address has changed
		winSrv2 = *winSrv
		winSrv2.Spec.Addr = "new-win-desktop.example.com:3389"
		require.Equal(t, Different, CompareServers(winSrv, &winSrv2))

		// TeleportVersion has changed
		winSrv2 = *winSrv
		winSrv2.Spec.TeleportVersion = "5.0.0"
		require.Equal(t, Different, CompareServers(winSrv, &winSrv2))

		// ProxyIDs have changed
		winSrv2 = *winSrv
		winSrv2.Spec.ProxyIDs = []string{"new-proxy-id"}
		require.Equal(t, Different, CompareServers(winSrv, &winSrv2))

		// RelayGroup has changed
		winSrv2 = *winSrv
		winSrv2.Spec.RelayGroup = "new-relay-group"
		require.Equal(t, Different, CompareServers(winSrv, &winSrv2))

		// RelayIDs have changed
		winSrv2 = *winSrv
		winSrv2.Spec.RelayIds = []string{"new-relay-id"}
		require.Equal(t, Different, CompareServers(winSrv, &winSrv2))

		// Labels are different
		winSrv2 = *winSrv
		winSrv2.Metadata.Labels = map[string]string{"a": "d"}
		require.Equal(t, Different, CompareServers(winSrv, &winSrv2))

		// Only timestamps are different
		winSrv2 = *winSrv
		winSrv2.SetExpiry(time.Date(2018, 1, 2, 3, 4, 5, 8, time.UTC))
		require.Equal(t, OnlyTimestampsDifferent, CompareServers(winSrv, &winSrv2))

	})
}

// TestGuessProxyHostAndVersion checks that the GuessProxyHostAndVersion
// correctly guesses the public address of the proxy (Teleport Cluster).
func TestGuessProxyHostAndVersion(t *testing.T) {
	t.Parallel()

	// No proxies passed in.
	host, version, err := GuessProxyHostAndVersion(nil)
	require.Empty(t, host)
	require.Empty(t, version)
	require.True(t, trace.IsNotFound(err))

	// No proxies have public address set.
	proxyA := types.ServerV2{}
	proxyA.Spec.Hostname = "test-A"
	proxyA.Spec.Version = "test-A"

	host, version, err = GuessProxyHostAndVersion([]types.Server{&proxyA})
	require.Equal(t, host, fmt.Sprintf("%v:%v", proxyA.Spec.Hostname, defaults.HTTPListenPort))
	require.Equal(t, version, proxyA.Spec.Version)
	require.NoError(t, err)

	// At least one proxy has proxy address set.
	proxyB := types.ServerV2{}
	proxyB.Spec.PublicAddrs = []string{"test-B"}
	proxyB.Spec.Version = "test-B"

	host, version, err = GuessProxyHostAndVersion([]types.Server{&proxyA, &proxyB})
	require.Equal(t, host, proxyB.Spec.PublicAddrs[0])
	require.Equal(t, version, proxyB.Spec.Version)
	require.NoError(t, err)
}
