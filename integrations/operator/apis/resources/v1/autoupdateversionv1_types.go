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
	SchemeBuilder.Register(&TeleportAutoupdateVersionV1{}, &TeleportAutoupdateVersionV1List{})
}

// Note regarding naming:
// In Teleport's codebase we use AutoUpdateConfig but the user-facing kind is autoupdate_config.
// Here, we use the struct name to generate user-facing code, so the resource is intentionally written
// AutoupdateConfig instead of AutoUpdateConfig. Thi guarantees naming consistency for the user
// regardless of the IaC tool used.

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportAutoupdateVersionV1 holds the kubernetes custom resources for teleport's autoupdate_version v1 resource.
type TeleportAutoupdateVersionV1 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   *TeleportAutoupdateVersionV1Spec `json:"spec,omitempty"`
	Status resources.Status                 `json:"status,omitempty"`
}

// TeleportAutoupdateVersionV1Spec defines the desired state of TeleportAutoupdateVersionV1
type TeleportAutoupdateVersionV1Spec autoupdatev1pb.AutoUpdateVersionSpec

//+kubebuilder:object:root=true

// TeleportAutoupdateVersionV1List contains a list of TeleportAutoupdateVersionV1
type TeleportAutoupdateVersionV1List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TeleportAutoupdateVersionV1 `json:"items"`
}

// ToTeleport returns an AutoUpdateVersion that can be sent to Teleport.
func (l *TeleportAutoupdateVersionV1) ToTeleport() *autoupdatev1pb.AutoUpdateVersion {
	resource := &autoupdatev1pb.AutoUpdateVersion{
		Kind:    types.KindAutoUpdateVersion,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        l.Name,
			Description: l.Annotations[resources.DescriptionKey],
			Labels:      l.Labels,
		},
		Spec: (*autoupdatev1pb.AutoUpdateVersionSpec)(l.Spec),
	}
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
func (l *TeleportAutoupdateVersionV1) StatusConditions() *[]metav1.Condition {
	return &l.Status.Conditions
}

// UnmarshalJSON delegates unmarshaling of the Spec to protojson, which is
// necessary for Proto RFD153 resources to be
// unmarshaled correctly from the unstructured object.
func (spec *TeleportAutoupdateVersionV1Spec) UnmarshalJSON(data []byte) error {
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(data, (*autoupdatev1pb.AutoUpdateVersionSpec)(spec))
}

// MarshalJSON delegates marshaling of the Spec to protojson, which is
// necessary for Proto RFD153 resources to be
// marshaled correctly into the unstructured object.
func (spec *TeleportAutoupdateVersionV1Spec) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal((*autoupdatev1pb.AutoUpdateVersionSpec)(spec))
}

// DeepCopyInto deep-copies one user spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportAutoupdateVersionV1Spec) DeepCopyInto(out *TeleportAutoupdateVersionV1Spec) {
	proto.Reset((*autoupdatev1pb.AutoUpdateVersionSpec)(out))
	proto.Merge((*autoupdatev1pb.AutoUpdateVersionSpec)(out), (*autoupdatev1pb.AutoUpdateVersionSpec)(spec))
}
