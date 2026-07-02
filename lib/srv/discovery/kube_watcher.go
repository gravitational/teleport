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

package discovery

import (
	"context"
	"sync"

	"github.com/gravitational/trace"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/srv/discovery/fetchers"
	"github.com/gravitational/teleport/lib/utils"
)

const kubeEventPrefix = "kube/"

func (s *Server) startKubeWatchers() error {
	if len(s.getKubeNonIntegrationFetchers()) == 0 && s.DiscoveryGroup == "" {
		return nil
	}

	var (
		newResources []types.KubeCluster
		// currentResources is the existing enrolled clusters. The watcher reads
		// the set early so the eks access manager can maintain existing access
		// metadata.
		currentResources map[string]types.KubeCluster
		mu               sync.Mutex
	)

	reconciler, err := services.NewReconciler(
		services.ReconcilerConfig[types.KubeCluster]{
			Matcher: func(_ types.KubeCluster) bool { return true },
			GetCurrentResources: func() map[string]types.KubeCluster {
				mu.Lock()
				defer mu.Unlock()
				return currentResources
			},
			GetNewResources: func() map[string]types.KubeCluster {
				mu.Lock()
				defer mu.Unlock()
				return utils.FromSlice(newResources, types.KubeCluster.GetName)
			},
			CompareResources: func(newCluster, oldCluster types.KubeCluster) int {
				if !newCluster.IsEqual(oldCluster) {
					return services.Different
				}
				if newCluster.GetStatus().IsEqual(oldCluster.GetStatus()) {
					return services.Equal
				}
				return services.Different
			},
			Logger:   s.Log.With("kind", types.KindKubernetesCluster),
			OnCreate: s.onKubeCreate,
			OnUpdate: s.onKubeUpdate,
			OnDelete: s.onKubeDelete,
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	watcher, err := common.NewWatcher(s.ctx, common.WatcherConfig{
		FetchersFn: func() []common.Fetcher {
			kubeNonIntegrationFetchers := s.getKubeNonIntegrationFetchers()
			s.submitFetchersEvent(kubeNonIntegrationFetchers)
			return kubeNonIntegrationFetchers
		},
		Logger:         s.Log.With("kind", types.KindKubernetesCluster),
		DiscoveryGroup: s.DiscoveryGroup,
		Interval:       s.PollInterval,
		Origin:         types.OriginCloud,
		Clock:          s.clock,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go watcher.Start()

	go func() {
		for {
			select {
			case fetchedResources := <-watcher.ResourcesC():
				// Skip this cycle if the existing enrolled kube clusters can't be fetched for
				// accurate reconciliation.
				current, err := s.currentKubeClusters(s.ctx)
				if err != nil {
					s.Log.WarnContext(s.ctx, "Unable to get Kubernetes clusters from cache, skipping reconcile", "error", err)
					continue
				}

				clusters := make([]types.KubeCluster, 0, len(fetchedResources))
				eksClusters := make([]*fetchers.DiscoveredEKSCluster, 0, len(fetchedResources))
				for _, r := range fetchedResources {
					if eksCluster, ok := r.(*fetchers.DiscoveredEKSCluster); ok {
						eksClusters = append(eksClusters, eksCluster)
						continue
					}
					if cluster, ok := r.(types.KubeCluster); ok {
						clusters = append(clusters, cluster)
					}
				}

				// Provision access for the clusters. When ProvisionAll returns
				// nil for a cluster, keep its stored Status so a no-op cycle
				// does not erase access metadata.
				statuses := s.eksAccessManager.ProvisionAll(s.ctx, eksClusters)
				for i, eksCluster := range eksClusters {
					status := statuses[i]
					if status == nil {
						if existing, ok := current[eksCluster.GetName()]; ok {
							status = existing.GetStatus()
						}
					}

					eksCluster.SetStatus(status)
					clusters = append(clusters, eksCluster.GetKubeCluster())
				}
				mu.Lock()
				newResources = clusters
				currentResources = current
				mu.Unlock()

				if err := reconciler.Reconcile(s.ctx); err != nil {
					s.Log.WarnContext(s.ctx, "Unable to reconcile resources", "error", err)
				}

				if s.onKubernetesClusterReconcile != nil {
					s.onKubernetesClusterReconcile()
				}

			case <-s.ctx.Done():
				return
			}
		}
	}()
	return nil
}

// currentKubeClusters reads this discovery group's cloud kube clusters from the
// cache.
func (s *Server) currentKubeClusters(ctx context.Context) (map[string]types.KubeCluster, error) {
	kcs, err := s.AccessPoint.GetKubernetesClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return utils.FromSlice(filterResources(kcs, types.OriginCloud, s.DiscoveryGroup), types.KubeCluster.GetName), nil
}

func (s *Server) onKubeCreate(ctx context.Context, kubeCluster types.KubeCluster) error {
	s.Log.DebugContext(ctx, "Creating kube_cluster", "kube_cluster_name", kubeCluster.GetName())
	err := s.AccessPoint.CreateKubernetesCluster(ctx, kubeCluster)
	// If the kube already exists but has an empty discovery group, update it.
	if err != nil {
		err := s.resolveCreateErr(err, types.OriginCloud, func() (types.ResourceWithLabels, error) {
			return s.AccessPoint.GetKubernetesCluster(ctx, kubeCluster.GetName())
		})
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(s.onKubeUpdate(ctx, kubeCluster, nil))
	}
	err = s.emitUsageEvent(kubeEventPrefix+kubeCluster.GetName(), &usageeventsv1.ResourceCreateEvent{
		ResourceType:        types.DiscoveredResourceKubernetes,
		ResourceOrigin:      types.OriginCloud,
		CloudProvider:       kubeCluster.GetCloud(),
		DiscoveryConfigName: kubeCluster.GetStaticLabels()[types.TeleportInternalDiscoveryConfigName],
	})
	if err != nil {
		s.Log.DebugContext(ctx, "Error emitting usage event", "error", err)
	}
	return nil
}

func (s *Server) onKubeUpdate(ctx context.Context, kubeCluster, _ types.KubeCluster) error {
	s.Log.DebugContext(ctx, "Updating kube_cluster", "kube_cluster_name", kubeCluster.GetName())
	return trace.Wrap(s.AccessPoint.UpdateKubernetesCluster(ctx, kubeCluster))
}

func (s *Server) onKubeDelete(ctx context.Context, kubeCluster types.KubeCluster) error {
	s.Log.DebugContext(ctx, "Deleting kube_cluster", "kube_cluster_name", kubeCluster.GetName())
	if err := s.eksAccessManager.Cleanup(ctx, kubeCluster); err != nil {
		s.Log.WarnContext(ctx, "Failed to delete dangling resources for kube_cluster",
			"kube_cluster_name", kubeCluster.GetName(),
			"error", err)
	}
	return trace.Wrap(s.AccessPoint.DeleteKubernetesCluster(ctx, kubeCluster.GetName()))
}
