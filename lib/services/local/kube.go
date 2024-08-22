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
	startKey := backend.ExactKey(kubernetesPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	kubeClusters := make([]types.KubeCluster, len(result.Items))
	for i, item := range result.Items {
		cluster, err := services.UnmarshalKubeCluster(item.Value,
			services.WithExpires(item.Expires), services.WithRevision(item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		kubeClusters[i] = cluster
	}
	return kubeClusters, nil
}

// GetKubernetesCluster returns the specified kubernetes cluster resource.
func (s *KubernetesService) GetKubernetesCluster(ctx context.Context, name string) (types.KubeCluster, error) {
	item, err := s.Get(ctx, backend.NewKey(kubernetesPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("kubernetes cluster %q doesn't exist", name)
		}
		return nil, trace.Wrap(err)
	}
	cluster, err := services.UnmarshalKubeCluster(item.Value,
		services.WithExpires(item.Expires), services.WithRevision(item.Revision))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cluster, nil
}

// CreateKubernetesCluster creates a new kubernetes cluster resource.
func (s *KubernetesService) CreateKubernetesCluster(ctx context.Context, cluster types.KubeCluster) error {
	if err := services.CheckAndSetDefaults(cluster); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalKubeCluster(cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.NewKey(kubernetesPrefix, cluster.GetName()),
		Value:   value,
		Expires: cluster.Expiry(),
	}
	_, err = s.Create(ctx, item)
	if trace.IsAlreadyExists(err) {
		return trace.AlreadyExists("kubernetes cluster %q already exists", cluster.GetName())
	}

	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpdateKubernetesCluster updates an existing kubernetes cluster resource.
func (s *KubernetesService) UpdateKubernetesCluster(ctx context.Context, cluster types.KubeCluster) error {
	if err := services.CheckAndSetDefaults(cluster); err != nil {
		return trace.Wrap(err)
	}
	rev := cluster.GetRevision()
	value, err := services.MarshalKubeCluster(cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(kubernetesPrefix, cluster.GetName()),
		Value:    value,
		Expires:  cluster.Expiry(),
		Revision: rev,
	}
	_, err = s.Update(ctx, item)
	if trace.IsNotFound(err) {
		return trace.NotFound("kubernetes cluster %q doesn't exist", cluster.GetName())
	}
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteKubernetesCluster removes the specified kubernetes cluster resource.
func (s *KubernetesService) DeleteKubernetesCluster(ctx context.Context, name string) error {
	err := s.Delete(ctx, backend.NewKey(kubernetesPrefix, name))
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
	startKey := backend.ExactKey(kubernetesPrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

const (
	kubernetesPrefix = "kubernetes"
)
