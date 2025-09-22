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

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
)

func init() {
	SchemeBuilder.Register(&TeleportAutoupdateConfigV1{}, &TeleportAutoupdateConfigV1List{})
}

// Note regarding naming:
// In Teleport's codebase we use AutoUpdateConfig but the user-facing kind is autoupdate_config.
// Here, we use the struct name to generate user-facing code, so the resource is intentionally written
// AutoupdateConfig instead of AutoUpdateConfig. Thi guarantees naming consistency for the user
// regardless of the IaC tool used.

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportAutoupdateConfigV1 holds the kubernetes custom resources for teleport's autoupdate_config v1 resource.
type TeleportAutoupdateConfigV1 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   *TeleportAutoupdateConfigV1Spec `json:"spec,omitempty"`
	Status resources.Status                `json:"status"`
}

// TeleportAutoupdateConfigV1Spec defines the desired state of TeleportAutoupdateConfigV1
type TeleportAutoupdateConfigV1Spec autoupdatev1pb.AutoUpdateConfigSpec

//+kubebuilder:object:root=true

// TeleportAutoupdateConfigV1List contains a list of TeleportAutoupdateConfigV1
type TeleportAutoupdateConfigV1List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportAutoupdateConfigV1 `json:"items"`
}

// ToTeleport returns an AutoUpdateConfig that can be sent to Teleport.
func (l *TeleportAutoupdateConfigV1) ToTeleport() *autoupdatev1pb.AutoUpdateConfig {
	resource := &autoupdatev1pb.AutoUpdateConfig{
		Kind:    types.KindAutoUpdateConfig,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        l.Name,
			Description: l.Annotations[resources.DescriptionKey],
			Labels:      l.Labels,
		},
		Spec: (*autoupdatev1pb.AutoUpdateConfigSpec)(l.Spec),
	}
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
func (l *TeleportAutoupdateConfigV1) StatusConditions() *[]metav1.Condition {
	return &l.Status.Conditions
}

// UnmarshalJSON delegates unmarshaling of the Spec to protojson, which is
// necessary for Proto RFD153 resources to be
// unmarshaled correctly from the unstructured object.
func (spec *TeleportAutoupdateConfigV1Spec) UnmarshalJSON(data []byte) error {
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(data, (*autoupdatev1pb.AutoUpdateConfigSpec)(spec))
}

// MarshalJSON delegates marshaling of the Spec to protojson, which is
// necessary for Proto RFD153 resources to be
// marshaled correctly into the unstructured object.
func (spec *TeleportAutoupdateConfigV1Spec) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal((*autoupdatev1pb.AutoUpdateConfigSpec)(spec))
}

// DeepCopyInto deep-copies one user spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportAutoupdateConfigV1Spec) DeepCopyInto(out *TeleportAutoupdateConfigV1Spec) {
	proto.Reset((*autoupdatev1pb.AutoUpdateConfigSpec)(out))
	proto.Merge((*autoupdatev1pb.AutoUpdateConfigSpec)(out), (*autoupdatev1pb.AutoUpdateConfigSpec)(spec))
}
