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

package discovery

import (
	"context"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

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
		Fetchers:       s.kubeFetchers,
		Log:            s.Log.WithField("kind", types.KindKubernetesCluster),
		DiscoveryGroup: s.DiscoveryGroup,
		Interval:       s.PollInterval,
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
	// TODO(tigrato): DELETE on 14.0.0
	if trace.IsAlreadyExists(err) {
		return trace.Wrap(s.onKubeUpdate(ctx, rwl))
	}
	return trace.Wrap(err)
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
