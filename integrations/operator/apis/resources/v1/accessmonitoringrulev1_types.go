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

	accessmonitoringrulesv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources/teleportcr"
)

func init() {
	SchemeBuilder.Register(&TeleportAccessMonitoringRuleV1{}, &TeleportAccessMonitoringRuleV1List{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportAccessMonitoringRuleV1 holds the kubernetes custom resources for teleport's autoupdate_config v1 resource.
type TeleportAccessMonitoringRuleV1 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   *TeleportAccessMonitoringRuleV1Spec `json:"spec,omitempty"`
	Status teleportcr.Status                   `json:"status"`
}

// TeleportAccessMonitoringRuleV1Spec defines the desired state of TeleportAccessMonitoringRuleV1
type TeleportAccessMonitoringRuleV1Spec accessmonitoringrulesv1pb.AccessMonitoringRuleSpec

//+kubebuilder:object:root=true

// TeleportAccessMonitoringRuleV1List contains a list of TeleportAccessMonitoringRuleV1
type TeleportAccessMonitoringRuleV1List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportAccessMonitoringRuleV1 `json:"items"`
}

// ToTeleport returns an AccessMonitoringRule that can be sent to Teleport.
func (l *TeleportAccessMonitoringRuleV1) ToTeleport() *accessmonitoringrulesv1pb.AccessMonitoringRule {
	resource := &accessmonitoringrulesv1pb.AccessMonitoringRule{
		Kind:    types.KindAccessMonitoringRule,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        l.Name,
			Description: l.Annotations[teleportcr.DescriptionKey],
			Labels:      l.Labels,
		},
		Spec: (*accessmonitoringrulesv1pb.AccessMonitoringRuleSpec)(l.Spec),
	}
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
func (l *TeleportAccessMonitoringRuleV1) StatusConditions() *[]metav1.Condition {
	return &l.Status.Conditions
}

// UnmarshalJSON delegates unmarshaling of the Spec to protojson, which is
// necessary for Proto RFD153 resources to be
// unmarshaled correctly from the unstructured object.
func (spec *TeleportAccessMonitoringRuleV1Spec) UnmarshalJSON(data []byte) error {
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(data, (*accessmonitoringrulesv1pb.AccessMonitoringRuleSpec)(spec))
}

// MarshalJSON delegates marshaling of the Spec to protojson, which is
// necessary for Proto RFD153 resources to be
// marshaled correctly into the unstructured object.
func (spec *TeleportAccessMonitoringRuleV1Spec) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal((*accessmonitoringrulesv1pb.AccessMonitoringRuleSpec)(spec))
}

// DeepCopyInto deep-copies one spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportAccessMonitoringRuleV1Spec) DeepCopyInto(out *TeleportAccessMonitoringRuleV1Spec) {
	proto.Reset((*accessmonitoringrulesv1pb.AccessMonitoringRuleSpec)(out))
	proto.Merge((*accessmonitoringrulesv1pb.AccessMonitoringRuleSpec)(out), (*accessmonitoringrulesv1pb.AccessMonitoringRuleSpec)(spec))
}
