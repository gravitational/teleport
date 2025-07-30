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
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/operator/apis/resources"
)

func init() {
	SchemeBuilder.Register(&TeleportWorkloadIdentityV1{}, &TeleportWorkloadIdentityV1List{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TeleportWorkloadIdentityV1 holds the kubernetes custom resources for
// WorkloadIdentity
type TeleportWorkloadIdentityV1 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   *TeleportWorkloadIdentityV1Spec `json:"spec,omitempty"`
	Status resources.Status                `json:"status"`
}

// TeleportWorkloadIdentityV1Spec defines the desired state of TeleportWorkloadIdentityV1
type TeleportWorkloadIdentityV1Spec workloadidentityv1.WorkloadIdentitySpec

//+kubebuilder:object:root=true

// TeleportWorkloadIdentityV1List contains a list of TeleportWorkloadIdentityV1
type TeleportWorkloadIdentityV1List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TeleportWorkloadIdentityV1 `json:"items"`
}

// ToTeleport returns a WorkloadIdentity, which wraps the actual
// [workloadidentityv1.WorkloadIdentity] and implements the necessary interface
// methods used by the TeleportResourceReconciler.
func (l *TeleportWorkloadIdentityV1) ToTeleport() *workloadidentityv1.WorkloadIdentity {
	resource := &workloadidentityv1.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:        l.Name,
			Description: l.Annotations[resources.DescriptionKey],
			Labels:      l.Labels,
		},
		Spec: (*workloadidentityv1.WorkloadIdentitySpec)(l.Spec),
	}
	return resource
}

// StatusConditions returns a pointer to Status.Conditions slice. This is used
// by the teleport resource controller to report conditions back to on resource.
func (l *TeleportWorkloadIdentityV1) StatusConditions() *[]metav1.Condition {
	return &l.Status.Conditions
}

// UnmarshalJSON delegates unmarshaling of the WorkloadIdentitySpec to
// protojson, which is necessary for the WorkloadIdentitySpec (and other Proto
// RFD153 resources) to be unmarshaled correctly from the unstructured object.
func (spec *TeleportWorkloadIdentityV1Spec) UnmarshalJSON(data []byte) error {
	return protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}.Unmarshal(data, (*workloadidentityv1.WorkloadIdentitySpec)(spec))
}

// MarshalJSON delegates marshaling of the WorkloadIdentitySpec to protojson,
// which is necessary for the WorkloadIdentitySpec (and other Proto RFD153
// resources) to be marshaled correctly into the unstructured object.
func (spec *TeleportWorkloadIdentityV1Spec) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal((*workloadidentityv1.WorkloadIdentitySpec)(spec))
}

// DeepCopyInto deep-copies one user spec into another.
// Required to satisfy runtime.Object interface.
func (spec *TeleportWorkloadIdentityV1Spec) DeepCopyInto(out *TeleportWorkloadIdentityV1Spec) {
	proto.Reset((*workloadidentityv1.WorkloadIdentitySpec)(out))
	proto.Merge((*workloadidentityv1.WorkloadIdentitySpec)(out), (*workloadidentityv1.WorkloadIdentitySpec)(spec))
}
