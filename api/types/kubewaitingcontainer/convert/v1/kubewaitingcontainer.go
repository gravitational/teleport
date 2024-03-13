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

package v1

import (
	"github.com/gravitational/trace"

	kubewaitingcontainerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	headerv1 "github.com/gravitational/teleport/api/types/header/convert/v1"
	"github.com/gravitational/teleport/api/types/kubewaitingcontainer"
)

// FromProto converts from the external representation of KubernetesWaitingContainer
// to the internal representation.
func FromProto(in *kubewaitingcontainerv1.KubernetesWaitingContainer) (*kubewaitingcontainer.KubeWaitingContainer, error) {
	if in == nil {
		return nil, trace.BadParameter("message is nil")
	}
	if in.Spec == nil {
		return nil, trace.BadParameter("spec is missing")
	}

	out, err := kubewaitingcontainer.NewKubeWaitingContainer(in.Metadata.Name, kubewaitingcontainer.KubeWaitingContainerSpec{
		Username:      in.Spec.Username,
		Cluster:       in.Spec.Cluster,
		Namespace:     in.Spec.Namespace,
		PodName:       in.Spec.PodName,
		ContainerName: in.Spec.ContainerName,
		Patch:         in.Spec.Patch,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// ToProto converts from the internal representation of KubernetesWaitingContainer
// to the external representation.
func ToProto(in *kubewaitingcontainer.KubeWaitingContainer) *kubewaitingcontainerv1.KubernetesWaitingContainer {
	return &kubewaitingcontainerv1.KubernetesWaitingContainer{
		Kind:     in.Kind,
		SubKind:  in.SubKind,
		Version:  in.Version,
		Metadata: headerv1.ToMetadataProto(in.Metadata),
		Spec: &kubewaitingcontainerv1.KubernetesWaitingContainerSpec{
			Username:      in.Spec.Username,
			Cluster:       in.Spec.Cluster,
			Namespace:     in.Spec.Namespace,
			PodName:       in.Spec.PodName,
			ContainerName: in.Spec.ContainerName,
			Patch:         in.Spec.Patch,
		},
	}
}
