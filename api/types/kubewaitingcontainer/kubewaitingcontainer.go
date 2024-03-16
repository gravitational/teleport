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
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	kubewaitingcontainerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	"github.com/gravitational/teleport/api/types"
)

// NewKubeWaitingContainer creates a new Kubernetes ephemeral
// container that are waiting to be created until moderated
// session conditions are met.
func NewKubeWaitingContainer(name string, spec *kubewaitingcontainerv1.KubernetesWaitingContainerSpec) (*kubewaitingcontainerv1.KubernetesWaitingContainer, error) {
	waitingCont := &kubewaitingcontainerv1.KubernetesWaitingContainer{
		Kind:    types.KindKubeWaitingContainer,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    name,
			Expires: timestamppb.New(time.Now().Add(time.Hour)),
		},
		Spec: spec,
	}
	if err := ValidateKubeWaitingContainer(waitingCont); err != nil {
		return nil, trace.Wrap(err)
	}

	return waitingCont, nil
}

// ValidateKubeWaitingContainer checks that required parameters are set
// for the specified KubeWaitingContainer
func ValidateKubeWaitingContainer(k *kubewaitingcontainerv1.KubernetesWaitingContainer) error {
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
