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

package proxy

import (
	"context"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/utils"
)

// startReconciler starts reconciler that registers/unregisters proxied
// kubernetes clusters according to the up-to-date list of kube_cluster resources.
func (s *TLSServer) startReconciler(ctx context.Context) (err error) {
	if len(s.ResourceMatchers) == 0 || s.KubeServiceType != KubeService {
		s.log.DebugContext(ctx, "Not initializing Kube Cluster resource watcher")
		return nil
	}
	s.reconciler, err = services.NewReconciler(services.ReconcilerConfig[types.KubeCluster]{
		Matcher:             s.matcher,
		GetCurrentResources: s.getResources,
		GetNewResources:     s.monitoredKubeClusters.get,
		OnCreate:            s.onCreate,
		OnUpdate:            s.onUpdate,
		OnDelete:            s.onDelete,
		Logger:              s.log.With("kind", types.KindKubernetesCluster),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		// reconcileTicker is used to force reconciliation when the watcher was
		// previously informed that a `kube_cluster` resource exists/changed but the
		// creation/update operation failed - e.g. login to AKS/EKS clusters can
		// fail due to missing permissions.
		// Once this happens, the state of the resource watcher won't change until
		// a new update operation is triggered (which can take a lot of time).
		// This results in the service not being able to enroll the failing cluster,
		// even if the original issue was already fixed because we won't run reconciliation again.
		// We force the reconciliation to make sure we don't drift from watcher state if
		// the issue was fixed.
		reconcileTicker := time.NewTicker(2 * time.Minute)
		defer reconcileTicker.Stop()
		for {
			select {
			case <-reconcileTicker.C:
				if err := s.reconciler.Reconcile(ctx); err != nil {
					s.log.ErrorContext(ctx, "Failed to reconcile", "error", err)
				}
			case <-s.reconcileCh:
				if err := s.reconciler.Reconcile(ctx); err != nil {
					s.log.ErrorContext(ctx, "Failed to reconcile", "error", err)
				} else if s.OnReconcile != nil {
					s.OnReconcile(s.fwd.kubeClusters())
				}
			case <-ctx.Done():
				s.log.DebugContext(ctx, "Reconciler done")
				return
			}
		}
	}()
	return nil
}

// startKubeClusterResourceWatcher starts watching changes to Kube Clusters resources and
// registers/unregisters the proxied Kube Cluster accordingly.
func (s *TLSServer) startKubeClusterResourceWatcher(ctx context.Context) (*services.GenericWatcher[types.KubeCluster, readonly.KubeCluster], error) {
	if len(s.ResourceMatchers) == 0 || s.KubeServiceType != KubeService {
		s.log.DebugContext(ctx, "Not initializing Kube Cluster resource watcher")
		return nil, nil
	}
	s.log.DebugContext(ctx, "Initializing Kube Cluster resource watcher")
	watcher, err := services.NewKubeClusterWatcher(ctx, services.KubeClusterWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: s.Component,
			Logger:    s.log,
			Client:    s.AccessPoint,
		},
		KubernetesClusterGetter: s.AccessPoint,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go func() {
		defer watcher.Close()
		for {
			select {
			case clusters := <-watcher.ResourcesC:
				s.monitoredKubeClusters.setResources(clusters)
				select {
				case s.reconcileCh <- struct{}{}:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				s.log.DebugContext(ctx, "Kube Cluster resource watcher done")
				return
			}
		}
	}()
	return watcher, nil
}

func (s *TLSServer) getResources() map[string]types.KubeCluster {
	return utils.FromSlice(s.fwd.kubeClusters(), types.KubeCluster.GetName)
}

func (s *TLSServer) onCreate(ctx context.Context, cluster types.KubeCluster) error {
	return s.registerKubeCluster(ctx, cluster)
}

func (s *TLSServer) onUpdate(ctx context.Context, cluster, _ types.KubeCluster) error {
	return s.updateKubeCluster(ctx, cluster)
}

func (s *TLSServer) onDelete(ctx context.Context, cluster types.KubeCluster) error {
	return s.unregisterKubeCluster(ctx, cluster.GetName())
}

func (s *TLSServer) matcher(cluster types.KubeCluster) bool {
	return services.MatchResourceLabels(s.ResourceMatchers, cluster.GetAllLabels())
}

// monitoredKubeClusters is a collection of clusters from different sources
// like configuration file and dynamic resources.
//
// It's updated by respective watchers and is used for reconciling with the
// currently proxied clusters.
type monitoredKubeClusters struct {
	// static are clusters from the agent's YAML configuration.
	static types.KubeClusters
	// resources are clusters created via CLI or API.
	resources types.KubeClusters
	// mu protects access to the fields.
	mu sync.Mutex
}

func (m *monitoredKubeClusters) setResources(clusters types.KubeClusters) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resources = clusters
}

func (m *monitoredKubeClusters) get() map[string]types.KubeCluster {
	m.mu.Lock()
	defer m.mu.Unlock()
	return utils.FromSlice(append(m.static, m.resources...), types.KubeCluster.GetName)
}

func (s *TLSServer) buildClusterDetailsConfigForCluster(cluster types.KubeCluster) clusterDetailsConfig {
	return clusterDetailsConfig{
		cloudClients:     s.CloudClients,
		awsCloudClients:  s.awsClients,
		cluster:          cluster,
		log:              s.log,
		checker:          s.CheckImpersonationPermissions,
		resourceMatchers: s.ResourceMatchers,
		clock:            s.Clock,
		component:        s.KubeServiceType,
	}
}

func (s *TLSServer) registerKubeCluster(ctx context.Context, cluster types.KubeCluster) error {
	clusterDetails, err := newClusterDetails(
		ctx,
		s.buildClusterDetailsConfigForCluster(cluster),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	s.fwd.upsertKubeDetails(cluster.GetName(), clusterDetails)
	return trace.Wrap(s.startHeartbeat(cluster.GetName()))
}

func (s *TLSServer) updateKubeCluster(ctx context.Context, cluster types.KubeCluster) error {
	clusterDetails, err := newClusterDetails(
		ctx,
		s.buildClusterDetailsConfigForCluster(cluster),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	s.fwd.upsertKubeDetails(cluster.GetName(), clusterDetails)
	return nil
}

// unregisterKubeCluster unregisters the proxied Kube Cluster from the agent.
// This function is called when the dynamic cluster is deleted/no longer match
// the agent's resource matcher or when the agent is shutting down.
func (s *TLSServer) unregisterKubeCluster(ctx context.Context, name string) error {
	var errs []error

	errs = append(errs, s.stopHeartbeat(name))
	s.fwd.removeKubeDetails(name)

	shouldDeleteCluster := services.ShouldDeleteServerHeartbeatsOnShutdown(ctx)
	sender, ok := s.TLSServerConfig.InventoryHandle.GetSender()
	if ok {
		// Manual deletion per cluster is only required if the auth server
		// doesn't support actively cleaning up database resources when the
		// inventory control stream is terminated during shutdown.
		if capabilities := sender.Hello().Capabilities; capabilities != nil {
			shouldDeleteCluster = shouldDeleteCluster && !capabilities.KubernetesCleanup
		}
	}

	// A child process can be forked to upgrade the Teleport binary. The child
	// will take over the heartbeats so do NOT delete them in that case.
	// When unregistering a dynamic cluster, the context is empty and the
	// decision will be to delete the kubernetes server.
	if shouldDeleteCluster {
		errs = append(errs, s.deleteKubernetesServer(ctx, name))
	}

	// close active sessions before returning.
	s.fwd.mu.Lock()
	// collect all sessions to avoid holding the lock while closing them
	sessions := slices.Collect(maps.Values(s.fwd.sessions))
	s.fwd.mu.Unlock()
	// close active sessions
	for _, sess := range sessions {
		if sess.ctx.kubeClusterName == name {
			// TODO(tigrato): check if we should send errors to each client
			errs = append(errs, sess.Close())
		}
	}

	return trace.NewAggregate(errs...)
}

// deleteKubernetesServer deletes kubernetes server for the specified cluster.
func (s *TLSServer) deleteKubernetesServer(ctx context.Context, name string) error {
	err := s.AuthClient.DeleteKubernetesServer(ctx, s.HostID, name)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}
