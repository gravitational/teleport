/*
Copyright 2024 Gravitational, Inc.

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

package kubewaitingcontainer

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/header/convert/legacy"
)

// KubeWaitingContainer is a Kubernetes ephemeral
// container that are waiting to be created until moderated
// session conditions are met.
type KubeWaitingContainer struct {
	header.ResourceHeader

	// Spec is the resource specification.
	Spec KubeWaitingContainerSpec `json:"spec" yaml:"spec"`
}

// KubeWaitingContainerSpec is the specification for KubeWaitingContainers.
type KubeWaitingContainerSpec struct {
	// Username is the Teleport user that attempted to create the container
	Username string `json:"username" yaml:"username"`
	// Cluster is the Kubernetes cluster of the container
	Cluster string `json:"cluster" yaml:"cluster"`
	// Namespace is the Kubernetes namespace of the container
	Namespace string `json:"namespace" yaml:"namespace"`
	// PodName is the parent pod of the container
	PodName string `json:"pod_name" yaml:"pod_name"`
	// ContainerName is the name of the container
	ContainerName string `json:"container_name" yaml:"container_name"`
	// Patch is the patch that should be applied to the parent pod
	// to create this ephemeral container
	Patch []byte `json:"patch" yaml:"patch"`
}

// NewKubeWaitingContainer creates a new Kubernetes ephemeral
// container that are waiting to be created until moderated
// session conditions are met.
func NewKubeWaitingContainer(name string, spec KubeWaitingContainerSpec) (*KubeWaitingContainer, error) {
	waitingCont := &KubeWaitingContainer{
		ResourceHeader: header.ResourceHeader{
			Kind:    types.KindKubeWaitingContainer,
			Version: types.V1,
			Metadata: header.Metadata{
				Name:    name,
				Expires: time.Now().Add(time.Hour),
			},
		},
		Spec: spec,
	}
	if err := ValidateKubeWaitingContainer(waitingCont); err != nil {
		return nil, err
	}

	return waitingCont, nil
}

// ValidateKubeWaitingContainer checks that required parameters are set
// for the specified KubeWaitingContainer
func ValidateKubeWaitingContainer(k *KubeWaitingContainer) error {
	if k.Spec.Username == "" {
		return trace.BadParameter("Username is unset")
	}
	if k.Spec.Cluster == "" {
		return trace.BadParameter("KubeCluster is unset")
	}
	if k.Spec.Namespace == "" {
		return trace.BadParameter("KubeNamespace is unset")
	}
	if k.Spec.PodName == "" {
		return trace.BadParameter("PodName is unset")
	}
	if k.Spec.ContainerName == "" {
		return trace.BadParameter("ContainerName is unset")
	}
	if len(k.Spec.Patch) == 0 {
		return trace.BadParameter("Patch is unset")
	}

	if k.Metadata.Name == "" {
		return trace.BadParameter("Name is unset")
	}
	if k.Metadata.Name != k.Spec.ContainerName {
		return trace.BadParameter("Name must be ContainerName")
	}

	return nil
}

// GetMetadata returns metadata. This is specifically for conforming to the Resource interface,
// and should be removed when possible.
func (k *KubeWaitingContainer) GetMetadata() types.Metadata {
	return legacy.FromHeaderMetadata(k.Metadata)
}

func (k *KubeWaitingContainer) GetParts() []string {
	return []string{
		k.Spec.Username,
		k.Spec.Cluster,
		k.Spec.Namespace,
		k.Spec.PodName,
	}
}
