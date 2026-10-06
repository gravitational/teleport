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
	"errors"
	"fmt"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
)

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

// clusterConnector provides auth clients for Teleport clusters.
// [client.ClusterClient] implements it.
type clusterConnector interface {
	ConnectToCluster(ctx context.Context, clusterName string) (authclient.ClientI, error)
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
// A failure does not cancel the siblings: the returned error aggregates every one, naming its cluster.
func (g *kubeClusterGroup) ForEach(ctx context.Context, fn func(ctx context.Context, cluster kubeconfig.LocalProxyCluster) error) error {
	var eg errgroup.Group
	eg.SetLimit(g.concurrency)

	// Indexed by cluster so the aggregate reads in cluster order.
	errs := make([]error, len(g.clusters))
	for i, cluster := range g.clusters {
		eg.Go(func() error {
			if ctx.Err() != nil {
				return nil
			}
			if err := fn(ctx, cluster); err != nil {
				errs[i] = fmt.Errorf("%s: %w", localProxyClusterKey(cluster), err)
			}
			return nil
		})
	}
	// No callback above returns an error, so Wait only waits for the group.
	_ = eg.Wait()

	// A cancellation outranks whatever was collected.
	if ctx.Err() != nil {
		return trace.Wrap(ctx.Err())
	}
	return trace.Wrap(g.report(errs))
}

// report turns the per-cluster failures, indexed by cluster with a nil for every cluster that succeeded, into one error.
// A lone failure is returned as it is, so the common case reads as one line.
// Several are listed one per line under a count, as the caller prints them:
//
//	ERROR: root/kube-1: kubernetes cluster "kube-1" not found
//
//	ERROR: 2 of 5 Kubernetes clusters failed:
//	root/kube-1: kubernetes cluster "kube-1" not found
//	root/kube-2: kubernetes cluster "kube-2" not found
func (g *kubeClusterGroup) report(errs []error) error {
	failures := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			failures = append(failures, err)
		}
	}
	switch len(failures) {
	case 0:
		return nil
	case 1:
		return failures[0]
	default:
		return fmt.Errorf("%d of %d Kubernetes clusters failed:\n%w",
			len(failures), len(g.clusters), errors.Join(failures...))
	}
}

// AuthClient returns the group's shared auth client for the Teleport cluster, connecting it on first use.
// The group owns the client: callers must not close it.
func (g *kubeClusterGroup) AuthClient(ctx context.Context, teleportCluster string) (authclient.ClientI, error) {
	lazy, ok := g.clients[teleportCluster]
	if !ok {
		return nil, trace.BadParameter("teleport cluster %q is not part of this group", teleportCluster)
	}
	lazy.once.Do(func() {
		lazy.client, lazy.err = g.connector.ConnectToCluster(ctx, teleportCluster)
	})
	if lazy.err != nil {
		return nil, trace.Wrap(lazy.err)
	}
	return lazy.client, nil
}

// Close closes every auth client the group connected.
// Call it only after every ForEach and AuthClient call returned.
func (g *kubeClusterGroup) Close(ctx context.Context) {
	for _, lazy := range g.clients {
		if lazy.client == nil {
			continue
		}
		if err := lazy.client.Close(); err != nil {
			logger.WarnContext(ctx, "Failed to close auth client", "error", err)
		}
		lazy.client = nil
	}
}
