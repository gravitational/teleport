/*
Copyright 2021 Gravitational, Inc.

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

package local

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// KubernetesService manages kubernetes resources in the backend.
type KubernetesService struct {
	backend.Backend
}

// NewKubernetesService creates a new KubernetesService.
func NewKubernetesService(backend backend.Backend) *KubernetesService {
	return &KubernetesService{Backend: backend}
}

// GetKubernetesClusters returns all kubernetes cluster resources.
func (s *KubernetesService) GetKubernetesClusters(ctx context.Context) ([]types.KubeCluster, error) {
	startKey := backend.Key(kubernetesPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	kubeClusters := make([]types.KubeCluster, len(result.Items))
	for i, item := range result.Items {
		cluster, err := services.UnmarshalKubeCluster(item.Value,
			services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		kubeClusters[i] = cluster
	}
	return kubeClusters, nil
}

// GetKubernetesCluster returns the specified kubernetes cluster resource.
func (s *KubernetesService) GetKubernetesCluster(ctx context.Context, name string) (types.KubeCluster, error) {
	item, err := s.Get(ctx, backend.Key(kubernetesPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("kubernetes cluster %q doesn't exist", name)
		}
		return nil, trace.Wrap(err)
	}
	cluster, err := services.UnmarshalKubeCluster(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires), services.WithRevision(item.Revision))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cluster, nil
}

// CreateKubernetesCluster creates a new kubernetes cluster resource.
func (s *KubernetesService) CreateKubernetesCluster(ctx context.Context, cluster types.KubeCluster) error {
	if err := cluster.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalKubeCluster(cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(kubernetesPrefix, cluster.GetName()),
		Value:   value,
		Expires: cluster.Expiry(),
		ID:      cluster.GetResourceID(),
	}
	_, err = s.Create(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpdateKubernetesCluster updates an existing kubernetes cluster resource.
func (s *KubernetesService) UpdateKubernetesCluster(ctx context.Context, cluster types.KubeCluster) error {
	if err := cluster.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalKubeCluster(cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(kubernetesPrefix, cluster.GetName()),
		Value:   value,
		Expires: cluster.Expiry(),
		ID:      cluster.GetResourceID(),
	}
	_, err = s.Update(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteKubernetesCluster removes the specified kubernetes cluster resource.
func (s *KubernetesService) DeleteKubernetesCluster(ctx context.Context, name string) error {
	err := s.Delete(ctx, backend.Key(kubernetesPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("kubernetes cluster %q doesn't exist", name)
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllKubernetesClusters removes all kubernetes cluster resources.
func (s *KubernetesService) DeleteAllKubernetesClusters(ctx context.Context) error {
	startKey := backend.Key(kubernetesPrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

const (
	kubernetesPrefix = "kubernetes"
)
