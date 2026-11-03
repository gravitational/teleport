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
	SchemeBuilder.Register(&TeleportClassifierV1{}, &TeleportClassifierV1List{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportClassifierV1 represents a Kubernetes custom resource for Classifier.
type TeleportClassifierV1 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   *TeleportClassifierV1Spec `json:"spec"`
	Status teleportcr.Status         `json:"status"`
}

// TeleportClassifierV1Spec defines the desired state of [TeleportClassifierV1].
type TeleportClassifierV1Spec summarizerv1.ClassifierSpec

//+kubebuilder:object:root=true

// TeleportClassifierV1List contains a list of [TeleportClassifierV1] objects.
type TeleportClassifierV1List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportClassifierV1 `json:"items"`
}

// ToTeleport returns a Teleport representation of this Kubernetes resource.
func (c *TeleportClassifierV1) ToTeleport() *summarizerv1.Classifier {
	resource := summarizerv1.Classifier_builder{
		Kind:    types.KindClassifier,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:        c.Name,
			Description: c.Annotations[teleportcr.DescriptionKey],
			Labels:      c.Labels,
		}.Build(),
		Spec: (*summarizerv1.ClassifierSpec)(c.Spec),
	}.Build()
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice.
func (c *TeleportClassifierV1) StatusConditions() *[]metav1.Condition {
	return &c.Status.Conditions
}

// UnmarshalJSON delegates unmarshaling of the TeleportClassifierV1Spec to protojson.
func (spec *TeleportClassifierV1Spec) UnmarshalJSON(data []byte) error {
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(data, (*summarizerv1.ClassifierSpec)(spec))
}

// MarshalJSON delegates marshaling of the TeleportClassifierV1Spec to protojson.
func (spec *TeleportClassifierV1Spec) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal((*summarizerv1.ClassifierSpec)(spec))
}

// DeepCopyInto deep-copies one TeleportClassifierV1Spec into another.
func (spec *TeleportClassifierV1Spec) DeepCopyInto(out *TeleportClassifierV1Spec) {
	proto.Reset((*summarizerv1.ClassifierSpec)(out))
	proto.Merge((*summarizerv1.ClassifierSpec)(out), (*summarizerv1.ClassifierSpec)(spec))
}
