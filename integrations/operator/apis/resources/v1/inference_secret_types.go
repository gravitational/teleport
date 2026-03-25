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
	SchemeBuilder.Register(&TeleportInferenceSecret{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportInferenceSecret represents a Kubernetes custom resource for
// InferenceSecret.
type TeleportInferenceSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   *TeleportInferenceSecretSpec `json:"spec"`
	Status teleportcr.Status            `json:"status"`
}

// TeleportInferenceSecretSpec defines the desired state of
// [TeleportInferenceSecret].
type TeleportInferenceSecretSpec summarizerv1.InferenceSecretSpec

//+kubebuilder:object:root = true

// TeleportInferenceSecretList contains a list of [TeleportInferenceSecret]
// objects.
type TeleportInferenceSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportInferenceSecret `json:"items"`
}

// ToTeleport returns a Teleport representation of this Kubernetes resource.
func (s *TeleportInferenceSecret) ToTeleport() *summarizerv1.InferenceSecret {
	resource := &summarizerv1.InferenceSecret{
		Kind:    types.KindInferenceSecret,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        s.Name,
			Description: s.Annotations[teleportcr.DescriptionKey],
			Labels:      s.Labels,
		},
		Spec: (*summarizerv1.InferenceSecretSpec)(s.Spec),
	}
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the Teleport resource controller to report conditions back to resource.
func (s *TeleportInferenceSecret) StatusConditions() *[]metav1.Condition {
	return &s.Status.Conditions
}

// UnmarshalJSON delegates unmarshaling of the TeleportInferenceSecretSpec to
// protojson, which is necessary for Proto RFD153 resources to be unmarshaled
// correctly from the unstructured object.
func (spec *TeleportInferenceSecretSpec) UnmarshalJSON(data []byte) error {
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(data, (*summarizerv1.InferenceSecretSpec)(spec))
}

// MarshalJSON delegates marshaling of the TeleportInferenceSecretSpec to
// protojson, which is necessary for Proto RFD153 resources to be marshaled
// correctly into an unstructured object.
func (spec *TeleportInferenceSecretSpec) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal((*summarizerv1.InferenceSecretSpec)(spec))
}

// DeepCopyInto deep-copies one TeleportInferenceSecretSpec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportInferenceSecretSpec) DeepCopyInto(out *TeleportInferenceSecretSpec) {
	proto.Reset((*summarizerv1.InferenceSecretSpec)(out))
	proto.Merge((*summarizerv1.InferenceSecretSpec)(out), (*summarizerv1.InferenceSecretSpec)(spec))
}
