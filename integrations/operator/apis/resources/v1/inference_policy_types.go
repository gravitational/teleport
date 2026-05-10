// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package v1

import (
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources/teleportcr"
)

func init() {
	SchemeBuilder.Register(&TeleportInferencePolicy{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportInferencePolicy represents a Kubernetes custom resource for
// InferencePolicy.
type TeleportInferencePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   *TeleportInferencePolicySpec `json:"spec"`
	Status teleportcr.Status            `json:"status"`
}

// TeleportInferencePolicySpec defines the desired state of
// [TeleportInferencePolicy].
type TeleportInferencePolicySpec summarizerv1.InferencePolicySpec

//+kubebuilder:object:root = true

// TeleportInferencePolicyList contains a list of [TeleportInferencePolicy]
// objects.
type TeleportInferencePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportInferencePolicy `json:"items"`
}

// ToTeleport returns a Teleport representation of this Kubernetes resource.
func (p *TeleportInferencePolicy) ToTeleport() *summarizerv1.InferencePolicy {
	resource := &summarizerv1.InferencePolicy{
		Kind:    types.KindInferencePolicy,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        p.Name,
			Description: p.Annotations[teleportcr.DescriptionKey],
			Labels:      p.Labels,
		},
		Spec: (*summarizerv1.InferencePolicySpec)(p.Spec),
	}
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the Teleport resource controller to report conditions back to resource.
func (p *TeleportInferencePolicy) StatusConditions() *[]metav1.Condition {
	return &p.Status.Conditions
}

// UnmarshalJSON delegates unmarshaling of the TeleportInferencePolicySpec to
// protojson, which is necessary for Proto RFD153 resources to be unmarshaled
// correctly from the unstructured object.
func (spec *TeleportInferencePolicySpec) UnmarshalJSON(data []byte) error {
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(data, (*summarizerv1.InferencePolicySpec)(spec))
}

// MarshalJSON delegates marshaling of the TeleportInferencePolicySpec to
// protojson, which is necessary for Proto RFD153 resources to be marshaled
// correctly into an unstructured object.
func (spec *TeleportInferencePolicySpec) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal((*summarizerv1.InferencePolicySpec)(spec))
}

// DeepCopyInto deep-copies one TeleportInferencePolicySpec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportInferencePolicySpec) DeepCopyInto(out *TeleportInferencePolicySpec) {
	proto.Reset((*summarizerv1.InferencePolicySpec)(out))
	proto.Merge((*summarizerv1.InferencePolicySpec)(out), (*summarizerv1.InferencePolicySpec)(spec))
}
