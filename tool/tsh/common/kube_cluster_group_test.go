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
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
)

// TestKubeClusterGroup_SharedClientPerTeleportCluster verifies that
// repeated AuthClient calls connect each Teleport cluster once.
func TestKubeClusterGroup_SharedClientPerTeleportCluster(t *testing.T) {
	t.Parallel()

	group, connector := newTestKubeClusterGroup()

	root1, err := group.AuthClient(t.Context(), "root")
	require.NoError(t, err)
	root2, err := group.AuthClient(t.Context(), "root")
	require.NoError(t, err)
	require.Same(t, root1, root2)

	leaf, err := group.AuthClient(t.Context(), "leaf")
	require.NoError(t, err)
	require.NotSame(t, root1, leaf)
	require.Len(t, connector.clients, 2)
}

// TestKubeClusterGroup_ConcurrentCallbacksShareClients verifies that
// ForEach callbacks fetching their clients concurrently still connect each Teleport cluster once.
func TestKubeClusterGroup_ConcurrentCallbacksShareClients(t *testing.T) {
	t.Parallel()

	group, connector := newTestKubeClusterGroup()

	err := group.ForEach(t.Context(), func(ctx context.Context, cluster kubeconfig.LocalProxyCluster) error {
		_, err := group.AuthClient(ctx, cluster.TeleportCluster)
		return trace.Wrap(err)
	})
	require.NoError(t, err)
	require.Len(t, connector.clients, 2)
}

// TestKubeClusterGroup_UnknownTeleportCluster verifies that
// asking for a Teleport cluster outside the group fails without touching the connector.
func TestKubeClusterGroup_UnknownTeleportCluster(t *testing.T) {
	t.Parallel()

	group, connector := newTestKubeClusterGroup()

	_, err := group.AuthClient(t.Context(), "other")
	require.True(t, trace.IsBadParameter(err))
	require.Empty(t, connector.clients)
}

// TestKubeClusterGroup_ConnectErrorsAreCached verifies that
// a failed connect is not retried within the group.
func TestKubeClusterGroup_ConnectErrorsAreCached(t *testing.T) {
	t.Parallel()

	group, connector := newTestKubeClusterGroup()

	connectErr := trace.ConnectionProblem(nil, "connect failed")
	connector.err = connectErr
	_, err := group.AuthClient(t.Context(), "root")
	require.ErrorIs(t, err, connectErr)

	connector.err = nil
	_, err = group.AuthClient(t.Context(), "root")
	require.ErrorIs(t, err, connectErr, "a failed connect must not be retried within the group")
	require.Empty(t, connector.clients)
}

// TestKubeClusterGroup_CloseClosesOnlyConnectedClients verifies that
// Close closes the connected clients and skips the never-connected ones.
func TestKubeClusterGroup_CloseClosesOnlyConnectedClients(t *testing.T) {
	t.Parallel()

	group, connector := newTestKubeClusterGroup()

	_, err := group.AuthClient(t.Context(), "root")
	require.NoError(t, err)

	group.Close(t.Context())
	require.Len(t, connector.clients, 1)
	require.Equal(t, 1, connector.clients["root"].closes)
}

// TestKubeClusterGroup_BoundedConcurrency verifies that
// ForEach runs at most the group's concurrency callbacks at a time.
func TestKubeClusterGroup_BoundedConcurrency(t *testing.T) {
	t.Parallel()

	const numClusters, concurrency = 4, 2
	clusters := newTestKubeClusters(numClusters)

	synctest.Test(t, func(t *testing.T) {
		group := newKubeClusterGroup(&fakeClusterConnector{}, clusters, concurrency)

		start := time.Now()
		err := group.ForEach(t.Context(), func(ctx context.Context, cluster kubeconfig.LocalProxyCluster) error {
			time.Sleep(time.Second)
			return nil
		})
		require.NoError(t, err)
		// Four one-second callbacks, two at a time.
		require.Equal(t, 2*time.Second, time.Since(start))
	})
}

// newTestKubeClusterGroup returns a group of kube clusters spanning two Teleport clusters, backed by a fake connector.
func newTestKubeClusterGroup() (*kubeClusterGroup, *fakeClusterConnector) {
	clusters := kubeconfig.LocalProxyClusters{
		{TeleportCluster: "root", KubeCluster: "kube-root-0"},
		{TeleportCluster: "root", KubeCluster: "kube-root-1"},
		{TeleportCluster: "leaf", KubeCluster: "kube-leaf-0"},
	}
	connector := &fakeClusterConnector{}
	return newKubeClusterGroup(connector, clusters, 2), connector
}

type fakeGroupAuthClient struct {
	authclient.ClientI
	closes int
}

func (f *fakeGroupAuthClient) Close() error {
	f.closes++
	return nil
}

type fakeClusterConnector struct {
	err error

	// mu guards clients: [kubeClusterGroup.ForEach] callbacks connect concurrently.
	mu      sync.Mutex
	clients map[string]*fakeGroupAuthClient
}

func (f *fakeClusterConnector) ConnectToCluster(ctx context.Context, clusterName string) (authclient.ClientI, error) {
	if f.err != nil {
		return nil, trace.Wrap(f.err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	client := &fakeGroupAuthClient{}
	if f.clients == nil {
		f.clients = map[string]*fakeGroupAuthClient{}
	}
	f.clients[clusterName] = client
	return client, nil
}
