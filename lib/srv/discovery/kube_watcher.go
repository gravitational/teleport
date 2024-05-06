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
	"github.com/gravitational/teleport/lib/utils"
)

const kubeEventPrefix = "kube/"

func (s *Server) startKubeWatchers() error {
	if len(s.getKubeNonIntegrationFetchers()) == 0 && s.dynamicMatcherWatcher == nil {
		return nil
	}

	var (
		kubeResources []types.KubeCluster
		mu            sync.Mutex
	)

	reconciler, err := services.NewReconciler(
		services.ReconcilerConfig[types.KubeCluster]{
			Matcher: func(_ types.KubeCluster) bool { return true },
			GetCurrentResources: func() map[string]types.KubeCluster {
				kcs, err := s.AccessPoint.GetKubernetesClusters(s.ctx)
				if err != nil {
					s.Log.WithError(err).Warn("Unable to get Kubernetes clusters from cache.")
					return nil
				}

				return utils.FromSlice(filterResources(kcs, types.OriginCloud, s.DiscoveryGroup), types.KubeCluster.GetName)
			},
			GetNewResources: func() map[string]types.KubeCluster {
				mu.Lock()
				defer mu.Unlock()
				return utils.FromSlice(kubeResources, types.KubeCluster.GetName)
			},
			Log:      s.Log.WithField("kind", types.KindKubernetesCluster),
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
		Log:            s.Log.WithField("kind", types.KindKubernetesCluster),
		DiscoveryGroup: s.DiscoveryGroup,
		Interval:       s.PollInterval,
		Origin:         types.OriginCloud,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go watcher.Start()

	go func() {
		for {
			select {
			case newResources := <-watcher.ResourcesC():
				clusters := make([]types.KubeCluster, 0, len(newResources))
				for _, r := range newResources {
					if cluster, ok := r.(types.DiscoveredEKSCluster); ok {
						clusters = append(clusters, cluster.GetKubeCluster())
						continue
					}
					if cluster, ok := r.(types.KubeCluster); ok {
						clusters = append(clusters, cluster)
						continue
					}
				}
				mu.Lock()
				kubeResources = clusters
				mu.Unlock()

				if err := reconciler.Reconcile(s.ctx); err != nil {
					s.Log.WithError(err).Warn("Unable to reconcile resources.")
				}

			case <-s.ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (s *Server) onKubeCreate(ctx context.Context, kubeCluster types.KubeCluster) error {
	s.Log.Debugf("Creating kube_cluster %s.", kubeCluster.GetName())
	err := s.AccessPoint.CreateKubernetesCluster(ctx, kubeCluster)
	// If the kube already exists but has an empty discovery group, update it.
	if trace.IsAlreadyExists(err) && s.updatesEmptyDiscoveryGroup(
		func() (types.ResourceWithLabels, error) {
			return s.AccessPoint.GetKubernetesCluster(ctx, kubeCluster.GetName())
		}) {
		return trace.Wrap(s.onKubeUpdate(ctx, kubeCluster, nil))
	}
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.emitUsageEvents(map[string]*usageeventsv1.ResourceCreateEvent{
		kubeEventPrefix + kubeCluster.GetName(): {
			ResourceType:   types.DiscoveredResourceKubernetes,
			ResourceOrigin: types.OriginCloud,
			CloudProvider:  kubeCluster.GetCloud(),
		},
	})
	if err != nil {
		s.Log.WithError(err).Debug("Error emitting usage event.")
	}
	return nil
}

func (s *Server) onKubeUpdate(ctx context.Context, kubeCluster, _ types.KubeCluster) error {
	s.Log.Debugf("Updating kube_cluster %s.", kubeCluster.GetName())
	return trace.Wrap(s.AccessPoint.UpdateKubernetesCluster(ctx, kubeCluster))
}

func (s *Server) onKubeDelete(ctx context.Context, kubeCluster types.KubeCluster) error {
	s.Log.Debugf("Deleting kube_cluster %s.", kubeCluster.GetName())
	return trace.Wrap(s.AccessPoint.DeleteKubernetesCluster(ctx, kubeCluster.GetName()))
}
