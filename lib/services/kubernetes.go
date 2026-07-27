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

package services

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/utils"
)

// KubernetesClusterGetter defines interface for fetching kubernetes cluster resources.
type KubernetesClusterGetter interface {
	// GetKubernetesClusters returns all kubernetes cluster resources.
	GetKubernetesClusters(context.Context) ([]types.KubeCluster, error)
	// ListKubeClusters returns a page of registered kube clusters with the ability to apply
	// scope filters.
	ListKubeClusters(ctx context.Context, req *presencev1.ListKubeClustersRequest) ([]types.KubeCluster, string, error)
	// RangeKubeClusters returns a sequence of kube clusters filtered by the given
	// [*presencev1.ListKubeClustersRequest].
	RangeKubeClusters(ctx context.Context, req *presencev1.ListKubeClustersRequest) iter.Seq2[types.KubeCluster, error]
	// GetKubeCluster returns the specified kube cluster resource by scope and name.
	GetKubeCluster(ctx context.Context, req *presencev1.GetKubeClusterRequest) (types.KubeCluster, error)
}

// KubernetesServerGetter defines interface for fetching kubernetes server resources.
type KubernetesServerGetter interface {
	// GetKubernetesServers returns all kubernetes server resources.
	GetKubernetesServers(context.Context) ([]types.KubeServer, error)
}

// Kubernetes defines an interface for managing kubernetes clusters resources.
type Kubernetes interface {
	// KubernetesClusterGetter provides methods for fetching kubernetes resources.
	KubernetesClusterGetter
	// CreateKubernetesCluster creates a new kubernetes cluster resource.
	CreateKubernetesCluster(context.Context, types.KubeCluster) error
	// UpdateKubernetesCluster updates an existing kubernetes cluster resource.
	UpdateKubernetesCluster(context.Context, types.KubeCluster) error
	// DeleteAllKubernetesClusters removes all kubernetes resources.
	DeleteAllKubernetesClusters(context.Context) error
	// DeleteKubeCluster removes the specified kube cluster resource with
	// respect to its scope.
	DeleteKubeCluster(ctx context.Context, req *presencev1.DeleteKubeClusterRequest) error
}

// MarshalKubeServer marshals the KubeServer resource to JSON.
func MarshalKubeServer(kubeServer types.KubeServer, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch server := kubeServer.(type) {
	case *types.KubernetesServerV3:
		if err := server.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, server))
	default:
		return nil, trace.BadParameter("unsupported kube server resource %T", server)
	}
}

// UnmarshalKubeServer unmarshals KubeServer resource from JSON.
func UnmarshalKubeServer(data []byte, opts ...MarshalOption) (types.KubeServer, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing kube server data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V3:
		var s types.KubernetesServerV3
		if err := utils.FastUnmarshal(data, &s); err != nil {
			return nil, trace.BadParameter("%s", err)
		}
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.Revision != "" {
			s.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			s.SetExpiry(cfg.Expires)
		}
		return &s, nil
	}
	return nil, trace.BadParameter("unsupported kube server resource version %q", h.Version)
}

// MarshalKubeCluster marshals the KubeCluster resource to JSON.
func MarshalKubeCluster(kubeCluster types.KubeCluster, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if c, ok := kubeCluster.(types.DiscoveredEKSCluster); ok {
		kubeCluster = c.GetKubeCluster()
	}

	switch cluster := kubeCluster.(type) {
	case *types.KubernetesClusterV3:
		if err := cluster.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, cluster))
	default:
		return nil, trace.BadParameter("unsupported kube cluster resource %T", cluster)
	}
}

// UnmarshalKubeCluster unmarshals KubeCluster resource from JSON.
func UnmarshalKubeCluster(data []byte, opts ...MarshalOption) (types.KubeCluster, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing kube cluster data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V3:
		var s types.KubernetesClusterV3
		if err := utils.FastUnmarshal(data, &s); err != nil {
			return nil, trace.BadParameter("%s", err)
		}
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.Revision != "" {
			s.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			s.SetExpiry(cfg.Expires)
		}
		return &s, nil
	}
	return nil, trace.BadParameter("unsupported kube cluster resource version %q", h.Version)
}

// GetCursorForKubeCluster returns the backend key for a kube cluster with
// consideration for whether or not it is scoped.
func GetCursorForKubeCluster(cluster types.KubeCluster) string {
	return scopes.MakeResourceCursor(cluster.GetScope(), cluster.GetName())
}
