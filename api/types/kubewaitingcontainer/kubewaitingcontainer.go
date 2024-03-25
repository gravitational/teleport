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
	"slices"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	"github.com/gravitational/teleport/api/types"
)

const (
	// JSONPatchType is the JSON patch type supported by Kubernetes
	JSONPatchType string = "application/json-patch+json"
	// MergePatchType is the merge patch type supported by Kubernetes
	MergePatchType string = "application/merge-patch+json"
	// StrategicMergePatchType is the strategic merge patch type supported by Kubernetes
	StrategicMergePatchType string = "application/strategic-merge-patch+json"
	// ApplyPatchType is the apply patch type supported by Kubernetes
	ApplyPatchType string = "application/apply-patch+yaml"
)

var (
	// PatchTypes is a list of all supported patch types
	PatchTypes = []string{
		JSONPatchType,
		MergePatchType,
		StrategicMergePatchType,
		ApplyPatchType,
	}
)

// NewKubeWaitingContainer creates a new Kubernetes ephemeral
// container that are waiting to be created until moderated
// session conditions are met.
func NewKubeWaitingContainer(name string, spec *kubewaitingcontainerpb.KubernetesWaitingContainerSpec) (*kubewaitingcontainerpb.KubernetesWaitingContainer, error) {
	waitingCont := &kubewaitingcontainerpb.KubernetesWaitingContainer{
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
func ValidateKubeWaitingContainer(k *kubewaitingcontainerpb.KubernetesWaitingContainer) error {
	if k == nil {
		return trace.BadParameter("KubernetesWaitingContainer is nil")
	}
	if k.Metadata == nil {
		return trace.BadParameter("Metadata is nil")
	}
	if k.Spec == nil {
		return trace.BadParameter("Spec is nil")
	}

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
	if len(k.Spec.PatchType) == 0 {
		return trace.BadParameter("PatchType is unset")
	}
	if !slices.Contains(PatchTypes, k.Spec.PatchType) {
		return trace.BadParameter("PatchType is invalid: valid types are %v", PatchTypes)
	}
	if k.Metadata.Name == "" {
		return trace.BadParameter("Name is unset")
	}
	if k.Metadata.Name != k.Spec.ContainerName {
		return trace.BadParameter("Name must be ContainerName")
	}

	return nil
}
