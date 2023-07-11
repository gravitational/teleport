/*
Copyright 2022 Gravitational, Inc.

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

package proxy

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/exp/maps"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// startReconciler starts reconciler that registers/unregisters proxied
// kubernetes clusters according to the up-to-date list of kube_cluster resources.
func (s *TLSServer) startReconciler(ctx context.Context) (err error) {
	if len(s.ResourceMatchers) == 0 || s.KubeServiceType != KubeService {
		s.log.Debug("Not initializing Kube Cluster resource watcher.")
		return nil
	}
	s.reconciler, err = services.NewReconciler(services.ReconcilerConfig{
		Matcher:             s.matcher,
		GetCurrentResources: s.getResources,
		GetNewResources:     s.monitoredKubeClusters.get,
		OnCreate:            s.onCreate,
		OnUpdate:            s.onUpdate,
		OnDelete:            s.onDelete,
		Log:                 s.log,
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
					s.log.WithError(err).Error("Failed to reconcile.")
				}
			case <-s.reconcileCh:
				if err := s.reconciler.Reconcile(ctx); err != nil {
					s.log.WithError(err).Error("Failed to reconcile.")
				} else if s.OnReconcile != nil {
					s.OnReconcile(s.fwd.kubeClusters())
				}
			case <-ctx.Done():
				s.log.Debug("Reconciler done.")
				return
			}
		}
	}()
	return nil
}

// startKubeClusterResourceWatcher starts watching changes to Kube Clusters resources and
// registers/unregisters the proxied Kube Cluster accordingly.
func (s *TLSServer) startKubeClusterResourceWatcher(ctx context.Context) (*services.KubeClusterWatcher, error) {
	if len(s.ResourceMatchers) == 0 || s.KubeServiceType != KubeService {
		s.log.Debug("Not initializing Kube Cluster resource watcher.")
		return nil, nil
	}
	s.log.Debug("Initializing Kube Cluster resource watcher.")
	watcher, err := services.NewKubeClusterWatcher(ctx, services.KubeClusterWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: s.Component,
			Log:       s.log,
			Client:    s.AccessPoint,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go func() {
		defer watcher.Close()
		for {
			select {
			case clusters := <-watcher.KubeClustersC:
				s.monitoredKubeClusters.setResources(clusters)
				select {
				case s.reconcileCh <- struct{}{}:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				s.log.Debug("Kube Cluster resource watcher done.")
				return
			}
		}
	}()
	return watcher, nil
}

func (s *TLSServer) getResources() (resources types.ResourcesWithLabelsMap) {
	return s.fwd.kubeClusters().AsResources().ToMap()
}

func (s *TLSServer) onCreate(ctx context.Context, resource types.ResourceWithLabels) error {
	cluster, ok := resource.(types.KubeCluster)
	if !ok {
		return trace.BadParameter("expected types.KubeCluster, got %T", resource)
	}
	return s.registerKubeCluster(ctx, cluster)
}

func (s *TLSServer) onUpdate(ctx context.Context, resource types.ResourceWithLabels) error {
	cluster, ok := resource.(types.KubeCluster)
	if !ok {
		return trace.BadParameter("expected types.KubeCluster, got %T", resource)
	}
	return s.updateKubeCluster(ctx, cluster)
}

func (s *TLSServer) onDelete(ctx context.Context, resource types.ResourceWithLabels) error {
	return s.unregisterKubeCluster(ctx, resource.GetName())
}

func (s *TLSServer) matcher(resource types.ResourceWithLabels) bool {
	cluster, ok := resource.(types.KubeCluster)
	if !ok {
		return false
	}
	return services.MatchResourceLabels(s.ResourceMatchers, cluster)
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

func (m *monitoredKubeClusters) get() types.ResourcesWithLabelsMap {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append(m.static, m.resources...).AsResources().ToMap()
}

func (s *TLSServer) registerKubeCluster(ctx context.Context, cluster types.KubeCluster) error {
	clusterDetails, err := newClusterDetails(
		ctx,
		clusterDetailsConfig{
			cloudClients:     s.CloudClients,
			cluster:          cluster,
			log:              s.log,
			checker:          s.CheckImpersonationPermissions,
			resourceMatchers: s.ResourceMatchers,
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	s.fwd.upsertKubeDetails(cluster.GetName(), clusterDetails)
	return trace.Wrap(s.startHeartbeat(ctx, cluster.GetName()))
}

func (s *TLSServer) updateKubeCluster(ctx context.Context, cluster types.KubeCluster) error {
	clusterDetails, err := newClusterDetails(
		ctx,
		clusterDetailsConfig{
			cloudClients:     s.CloudClients,
			cluster:          cluster,
			log:              s.log,
			checker:          s.CheckImpersonationPermissions,
			resourceMatchers: s.ResourceMatchers,
		},
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

	// A child process can be forked to upgrade the Teleport binary. The child
	// will take over the heartbeats so do NOT delete them in that case.
	// When unregistering a dynamic cluster, the context is empty and the
	// decision will be to delete the kubernetes server.
	if services.ShouldDeleteServerHeartbeatsOnShutdown(ctx) {
		errs = append(errs, s.deleteKubernetesServer(ctx, name))
	}

	// close active sessions before returning.
	s.fwd.mu.Lock()
	sessions := maps.Values(s.fwd.sessions)
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
