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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// KubernetesClusterGetter defines interface for fetching kubernetes cluster resources.
type KubernetesClusterGetter interface {
	// GetKubernetesClusters returns all kubernetes cluster resources.
	GetKubernetesClusters(context.Context) ([]types.KubeCluster, error)
	// GetKubernetesCluster returns the specified kubernetes cluster resource.
	GetKubernetesCluster(ctx context.Context, name string) (types.KubeCluster, error)
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
	// DeleteKubernetesCluster removes the specified kubernetes cluster resource.
	DeleteKubernetesCluster(ctx context.Context, name string) error
	// DeleteAllKubernetesClusters removes all kubernetes resources.
	DeleteAllKubernetesClusters(context.Context) error
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
