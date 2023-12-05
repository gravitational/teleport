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
)

const kubeEventPrefix = "kube/"

func (s *Server) startKubeWatchers() error {
	if len(s.kubeFetchers) == 0 {
		return nil
	}
	var (
		kubeResources types.ResourcesWithLabels
		mu            sync.Mutex
	)

	reconciler, err := services.NewReconciler(
		services.ReconcilerConfig{
			Matcher: func(_ types.ResourceWithLabels) bool { return true },
			GetCurrentResources: func() types.ResourcesWithLabelsMap {
				kcs, err := s.AccessPoint.GetKubernetesClusters(s.ctx)
				if err != nil {
					s.Log.WithError(err).Warn("Unable to get Kubernetes clusters from cache.")
					return nil
				}

				return types.KubeClusters(filterResources(kcs, types.OriginCloud, s.DiscoveryGroup)).AsResources().ToMap()
			},
			GetNewResources: func() types.ResourcesWithLabelsMap {
				mu.Lock()
				defer mu.Unlock()
				return kubeResources.ToMap()
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
		FetchersFn:     common.StaticFetchers(s.kubeFetchers),
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
				mu.Lock()
				kubeResources = newResources
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

func (s *Server) onKubeCreate(ctx context.Context, rwl types.ResourceWithLabels) error {
	kubeCluster, ok := rwl.(types.KubeCluster)
	if !ok {
		return trace.BadParameter("invalid type received; expected types.KubeCluster, received %T", kubeCluster)
	}
	s.Log.Debugf("Creating kube_cluster %s.", kubeCluster.GetName())
	err := s.AccessPoint.CreateKubernetesCluster(ctx, kubeCluster)
	// If the resource already exists, it means that the resource was created
	// by a previous discovery_service instance that didn't support the discovery
	// group feature or the discovery group was changed.
	// In this case, we need to update the resource with the
	// discovery group label to ensure the user doesn't have to manually delete
	// the resource.
	// TODO(tigrato): DELETE on 15.0.0
	if trace.IsAlreadyExists(err) {
		return trace.Wrap(s.onKubeUpdate(ctx, rwl))
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

func (s *Server) onKubeUpdate(ctx context.Context, rwl types.ResourceWithLabels) error {
	kubeCluster, ok := rwl.(types.KubeCluster)
	if !ok {
		return trace.BadParameter("invalid type received; expected types.KubeCluster, received %T", kubeCluster)
	}
	s.Log.Debugf("Updating kube_cluster %s.", kubeCluster.GetName())
	return trace.Wrap(s.AccessPoint.UpdateKubernetesCluster(ctx, kubeCluster))
}

func (s *Server) onKubeDelete(ctx context.Context, rwl types.ResourceWithLabels) error {
	kubeCluster, ok := rwl.(types.KubeCluster)
	if !ok {
		return trace.BadParameter("invalid type received; expected types.KubeCluster, received %T", kubeCluster)
	}
	s.Log.Debugf("Deleting kube_cluster %s.", kubeCluster.GetName())
	return trace.Wrap(s.AccessPoint.DeleteKubernetesCluster(ctx, kubeCluster.GetName()))
}
