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
	SchemeBuilder.Register(&TeleportInferenceModel{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportInferenceModel represents a Kubernetes custom resource for
// InferenceModel.
type TeleportInferenceModel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   *TeleportInferenceModelSpec `json:"spec"`
	Status teleportcr.Status           `json:"status"`
}

// TeleportInferenceModelSpec defines the desired state of
// [TeleportInferenceModel].
type TeleportInferenceModelSpec summarizerv1.InferenceModelSpec

//+kubebuilder:object:root = true

// TeleportInferenceModelList contains a list of [TeleportInferenceModel]
// objects.
type TeleportInferenceModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportInferenceModel `json:"items"`
}

// ToTeleport returns a Teleport representation of this Kubernetes resource.
func (m *TeleportInferenceModel) ToTeleport() *summarizerv1.InferenceModel {
	resource := &summarizerv1.InferenceModel{
		Kind:    types.KindInferenceModel,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        m.Name,
			Description: m.Annotations[teleportcr.DescriptionKey],
			Labels:      m.Labels,
		},
		Spec: (*summarizerv1.InferenceModelSpec)(m.Spec),
	}
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the Teleport resource controller to report conditions back to resource.
func (m *TeleportInferenceModel) StatusConditions() *[]metav1.Condition {
	return &m.Status.Conditions
}

// UnmarshalJSON delegates unmarshaling of the TeleportInferenceModelSpec to
// protojson, which is necessary for Proto RFD153 resources to be unmarshaled
// correctly from the unstructured object.
func (spec *TeleportInferenceModelSpec) UnmarshalJSON(data []byte) error {
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(data, (*summarizerv1.InferenceModelSpec)(spec))
}

// MarshalJSON delegates marshaling of the TeleportInferenceModelSpec to
// protojson, which is necessary for Proto RFD153 resources to be marshaled
// correctly into an unstructured object.
func (spec *TeleportInferenceModelSpec) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal((*summarizerv1.InferenceModelSpec)(spec))
}

// DeepCopyInto deep-copies one TeleportInferenceModelSpec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportInferenceModelSpec) DeepCopyInto(out *TeleportInferenceModelSpec) {
	proto.Reset((*summarizerv1.InferenceModelSpec)(out))
	proto.Merge((*summarizerv1.InferenceModelSpec)(out), (*summarizerv1.InferenceModelSpec)(spec))
}
