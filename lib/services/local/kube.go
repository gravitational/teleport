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
	"iter"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	kubev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// KubernetesService manages kubernetes resources in the backend.
type KubernetesService struct {
	backend.Backend
	logger *slog.Logger
	svc    *generic.ScopeAwareService[types.KubeCluster]
}

type kubeCluster struct {
	types.KubeCluster
}

// NewKubernetesService creates a new KubernetesService.
func NewKubernetesService(b backend.Backend) (*KubernetesService, error) {
	svc, err := generic.NewScopeAwareService(&generic.ScopeAwareServiceConfig[types.KubeCluster]{
		Backend:               b,
		ResourceKind:          types.KindKubernetesCluster,
		UnscopedBackendPrefix: backend.NewKey(kubernetesPrefix),
		ScopedBackendPrefix:   backend.NewKey("scoped", kubernetesPrefix),
		MarshalFunc: func(kc types.KubeCluster, option ...services.MarshalOption) ([]byte, error) {
			return services.MarshalKubeCluster(kc, option...)
		},
		UnmarshalFunc: func(bytes []byte, option ...services.MarshalOption) (types.KubeCluster, error) {
			cluster, err := services.UnmarshalKubeCluster(bytes, option...)
			if err != nil {
				return kubeCluster{}, trace.Wrap(err)
			}
			return cluster, nil
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &KubernetesService{
		Backend: b,
		logger:  slog.With(teleport.ComponentKey, "KubernetesService"),
		svc:     svc,
	}, nil
}

// GetKubernetesClusters returns all kubernetes cluster resources.
func (s *KubernetesService) GetKubernetesClusters(ctx context.Context) ([]types.KubeCluster, error) {
	out, err := stream.Collect(s.RangeKubernetesClusters(ctx, "", ""))

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// ListKubernetesClusters returns a page of registered kubernetes clusters.
func (s *KubernetesService) ListKubernetesClusters(ctx context.Context, limit int, start string) ([]types.KubeCluster, string, error) {
	return s.svc.ListResources(ctx, limit, start)
}

// ListKubeClusters returns a page of registered kube clusters respecting scope filters.
func (s *KubernetesService) ListKubeClusters(ctx context.Context, req *kubev1.ListKubeClustersRequest) ([]types.KubeCluster, string, error) {
	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return nil, "", trace.Wrap(err)
	}
	filterFn := func(kc types.KubeCluster) bool {
		return scopes.MatchScope(scopeFilter, kc.GetScope())
	}

	return s.svc.ListResourcesWithFilter(ctx, int(req.GetPageSize()), req.GetPageToken(), filterFn)
}

// RangeKubernetesClusters returns kubernetes clusters within the range [start, end).
func (s *KubernetesService) RangeKubernetesClusters(ctx context.Context, start, end string) iter.Seq2[types.KubeCluster, error] {
	return s.svc.Resources(ctx, start, end)
}

// RangeKubeClusters returns kubernetes clusters within the range [start, end).
func (s *KubernetesService) RangeKubeClusters(ctx context.Context, req *kubev1.ListKubeClustersRequest, start, end string) iter.Seq2[types.KubeCluster, error] {
	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return stream.Fail[types.KubeCluster](trace.Wrap(err))
	}
	filterFn := func(kc types.KubeCluster) (types.KubeCluster, bool) {
		return kc, scopes.MatchScope(scopeFilter, kc.GetScope())
	}

	return stream.FilterMap(s.svc.Resources(ctx, start, end), filterFn)
}

// GetKubernetesCluster returns the specified kubernetes cluster resource.
func (s *KubernetesService) GetKubernetesCluster(ctx context.Context, name string) (types.KubeCluster, error) {
	return s.GetKubeCluster(ctx, kubev1.GetKubeClusterRequest_builder{Name: name}.Build())
}

// GetKubeCluster returns the specified kubernetes cluster resource.
func (s *KubernetesService) GetKubeCluster(ctx context.Context, req *kubev1.GetKubeClusterRequest) (types.KubeCluster, error) {
	sqn := scopes.QualifiedName{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	}
	if sqn.Scope != "" {
		if err := sqn.WeakValidate(); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return s.svc.GetResource(ctx, sqn)
}

// CreateKubernetesCluster creates a new kubernetes cluster resource.
func (s *KubernetesService) CreateKubernetesCluster(ctx context.Context, cluster types.KubeCluster) error {
	if err := validateKubeCluster(cluster); err != nil {
		return trace.Wrap(err)
	}

	_, err := s.svc.CreateResource(ctx, cluster)
	return trace.Wrap(err)
}

// UpdateKubernetesCluster updates an existing kubernetes cluster resource.
func (s *KubernetesService) UpdateKubernetesCluster(ctx context.Context, cluster types.KubeCluster) error {
	if err := validateKubeCluster(cluster); err != nil {
		return trace.Wrap(err)
	}
	_, err := s.svc.UpdateResource(ctx, cluster)
	return trace.Wrap(err)
}

// DeleteKubernetesCluster removes the specified kubernetes cluster resource.
func (s *KubernetesService) DeleteKubernetesCluster(ctx context.Context, name string) error {
	return s.svc.DeleteResource(ctx, scopes.QualifiedName{
		Name: name,
	})
}

// DeleteKubeCluster removes the specified kubernetes cluster resource.
func (s *KubernetesService) DeleteKubeCluster(ctx context.Context, req *kubev1.DeleteKubeClusterRequest) error {
	return s.svc.DeleteResource(ctx, scopes.QualifiedName{
		Scope: req.GetScope(),
		Name:  req.GetName(),
	})
}

// DeleteAllKubernetesClusters removes all kubernetes cluster resources.
func (s *KubernetesService) DeleteAllKubernetesClusters(ctx context.Context) error {
	return s.svc.DeleteAllResources(ctx)
}

// DeleteAllKubeClusters removes all kubernetes cluster resources within the given scope.
func (s *KubernetesService) DeleteAllKubeClusters(ctx context.Context, scope string) error {
	svc, err := s.svc.WithScopePrefix(scope)
	if err != nil {
		return trace.Wrap(err)
	}

	return svc.DeleteAllResources(ctx)
}

func validateKubeCluster(cluster types.KubeCluster) error {
	if err := services.CheckAndSetDefaults(cluster); err != nil {
		return trace.Wrap(err)
	}

	if cluster.GetScope() == "" {
		return nil
	}

	if err := scopes.StrongValidate(cluster.GetScope()); err != nil {
		return trace.Wrap(err)
	}

	if len(cluster.GetDynamicLabels()) > 0 {
		return trace.BadParameter("scoped kubernetes clusters do not support dynamic labels")
	}

	return nil
}

const (
	kubernetesPrefix = "kubernetes"
)
