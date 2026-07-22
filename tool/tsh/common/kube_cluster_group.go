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

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
)

// clusterConnector provides auth clients for Teleport clusters.
// [client.ClusterClient] implements it.
type clusterConnector interface {
	ConnectToCluster(ctx context.Context, clusterName string) (authclient.ClientI, error)
}

// kubeClusterGroup runs operations across kube clusters spanning one or more Teleport clusters.
// Operations run with bounded concurrency, and operations touching the same Teleport cluster
// share one lazily-connected auth client, so a Teleport cluster costs at most one connection per group.
type kubeClusterGroup struct {
	connector   clusterConnector
	clusters    kubeconfig.LocalProxyClusters
	concurrency int
	clients     map[string]*lazyAuthClient
}

type lazyAuthClient struct {
	once   sync.Once
	client authclient.ClientI
	err    error
}

func newKubeClusterGroup(connector clusterConnector, clusters kubeconfig.LocalProxyClusters, concurrency int) *kubeClusterGroup {
	clients := make(map[string]*lazyAuthClient, len(clusters))
	for _, cluster := range clusters {
		clients[cluster.TeleportCluster] = &lazyAuthClient{}
	}
	return &kubeClusterGroup{
		connector:   connector,
		clusters:    clusters,
		concurrency: max(concurrency, 1),
		clients:     clients,
	}
}

// ForEach runs fn for every cluster in the group concurrently, bounded by the group's concurrency.
// fn receives the shared auth client of the cluster's Teleport cluster, built on first use.
// The client dials lazily, so callbacks that never use it cost no connection.
func (g *kubeClusterGroup) ForEach(ctx context.Context, fn func(ctx context.Context, authClient authclient.ClientI, cluster kubeconfig.LocalProxyCluster) error) error {
	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(g.concurrency)
	for _, cluster := range g.clusters {
		eg.Go(func() error {
			lazy := g.clients[cluster.TeleportCluster]
			lazy.once.Do(func() {
				lazy.client, lazy.err = g.connector.ConnectToCluster(egCtx, cluster.TeleportCluster)
			})
			if lazy.err != nil {
				return trace.Wrap(lazy.err)
			}
			return trace.Wrap(fn(egCtx, lazy.client, cluster))
		})
	}
	return trace.Wrap(eg.Wait())
}

// Close closes every auth client the group connected.
func (g *kubeClusterGroup) Close(ctx context.Context) {
	for _, lazy := range g.clients {
		if lazy.client == nil {
			continue
		}
		if err := lazy.client.Close(); err != nil {
			logger.WarnContext(ctx, "Failed to close auth client", "error", err)
		}
	}
}
