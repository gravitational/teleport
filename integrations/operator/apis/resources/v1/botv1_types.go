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
	metav1.ObjectMeta `json:"metadata"`

	Spec   *TeleportBotV1Spec `json:"spec,omitempty"`
	Status resources.Status   `json:"status"`
}

// TeleportBotV1Spec defines the desired state of TeleportBotV1
type TeleportBotV1Spec machineidv1.BotSpec

//+kubebuilder:object:root=true

// TeleportBotV1List contains a list of TeleportBotV1
type TeleportBotV1List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
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

// UnmarshalJSON delegates unmarshaling of the BotSpec to protojson, which is
// necessary for the BotSpec (and other Proto RFD153 resources) to be
// unmarshaled correctly from the unstructured object.
func (spec *TeleportBotV1Spec) UnmarshalJSON(data []byte) error {
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(data, (*machineidv1.BotSpec)(spec))
}

// MarshalJSON delegates marshaling of the BotSpec to protojson, which is
// necessary for the BotSpec (and other Proto RFD153 resources) to be
// marshaled correctly into the unstructured object.
func (spec *TeleportBotV1Spec) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal((*machineidv1.BotSpec)(spec))
}

// DeepCopyInto deep-copies one user spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportBotV1Spec) DeepCopyInto(out *TeleportBotV1Spec) {
	proto.Reset((*machineidv1.BotSpec)(out))
	proto.Merge((*machineidv1.BotSpec)(out), (*machineidv1.BotSpec)(spec))
}
