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
	if err := kubeServer.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch server := kubeServer.(type) {
	case *types.KubernetesServerV3:
		if !cfg.PreserveResourceID {
			copy := *server
			copy.SetResourceID(0)
			copy.SetRevision("")
			server = &copy
		}
		return utils.FastMarshal(server)
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
			return nil, trace.BadParameter(err.Error())
		}
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			s.SetResourceID(cfg.ID)
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
	if err := kubeCluster.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch cluster := kubeCluster.(type) {
	case *types.KubernetesClusterV3:
		if !cfg.PreserveResourceID {
			copy := *cluster
			copy.SetResourceID(0)
			copy.SetRevision("")
			cluster = &copy
		}
		return utils.FastMarshal(cluster)
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
			return nil, trace.BadParameter(err.Error())
		}
		if err := s.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			s.SetResourceID(cfg.ID)
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
