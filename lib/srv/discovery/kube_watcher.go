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

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/utils"
)

const kubeEventPrefix = "kube/"

func (s *Server) startKubeWatchers() error {
	if len(s.kubeFetchers) == 0 {
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
				clusters := make([]types.KubeCluster, 0, len(newResources))
				for _, r := range newResources {
					cluster, ok := r.(types.KubeCluster)
					if !ok {
						continue
					}

					clusters = append(clusters, cluster)
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
	// If the resource already exists, it means that the resource was created
	// by a previous discovery_service instance that didn't support the discovery
	// group feature or the discovery group was changed.
	// In this case, we need to update the resource with the
	// discovery group label to ensure the user doesn't have to manually delete
	// the resource.
	// TODO(tigrato): DELETE on 15.0.0
	if trace.IsAlreadyExists(err) {
		return trace.Wrap(s.onKubeUpdate(ctx, kubeCluster))
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

func (s *Server) onKubeUpdate(ctx context.Context, kubeCluster types.KubeCluster) error {
	s.Log.Debugf("Updating kube_cluster %s.", kubeCluster.GetName())
	return trace.Wrap(s.AccessPoint.UpdateKubernetesCluster(ctx, kubeCluster))
}

func (s *Server) onKubeDelete(ctx context.Context, kubeCluster types.KubeCluster) error {
	s.Log.Debugf("Deleting kube_cluster %s.", kubeCluster.GetName())
	return trace.Wrap(s.AccessPoint.DeleteKubernetesCluster(ctx, kubeCluster.GetName()))
}
