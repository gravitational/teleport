// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
)

func init() {
	SchemeBuilder.Register(&TeleportBotV1{}, &TeleportBotV1List{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportBotV1 holds the kubernetes custom resources for Bot
type TeleportBotV1 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   *TeleportBotV1Spec `json:"spec,omitempty"`
	Status resources.Status   `json:"status,omitempty"`
}

// TeleportBotV1Spec defines the desired state of TeleportBotV1
type TeleportBotV1Spec machineidv1.BotSpec

//+kubebuilder:object:root=true

// TeleportBotV1List contains a list of TeleportBotV1
type TeleportBotV1List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TeleportBotV1 `json:"items"`
}

// ToTeleport returns a Bot, which wraps the actual
// [machineidv1.Bot] and implements the necessary interface methods used
// by the TeleportResourceReconciler.
func (l *TeleportBotV1) ToTeleport() *machineidv1.Bot {
	resource := &machineidv1.Bot{
		Kind:    types.KindBot,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        l.Name,
			Description: l.Annotations[resources.DescriptionKey],
			Labels:      l.Labels,
		},
		Spec: (*machineidv1.BotSpec)(l.Spec),
	}
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
func (l *TeleportBotV1) StatusConditions() *[]metav1.Condition {
	return &l.Status.Conditions
}

// Marshal serializes a spec into binary data.
func (spec *TeleportBotV1Spec) Marshal() ([]byte, error) {
	return proto.Marshal((*machineidv1.BotSpec)(spec))
}

// Unmarshal deserializes a spec from binary data.
func (spec *TeleportBotV1Spec) Unmarshal(data []byte) error {
	return proto.Unmarshal(data, (*machineidv1.BotSpec)(spec))
}

// UnmarshalJSON delegates unmarshaling of the BotSpec to protojson, which is
// necessary for the BotSpec (and other Proto RFD153 resources) to be
// unmarshaled correctly from the unstructured object.
func (spec *TeleportBotV1Spec) UnmarshalJSON(data []byte) error {
	return protojson.Unmarshal(data, (*machineidv1.BotSpec)(spec))
}

// MarshalJSON delegates marshaling of the BotSpec to protojson, which is
// necessary for the BotSpec (and other Proto RFD153 resources) to be
// marshaled correctly into the unstructured object.
func (spec *TeleportBotV1Spec) MarshalJSON() ([]byte, error) {
	return protojson.Marshal((*machineidv1.BotSpec)(spec))
}

// DeepCopyInto deep-copies one user spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportBotV1Spec) DeepCopyInto(out *TeleportBotV1Spec) {
	data, err := spec.Marshal()
	if err != nil {
		panic(err)
	}
	*out = TeleportBotV1Spec{}
	if err = out.Unmarshal(data); err != nil {
		panic(err)
	}
}
