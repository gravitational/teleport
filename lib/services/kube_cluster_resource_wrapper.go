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

package services

import (
	"github.com/gravitational/teleport/api/types"
)

// KubeClusterResourceWrapper stubs out a minimal implementation of
// services.AccessCheckable for *types.KubernetesCluster.
// This allows us to use common RBAC checks for Kubernetes Clusters
// and other resources.
type KubeClusterResourceWrapper struct {
	server  types.Server
	cluster *types.KubernetesCluster
}

func NewKubeClusterWrapperForRBAC(s types.Server, kube *types.KubernetesCluster) KubeClusterResourceWrapper {
	return KubeClusterResourceWrapper{
		server:  s,
		cluster: kube,
	}
}

func (k KubeClusterResourceWrapper) GetKind() string {
	return "kube_cluster"
}

func (k KubeClusterResourceWrapper) GetAllLabels() map[string]string {
	return types.CombineLabels(k.cluster.StaticLabels, k.cluster.DynamicLabels)
}

func (k KubeClusterResourceWrapper) GetName() string {
	return k.cluster.Name
}

func (k KubeClusterResourceWrapper) GetMetadata() types.Metadata {
	return types.Metadata{
		Name:      k.GetName(),
		Namespace: types.ProcessNamespace(k.server.GetNamespace()),
	}
}
