/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

	crownjewelv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources/teleportcr"
)

func init() {
	SchemeBuilder.Register(&TeleportCrownJewelV1{}, &TeleportCrownJewelV1List{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportCrownJewelV1 holds the kubernetes custom resources for teleport's crown_jewel v1 resource.
type TeleportCrownJewelV1 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   *TeleportCrownJewelV1Spec `json:"spec,omitempty"`
	Status teleportcr.Status         `json:"status"`
}

// TeleportCrownJewelV1Spec defines the desired state of TeleportCrownJewelV1
type TeleportCrownJewelV1Spec crownjewelv1pb.CrownJewelSpec

//+kubebuilder:object:root=true

// TeleportCrownJewelV1List contains a list of TeleportCrownJewelV1
type TeleportCrownJewelV1List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportCrownJewelV1 `json:"items"`
}

// ToTeleport returns a CrownJewel that can be sent to Teleport.
func (l *TeleportCrownJewelV1) ToTeleport() *crownjewelv1pb.CrownJewel {
	resource := crownjewelv1pb.CrownJewel_builder{
		Kind:    types.KindCrownJewel,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:        l.Name,
			Description: l.Annotations[teleportcr.DescriptionKey],
			Labels:      l.Labels,
		}.Build(),
		Spec: (*crownjewelv1pb.CrownJewelSpec)(l.Spec),
	}.Build()
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
func (l *TeleportCrownJewelV1) StatusConditions() *[]metav1.Condition {
	return &l.Status.Conditions
}

// UnmarshalJSON delegates unmarshaling of the Spec to protojson, which is
// necessary for Proto RFD153 resources to be
// unmarshaled correctly from the unstructured object.
func (spec *TeleportCrownJewelV1Spec) UnmarshalJSON(data []byte) error {
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(data, (*crownjewelv1pb.CrownJewelSpec)(spec))
}

// MarshalJSON delegates marshaling of the Spec to protojson, which is
// necessary for Proto RFD153 resources to be
// marshaled correctly into the unstructured object.
func (spec *TeleportCrownJewelV1Spec) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal((*crownjewelv1pb.CrownJewelSpec)(spec))
}

func (spec *TeleportCrownJewelV1Spec) DeepCopyInto(out *TeleportCrownJewelV1Spec) {
	proto.Reset((*crownjewelv1pb.CrownJewelSpec)(out))
	proto.Merge((*crownjewelv1pb.CrownJewelSpec)(out), (*crownjewelv1pb.CrownJewelSpec)(spec))
}
