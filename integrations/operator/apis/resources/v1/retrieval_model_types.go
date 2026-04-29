/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
	SchemeBuilder.Register(&TeleportRetrievalModelV1{}, &TeleportRetrievalModelV1List{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportRetrievalModelV1 holds the kubernetes custom resources for teleport's retrieval_model v1 resource.
type TeleportRetrievalModelV1 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   *TeleportRetrievalModelV1Spec `json:"spec,omitempty"`
	Status teleportcr.Status             `json:"status"`
}

// TeleportRetrievalModelV1Spec defines the desired state of TeleportRetrievalModelV1
type TeleportRetrievalModelV1Spec summarizerv1.RetrievalModelSpec

//+kubebuilder:object:root=true

// TeleportRetrievalModelV1List contains a list of TeleportRetrievalModelV1
type TeleportRetrievalModelV1List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportRetrievalModelV1 `json:"items"`
}

// ToTeleport returns a RetrievalModel that can be sent to Teleport.
// The Metadata.Name is always set to types.MetaNameRetrievalModel regardless of
// the Kubernetes CR name, because RetrievalModel is a singleton that can only
// exist under that fixed name.
func (l *TeleportRetrievalModelV1) ToTeleport() *summarizerv1.RetrievalModel {
	resource := &summarizerv1.RetrievalModel{
		Kind:    types.KindRetrievalModel,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        types.MetaNameRetrievalModel,
			Description: l.Annotations[teleportcr.DescriptionKey],
			Labels:      l.Labels,
		},
		Spec: (*summarizerv1.RetrievalModelSpec)(l.Spec),
	}
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the Teleport resource controller to report conditions back to resource.
func (l *TeleportRetrievalModelV1) StatusConditions() *[]metav1.Condition {
	return &l.Status.Conditions
}

// UnmarshalJSON delegates unmarshaling of the Spec to protojson, which is
// necessary for Proto RFD153 resources to be unmarshaled correctly from the unstructured object.
func (spec *TeleportRetrievalModelV1Spec) UnmarshalJSON(data []byte) error {
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(data, (*summarizerv1.RetrievalModelSpec)(spec))
}

// MarshalJSON delegates marshaling of the Spec to protojson, which is
// necessary for Proto RFD153 resources to be marshaled correctly into the unstructured object.
func (spec *TeleportRetrievalModelV1Spec) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal((*summarizerv1.RetrievalModelSpec)(spec))
}

// DeepCopyInto deep-copies one spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportRetrievalModelV1Spec) DeepCopyInto(out *TeleportRetrievalModelV1Spec) {
	proto.Reset((*summarizerv1.RetrievalModelSpec)(out))
	proto.Merge((*summarizerv1.RetrievalModelSpec)(out), (*summarizerv1.RetrievalModelSpec)(spec))
}
